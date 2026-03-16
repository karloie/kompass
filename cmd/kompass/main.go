package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

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
	modeServiceAndTUI
)

type outputFormat int

const (
	outputFormatUnset outputFormat = iota
	outputFormatJSON
	outputFormatText
	outputFormatPlain
	outputFormatHTML
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
		if (args[i] == "--service" || args[i] == "-s") && i+1 < len(args) {
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
	outputArg := flag.String("output", "", "Output format: json|text|plain|html")
	tuiArg := flag.Bool("tui", false, "Start interactive terminal UI")
	serviceArg := &serviceFlag{}
	flag.Var(serviceArg, "service", "Start web server (format: host:port, default localhost:8080)")
	helpArg := flag.Bool("help", false, "Show help message")
	versionArg := flag.Bool("version", false, "Show version information")
	flag.BoolVar(helpArg, "h", false, "Shorthand for --help")
	flag.BoolVar(versionArg, "v", false, "Shorthand for --version")
	flag.BoolVar(debugArg, "d", false, "Shorthand for --debug")
	flag.BoolVar(mockArg, "m", false, "Shorthand for --mock")
	flag.BoolVar(tuiArg, "t", false, "Shorthand for --tui")
	flag.Var(serviceArg, "s", "Shorthand for --service")
	flag.StringVar(outputArg, "o", "", "Shorthand for --output")
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
	format, err := resolveOutputFormat(*outputArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	selectors := flag.Args()

	mode := resolveExecutionMode(serviceArg.set, *tuiArg, format, isInteractiveTerminal())

	if mode == modeService || mode == modeServiceAndTUI {
		addr := serviceArg.addr
		if addr == "" {
			addr = "localhost:8080"
		}
		if !strings.Contains(addr, ":") {
			fmt.Fprintf(os.Stderr, "Error: --service address must be in format 'host:port' (e.g., localhost:8080 or 0.0.0.0:8080)\n")
			os.Exit(1)
		}
		if mode == modeServiceAndTUI {
			go startServer(addr, *contextArg, *namespaceArg, *mockArg)
		} else {
			startServer(addr, *contextArg, *namespaceArg, *mockArg)
			return
		}
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

	if mode == modeTUISelector || mode == modeServiceAndTUI {
		selectorResult := tree.BuildResponseTree(result)
		if err := tui.Run(tui.Options{
			Mode:  tui.ModeSelector,
			Trees: selectorResult,
			Reload: func() (*kube.Response, error) {
				next, err := pipeline.InferGraphs(provider, selectors)
				if err != nil {
					return nil, err
				}
				return tree.BuildResponseTree(next), nil
			},
			RefreshInterval: 15 * time.Second,
			Context:         context_,
			Namespace:       namespace_,
			OutputJSON:      false,
			Plain:           false,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	effectiveFormat := format
	if effectiveFormat == outputFormatUnset {
		effectiveFormat = outputFormatText
	}

	switch effectiveFormat {
	case outputFormatJSON:
		printJsonGraphs(result, context_, namespace_, configPath, selectors)
	case outputFormatHTML:
		printTreesHtml(tree.BuildResponseTree(result), context_, namespace_, configPath, selectors)
	case outputFormatPlain:
		printTreesText(tree.BuildResponseTree(result), context_, namespace_, configPath, selectors, true)
	default:
		printTreesText(tree.BuildResponseTree(result), context_, namespace_, configPath, selectors, false)
	}
}

// resolveExecutionMode applies flag precedence in order:
//  1. --output (-o): forces one-shot CLI output; overrides --service and --tui
//  2. --service + --tui: starts the server in background, then launches TUI
//  3. --service:     starts the server (foreground, blocking)
//  4. --tui (-t):    interactive TUI; also the default when stdout is a terminal
//  5. default:       one-shot text output (non-interactive fallback)
//
// -d/--debug, -c/--context, -n/--namespace always apply in every mode.
func resolveExecutionMode(serviceEnabled, tuiEnabled bool, format outputFormat, interactive bool) executionMode {
	if format != outputFormatUnset {
		return modeCLI
	}
	if serviceEnabled && tuiEnabled {
		return modeServiceAndTUI
	}
	if serviceEnabled {
		return modeService
	}
	if tuiEnabled || interactive {
		return modeTUISelector
	}
	return modeCLI
}

func resolveOutputFormat(raw string) (outputFormat, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "":
		return outputFormatUnset, nil
	case "json":
		return outputFormatJSON, nil
	case "text":
		return outputFormatText, nil
	case "plain":
		return outputFormatPlain, nil
	case "html":
		return outputFormatHTML, nil
	default:
		return outputFormatUnset, fmt.Errorf("invalid --output value %q (allowed: json|text|plain|html)", raw)
	}
}

func isInteractiveTerminal() bool {
	return isCharDevice(os.Stdin) && isCharDevice(os.Stdout)
}

func isCharDevice(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
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
