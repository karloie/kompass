package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/karloie/kompass/pkg/graph"
	"github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/pipeline"
	"github.com/karloie/kompass/pkg/tree"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type server struct {
	contextArg    string
	namespaceArg  string
	client        *kube.Client
	clientFactory func(contextArg, namespace string) (kube.Kube, error)
}

func startServer(addr, contextArg, namespaceArg string) {
	if strings.HasPrefix(addr, ":") {
		addr = "0.0.0.0" + addr
	}
	slog.Info("Starting kompass server", "addr", addr, "context", contextArg, "namespace", namespaceArg)
	parts := strings.Split(addr, ":")
	port := ":" + parts[len(parts)-1]
	client, err := kube.NewClient(contextArg, namespaceArg)
	if err != nil {
		slog.Error("Failed to create kubernetes client", "error", err)
		os.Exit(1)
	}
	namespacesToWatch := []string{namespaceArg}
	if namespaceArg == "" {
		namespacesToWatch = []string{"default"}
	}
	if err := client.StartSync(30*time.Second, namespacesToWatch); err != nil {
		slog.Warn("Failed to start cache sync", "error", err)
	} else {
		slog.Info("Cache sync started", "interval", "30s", "namespaces", namespacesToWatch)
	}
	srv := &server{contextArg: contextArg, namespaceArg: namespaceArg, client: client}
	http.HandleFunc("/graph", srv.handleGraph)
	http.HandleFunc("/tree", srv.handleTree)
	http.HandleFunc("/health", srv.handleHealth("json", false))
	http.HandleFunc("/healthz", srv.handleHealth("text", false))
	http.HandleFunc("/readyz", srv.handleHealth("text", true))
	http.HandleFunc("/stats", srv.handleStats)
	httpServer := &http.Server{Addr: addr}
	go func() {
		slog.Info("Server ready", "url", "http://localhost"+port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")
	client.StopSync()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}
	slog.Info("Server stopped")
}

func (s *server) handleHealth(format string, checkReady bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if checkReady && s.client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if _, err := s.client.GetPods(s.namespaceArg, ctx, metav1.ListOptions{Limit: 1}); err != nil {
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
		} else if checkReady {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		if format == "json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok"}`))
		} else {
			w.Write([]byte("ok"))
		}
	}
}

func (s *server) handleStats(w http.ResponseWriter, r *http.Request) {
	if s.client == nil {
		http.Error(w, "No active client", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.client.GetCacheStats())
}

func (s *server) handleGraph(w http.ResponseWriter, r *http.Request) {
	selectors := graph.ParseSelectors(r.URL.Query().Get("selector"))
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = s.namespaceArg
	}

	provider, err := s.getProvider(r.URL.Query().Get("mock"), namespace)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result, err := pipeline.InferGraphs(provider, selectors)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	context_, _ := provider.GetContext()
	namespace_, _ := provider.GetNamespace()
	configPath, _ := provider.GetConfigPath()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	if err := json.NewEncoder(w).Encode(JSONOutput{
		Request:  RequestMetadata{context_, namespace_, configPath, selectors},
		Response: result,
	}); err != nil {
		slog.Error("JSON encoding error", "error", err)
	}
}

func (s *server) handleTree(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")

	selectors := graph.ParseSelectors(r.URL.Query().Get("selector"))
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = s.namespaceArg
	}

	provider, err := s.getProvider(r.URL.Query().Get("mock"), namespace)
	if err != nil {
		w.Write([]byte("Error: Failed to connect to cluster\n"))
		return
	}

	result, err := pipeline.InferGraphs(provider, selectors)
	if err != nil {
		w.Write([]byte(fmt.Sprintf("Error: %s\n", err.Error())))
		return
	}

	context_, _ := provider.GetContext()
	namespace_, _ := provider.GetNamespace()
	configPath, _ := provider.GetConfigPath()

	var output strings.Builder
	output.WriteString(fmt.Sprintf("🌍 Context: %s, Namespace: %s, Selectors: %v, Config: %s\n\n", context_, namespace_, selectors, configPath))

	for i := range result.Graphs {
		g := &result.Graphs[i]
		if g.Tree != nil {
			output.WriteString(tree.RenderTree(g.Tree, pipeline.GraphNodesForGraph(result, g), true))
		}
		if i < len(result.Graphs)-1 {
			output.WriteString("\n")
		}
	}

	w.Write([]byte(output.String()))
}

func (s *server) getProvider(mockProvider, namespace string) (kube.Kube, error) {
	if s.clientFactory != nil {
		return s.clientFactory(s.contextArg, namespace)
	}
	if mockProvider != "" {
		if mockProvider != "mock" {
			return nil, fmt.Errorf("unknown mock provider: %s", mockProvider)
		}
		provider, _, _, err := initProvider(true, "", namespace)
		return provider, err
	}
	if s.client != nil {
		currentNS, _ := s.client.GetNamespace()
		if namespace != "" && namespace != currentNS {
			s.client.SetNamespace(namespace)
		}
		return s.client, nil
	}
	provider, _, _, err := initProvider(false, s.contextArg, namespace)
	return provider, err
}
