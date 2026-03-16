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

Global options (always apply):

  -c, --context <name>     Kubernetes context
  -n, --namespace <name>   Kubernetes namespace

  -d, --debug              Enable debug logging
  -m, --mock               Use mock provider

Mode flags (mutually exclusive; listed in precedence order):

  -h, --help               Show help message
  -v, --version            Show version

  -o, --output <mode>      One-shot output: json|text|plain|html
                           Overrides --service and --tui
  -s, --service [addr]     Start web server (default localhost:8080)
                           Combine with --tui to also open interactive UI
  -t, --tui                Interactive terminal UI
                           Default when stdout is a terminal

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

func printJsonGraphs(result *kube.Response, context, namespace, configPath string, selectors []string) {
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

func printTreesText(result *kube.Response, context, namespace, configPath string, selectors []string, plain bool) {
	header := "🌍 " + tree.FormatTreeHeader(context, namespace, configPath, selectors)
	fmt.Print(tree.RenderText(result, header, plain))
}

func printTreesHtml(result *kube.Response, context, namespace, configPath string, selectors []string) {
	fmt.Print(tree.RenderHtml(result, context, namespace, configPath, selectors, false))
}
