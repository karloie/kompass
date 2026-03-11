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
func InferGraphs(provider kube.Kube, selectors []string) (*kube.ResponseGraph, error) {
	graphStart := time.Now()
	slog.Debug("graph generation started", "selectors", selectors)

	req := kube.Request{KeySelector: strings.Join(selectors, ",")}
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

	policyStart := time.Now()
	slog.Debug("tree policy started", "graphs", len(result.Graphs))
	defer func() {
		if r := recover(); r != nil {
			slog.Debug("tree policy failed", "graphs", len(result.Graphs), "duration", time.Since(policyStart).String(), "error", r)
			err = fmt.Errorf("tree policy panic: %v", r)
			result = nil
		}
	}()
	tree.FilterOwnedJobRoots(result)
	slog.Debug("tree policy completed", "graphs", len(result.Graphs), "duration", time.Since(policyStart).String())
	return result, err
}

// GraphNodesForGraph resolves node maps from response-level nodes.
func GraphNodesForGraph(result *kube.ResponseGraph, graph *kube.Graph) map[string]*kube.Resource {
	if graph == nil {
		return nil
	}

	if len(result.Nodes) > 0 {
		return result.Nodes
	}

	return nil
}
