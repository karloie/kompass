package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/mock"
	"github.com/karloie/kompass/pkg/pipeline"
	"github.com/karloie/kompass/pkg/tree"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const jsonAPIVersion = "v1"

type CacheStats struct {
	Calls   int64
	Hits    int64
	Misses  int64
	HitRate float64
}

type JSONOutput struct {
	APIVersion string          `json:"apiVersion"`
	Request    RequestMetadata `json:"request"`
	Response   *kube.Graphs    `json:"response"`
}

type JSONOutputGraph struct {
	APIVersion string          `json:"apiVersion"`
	Request    RequestMetadata `json:"request"`
	Response   *kube.Graphs    `json:"response"`
}

type JSONOutputTree struct {
	APIVersion string          `json:"apiVersion"`
	Request    RequestMetadata `json:"request"`
	Response   *kube.Trees     `json:"response"`
}

type RequestMetadata struct {
	Context    string   `json:"context"`
	Namespace  string   `json:"namespace"`
	ConfigPath string   `json:"configPath,omitempty"`
	Selectors  []string `json:"selectors"`
}

type serviceFlag struct {
	set  bool
	addr string
}

func (s *serviceFlag) String() string {
	if s == nil {
		return ""
	}
	return s.addr
}

func (s *serviceFlag) Set(v string) error {
	s.set = true
	s.addr = v
	if v == "" || v == "true" {
		s.addr = "localhost:8080"
	}
	if v == "false" {
		s.set = false
		s.addr = ""
	}
	return nil
}

func (s *serviceFlag) IsBoolFlag() bool { return true }

func normalizeServiceArgs(args []string) []string {
	normalized := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--service" && i+1 < len(args) {
			next := args[i+1]
			if next != "" && !strings.HasPrefix(next, "-") {
				normalized = append(normalized, "--service="+next)
				i++
				continue
			}
		}
		normalized = append(normalized, args[i])
	}
	return normalized
}

func init() {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(h))
}

func main() {
	contextArg := flag.String("context", "", "Kubernetes context (defaults to current context)")
	namespaceArg := flag.String("namespace", "", "Kubernetes namespace (defaults to current namespace or 'default')")
	mockArg := flag.Bool("mock", false, "Use mock provider")
	debugArg := flag.Bool("debug", false, "Enable debug logging")
	jsonArg := flag.Bool("json", false, "JSON output")
	plainArg := flag.Bool("plain", false, "Plain output without ANSI colors")
	serviceArg := &serviceFlag{}
	flag.Var(serviceArg, "service", "Start web server (format: host:port, default localhost:8080)")
	helpArg := flag.Bool("help", false, "Show help message")
	versionArg := flag.Bool("version", false, "Show version information")
	flag.BoolVar(helpArg, "h", false, "Shorthand for --help")
	flag.BoolVar(versionArg, "v", false, "Shorthand for --version")
	flag.BoolVar(debugArg, "d", false, "Shorthand for --debug")
	flag.StringVar(contextArg, "c", "", "Shorthand for --context")
	flag.StringVar(namespaceArg, "n", "", "Shorthand for --namespace")
	_ = flag.CommandLine.Parse(normalizeServiceArgs(os.Args[1:]))

	level := slog.LevelInfo
	if *debugArg {
		level = slog.LevelDebug
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))

	if *versionArg {
		fmt.Printf("kompass %s\n  commit: %s\n  built:  %s\n", version, commit, date)
		os.Exit(0)
	}
	if *helpArg {
		printHelp()
		os.Exit(0)
	}
	selectors := flag.Args()
	if serviceArg.set {
		addr := serviceArg.addr
		if addr == "" {
			addr = "localhost:8080"
		}
		if !strings.Contains(addr, ":") {
			fmt.Fprintf(os.Stderr, "Error: --service address must be in format 'host:port' (e.g., localhost:8080 or 0.0.0.0:8080)\n")
			os.Exit(1)
		}
		startServer(addr, *contextArg, *namespaceArg, *mockArg)
		return
	}

	provider, _, _, err := initProvider(*mockArg, *contextArg, *namespaceArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	context_, _ := provider.GetContext()
	namespace_, _ := provider.GetNamespace()
	configPath, _ := provider.GetConfigPath()

	result, err := pipeline.InferGraphs(provider, selectors)
	if err != nil {
		slog.Error("failed to infer graph", "cluster", context_, "namespace", namespace_, "selectors", selectors, "error", err.Error())
		os.Exit(1)
	}

	totalNodes, totalEdges := len(result.Nodes), 0
	for _, g := range result.Graphs {
		totalEdges += len(g.Edges)
	}
	slog.Debug("graphs inferred", "cluster", context_, "namespace", namespace_, "selectors", selectors, "components", len(result.Graphs), "nodes", totalNodes, "edges", totalEdges)

	if *jsonArg {
		printGraphs(result, context_, namespace_, configPath, selectors)
	} else {
		printTrees(tree.BuildResponseTree(result), context_, namespace_, configPath, selectors, *plainArg, extractStats(provider))
	}
}

func initProvider(useMock bool, contextArg, namespaceArg string) (kube.Kube, string, string, error) {
	slog.Debug("initializing provider", "provider", map[bool]string{true: "mock", false: "cluster"}[useMock], "requestedContext", contextArg, "requestedNamespace", namespaceArg)

	if useMock {
		provider := kube.NewMockClient(mock.GenerateMock())
		if namespaceArg == "" {
			namespaceArg = "petshop"
		}
		provider.SetNamespace(namespaceArg)
		resolvedContext, _ := provider.GetContext()
		resolvedNamespace, _ := provider.GetNamespace()
		configPath, _ := provider.GetConfigPath()
		slog.Debug("provider initialized", "provider", "mock", "context", resolvedContext, "namespace", resolvedNamespace, "configPath", configPath)
		return provider, "mock", namespaceArg, nil
	}

	client, err := kube.NewClient(contextArg, namespaceArg)
	if err != nil {
		slog.Debug("provider initialization failed", "provider", "cluster", "requestedContext", contextArg, "requestedNamespace", namespaceArg, "error", err)
		return nil, "", "", fmt.Errorf("error connecting to cluster: %w", err)
	}
	resolvedContext, _ := client.GetContext()
	resolvedNamespace, _ := client.GetNamespace()
	configPath, _ := client.GetConfigPath()
	slog.Debug("provider initialized", "provider", "cluster", "context", resolvedContext, "namespace", resolvedNamespace, "configPath", configPath)
	if contextArg == "" {
		contextArg, _ = client.GetContext()
	}
	return client, contextArg, namespaceArg, nil
}

func extractStats(provider kube.Kube) map[string]interface{} {
	if client, ok := provider.(*kube.Client); ok {
		return client.GetStats()
	}
	return nil
}

func getStats(stats map[string]interface{}) *CacheStats {
	if stats == nil {
		return nil
	}
	if enabled, _ := stats["enabled"].(bool); !enabled {
		return nil
	}
	calls, _ := stats["calls"].(int64)
	if calls == 0 {
		return nil
	}
	hits, _ := stats["hits"].(int64)
	misses, _ := stats["misses"].(int64)
	hitRate, _ := stats["hitRate"].(float64)
	return &CacheStats{Calls: calls, Hits: hits, Misses: misses, HitRate: hitRate}
}
