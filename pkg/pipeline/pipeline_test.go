package pipeline

import (
	"testing"

	"github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/mock"
)

func TestInferGraphsSuccess(t *testing.T) {
	provider := kube.NewMockClient(mock.GenerateMock())
	res, err := InferGraphs(provider, []string{"*/petshop/*"})
	if err != nil {
		t.Fatalf("expected infer graphs success, got err: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil graph response")
	}
	if len(res.Graphs) == 0 {
		t.Fatalf("expected at least one graph in response")
	}
	hasTree := false
	for _, g := range res.Graphs {
		if g.Tree != nil {
			hasTree = true
			break
		}
	}
	if !hasTree {
		t.Fatalf("expected at least one built tree in graphs")
	}
}

func TestInferGraphsPropagatesProviderError(t *testing.T) {
	provider := kube.NewMockClient(mock.GenerateMock(), kube.MockConfig{AllError: true})
	res, err := InferGraphs(provider, []string{"*/petshop/*"})
	if err == nil {
		t.Fatalf("expected error when provider fails")
	}
	if res != nil {
		t.Fatalf("expected nil response on error, got %#v", res)
	}
}

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

func TestGraphNodesForGraph_NilGraph(t *testing.T) {
	resp := &kube.GraphResponse{}
	if nodes := GraphNodesForGraph(resp, nil); nodes != nil {
		t.Fatalf("expected nil nodes for nil graph input, got %#v", nodes)
	}
}

func TestGraphNodesForGraph_ResponseNodesMissingKeyFilteredOut(t *testing.T) {
	resp := &kube.GraphResponse{
		Nodes: map[string]*kube.Resource{
			"pod/default/a": {Key: "pod/default/a", Type: "pod"},
		},
		Graphs: []kube.Graph{{
			ID:       "pod/default/a",
			NodeKeys: []string{"pod/default/a", "service/default/missing"},
		}},
	}

	nodes := GraphNodesForGraph(resp, &resp.Graphs[0])
	if len(nodes) != 1 {
		t.Fatalf("expected only existing response nodes to be mapped, got %#v", nodes)
	}
	if nodes["pod/default/a"] == nil {
		t.Fatalf("expected mapped pod node")
	}
	if nodes["service/default/missing"] != nil {
		t.Fatalf("expected missing key to be skipped")
	}
}
