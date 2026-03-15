package tree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

type snapshotEnvelope struct {
	Response kube.Response `json:"response"`
}

func loadMockSnapshotGraphs(t *testing.T) *kube.Response {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", "fixtures", "mock.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mock snapshot fixture: %v", err)
	}

	var direct kube.Response
	if err := json.Unmarshal(data, &direct); err == nil && (len(direct.Graphs) > 0 || len(direct.Nodes) > 0) {
		return &direct
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

func renderAllTrees(t *testing.T, trees *kube.Response, plain bool) string {
	t.Helper()

	var rendered []string
	for i := range trees.Trees {
		rendered = append(rendered, RenderTree(&trees.Trees[i], trees.Nodes, plain))
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
	if !regexp.MustCompile(`EXPIRED\s+\d+D\s+AGO`).MatchString(plain) {
		t.Fatalf("expected expired certificate status in plain render")
	}

	if !strings.Contains(colored, "\x1b[") {
		t.Fatalf("expected ansi color output in non-plain render")
	}
}
