package graph

import (
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestInferSubjectFromResourceAndMeta(t *testing.T) {
	subjects := []any{map[string]any{"kind": "ServiceAccount", "name": "default", "namespace": "petshop"}}

	fromResource := inferSubject(map[string]any{"subjects": subjects}, map[string]any{})
	if len(fromResource) != 1 {
		t.Fatalf("expected subjects from resource, got %#v", fromResource)
	}

	fromMeta := inferSubject(map[string]any{}, map[string]any{"subjects": subjects})
	if len(fromMeta) != 1 {
		t.Fatalf("expected subjects from meta, got %#v", fromMeta)
	}

	none := inferSubject(map[string]any{}, map[string]any{})
	if none != nil {
		t.Fatalf("expected nil subjects when absent, got %#v", none)
	}
}

func TestInferServiceAccountsAddsBoundByEdges(t *testing.T) {
	edges := []kube.ResourceEdge{}
	subjects := []any{
		map[string]any{"kind": "ServiceAccount", "name": "default", "namespace": "petshop"},
		map[string]any{"kind": "User", "name": "alice"},
	}

	inferServiceAccounts(&edges, subjects, "rolebinding/petshop/readers")
	if len(edges) != 1 {
		t.Fatalf("expected one edge for service account, got %#v", edges)
	}
	if edges[0].Source != "serviceaccount/petshop/default" || edges[0].Label != "bound-by" {
		t.Fatalf("unexpected edge: %#v", edges[0])
	}
}

func TestInferServiceAccountLinksPods(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"pod/petshop/api": {
			Key:  "pod/petshop/api",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "name": "api"},
				"spec":     map[string]any{"serviceAccountName": "default"},
			},
		},
	}
	item := &kube.Resource{Resource: map[string]any{"metadata": map[string]any{"namespace": "petshop", "name": "default"}}}

	if err := inferServiceAccount(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferServiceAccount error: %v", err)
	}
	if _, ok := nodes["serviceaccount/petshop/default"]; !ok {
		t.Fatalf("expected serviceaccount node to be added")
	}

	foundUses := false
	for _, e := range edges {
		if e.Source == "pod/petshop/api" && e.Target == "serviceaccount/petshop/default" && e.Label == "uses" {
			foundUses = true
			break
		}
	}
	if !foundUses {
		t.Fatalf("expected pod uses serviceaccount edge, got %#v", edges)
	}
}

func TestInferRoleAddsNodeAndSubjectEdges(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "reader"},
		"subjects": []any{map[string]any{"kind": "ServiceAccount", "name": "default", "namespace": "petshop"}},
	}}

	if err := inferRole(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferRole error: %v", err)
	}
	if _, ok := nodes["role/petshop/reader"]; !ok {
		t.Fatalf("expected role node to be added")
	}

	found := false
	for _, e := range edges {
		if e.Source == "serviceaccount/petshop/default" && e.Target == "role/petshop/reader" && e.Label == "bound-by" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected role bound-by edge, got %#v", edges)
	}
}

func TestInferClusterRoleAddsGrantedByEdge(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"clusterrolebinding/readers": {
			Key:  "clusterrolebinding/readers",
			Type: "clusterrolebinding",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "readers"},
				"roleRef":  map[string]any{"name": "reader"},
			},
		},
	}
	item := &kube.Resource{Resource: map[string]any{"metadata": map[string]any{"name": "reader"}}}

	if err := inferClusterRole(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferClusterRole error: %v", err)
	}
	if _, ok := nodes["clusterrole/reader"]; !ok {
		t.Fatalf("expected clusterrole node to be added")
	}

	found := false
	for _, e := range edges {
		if e.Source == "clusterrole/reader" && e.Target == "clusterrolebinding/readers" && e.Label == "granted-by" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected granted-by edge, got %#v", edges)
	}
}

func TestInferBindingRoleNamespaceAndSubjects(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "reader-binding"},
		"roleRef":  map[string]any{"name": "reader"},
		"subjects": []any{map[string]any{"kind": "ServiceAccount", "name": "default", "namespace": "petshop"}},
	}}

	if err := inferBinding(&edges, item, &nodes, nil, "rolebinding", "role"); err != nil {
		t.Fatalf("inferBinding error: %v", err)
	}
	if _, ok := nodes["rolebinding/petshop/reader-binding"]; !ok {
		t.Fatalf("expected rolebinding node to be added")
	}

	foundGrants := false
	foundBoundBy := false
	for _, e := range edges {
		if e.Source == "rolebinding/petshop/reader-binding" && e.Target == "role/petshop/reader" && e.Label == "grants" {
			foundGrants = true
		}
		if e.Source == "serviceaccount/petshop/default" && e.Target == "rolebinding/petshop/reader-binding" && e.Label == "bound-by" {
			foundBoundBy = true
		}
	}
	if !foundGrants || !foundBoundBy {
		t.Fatalf("expected grants and bound-by edges, got %#v", edges)
	}
}

func TestInferBindingClusterRoleRefWithoutNamespace(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"name": "cluster-reader-binding"},
		"roleRef":  map[string]any{"name": "cluster-reader"},
	}}

	if err := inferBinding(&edges, item, &nodes, nil, "clusterrolebinding", "clusterrole"); err != nil {
		t.Fatalf("inferBinding error: %v", err)
	}

	found := false
	for _, e := range edges {
		if e.Source == "clusterrolebinding/cluster-reader-binding" && e.Target == "clusterrole/cluster-reader" && e.Label == "grants" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected clusterrole grants edge, got %#v", edges)
	}
}
