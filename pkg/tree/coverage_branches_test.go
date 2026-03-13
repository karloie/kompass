package tree

import (
	"strings"
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestProbeStatusStyle(t *testing.T) {
	tests := []struct {
		in       string
		wantText string
		wantGood bool
	}{
		{in: "ready", wantText: "READY", wantGood: true},
		{in: "not-ready", wantText: "NOT READY", wantGood: false},
		{in: "started", wantText: "STARTED", wantGood: true},
		{in: "not-started", wantText: "NOT STARTED", wantGood: false},
		{in: "passing", wantText: "PASSING", wantGood: true},
		{in: "failed", wantText: "FAILED", wantGood: false},
		{in: "weird", wantText: "WEIRD", wantGood: false},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			gotText, gotGood := probeStatusStyle(tc.in)
			if gotText != tc.wantText || gotGood != tc.wantGood {
				t.Fatalf("status=%q got (%q,%v), want (%q,%v)", tc.in, gotText, gotGood, tc.wantText, tc.wantGood)
			}
		})
	}
}

func TestBuildContainerNode_LegacyPathWithProbes(t *testing.T) {
	containerSpec := map[string]any{
		"name": "app",
		"livenessProbe": map[string]any{
			"httpGet": map[string]any{"path": "/live", "port": 8080},
		},
		"readinessProbe": map[string]any{
			"httpGet": map[string]any{"path": "/ready", "port": 8080},
		},
		"startupProbe": map[string]any{
			"exec": map[string]any{"command": []any{"/bin/true"}},
		},
	}

	containerStatus := map[string]any{
		"ready":   true,
		"started": true,
		"state": map[string]any{
			"running": map[string]any{"startedAt": "2026-03-12T10:00:00Z"},
		},
	}

	node := buildContainerNode(
		"pod/petshop/legacy-probe-pod",
		"petshop",
		0,
		containerSpec,
		containerStatus,
		nil,
		map[string][]string{},
		newTreeBuildState(),
		map[string]kube.Resource{},
	)
	if node == nil {
		t.Fatalf("expected container node")
	}

	got := RenderTree(node, map[string]*kube.Resource{}, true)
	if !strings.Contains(got, "livenessprobe") || !strings.Contains(got, "PASSING") {
		t.Fatalf("expected liveness probe with PASSING status, got: %s", got)
	}
	if !strings.Contains(got, "readinessprobe") || !strings.Contains(got, "READY") {
		t.Fatalf("expected readiness probe with READY status, got: %s", got)
	}
	if !strings.Contains(got, "startupprobe") || !strings.Contains(got, "STARTED") {
		t.Fatalf("expected startup probe with STARTED status, got: %s", got)
	}
}

func TestApplyMetadataRules_NodeAndHPA(t *testing.T) {
	nodeRes := kube.Resource{
		Type: "node",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "psb-boys-01"},
			"status": map[string]any{
				"allocatable": map[string]any{"cpu": "8", "memory": "32Gi"},
				"nodeInfo": map[string]any{
					"osImage":        "Ubuntu 24.04 LTS",
					"kernelVersion":  "6.8.0-40-generic",
					"kubeletVersion": "v1.32.3",
				},
				"conditions": []any{map[string]any{"type": "Ready", "status": "True"}},
			},
		},
	}
	nodeMeta := ApplyMetadataRules(nodeRes, nil)
	if nodeMeta["conditions"] != "Ready" {
		t.Fatalf("expected node Ready condition, got %#v", nodeMeta)
	}
	if _, ok := nodeMeta["nodeInfo"].(map[string]any); !ok {
		t.Fatalf("expected nodeInfo metadata map, got %#v", nodeMeta)
	}

	hpaRes := kube.Resource{
		Type: "horizontalpodautoscaler",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "petshop-boys-motor-hpa", "namespace": "petshop"},
			"spec":     map[string]any{"minReplicas": float64(2), "maxReplicas": float64(6)},
			"status":   map[string]any{"currentReplicas": float64(3), "desiredReplicas": float64(4)},
		},
	}
	hpaMeta := ApplyMetadataRules(hpaRes, nil)
	if _, ok := hpaMeta["hpaConfig"].(map[string]any); !ok {
		t.Fatalf("expected hpaConfig metadata map, got %#v", hpaMeta)
	}
	if _, ok := hpaMeta["hpaStatus"].(map[string]any); !ok {
		t.Fatalf("expected hpaStatus metadata map, got %#v", hpaMeta)
	}
}
