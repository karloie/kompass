package pipeline

import (
	"strings"

	"github.com/karloie/kompass/pkg/graph"
	"github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/tree"
)

// InferGraphs builds the graph response and applies tree-oriented root policies.
func InferGraphs(provider kube.Kube, selectors []string) (*kube.GraphResponse, error) {
	req := kube.GraphRequest{KeySelector: strings.Join(selectors, ",")}
	result, err := graph.InferGraphs(provider, req)
	if err != nil {
		return nil, err
	}
	tree.FilterOwnedJobRoots(result)
	tree.BuildTrees(result)
	return result, nil
}

// GraphNodesForGraph resolves node maps from response-level nodes when available,
// and falls back to legacy per-graph maps for backward compatibility.
func GraphNodesForGraph(result *kube.GraphResponse, graph *kube.Graph) map[string]*kube.Resource {
	if graph == nil {
		return nil
	}

	if len(result.Nodes) > 0 && len(graph.NodeKeys) > 0 {
		nodeMap := make(map[string]*kube.Resource, len(graph.NodeKeys))
		for _, key := range graph.NodeKeys {
			if node := result.Nodes[key]; node != nil {
				nodeMap[key] = node
			}
		}
		return nodeMap
	}

	return graph.Nodes
}
