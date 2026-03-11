package graph

import (
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestExtractOwnerReferences(t *testing.T) {
	owners := extractOwnerReferences(M{
		"ownerReferences": []any{
			map[string]any{"kind": "ReplicaSet", "name": "api-rs"},
			"skip",
		},
	})
	if len(owners) != 1 {
		t.Fatalf("expected one parsed owner reference, got %#v", owners)
	}
	if owners[0]["kind"] != "ReplicaSet" || owners[0]["name"] != "api-rs" {
		t.Fatalf("unexpected owner reference: %#v", owners[0])
	}

	empty := extractOwnerReferences(nil)
	if len(empty) != 0 {
		t.Fatalf("expected no owners for nil meta, got %#v", empty)
	}
}

func TestInferWorkloadOwnerAddsManagedByEdge(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"pod/petshop/api-0": {
			Key:  "pod/petshop/api-0",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{
					"name":            "api-0",
					"ownerReferences": []any{map[string]any{"kind": "ReplicaSet", "name": "api-rs"}},
				},
			},
		},
	}
	item := &kube.Resource{Resource: map[string]any{"metadata": map[string]any{"name": "api-rs"}}}

	inferWorkloadOwner(&edges, item, &nodes, "replicaset/petshop/api-rs", "pod", "ReplicaSet")
	if len(edges) != 1 {
		t.Fatalf("expected one managed-by edge, got %#v", edges)
	}
	e := edges[0]
	if e.Source != "pod/petshop/api-0" || e.Target != "replicaset/petshop/api-rs" || e.Label != "managed-by" {
		t.Fatalf("unexpected edge: %#v", e)
	}
}

func TestInferWorkloadDeploymentManagedBy(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"replicaset/petshop/api-rs": {
			Key:  "replicaset/petshop/api-rs",
			Type: "replicaset",
			Resource: map[string]any{
				"metadata": map[string]any{
					"namespace":       "petshop",
					"name":            "api-rs",
					"ownerReferences": []any{map[string]any{"kind": "Deployment", "name": "api"}},
				},
			},
		},
	}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "api"},
	}}

	if err := inferWorkload("deployment")(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferWorkload deployment error: %v", err)
	}
	if _, ok := nodes["deployment/petshop/api"]; !ok {
		t.Fatalf("expected deployment node to be added")
	}

	found := false
	for _, e := range edges {
		if e.Source == "replicaset/petshop/api-rs" && e.Target == "deployment/petshop/api" && e.Label == "managed-by" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected replicaset managed-by deployment edge, got %#v", edges)
	}
}

func TestInferHorizontalPodAutoscalerScalesSupportedTargets(t *testing.T) {
	tests := []struct {
		name       string
		targetKind string
		wantTarget string
	}{
		{name: "deployment", targetKind: "Deployment", wantTarget: "deployment/petshop/api"},
		{name: "statefulset", targetKind: "StatefulSet", wantTarget: "statefulset/petshop/api"},
		{name: "daemonset", targetKind: "DaemonSet", wantTarget: "daemonset/petshop/api"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			edges := []kube.ResourceEdge{}
			nodes := map[string]kube.Resource{}
			item := &kube.Resource{Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "name": "api-hpa"},
				"spec": map[string]any{
					"scaleTargetRef": map[string]any{"kind": tc.targetKind, "name": "api"},
				},
			}}

			if err := inferHorizontalPodAutoscaler(&edges, item, &nodes, nil); err != nil {
				t.Fatalf("inferHorizontalPodAutoscaler error: %v", err)
			}
			if _, ok := nodes["horizontalpodautoscaler/petshop/api-hpa"]; !ok {
				t.Fatalf("expected hpa node to be added")
			}
			if len(edges) != 1 {
				t.Fatalf("expected one scales edge, got %#v", edges)
			}
			e := edges[0]
			if e.Source != "horizontalpodautoscaler/petshop/api-hpa" || e.Target != tc.wantTarget || e.Label != "scales" {
				t.Fatalf("unexpected scales edge: %#v", e)
			}
		})
	}
}

func TestInferHorizontalPodAutoscalerUnsupportedTargetNoEdge(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "api-hpa"},
		"spec": map[string]any{
			"scaleTargetRef": map[string]any{"kind": "Job", "name": "api"},
		},
	}}

	if err := inferHorizontalPodAutoscaler(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferHorizontalPodAutoscaler error: %v", err)
	}
	if len(edges) != 0 {
		t.Fatalf("expected no edges for unsupported target kind, got %#v", edges)
	}
}

func TestInferReplicaSetSelectorFallbackAddsSelectorMatch(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"pod/petshop/api-0": {
			Key:  "pod/petshop/api-0",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{
					"namespace": "petshop",
					"name":      "api-0",
					"labels":    map[string]any{"app": "api", "pod-template-hash": "abc"},
				},
			},
		},
		"pod/petshop/db-0": {
			Key:  "pod/petshop/db-0",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{
					"namespace": "petshop",
					"name":      "db-0",
					"labels":    map[string]any{"app": "db"},
				},
			},
		},
	}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "api-rs"},
		"spec": map[string]any{
			"selector": map[string]any{"matchLabels": map[string]any{"app": "api", "pod-template-hash": "xyz"}},
		},
	}}

	if err := inferReplicaSet(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferReplicaSet error: %v", err)
	}

	found := false
	for _, e := range edges {
		if e.Source == "pod/petshop/api-0" && e.Target == "replicaset/petshop/api-rs" && e.Label == "selector-match" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected selector-match edge from matching pod, got %#v", edges)
	}
}

func TestInferReplicaSetPrefersOwnerReferenceOverSelectorFallback(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"pod/petshop/api-0": {
			Key:  "pod/petshop/api-0",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{
					"namespace":       "petshop",
					"name":            "api-0",
					"ownerReferences": []any{map[string]any{"kind": "ReplicaSet", "name": "api-rs"}},
					"labels":          map[string]any{"app": "api"},
				},
			},
		},
	}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "api-rs"},
		"spec": map[string]any{
			"selector": map[string]any{"matchLabels": map[string]any{"app": "api"}},
		},
	}}

	if err := inferReplicaSet(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferReplicaSet error: %v", err)
	}

	managedBy := 0
	selectorMatch := 0
	for _, e := range edges {
		if e.Source == "pod/petshop/api-0" && e.Target == "replicaset/petshop/api-rs" && e.Label == "managed-by" {
			managedBy++
		}
		if e.Source == "pod/petshop/api-0" && e.Target == "replicaset/petshop/api-rs" && e.Label == "selector-match" {
			selectorMatch++
		}
	}
	if managedBy != 1 || selectorMatch != 0 {
		t.Fatalf("expected managed-by only when owner reference exists, edges=%#v", edges)
	}
}

func TestSelectorsMatch(t *testing.T) {
	if !selectorsMatch(map[string]any{"app": "api"}, map[string]any{"app": "api", "tier": "backend"}) {
		t.Fatalf("expected selectorsMatch to be true for subset selector")
	}
	if selectorsMatch(map[string]any{"app": "api"}, map[string]any{"app": "web"}) {
		t.Fatalf("expected selectorsMatch false for mismatched value")
	}
}

func TestInferPodDisruptionBudgetProtectedByWorkloads(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"deployment/petshop/api": {
			Key:  "deployment/petshop/api",
			Type: "deployment",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "name": "api"},
				"spec": map[string]any{
					"selector": map[string]any{"matchLabels": map[string]any{"app": "api"}},
				},
			},
		},
		"statefulset/petshop/db": {
			Key:  "statefulset/petshop/db",
			Type: "statefulset",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "name": "db"},
				"spec": map[string]any{
					"selector": map[string]any{"matchLabels": map[string]any{"app": "db"}},
				},
			},
		},
	}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "api-pdb"},
		"spec": map[string]any{
			"selector": map[string]any{"matchLabels": map[string]any{"app": "api"}},
		},
	}}

	if err := inferPodDisruptionBudget(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferPodDisruptionBudget error: %v", err)
	}
	if _, ok := nodes["poddisruptionbudget/petshop/api-pdb"]; !ok {
		t.Fatalf("expected poddisruptionbudget node to be added")
	}

	protectedBy := 0
	for _, e := range edges {
		if e.Target == "poddisruptionbudget/petshop/api-pdb" && e.Label == "protected-by" {
			if e.Source == "deployment/petshop/api" {
				protectedBy++
			}
			if e.Source == "statefulset/petshop/db" {
				t.Fatalf("unexpected protected-by edge from non-matching workload: %#v", e)
			}
		}
	}
	if protectedBy != 1 {
		t.Fatalf("expected one protected-by edge from deployment, edges=%#v", edges)
	}
}
