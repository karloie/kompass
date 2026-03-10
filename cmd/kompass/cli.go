package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/karloie/kompass/pkg/graph"
	"github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/mock"
	"github.com/karloie/kompass/pkg/tree"
)

func printHelp() {
	fmt.Print(`Usage: kompass [options] [selector...]

Options:
  -c, --context <name>     K8s context
  -n, --namespace <name>   K8s namespace
  --mock <name>            Mock provider (mock)
  --json                   JSON output
  --plain                  Plain output without emojis and colors
  --service [addr]         Start web server (format: :port or host:port, default :8080)
  -v, --version            Show version
  -h, --help               Show help

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
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

func printTrees(result *kube.GraphResponse, context, namespace, configPath string, selectors []string, plain bool, stats map[string]interface{}) {
	statsStr := ""
	if cs := getCacheStats(stats); cs != nil {
		statsStr = fmt.Sprintf(", Cache: %d calls | %d hits | %d misses | %.1f%% hit rate", cs.Calls, cs.Hits, cs.Misses, cs.HitRate)
	}
	fmt.Printf("🌍 Context: %s, Namespace: %s, Selectors: %v, Config: %s%s\n\n", context, namespace, selectors, configPath, statsStr)

	for graphIdx, g := range result.Graphs {
		if g.Tree != nil {
			fmt.Print(tree.RenderTree(g.Tree, g.Nodes, plain))
			if graphIdx < len(result.Graphs)-1 && !plain {
				fmt.Println()
			}
		}
	}
}

func initProvider(useMock bool, contextArg, namespaceArg string) (kube.Kube, string, string, error) {
	if useMock {
		provider := kube.NewMockClient(mock.GenerateMock())
		if namespaceArg == "" {
			namespaceArg = "petshop"
		}
		provider.SetNamespace(namespaceArg)
		return provider, "mock", namespaceArg, nil
	}

	client, err := kube.NewClient(contextArg, namespaceArg)
	if err != nil {
		return nil, "", "", fmt.Errorf("error connecting to cluster: %w", err)
	}
	if contextArg == "" {
		contextArg, _ = client.GetContext()
	}
	return client, contextArg, namespaceArg, nil
}

func inferGraphs(provider kube.Kube, selectors []string) (*kube.GraphResponse, error) {
	req := kube.GraphRequest{KeySelector: strings.Join(selectors, ",")}
	result, err := graph.InferGraphs(provider, req)
	if err != nil {
		return nil, err
	}
	tree.BuildTrees(result)
	filterOwnedJobRoots(result)
	return result, nil
}

func filterOwnedJobRoots(result *kube.GraphResponse) {
	if result == nil || len(result.Graphs) < 2 {
		return
	}

	rootIDs := make(map[string]bool, len(result.Graphs))
	for _, g := range result.Graphs {
		rootIDs[g.ID] = true
	}

	filtered := make([]kube.Graph, 0, len(result.Graphs))
	for _, g := range result.Graphs {
		if tree.ParseResourceKeyRef(g.ID).Type != "job" {
			filtered = append(filtered, g)
			continue
		}

		rootNode, exists := g.Nodes[g.ID]
		if !exists || rootNode == nil {
			filtered = append(filtered, g)
			continue
		}

		meta := graph.M(rootNode.AsMap()).Map("metadata")
		namespace := meta.String("namespace")
		owners := meta.Slice("ownerReferences")

		hasCronJobRoot := false
		for _, owner := range owners {
			ownerMap, ok := owner.(map[string]any)
			if !ok {
				continue
			}
			if !strings.EqualFold(graph.M(ownerMap).String("kind"), "CronJob") {
				continue
			}
			ownerName := graph.M(ownerMap).String("name")
			if ownerName == "" {
				continue
			}
			cronJobKey := tree.BuildResourceKeyRef("cronjob", namespace, ownerName)
			if rootIDs[cronJobKey] {
				hasCronJobRoot = true
				break
			}
		}

		if hasCronJobRoot {
			continue
		}

		filtered = append(filtered, g)
	}

	result.Graphs = filtered
}

func extractStats(provider kube.Kube) map[string]interface{} {
	if client, ok := provider.(*kube.Client); ok {
		return client.GetCacheStats()
	}
	return nil
}

func getCacheStats(stats map[string]interface{}) *CacheStats {
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
