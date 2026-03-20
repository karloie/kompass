package pipeline

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/karloie/kompass/pkg/graph"
	"github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/tree"
)

// BuildGraphs builds the graph response, applies tree-oriented root policies,
// and attaches cache stats if the provider supports them.
func BuildGraphs(provider kube.Kube, selectors []string) (*kube.Response, error) {
	graphStart := time.Now()
	slog.Debug("graph generation started", "selectors", selectors)

	req := kube.Request{Selectors: selectors}
	result, err := graph.BuildGraphs(provider, req)
	if err != nil {
		slog.Debug("graph generation failed", "selectors", selectors, "duration", time.Since(graphStart).String(), "error", err)
		return nil, err
	}

	if client, ok := provider.(*kube.Client); ok {
		result.Metadata = client.GetResponseMeta()
	}
	nodeCount, edgeCount := len(result.Nodes), len(result.Edges)
	slog.Debug("graph generation completed", "selectors", selectors, "components", len(result.Components), "nodes", nodeCount, "edges", edgeCount, "duration", time.Since(graphStart).String())

	policyStart := time.Now()
	slog.Debug("tree policy started", "components", len(result.Components))
	defer func() {
		if r := recover(); r != nil {
			slog.Debug("tree policy failed", "components", len(result.Components), "duration", time.Since(policyStart).String(), "error", r)
			err = fmt.Errorf("tree policy panic: %v", r)
			result = nil
		}
	}()
	tree.FilterOwnedJobRoots(result)
	tree.FilterOwnedSecretRoots(result)
	slog.Debug("tree policy completed", "components", len(result.Components), "duration", time.Since(policyStart).String())
	return result, err
}
