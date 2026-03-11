package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/pipeline"
	"github.com/karloie/kompass/pkg/tree"
)

func printHelp() {
	fmt.Print(`Usage: kompass [options] [selector...]

Options:
  -c, --context <name>     K8s context
  -n, --namespace <name>   K8s namespace
  --service [addr]         Start web server (format: :port or host:port, default :8080)
  --json                   JSON output
  --mock <name>            Mock provider (mock)
  --plain                  Plain output without emojis and colors
  -d, --debug              Enable debug logging
  -h, --help               Show help
  -v, --version            Show version

Selectors (type/namespace/name format):
  (empty)                  All pods in current namespace + inferred resources
  */namespace/*            All resources in namespace + inferred resources
  type/namespace/name      Specific resource + inferred resources
  */namespace/prefix*      Resources matching pattern + inferred resources

Shorthand (auto-expanded to 3-part format):
  name                     → */current-namespace/name
  namespace/name           → */namespace/name
  *                        → */current-namespace/*

Examples:
  kompass                                      # All pods in current namespace
  kompass */petshop/*            # All resources in petshop
  kompass pod/default/nginx                    # Specific pod + related resources
  kompass */prod/api-*                         # All api-* resources in prod
  kompass deployment/kube-system/coredns       # Specific deployment
  kompass --mock                               # Mock mode: all pods
  kompass --mock '*/petshop/*'   # Mock: all in namespace
  kompass --service                            # Start web server on :8080
  kompass --service :9090                      # Start web server on :9090

Note: All selectors automatically include inferred/connected resources.
	Output ordering follows dependency-aware resource relationships.
`)
}

func printGraphs(result *kube.GraphResponse, context, namespace, configPath string, selectors []string) {
	output := JSONOutput{
		Request:  RequestMetadata{context, namespace, configPath, selectors},
		Response: result,
	}
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
}

func printTrees(result *kube.GraphResponse, context, namespace, configPath string, selectors []string, plain bool, stats map[string]interface{}) {
	statsStr := ""
	if cs := getStats(stats); cs != nil {
		statsStr = fmt.Sprintf(", Cache: %d calls | %d hits | %d misses | %.1f%% hit rate", cs.Calls, cs.Hits, cs.Misses, cs.HitRate)
	}
	fmt.Printf("🌍 Context: %s, Namespace: %s, Selectors: %v, Config: %s%s\n\n", context, namespace, selectors, configPath, statsStr)

	for graphIdx := range result.Graphs {
		g := &result.Graphs[graphIdx]
		if g.Tree != nil {
			fmt.Print(tree.RenderTree(g.Tree, pipeline.GraphNodesForGraph(result, g), plain))
			if graphIdx < len(result.Graphs)-1 && !plain {
				fmt.Println()
			}
		}
	}
}
