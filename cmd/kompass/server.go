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

func startServer(addr, contextArg, namespaceArg string, useMock bool) {
	if strings.HasPrefix(addr, ":") {
		addr = "0.0.0.0" + addr
	}
	slog.Info("Starting kompass server", "addr", addr, "context", contextArg, "namespace", namespaceArg, "provider", map[bool]string{true: "mock", false: "cluster"}[useMock])
	parts := strings.Split(addr, ":")
	port := ":" + parts[len(parts)-1]
	provider, _, _, err := initProvider(useMock, contextArg, namespaceArg)
	if err != nil {
		slog.Error("Failed to create provider", "error", err)
		os.Exit(1)
	}
	client, ok := provider.(*kube.Client)
	if !ok {
		slog.Error("Provider type assertion failed", "error", "provider is not *kube.Client")
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
	mux := http.NewServeMux()
	mux.HandleFunc("/graph", srv.handleGraph)
	mux.HandleFunc("/tree", srv.handleTree)
	mux.HandleFunc("/tree/text", srv.handleTreeText)
	mux.HandleFunc("/health", srv.handleHealth("json", false))
	mux.HandleFunc("/healthz", srv.handleHealth("text", false))
	mux.HandleFunc("/readyz", srv.handleHealth("text", true))
	mux.HandleFunc("/stats", srv.handleStats)
	httpServer := &http.Server{Addr: addr, Handler: mux}
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
		slog.Debug("endpoint reached", "endpoint", r.URL.Path, "method", r.Method, "readyCheck", checkReady, "format", format)
		if checkReady && s.client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if _, err := s.client.GetPods(s.namespaceArg, ctx, metav1.ListOptions{Limit: 1}); err != nil {
				slog.Debug("endpoint failed", "endpoint", r.URL.Path, "method", r.Method, "error", err)
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
		} else if checkReady {
			slog.Debug("endpoint failed", "endpoint", r.URL.Path, "method", r.Method, "error", "no active client")
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		if format == "json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok"}`))
		} else {
			w.Write([]byte("ok"))
		}
		slog.Debug("endpoint completed", "endpoint", r.URL.Path, "method", r.Method, "status", http.StatusOK)
	}
}

func (s *server) handleStats(w http.ResponseWriter, r *http.Request) {
	slog.Debug("endpoint reached", "endpoint", r.URL.Path, "method", r.Method)
	if s.client == nil {
		slog.Debug("endpoint failed", "endpoint", r.URL.Path, "method", r.Method, "error", "no active client")
		http.Error(w, "No active client", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.client.GetStats()); err != nil {
		slog.Debug("endpoint failed", "endpoint", r.URL.Path, "method", r.Method, "error", err)
		return
	}
	slog.Debug("endpoint completed", "endpoint", r.URL.Path, "method", r.Method, "status", http.StatusOK)
}

func (s *server) handleGraph(w http.ResponseWriter, r *http.Request) {
	selectors, namespace, provider, result, err := s.inferForRequest(r)
	if err != nil {
		slog.Debug("endpoint failed", "endpoint", r.URL.Path, "method", r.Method, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	context_, _ := provider.GetContext()
	namespace_, _ := provider.GetNamespace()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	if err := json.NewEncoder(w).Encode(JSONOutputGraph{
		APIVersion: jsonAPIVersion,
		Request:    RequestMetadata{Context: context_, Namespace: namespace_, Selectors: selectors},
		Response:   graphOnlyResponse(result),
	}); err != nil {
		slog.Error("JSON encoding error", "error", err)
		return
	}
	slog.Debug("endpoint completed", "endpoint", r.URL.Path, "method", r.Method, "status", http.StatusOK, "graphs", len(result.Graphs), "nodes", len(result.Nodes))
	_ = namespace
}

func (s *server) handleTree(w http.ResponseWriter, r *http.Request) {
	selectors, _, provider, result, err := s.inferForRequest(r)
	if err != nil {
		slog.Debug("endpoint failed", "endpoint", r.URL.Path, "method", r.Method, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	context_, _ := provider.GetContext()
	namespace_, _ := provider.GetNamespace()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if err := json.NewEncoder(w).Encode(JSONOutputTree{
		APIVersion: jsonAPIVersion,
		Request:    RequestMetadata{Context: context_, Namespace: namespace_, Selectors: selectors},
		Response:   tree.BuildResponseTree(result),
	}); err != nil {
		slog.Error("JSON encoding error", "error", err)
		return
	}
	slog.Debug("endpoint completed", "endpoint", r.URL.Path, "method", r.Method, "status", http.StatusOK, "graphs", len(result.Graphs), "nodes", len(result.Nodes))
}

func (s *server) handleTreeText(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")

	plain := true
	switch strings.ToLower(r.URL.Query().Get("plain")) {
	case "0", "false", "no":
		plain = false
	case "1", "true", "yes":
		plain = true
	}

	selectors, _, provider, result, err := s.inferForRequest(r)
	if err != nil {
		slog.Debug("endpoint failed", "endpoint", r.URL.Path, "method", r.Method, "error", err)
		http.Error(w, fmt.Sprintf("Error: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	context_, _ := provider.GetContext()
	namespace_, _ := provider.GetNamespace()
	configPath, _ := provider.GetConfigPath()

	var output strings.Builder
	output.WriteString(fmt.Sprintf("🌍 Context: %s, Namespace: %s, Selectors: %v, Config: %s\n\n", context_, namespace_, selectors, configPath))

	treeResult := tree.BuildResponseTree(result)
	for i := range treeResult.Trees {
		treeNode := treeResult.Trees[i]
		if treeNode != nil {
			output.WriteString(tree.RenderTree(treeNode, treeResult.Nodes, plain))
		}
		if i < len(treeResult.Trees)-1 {
			output.WriteString("\n")
		}
	}

	w.Write([]byte(output.String()))
	slog.Debug("endpoint completed", "endpoint", r.URL.Path, "method", r.Method, "status", http.StatusOK, "graphs", len(result.Graphs), "nodes", len(result.Nodes))
}

func (s *server) inferForRequest(r *http.Request) ([]string, string, kube.Kube, *kube.ResponseGraph, error) {
	selectors := graph.ParseSelectors(r.URL.Query().Get("selector"))
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = s.namespaceArg
	}
	slog.Debug("endpoint reached", "endpoint", r.URL.Path, "method", r.Method, "selectors", selectors, "namespace", namespace, "mock", r.URL.Query().Get("mock"))

	provider, err := s.getProvider(r.URL.Query().Get("mock"), namespace)
	if err != nil {
		return nil, namespace, nil, nil, err
	}

	result, err := pipeline.InferGraphs(provider, selectors)
	if err != nil {
		return nil, namespace, provider, nil, err
	}

	return selectors, namespace, provider, result, nil
}

func graphOnlyResponse(result *kube.ResponseGraph) *kube.ResponseGraph {
	if result == nil {
		return nil
	}
	out := &kube.ResponseGraph{Nodes: result.Nodes, Graphs: make([]kube.Graph, 0, len(result.Graphs))}
	for _, g := range result.Graphs {
		out.Graphs = append(out.Graphs, kube.Graph{ID: g.ID, Edges: g.Edges})
	}
	return out
}

func (s *server) getProvider(mockProvider, namespace string) (kube.Kube, error) {
	slog.Debug("resolving provider", "mock", mockProvider, "namespace", namespace)
	if s.clientFactory != nil {
		slog.Debug("using custom client factory", "namespace", namespace)
		return s.clientFactory(s.contextArg, namespace)
	}
	if mockProvider != "" {
		if mockProvider != "mock" {
			slog.Debug("provider resolve failed", "mock", mockProvider, "error", "unknown mock provider")
			return nil, fmt.Errorf("unknown mock provider: %s", mockProvider)
		}
		provider, _, _, err := initProvider(true, "", namespace)
		if err != nil {
			slog.Debug("provider resolve failed", "mock", mockProvider, "namespace", namespace, "error", err)
			return nil, err
		}
		slog.Debug("provider resolved", "provider", "mock", "namespace", namespace)
		return provider, err
	}
	if s.client != nil {
		slog.Debug("provider resolved", "provider", "cluster", "namespace", namespace)
		return s.client, nil
	}
	provider, _, _, err := initProvider(false, s.contextArg, namespace)
	if err != nil {
		slog.Debug("provider resolve failed", "provider", "cluster", "namespace", namespace, "error", err)
		return nil, err
	}
	slog.Debug("provider resolved", "provider", "cluster", "namespace", namespace)
	return provider, err
}
