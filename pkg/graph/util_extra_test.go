package graph

import (
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

// --- M.IntOk ---

func TestMIntOk_Float64(t *testing.T) {
	m := M{"count": float64(7)}
	v, ok := m.IntOk("count")
	if !ok || v != 7 {
		t.Fatalf("expected 7 ok=true, got %d ok=%v", v, ok)
	}
}

func TestMIntOk_Int(t *testing.T) {
	m := M{"count": 3}
	v, ok := m.IntOk("count")
	if !ok || v != 3 {
		t.Fatalf("expected 3 ok=true, got %d ok=%v", v, ok)
	}
}

func TestMIntOk_Missing(t *testing.T) {
	m := M{"other": "x"}
	_, ok := m.IntOk("count")
	if ok {
		t.Fatal("expected ok=false for missing key")
	}
}

func TestMIntOk_Nil(t *testing.T) {
	var m M
	_, ok := m.IntOk("anything")
	if ok {
		t.Fatal("expected ok=false for nil map")
	}
}

func TestMIntOk_WrongType(t *testing.T) {
	m := M{"count": "notanint"}
	_, ok := m.IntOk("count")
	if ok {
		t.Fatal("expected ok=false for string value")
	}
}

// --- GetResourceEmoji ---

func TestGetResourceEmoji_KnownTypes(t *testing.T) {
	for _, tc := range []struct {
		rtype string
		want  string
	}{
		{"pod", "🫛"},
		{"deployment", "🚀"},
		{"service", "🤝"},
		{"secret", "🔒"},
		{"configmap", "⚙"},
	} {
		got := kube.GetResourceEmoji(tc.rtype)
		if got != tc.want {
			t.Errorf("type=%q: expected %q, got %q", tc.rtype, tc.want, got)
		}
	}
}

func TestGetResourceEmoji_UnknownType(t *testing.T) {
	got := kube.GetResourceEmoji("totally-unknown-type-xyz")
	if got != "📄" {
		t.Fatalf("expected fallback '📄', got %q", got)
	}
}

func TestGetResourceEmoji_EmptyString(t *testing.T) {
	got := kube.GetResourceEmoji("")
	if got != "📄" {
		t.Fatalf("expected fallback '📄' for empty type, got %q", got)
	}
}

// --- inferSimpleNode ---

func TestInferSimpleNode_AddsNodeToMap(t *testing.T) {
	fn := inferSimpleNode("configmap")
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{}
	item := kube.Resource{
		Key:  "configmap/ns/my-config",
		Type: "configmap",
		Resource: map[string]any{
			"metadata": map[string]any{"namespace": "ns", "name": "my-config"},
		},
	}

	err := fn(&edges, &item, &nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := nodes["configmap/ns/my-config"]; !ok {
		t.Fatal("expected node to be added to map")
	}
}

func TestInferSimpleNode_AlwaysReturnsNilError(t *testing.T) {
	fn := inferSimpleNode("secret")
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{}
	item := kube.Resource{Key: "secret/ns/s", Type: "secret", Resource: map[string]any{
		"metadata": map[string]any{"namespace": "ns", "name": "s"},
	}}
	if err := fn(&edges, &item, &nodes, nil); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

// --- inferPod ---

func TestInferPod_AddsNodeToMap(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{}
	item := kube.Resource{
		Key:  "pod/ns/my-pod",
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"namespace": "ns", "name": "my-pod"},
		},
	}
	err := inferPod(&edges, &item, &nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := nodes["pod/ns/my-pod"]; !ok {
		t.Fatal("expected pod node in map")
	}
}
