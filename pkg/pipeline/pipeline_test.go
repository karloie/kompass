package pipeline

import (
	"testing"

	"github.com/karloie/kompass/pkg/kube"
)

func TestGraphNodesForGraph_UsesResponseNodesWithNodeKeys(t *testing.T) {
	shared := &kube.Resource{Key: "service/default/api", Type: "service", Resource: map[string]any{"metadata": map[string]any{"name": "api", "namespace": "default"}}}
	podA := &kube.Resource{Key: "pod/default/a", Type: "pod", Resource: map[string]any{"metadata": map[string]any{"name": "a", "namespace": "default"}}}
	podB := &kube.Resource{Key: "pod/default/b", Type: "pod", Resource: map[string]any{"metadata": map[string]any{"name": "b", "namespace": "default"}}}

	resp := &kube.GraphResponse{
		Nodes: map[string]*kube.Resource{
			"pod/default/a":       podA,
			"pod/default/b":       podB,
			"service/default/api": shared,
		},
		Graphs: []kube.Graph{
			{ID: "pod/default/a", NodeKeys: []string{"pod/default/a", "service/default/api"}},
			{ID: "pod/default/b", NodeKeys: []string{"pod/default/b", "service/default/api"}},
		},
	}

	nodesA := GraphNodesForGraph(resp, &resp.Graphs[0])
	nodesB := GraphNodesForGraph(resp, &resp.Graphs[1])

	if len(nodesA) != 2 || len(nodesB) != 2 {
		t.Fatalf("expected 2 nodes per graph, got %d and %d", len(nodesA), len(nodesB))
	}
	if nodesA["service/default/api"] == nil || nodesB["service/default/api"] == nil {
		t.Fatalf("expected shared node to be resolved for both graphs")
	}
}

func TestGraphNodesForGraph_FallsBackToGraphNodes(t *testing.T) {
	pod := &kube.Resource{Key: "pod/default/a", Type: "pod"}
	resp := &kube.GraphResponse{Graphs: []kube.Graph{
		{ID: "pod/default/a", Nodes: map[string]*kube.Resource{"pod/default/a": pod}},
	}}

	nodes := GraphNodesForGraph(resp, &resp.Graphs[0])
	if len(nodes) != 1 {
		t.Fatalf("expected fallback graph nodes map")
	}
}
