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
	"github.com/karloie/kompass/pkg/tui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type serviceFlag struct {
	set  bool
	addr string
}

type executionMode int

const (
	modeCLI executionMode = iota
	modeService
	modeTUISelector
	modeTUIDashboard
)

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
	tuiArg := flag.Bool("tui", false, "Start interactive terminal UI")
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

	switch resolveExecutionMode(*tuiArg, serviceArg.set) {
	case modeTUIDashboard:
		if err := tui.Run(tui.Options{Mode: tui.ModeDashboard, OutputJSON: *jsonArg, Plain: *plainArg}); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	case modeService:
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

	totalNodes, totalEdges := len(result.Nodes), len(result.Edges)
	slog.Debug("graphs inferred", "cluster", context_, "namespace", namespace_, "selectors", selectors, "components", len(result.Components), "nodes", totalNodes, "edges", totalEdges)

	if resolveExecutionMode(*tuiArg, serviceArg.set) == modeTUISelector {
		selectorResult := tree.BuildResponseTree(result)
		if err := tui.Run(tui.Options{
			Mode:       tui.ModeSelector,
			Trees:      selectorResult,
			Context:    context_,
			Namespace:  namespace_,
			OutputJSON: *jsonArg,
			Plain:      *plainArg,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *jsonArg {
		printGraphs(result, context_, namespace_, configPath, selectors)
	} else {
		printTrees(tree.BuildResponseTree(result), context_, namespace_, configPath, selectors, *plainArg)
	}
}

func resolveExecutionMode(tuiEnabled, serviceEnabled bool) executionMode {
	if tuiEnabled && serviceEnabled {
		return modeTUIDashboard
	}
	if serviceEnabled {
		return modeService
	}
	if tuiEnabled {
		return modeTUISelector
	}
	return modeCLI
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
