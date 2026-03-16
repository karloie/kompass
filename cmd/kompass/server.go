package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path"
	"sync"
	"syscall"
	"time"

	"strings"

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
	providerMu    sync.Mutex
	webRoot       fs.FS
}

func startServer(addr, contextArg, namespaceArg string, useMock bool) {
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}
	fatalStart := func(msg string, err error) {
		slog.Error(msg, "error", err)
		os.Exit(1)
	}
	slog.Info("Starting kompass server", "addr", addr, "context", contextArg, "namespace", namespaceArg, "provider", map[bool]string{true: "mock", false: "cluster"}[useMock])
	parts := strings.Split(addr, ":")
	port := ":" + parts[len(parts)-1]
	provider, _, _, err := initProvider(useMock, contextArg, namespaceArg)
	if err != nil {
		fatalStart("Failed to create provider", err)
	}
	client, ok := provider.(*kube.Client)
	if !ok {
		fatalStart("Provider type assertion failed", fmt.Errorf("provider is %T, expected *kube.Client", provider))
	}
	namespacesToWatch := []string{namespaceArg}
	if namespaceArg == "" {
		// Empty list means "sync all namespaces" in performSync.
		namespacesToWatch = []string{}
	}
	if err := client.StartSync(30*time.Second, namespacesToWatch); err != nil {
		fatalStart("Failed to start cache sync", err)
	}
	slog.Info("Cache sync started", "interval", "30s", "namespaces", namespacesToWatch)
	srv := &server{
		contextArg:   contextArg,
		namespaceArg: namespaceArg,
		client:       client,
		webRoot:      tree.ResolveAppWebRoot(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/graph", srv.handleGraph)
	mux.HandleFunc("/api/tree", srv.handleTree)
	mux.HandleFunc("/api/app/desc", srv.handleAppDescribe)
	mux.HandleFunc("/api/app/logs", srv.handleAppLogs)
	mux.HandleFunc("/api/app/events", srv.handleAppEvents)
	mux.HandleFunc("/api/app/hubble", srv.handleAppHubble)
	mux.HandleFunc("/api/app/yaml", srv.handleAppYAML)
	mux.HandleFunc("/api/health", srv.handleHealth("json", false))
	mux.HandleFunc("/api/healthz", srv.handleHealth("text", false))
	mux.HandleFunc("/api/readyz", srv.handleHealth("text", true))
	mux.HandleFunc("/api/metadata", srv.handleMetadata)
	mux.HandleFunc("/", srv.handleWeb)
	httpServer := &http.Server{Addr: addr, Handler: mux}
	go func() {
		slog.Info("Server ready", "url", "http://localhost"+port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fatalStart("Server failed", err)
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

func (s *server) handleWeb(w http.ResponseWriter, r *http.Request) {
	if s.webRoot == nil {
		http.NotFound(w, r)
		return
	}

	clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if clean == "." || clean == "" {
		clean = "index.html"
	}

	if _, err := fs.Stat(s.webRoot, clean); err == nil {
		http.FileServer(http.FS(s.webRoot)).ServeHTTP(w, r)
		return
	}

	// SPA fallback for client-side routes.
	r.URL.Path = "/index.html"
	http.FileServer(http.FS(s.webRoot)).ServeHTTP(w, r)
}

func (s *server) handleHealth(format string, checkReady bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if checkReady {
			if s.client == nil {
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if _, err := s.client.GetPods(s.namespaceArg, ctx, metav1.ListOptions{Limit: 1}); err != nil {
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
		}
		if format == "json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok"}`))
		} else {
			w.Write([]byte("ok"))
		}
	}
}

func (s *server) handleMetadata(w http.ResponseWriter, r *http.Request) {
	if s.client == nil {
		http.Error(w, "No active client", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.client.GetResponseMeta())
}

func (s *server) handleGraph(w http.ResponseWriter, r *http.Request) {
	selectors, _, provider, result, err := s.inferForRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if result == nil {
		result = &kube.Response{}
	}
	context_, _ := provider.GetContext()
	namespace_, _ := provider.GetNamespace()
	configPath, _ := provider.GetConfigPath()
	result.APIVersion = "v1"
	result.Request = kube.Request{
		Context:    context_,
		Namespace:  namespace_,
		ConfigPath: configPath,
		Selectors:  selectors,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	json.NewEncoder(w).Encode(graphOnlyResponse(result))
}

func (s *server) handleTree(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Vary", "Accept")
	accept := r.Header.Get("Accept")
	switch {
	case strings.Contains(accept, "text/plain"):
		s.handleTreeText(w, r)
		return
	case strings.Contains(accept, "text/html"):
		s.handleTreeHTML(w, r)
		return
	}

	selectors, _, provider, result, err := s.inferForRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	treeResult := tree.BuildResponseTree(result)
	if treeResult == nil {
		treeResult = &kube.Response{}
	}
	context_, _ := provider.GetContext()
	namespace_, _ := provider.GetNamespace()
	configPath, _ := provider.GetConfigPath()
	treeResult.APIVersion = "v1"
	treeResult.Request = kube.Request{
		Context:    context_,
		Namespace:  namespace_,
		ConfigPath: configPath,
		Selectors:  selectors,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	json.NewEncoder(w).Encode(treeResult)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	context_, _ := provider.GetContext()
	namespace_, _ := provider.GetNamespace()
	configPath, _ := provider.GetConfigPath()
	header := fmt.Sprintf("🌍 Kompass Context: %s, Namespace: %s, Selectors: %v, Config: %s", context_, namespace_, selectors, configPath)
	w.Write([]byte(tree.RenderText(tree.BuildResponseTree(result), header, plain)))
}

func (s *server) handleTreeHTML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")

	selectors, _, provider, result, err := s.inferForRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	context_, _ := provider.GetContext()
	namespace_, _ := provider.GetNamespace()
	configPath, _ := provider.GetConfigPath()
	staticMode := strings.EqualFold(r.URL.Query().Get("static"), "1") || strings.EqualFold(r.URL.Query().Get("static"), "true")

	treeResult := tree.BuildResponseTree(result)
	if treeResult == nil {
		treeResult = &kube.Response{}
	}
	treeResult.APIVersion = "v1"
	treeResult.Request = kube.Request{
		Context:    context_,
		Namespace:  namespace_,
		ConfigPath: configPath,
		Selectors:  selectors,
	}

	mode := "dynamic"
	if staticMode {
		mode = "static"
	}

	w.Write([]byte(tree.RenderAppHTML(s.webRoot, treeResult, tree.HTMLBootstrapConfig{
		Mode:      mode,
		APIBase:   "/api/tree",
		Static:    staticMode,
		Context:   context_,
		Namespace: namespace_,
	})))
}

func (s *server) inferForRequest(r *http.Request) ([]string, string, kube.Kube, *kube.Response, error) {
	selectors := graph.ParseSelectors(r.URL.Query().Get("selector"))
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = s.namespaceArg
	}

	s.providerMu.Lock()
	defer s.providerMu.Unlock()

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

func graphOnlyResponse(result *kube.Response) *kube.Response {
	if result == nil {
		return nil
	}
	return &kube.Response{
		APIVersion: result.APIVersion,
		Request:    result.Request,
		Nodes:      result.Nodes,
		Edges:      result.Edges,
		Components: result.Components,
		Metadata:   result.Metadata,
	}
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
		s.client.SetNamespace(namespace)
		return s.client, nil
	}
	provider, _, _, err := initProvider(false, s.contextArg, namespace)
	return provider, err
}
