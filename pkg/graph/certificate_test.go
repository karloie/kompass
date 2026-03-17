package graph

import (
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestInferCertificateIssuedByIssuer(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"secret/petshop/api-cert": {Key: "secret/petshop/api-cert", Type: "secret"},
	}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "api-cert"},
		"spec": map[string]any{
			"secretName": "api-cert",
			"issuerRef": map[string]any{"kind": "Issuer", "name": "letsencrypt"},
		},
	}}

	if err := inferCertificate(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferCertificate error: %v", err)
	}
	if _, ok := nodes["certificate/petshop/api-cert"]; !ok {
		t.Fatalf("expected certificate node to be added")
	}

	found := false
	foundSecret := false
	for _, e := range edges {
		if e.Source == "certificate/petshop/api-cert" && e.Target == "issuer/petshop/letsencrypt" && e.Label == "issued-by" {
			found = true
		}
		if e.Source == "certificate/petshop/api-cert" && e.Target == "secret/petshop/api-cert" && e.Label == "stores" {
			foundSecret = true
		}
	}
	if !found {
		t.Fatalf("expected issued-by edge to issuer, got %#v", edges)
	}
	if !foundSecret {
		t.Fatalf("expected stores edge to backing secret, got %#v", edges)
	}
}

func TestInferCertificateIssuedByClusterIssuer(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "api-cert"},
		"spec": map[string]any{
			"issuerRef": map[string]any{"kind": "ClusterIssuer", "name": "letsencrypt"},
		},
	}}

	if err := inferCertificate(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferCertificate error: %v", err)
	}

	found := false
	for _, e := range edges {
		if e.Source == "certificate/petshop/api-cert" && e.Target == "clusterissuer/letsencrypt" && e.Label == "issued-by" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected issued-by edge to clusterissuer, got %#v", edges)
	}
}

func TestInferIssuerAddsRuleBasedEdges(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"secret/petshop/vault-token": {
			Key:  "secret/petshop/vault-token",
			Type: "secret",
		},
		"serviceaccount/petshop/issuer-sa": {
			Key:  "serviceaccount/petshop/issuer-sa",
			Type: "serviceaccount",
		},
	}
	item := &kube.Resource{
		Type: "issuer",
		Key:  "issuer/petshop/letsencrypt",
		Resource: map[string]any{
			"metadata": map[string]any{"namespace": "petshop", "name": "letsencrypt"},
			"spec": map[string]any{
				"vault": map[string]any{
					"auth": map[string]any{
						"kubernetes": map[string]any{
							"secretRef":         map[string]any{"name": "vault-token"},
							"serviceAccountRef": map[string]any{"name": "issuer-sa"},
						},
					},
				},
			},
		},
	}

	if err := inferIssuer(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferIssuer error: %v", err)
	}
	if _, ok := nodes["issuer/petshop/letsencrypt"]; !ok {
		t.Fatalf("expected issuer node to be added")
	}

	foundUses := false
	foundAuth := false
	for _, e := range edges {
		if e.Source == "issuer/petshop/letsencrypt" && e.Target == "secret/petshop/vault-token" && e.Label == "uses" {
			foundUses = true
		}
		if e.Source == "issuer/petshop/letsencrypt" && e.Target == "serviceaccount/petshop/issuer-sa" && e.Label == "authenticates-with" {
			foundAuth = true
		}
	}
	if !foundUses || !foundAuth {
		t.Fatalf("expected uses/authenticates-with edges, got %#v", edges)
	}
}

func TestInferClusterIssuerUsesSecretWithNamespacePriority(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"secret/cert-manager/acme-private": {
			Key:  "secret/cert-manager/acme-private",
			Type: "secret",
		},
		"secret/kube-system/acme-private": {
			Key:  "secret/kube-system/acme-private",
			Type: "secret",
		},
	}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"name": "letsencrypt"},
		"spec": map[string]any{
			"acme": map[string]any{
				"privateKeySecretRef": map[string]any{"name": "acme-private"},
			},
		},
	}}

	if err := inferClusterIssuer(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferClusterIssuer error: %v", err)
	}
	if _, ok := nodes["clusterissuer/letsencrypt"]; !ok {
		t.Fatalf("expected clusterissuer node to be added")
	}

	if len(edges) != 1 {
		t.Fatalf("expected exactly one uses edge, got %#v", edges)
	}
	e := edges[0]
	if e.Source != "clusterissuer/letsencrypt" || e.Target != "secret/cert-manager/acme-private" || e.Label != "uses" {
		t.Fatalf("unexpected edge: %#v", e)
	}
}

func TestInferClusterIssuerNoSecretNoEdge(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"name": "letsencrypt"},
		"spec": map[string]any{
			"acme": map[string]any{
				"privateKeySecretRef": map[string]any{"name": "missing-secret"},
			},
		},
	}}

	if err := inferClusterIssuer(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferClusterIssuer error: %v", err)
	}
	if len(edges) != 0 {
		t.Fatalf("expected no edges when secret is missing, got %#v", edges)
	}
}
