package tree

import (
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestFormatTreeHeader_UsesCommonOrdering(t *testing.T) {
	got := FormatTreeHeader("ctx", "ns", "mock", []string{"pod/ns/api"})
	want := "Kompass Context: ctx, Namespace: ns, Selectors: [pod/ns/api], Config: mock"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRenderNodeLabel_UsesTreeIconFallback(t *testing.T) {
	node := &kube.Tree{Key: "service/ns/api", Type: "service", Icon: "[svc]", Meta: map[string]any{}}

	got := RenderNodeLabel(node, nil, true, nil)
	if got != "[svc] service" {
		t.Fatalf("expected icon fallback label, got %q", got)
	}
}

func TestResolveNodeMetadata_UsesNodeMapRules(t *testing.T) {
	node := &kube.Tree{Key: "pod/ns/api", Type: "pod", Meta: map[string]any{}}
	nodes := map[string]*kube.Resource{
		"pod/ns/api": {
			Key:  "pod/ns/api",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "api", "namespace": "ns"},
				"status":   map[string]any{"phase": "Running"},
			},
		},
	}

	meta := ResolveNodeMetadata(node, nodes)
	if meta == nil {
		t.Fatalf("expected resolved metadata")
	}
	if got, _ := meta["name"].(string); got != "api" {
		t.Fatalf("expected metadata name api, got %q", got)
	}
	if got, _ := meta["namespace"].(string); got != "ns" {
		t.Fatalf("expected metadata namespace ns, got %q", got)
	}
}

func TestBuildChildParentMeta_InheritsNamespaceAndNodeType(t *testing.T) {
	meta := map[string]any{"name": "api-pod"}
	parent := map[string]any{"namespace": "ns"}

	got := BuildChildParentMeta("pod", meta, parent)
	if ns, _ := got["namespace"].(string); ns != "ns" {
		t.Fatalf("expected inherited namespace ns, got %q", ns)
	}
	if nodeType, _ := got["__nodeType"].(string); nodeType != "pod" {
		t.Fatalf("expected __nodeType pod, got %q", nodeType)
	}
}
