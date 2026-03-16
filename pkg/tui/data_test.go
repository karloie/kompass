package tui

import (
	"strings"
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/tree"
)

func TestFlattenTrees_UsesResolvedMetadataFromNodeMap(t *testing.T) {
	response := &kube.Response{
		Nodes: []kube.Resource{
			{
				Key:  "pod/ns/api",
				Type: "pod",
				Resource: map[string]any{
					"metadata": map[string]any{"name": "api", "namespace": "ns"},
					"status":   map[string]any{"phase": "Running"},
				},
			},
		},
		Trees: []kube.Tree{
			{Key: "pod/ns/api", Type: "pod", Meta: map[string]any{}},
		},
	}

	rows := flattenTrees(response)
	if len(rows) == 0 {
		t.Fatalf("expected flattened rows")
	}

	first := rows[0]
	if got, _ := first.Metadata["name"].(string); got != "api" {
		t.Fatalf("expected row metadata name api, got %q", got)
	}
	if got, _ := first.Metadata["namespace"].(string); got != "ns" {
		t.Fatalf("expected row metadata namespace ns, got %q", got)
	}
	if !strings.Contains(first.SearchText, "api") {
		t.Fatalf("expected search text to include resolved metadata, got %q", first.SearchText)
	}
}

func TestFlattenTrees_SearchTextUsesSharedTreeHelpers(t *testing.T) {
	response := &kube.Response{
		Nodes: []kube.Resource{
			{
				Key:  "pod/ns/api",
				Type: "pod",
				Resource: map[string]any{
					"metadata": map[string]any{"name": "api", "namespace": "ns"},
					"status":   map[string]any{"phase": "Running"},
				},
			},
		},
		Trees: []kube.Tree{
			{Key: "pod/ns/api", Type: "pod", Meta: map[string]any{}},
		},
	}

	rows := flattenTrees(response)
	if len(rows) == 0 {
		t.Fatalf("expected flattened rows")
	}

	node := &response.Trees[0]
	nodeMap := response.NodeMap()
	meta := tree.ResolveNodeMetadata(node, nodeMap)
	label := tree.RenderNodeLabel(node, nodeMap, true, nil)
	expected := strings.Join([]string{node.Key, tree.BuildNodeSearchText(node.Type, label, meta)}, " ")

	if rows[0].SearchText != expected {
		t.Fatalf("expected row search text to match shared helper output\nexpected: %q\nactual:   %q", expected, rows[0].SearchText)
	}
}

func TestFlattenTrees_PlainRowUsesSharedNodeLabel(t *testing.T) {
	response := &kube.Response{
		Nodes: []kube.Resource{
			{
				Key:  "pod/ns/api",
				Type: "pod",
				Resource: map[string]any{
					"metadata": map[string]any{"name": "api", "namespace": "ns"},
					"status":   map[string]any{"phase": "Running"},
				},
			},
		},
		Trees: []kube.Tree{
			{Key: "pod/ns/api", Type: "pod", Meta: map[string]any{}},
		},
	}

	rows := flattenTrees(response)
	if len(rows) == 0 {
		t.Fatalf("expected flattened rows")
	}

	node := &response.Trees[0]
	nodeMap := response.NodeMap()
	expected := tree.RenderNodeLabel(node, nodeMap, true, nil)

	if rows[0].Plain != expected {
		t.Fatalf("expected plain row label to match shared helper output\nexpected: %q\nactual:   %q", expected, rows[0].Plain)
	}
}

func TestFlattenTrees_ChildRowKeepsBranchPrefixAndSharedLabelBody(t *testing.T) {
	response := &kube.Response{
		Trees: []kube.Tree{
			{
				Key:  "deployment/ns/api",
				Type: "deployment",
				Meta: map[string]any{"name": "api", "namespace": "ns", "status": "Ready"},
				Children: []*kube.Tree{
					{
						Key:  "pod/ns/api-7f9d",
						Type: "pod",
						Meta: map[string]any{"name": "api-7f9d", "namespace": "ns", "status": "Running"},
					},
				},
			},
		},
	}

	rows := flattenTrees(response)
	if len(rows) < 2 {
		t.Fatalf("expected at least two rows, got %d", len(rows))
	}

	childRow := rows[1]
	if !strings.HasPrefix(childRow.Plain, "└─ ") {
		t.Fatalf("expected child row to keep tree branch prefix, got %q", childRow.Plain)
	}

	parentMeta := map[string]any{"name": "api", "namespace": "ns", "status": "Ready", "__nodeType": "deployment"}
	childNode := response.Trees[0].Children[0]
	expectedBody := tree.RenderNodeLabel(childNode, response.NodeMap(), true, parentMeta)
	actualBody := strings.TrimPrefix(childRow.Plain, "└─ ")

	if actualBody != expectedBody {
		t.Fatalf("expected child row body to match shared helper output\nexpected: %q\nactual:   %q", expectedBody, actualBody)
	}

	expectedSearch := strings.Join([]string{childNode.Key, tree.BuildNodeSearchText(childNode.Type, expectedBody, childNode.Meta)}, " ")
	if childRow.SearchText != expectedSearch {
		t.Fatalf("expected child row search text to match shared helper output\nexpected: %q\nactual:   %q", expectedSearch, childRow.SearchText)
	}
}
