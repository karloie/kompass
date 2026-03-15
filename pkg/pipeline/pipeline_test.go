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

func TestGraphNodesForGraph_UsesResponseNodes(t *testing.T) {
	shared := &kube.Resource{Key: "service/default/api", Type: "service", Resource: map[string]any{"metadata": map[string]any{"name": "api", "namespace": "default"}}}
	podA := &kube.Resource{Key: "pod/default/a", Type: "pod", Resource: map[string]any{"metadata": map[string]any{"name": "a", "namespace": "default"}}}
	podB := &kube.Resource{Key: "pod/default/b", Type: "pod", Resource: map[string]any{"metadata": map[string]any{"name": "b", "namespace": "default"}}}

	resp := &kube.Response{
		Nodes: map[string]*kube.Resource{
			"pod/default/a":       podA,
			"pod/default/b":       podB,
			"service/default/api": shared,
		},
		Graphs: []kube.Graph{
			{ID: "pod/default/a"},
			{ID: "pod/default/b"},
		},
	}

	nodesA := GraphNodesForGraph(resp, &resp.Graphs[0])
	nodesB := GraphNodesForGraph(resp, &resp.Graphs[1])

	if len(nodesA) != 3 || len(nodesB) != 3 {
		t.Fatalf("expected full response node map, got %d and %d", len(nodesA), len(nodesB))
	}
	if nodesA["service/default/api"] == nil || nodesB["service/default/api"] == nil {
		t.Fatalf("expected shared node to be resolved for both graphs")
	}
}

func TestGraphNodesForGraph_ReturnsNilWithoutResponseNodes(t *testing.T) {
	resp := &kube.Response{Graphs: []kube.Graph{{ID: "pod/default/a"}}}

	nodes := GraphNodesForGraph(resp, &resp.Graphs[0])
	if nodes != nil {
		t.Fatalf("expected nil when response nodes are not present, got %#v", nodes)
	}
}

func TestGraphNodesForGraph_NilGraph(t *testing.T) {
	resp := &kube.Response{}
	if nodes := GraphNodesForGraph(resp, nil); nodes != nil {
		t.Fatalf("expected nil nodes for nil graph input, got %#v", nodes)
	}
}

func TestGraphNodesForGraph_UsesAllResponseNodes(t *testing.T) {
	resp := &kube.Response{
		Nodes: map[string]*kube.Resource{
			"pod/default/a":           {Key: "pod/default/a", Type: "pod"},
			"service/default/missing": {Key: "service/default/missing", Type: "service"},
		},
		Graphs: []kube.Graph{{
			ID: "pod/default/a",
		}},
	}

	nodes := GraphNodesForGraph(resp, &resp.Graphs[0])
	if len(nodes) != 2 {
		t.Fatalf("expected full response nodes to be returned, got %#v", nodes)
	}
	if nodes["pod/default/a"] == nil {
		t.Fatalf("expected mapped pod node")
	}
	if nodes["service/default/missing"] == nil {
		t.Fatalf("expected additional response node to be present")
	}
}
