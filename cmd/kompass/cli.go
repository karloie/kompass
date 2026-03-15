package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/tree"
)

func printHelp() {
	fmt.Print(`Usage: kompass [options] [selector...]

Options:
  -c, --context <name>     K8s context
  -n, --namespace <name>   K8s namespace
  --service [addr]         Start web server (format: host:port, default localhost:8080)
  --json                   JSON output
  --mock <name>            Mock provider (mock)
  --plain                  Plain output without ANSI colors
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
  kompass */petshop/*                          # All resources in petshop
  kompass pod/default/nginx                    # Specific pod + related resources
  kompass */prod/api-*                         # All api-* resources in prod
  kompass deployment/kube-system/coredns       # Specific deployment
  kompass --mock                               # Mock mode: all pods
  kompass --mock '*/petshop/*'                 # Mock: all in namespace
  kompass --service                            # Start web server on localhost:8080
  kompass --service 0.0.0.0:8080               # Start web server, published on all interfaces

Note: All selectors automatically include inferred/connected resources.
	Output ordering follows dependency-aware resource relationships.
`)
}

func printGraphs(result *kube.Response, context, namespace, configPath string, selectors []string) {
	if result == nil {
		result = &kube.Response{}
	}
	result.APIVersion = "v1"
	result.Request = kube.Request{
		Context:    context,
		Namespace:  namespace,
		ConfigPath: configPath,
		Selectors:  selectors,
	}
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
}

func printTrees(result *kube.Response, context, namespace, configPath string, selectors []string, plain bool) {
	header := fmt.Sprintf("🌍 Kompass Context: %s, Namespace: %s, Selectors: %v, Config: %s", context, namespace, selectors, configPath)
	fmt.Print(tree.RenderText(result, header, plain))
}
