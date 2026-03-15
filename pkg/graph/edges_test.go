package graph

import (
	"context"
	"sort"
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsWorkloadType(t *testing.T) {
	if !isWorkloadType("deployment") {
		t.Fatalf("deployment should be workload type")
	}
	if !isWorkloadType("pod") {
		t.Fatalf("pod should be workload type")
	}
	if isWorkloadType("service") {
		t.Fatalf("service should not be workload type")
	}
}

func TestFindWorkloadRootFollowsPodReplicaSetDeployment(t *testing.T) {
	nodeMap := map[string]kube.Resource{
		"pod/petshop/api-0": {
			Key:  "pod/petshop/api-0",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{
					"namespace":       "petshop",
					"name":            "api-0",
					"ownerReferences": []any{map[string]any{"kind": "ReplicaSet", "name": "api-rs"}},
				},
			},
		},
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
		"deployment/petshop/api": {
			Key:  "deployment/petshop/api",
			Type: "deployment",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "name": "api"},
			},
		},
	}

	root := findWorkloadRoot("pod/petshop/api-0", "pod", nodeMap)
	if root != "deployment/petshop/api" {
		t.Fatalf("expected deployment root, got %q", root)
	}
}

func TestFindWorkloadRootFallbackCases(t *testing.T) {
	nodeMap := map[string]kube.Resource{
		"pod/petshop/orphan": {
			Key:      "pod/petshop/orphan",
			Type:     "pod",
			Resource: map[string]any{},
		},
	}

	if got := findWorkloadRoot("pod/petshop/orphan", "pod", nodeMap); got != "pod/petshop/orphan" {
		t.Fatalf("expected pod fallback for missing metadata, got %q", got)
	}
	if got := findWorkloadRoot("service/petshop/api", "service", nodeMap); got != "" {
		t.Fatalf("expected empty root for non-workload lookup, got %q", got)
	}
	if got := findWorkloadRoot("pod-only", "pod", nodeMap); got != "" {
		t.Fatalf("expected empty root for malformed key, got %q", got)
	}
}

func TestBuildComponentSortsNodeKeys(t *testing.T) {
	visited := map[string]bool{
		"service/petshop/api": true,
		"pod/petshop/api-0":   true,
	}

	component := buildComponent("pod/petshop/api-0", visited)
	if len(component.NodeKeys) != 2 {
		t.Fatalf("expected two node keys, got %#v", component.NodeKeys)
	}
	if component.NodeKeys[0] != "pod/petshop/api-0" {
		t.Fatalf("expected node keys sorted, got %#v", component.NodeKeys)
	}
}

func TestBuildGraphsWorkloadAndInferredOrdering(t *testing.T) {
	nodeMap := map[string]kube.Resource{
		"pod/petshop/api-0": {
			Key:  "pod/petshop/api-0",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "name": "api-0"},
			},
		},
		"service/petshop/api": {
			Key:  "service/petshop/api",
			Type: "service",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "name": "api"},
			},
		},
		"gateway/petshop/gw": {
			Key:  "gateway/petshop/gw",
			Type: "gateway",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "name": "gw"},
			},
		},
		"certificate/petshop/gw-cert": {
			Key:  "certificate/petshop/gw-cert",
			Type: "certificate",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "name": "gw-cert"},
			},
		},
	}
	edges := []kube.ResourceEdge{
		{Source: "pod/petshop/api-0", Target: "service/petshop/api", Label: "served-by"},
		{Source: "gateway/petshop/gw", Target: "certificate/petshop/gw-cert", Label: "tls"},
	}

	resp := buildGraphs([]string{"pod/petshop/api-0", "service/petshop/api"}, edges, nodeMap)
	if len(resp.Components) != 2 {
		t.Fatalf("expected workload and inferred component, got %#v", resp.Components)
	}
	if resp.Components[0].Root != "pod/petshop/api-0" || resp.Components[1].Root != "gateway/petshop/gw" {
		t.Fatalf("unexpected component order: %#v", []string{resp.Components[0].Root, resp.Components[1].Root})
	}

	if resp.Node("pod/petshop/api-0").Discovered {
		t.Fatalf("matched pod should not be marked discovered")
	}
	if !resp.Node("gateway/petshop/gw").Discovered || !resp.Node("certificate/petshop/gw-cert").Discovered {
		t.Fatalf("inferred nodes should be marked discovered")
	}
}

func TestInferGraphsLoadsCertificateNamespacesAndClusterIssuer(t *testing.T) {
	original := ResourceTypes
	t.Cleanup(func() { ResourceTypes = original })
	t.Setenv("KOMPASS_CERT_NAMESPACES", "extra-ns")

	callCount := map[string]int{}
	loadedCertNS := []string{}
	loadedIssuerNS := []string{}
	mk := func(key, typ, ns, name string) kube.Resource {
		return kube.Resource{Key: key, Type: typ, Resource: map[string]any{"metadata": map[string]any{"namespace": ns, "name": name}}}
	}

	ResourceTypes = map[string]ResourceType{
		"pod": {
			Loader: func(_ kube.Kube, ns string, _ context.Context, _ metav1.ListOptions) ([]kube.Resource, error) {
				callCount["pod"]++
				if ns == "petshop" {
					return []kube.Resource{mk("pod/petshop/api-0", "pod", "petshop", "api-0")}, nil
				}
				return nil, nil
			},
		},
		"ingress": {
			Loader: func(_ kube.Kube, ns string, _ context.Context, _ metav1.ListOptions) ([]kube.Resource, error) {
				callCount["ingress"]++
				if ns == "petshop" {
					return []kube.Resource{mk("ingress/petshop/web", "ingress", "petshop", "web")}, nil
				}
				return nil, nil
			},
		},
		"certificate": {
			Loader: func(_ kube.Kube, ns string, _ context.Context, _ metav1.ListOptions) ([]kube.Resource, error) {
				callCount["certificate"]++
				loadedCertNS = append(loadedCertNS, ns)
				if ns == "extra-ns" {
					cert := mk("certificate/extra-ns/web-cert", "certificate", "extra-ns", "web-cert")
					certMap, _ := cert.Resource.(map[string]any)
					certMap["spec"] = map[string]any{
						"issuerRef": map[string]any{"kind": "Issuer", "name": "shared-issuer"},
					}
					cert.Resource = certMap
					return []kube.Resource{cert}, nil
				}
				return nil, nil
			},
		},
		"issuer": {
			Loader: func(_ kube.Kube, ns string, _ context.Context, _ metav1.ListOptions) ([]kube.Resource, error) {
				callCount["issuer"]++
				loadedIssuerNS = append(loadedIssuerNS, ns)
				if ns == "extra-ns" {
					return []kube.Resource{mk("issuer/extra-ns/shared-issuer", "issuer", "extra-ns", "shared-issuer")}, nil
				}
				return nil, nil
			},
		},
		"clusterissuer": {
			Loader: func(_ kube.Kube, ns string, _ context.Context, _ metav1.ListOptions) ([]kube.Resource, error) {
				callCount["clusterissuer"]++
				if ns == "" {
					return []kube.Resource{{Key: "clusterissuer/letsencrypt", Type: "clusterissuer", Resource: map[string]any{"metadata": map[string]any{"name": "letsencrypt"}}}}, nil
				}
				return nil, nil
			},
		},
	}

	provider := kube.NewMockClient(mock.GenerateMock())
	resp, err := InferGraphs(provider, kube.Request{Selectors: []string{"pod/petshop/*"}})
	if err != nil {
		t.Fatalf("InferGraphs error: %v", err)
	}

	if callCount["pod"] == 0 || callCount["ingress"] == 0 {
		t.Fatalf("expected base loaders to run, calls=%#v", callCount)
	}
	if callCount["certificate"] == 0 || callCount["clusterissuer"] == 0 {
		t.Fatalf("expected cert/clusterissuer inferred loaders to run, calls=%#v", callCount)
	}
	if callCount["issuer"] == 0 {
		t.Fatalf("expected issuer inferred loader to run, calls=%#v", callCount)
	}
	sort.Strings(loadedCertNS)
	if len(loadedCertNS) == 0 || loadedCertNS[0] != "extra-ns" {
		t.Fatalf("expected certificate loader to include extra-ns, loaded=%#v", loadedCertNS)
	}
	sort.Strings(loadedIssuerNS)
	if len(loadedIssuerNS) == 0 || loadedIssuerNS[0] != "extra-ns" {
		t.Fatalf("expected issuer loader to include extra-ns, loaded=%#v", loadedIssuerNS)
	}

	if resp.Node("certificate/extra-ns/web-cert") == nil {
		t.Fatalf("expected inferred certificate node in response")
	}
	if resp.Node("issuer/extra-ns/shared-issuer") == nil {
		t.Fatalf("expected inferred issuer node in response")
	}
	foundPodGraph := false
	for _, component := range resp.Components {
		if component.Root == "pod/petshop/api-0" {
			foundPodGraph = true
			break
		}
	}
	if !foundPodGraph {
		t.Fatalf("expected workload component rooted at selected pod, got %#v", resp.Components)
	}
}
