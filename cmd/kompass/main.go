package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/karloie/kompass/pkg/kube"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type CacheStats struct {
	Calls   int64
	Hits    int64
	Misses  int64
	HitRate float64
}

type JSONOutput struct {
	Request  RequestMetadata     `json:"request"`
	Response *kube.GraphResponse `json:"response"`
}

type RequestMetadata struct {
	Context    string   `json:"context"`
	Namespace  string   `json:"namespace"`
	ConfigPath string   `json:"configPath,omitempty"`
	Selectors  []string `json:"selectors"`
}

func init() {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(h))
}

func main() {
	contextArg := flag.String("context", "", "Kubernetes context (defaults to current context)")
	namespaceArg := flag.String("namespace", "", "Kubernetes namespace (defaults to current namespace or 'default')")
	mockArg := flag.Bool("mock", false, "Use mock provider")
	jsonArg := flag.Bool("json", false, "Output as pretty-printed JSON")
	plainArg := flag.Bool("plain", false, "Plain output without emojis and colors")
	serviceArg := flag.String("service", "", "Start web server (format: :port or host:port, defaults to :8080)")
	helpArg := flag.Bool("help", false, "Show help message")
	versionArg := flag.Bool("version", false, "Show version information")
	flag.BoolVar(helpArg, "h", false, "Shorthand for --help")
	flag.BoolVar(versionArg, "v", false, "Shorthand for --version")
	flag.StringVar(contextArg, "c", "", "Shorthand for --context")
	flag.StringVar(namespaceArg, "n", "", "Shorthand for --namespace")
	flag.Parse()

	if *versionArg {
		fmt.Printf("kompass %s\n  commit: %s\n  built:  %s\n", version, commit, date)
		os.Exit(0)
	}
	if *helpArg {
		printHelp()
		os.Exit(0)
	}
	selectors := flag.Args()
	if flag.Lookup("service").Value.String() != "" {
		addr := *serviceArg
		if addr == "" {
			addr = ":8080"
		}
		if !strings.Contains(addr, ":") {
			fmt.Fprintf(os.Stderr, "Error: --service address must be in format ':port' or 'host:port' (e.g., :8080 or 0.0.0.0:8080)\n")
			os.Exit(1)
		}
		startServer(addr, *contextArg, *namespaceArg)
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

	result, err := inferGraphs(provider, selectors)
	if err != nil {
		slog.Error("failed to infer graph", "cluster", context_, "namespace", namespace_, "selectors", selectors, "error", err.Error())
		os.Exit(1)
	}

	totalNodes, totalEdges := 0, 0
	for _, g := range result.Graphs {
		totalNodes += len(g.Nodes)
		totalEdges += len(g.Edges)
	}
	slog.Debug("graphs inferred", "cluster", context_, "namespace", namespace_, "selectors", selectors, "components", len(result.Graphs), "nodes", totalNodes, "edges", totalEdges)

	if *jsonArg {
		printGraphs(result, context_, namespace_, configPath, selectors)
	} else {
		printTrees(result, context_, namespace_, configPath, selectors, *plainArg, extractStats(provider))
	}
}
