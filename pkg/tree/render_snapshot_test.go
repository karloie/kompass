package tree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

type snapshotEnvelope struct {
	Response kube.Graphs `json:"response"`
}

func loadMockSnapshotGraphs(t *testing.T) *kube.Graphs {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", "fixtures", "mock.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mock snapshot fixture: %v", err)
	}

	var env snapshotEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal mock snapshot fixture: %v", err)
	}
	if len(env.Response.Graphs) == 0 || len(env.Response.Nodes) == 0 {
		t.Fatalf("fixture response is empty")
	}

	return &env.Response
}

func renderAllTrees(t *testing.T, trees *kube.Trees, plain bool) string {
	t.Helper()

	var rendered []string
	for _, tr := range trees.Trees {
		if tr == nil {
			continue
		}
		rendered = append(rendered, RenderTree(tr, trees.Nodes, plain))
	}
	return strings.Join(rendered, "\n")
}

func TestBuildAndRenderTree_FromMockSnapshot_CoversVariationPaths(t *testing.T) {
	graphs := loadMockSnapshotGraphs(t)
	trees := BuildResponseTree(graphs)
	if trees == nil || len(trees.Trees) == 0 {
		t.Fatalf("expected response trees")
	}

	plain := renderAllTrees(t, trees, true)
	colored := renderAllTrees(t, trees, false)

	mustContain := []string{
		"deployment petshop-kafka",
		"petshop-kafka-6b7c8d9e0f-v58bh",
		"CrashLoopBackOff",
		"EXPIRED 5D AGO",
		"gateway internal-gateway",
		"poddisruptionbudget petshop-tennant-pdb",
		"petshop-lowe",
		"petshop-boys-motor-hpa",
	}
	for _, needle := range mustContain {
		if !strings.Contains(plain, needle) {
			t.Fatalf("plain render missing %q", needle)
		}
	}

	if !strings.Contains(plain, "PROGRAMMED") {
		t.Fatalf("expected gateway programmed status in plain render")
	}
	if !strings.Contains(plain, "HEALTHY") {
		t.Fatalf("expected pdb healthy status in plain render")
	}

	if !strings.Contains(colored, "\x1b[") {
		t.Fatalf("expected ansi color output in non-plain render")
	}
}
