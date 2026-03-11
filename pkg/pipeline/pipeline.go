package pipeline

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/karloie/kompass/pkg/graph"
	"github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/tree"
)

// InferGraphs builds the graph response and applies tree-oriented root policies.
func InferGraphs(provider kube.Kube, selectors []string) (*kube.GraphResponse, error) {
	graphStart := time.Now()
	slog.Debug("graph generation started", "selectors", selectors)

	req := kube.GraphRequest{KeySelector: strings.Join(selectors, ",")}
	result, err := graph.InferGraphs(provider, req)
	if err != nil {
		slog.Debug("graph generation failed", "selectors", selectors, "duration", time.Since(graphStart).String(), "error", err)
		return nil, err
	}

	nodeCount, edgeCount := len(result.Nodes), 0
	for _, g := range result.Graphs {
		edgeCount += len(g.Edges)
	}
	slog.Debug("graph generation completed", "selectors", selectors, "graphs", len(result.Graphs), "nodes", nodeCount, "edges", edgeCount, "duration", time.Since(graphStart).String())

	treeStart := time.Now()
	slog.Debug("tree generation started", "graphs", len(result.Graphs))
	defer func() {
		if r := recover(); r != nil {
			slog.Debug("tree generation failed", "graphs", len(result.Graphs), "duration", time.Since(treeStart).String(), "error", r)
			err = fmt.Errorf("tree generation panic: %v", r)
			result = nil
		}
	}()
	tree.FilterOwnedJobRoots(result)
	tree.BuildTrees(result)
	slog.Debug("tree generation completed", "graphs", len(result.Graphs), "duration", time.Since(treeStart).String())
	return result, err
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
