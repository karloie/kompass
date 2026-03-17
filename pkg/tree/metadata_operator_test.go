package tree

import (
	"strings"
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestExtractMetadataFromResource_DetectsOperatorWorkload(t *testing.T) {
	resource := kube.Resource{
		Type: "deployment",
		Resource: map[string]any{
			"metadata": map[string]any{
				"name":      "prometheus-operator",
				"namespace": "monitoring",
				"labels": map[string]any{
					"app.kubernetes.io/component": "operator",
					"app.kubernetes.io/name":      "prometheus-operator",
				},
			},
			"spec": map[string]any{
				"replicas": 1,
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{"image": "quay.io/prometheus-operator/prometheus-operator:v0.83.0"},
						},
					},
				},
			},
		},
	}

	meta := extractMetadataFromResource(resource, nil)

	if got, _ := meta["displayPrefix"].(string); got != "operator" {
		t.Fatalf("expected displayPrefix=operator, got %#v", meta["displayPrefix"])
	}
	if _, exists := meta["operator"]; exists {
		t.Fatalf("did not expect operator=true metadata, got %#v", meta["operator"])
	}
	if _, exists := meta["operatorConfidence"]; exists {
		t.Fatalf("did not expect operatorConfidence heuristic metadata, got %#v", meta["operatorConfidence"])
	}
	if _, exists := meta["operatorSignals"]; exists {
		t.Fatalf("did not expect operatorSignals heuristic metadata, got %#v", meta["operatorSignals"])
	}
}

func TestExtractMetadataFromResource_DetectsOLMOperator(t *testing.T) {
	resource := kube.Resource{
		Type: "deployment",
		Resource: map[string]any{
			"metadata": map[string]any{
				"name": "my-controller",
				"labels": map[string]any{
					"operators.coreos.com/my-operator.my-namespace": "",
					"olm.owner.kind": "ClusterServiceVersion",
				},
			},
		},
	}

	meta := extractMetadataFromResource(resource, nil)

	if got, _ := meta["displayPrefix"].(string); got != "operator" {
		t.Fatalf("expected displayPrefix=operator, got %#v", meta["displayPrefix"])
	}
	if got, _ := meta["operatorFramework"].(string); got != "olm" {
		t.Fatalf("expected operatorFramework=olm, got %#v", meta["operatorFramework"])
	}
}

func TestExtractMetadataFromResource_DoesNotTagRegularDeploymentAsOperator(t *testing.T) {
	resource := kube.Resource{
		Type: "deployment",
		Resource: map[string]any{
			"metadata": map[string]any{
				"name": "payments-api",
				"labels": map[string]any{
					"app.kubernetes.io/name": "payments-api",
				},
			},
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{"image": "ghcr.io/example/payments:1.2.3"},
						},
					},
				},
			},
		},
	}

	meta := extractMetadataFromResource(resource, nil)

	if _, exists := meta["displayPrefix"]; exists {
		t.Fatalf("did not expect displayPrefix for regular deployment, got %#v", meta["displayPrefix"])
	}
	if _, exists := meta["operator"]; exists {
		t.Fatalf("did not expect operator metadata for regular deployment, got %#v", meta["operator"])
	}
}

func TestFormatNodeName_AddsOperatorBadge(t *testing.T) {
	display := formatNodeName("deployment", map[string]any{
		"name":          "prometheus-operator",
		"displayPrefix": "operator",
	}, nil, true, nil)

	if !strings.Contains(display, "[OPERATOR]") {
		t.Fatalf("expected operator badge in display, got %q", display)
	}
}
