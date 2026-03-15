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
	if len(res.Components) == 0 {
		t.Fatalf("expected at least one component in response")
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

func TestGraphNodesForComponent_UsesResponseNodes(t *testing.T) {
	shared := kube.Resource{Key: "service/default/api", Type: "service", Resource: map[string]any{"metadata": map[string]any{"name": "api", "namespace": "default"}}}
	podA := kube.Resource{Key: "pod/default/a", Type: "pod", Resource: map[string]any{"metadata": map[string]any{"name": "a", "namespace": "default"}}}
	podB := kube.Resource{Key: "pod/default/b", Type: "pod", Resource: map[string]any{"metadata": map[string]any{"name": "b", "namespace": "default"}}}

	resp := &kube.Response{
		Nodes: []kube.Resource{podA, podB, shared},
		Components: []kube.Component{
			{ID: "pod/default/a", Root: "pod/default/a"},
			{ID: "pod/default/b", Root: "pod/default/b"},
		},
	}

	nodesA := GraphNodesForComponent(resp, &resp.Components[0])
	nodesB := GraphNodesForComponent(resp, &resp.Components[1])

	if len(nodesA) != 3 || len(nodesB) != 3 {
		t.Fatalf("expected full response node map, got %d and %d", len(nodesA), len(nodesB))
	}
	if nodesA["service/default/api"] == nil || nodesB["service/default/api"] == nil {
		t.Fatalf("expected shared node to be resolved for both graphs")
	}
}

func TestGraphNodesForComponent_ReturnsNilWithoutResponseNodes(t *testing.T) {
	resp := &kube.Response{Components: []kube.Component{{ID: "pod/default/a", Root: "pod/default/a"}}}

	nodes := GraphNodesForComponent(resp, &resp.Components[0])
	if nodes != nil {
		t.Fatalf("expected nil when response nodes are not present, got %#v", nodes)
	}
}

func TestGraphNodesForComponent_NilComponent(t *testing.T) {
	resp := &kube.Response{}
	if nodes := GraphNodesForComponent(resp, nil); nodes != nil {
		t.Fatalf("expected nil nodes for nil component input, got %#v", nodes)
	}
}

func TestGraphNodesForComponent_UsesAllResponseNodes(t *testing.T) {
	resp := &kube.Response{
		Nodes: []kube.Resource{
			{Key: "pod/default/a", Type: "pod"},
			{Key: "service/default/missing", Type: "service"},
		},
		Components: []kube.Component{{ID: "pod/default/a", Root: "pod/default/a"}},
	}

	nodes := GraphNodesForComponent(resp, &resp.Components[0])
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
