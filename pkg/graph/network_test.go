package graph

import (
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestInferIngressBackendAndTLSCertificate(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"service/petshop/api": {
			Key:  "service/petshop/api",
			Type: "service",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "name": "api"},
			},
		},
		"certificate/petshop/tls-cert": {
			Key:  "certificate/petshop/tls-cert",
			Type: "certificate",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "name": "tls-cert"},
				"spec":     map[string]any{"secretName": "ingress-tls"},
			},
		},
	}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "web"},
		"spec": map[string]any{
			"tls": []any{map[string]any{"secretName": "ingress-tls"}},
			"rules": []any{map[string]any{
				"http": map[string]any{"paths": []any{map[string]any{
					"backend": map[string]any{"service": map[string]any{"name": "api"}},
				}}},
			}},
		},
	}}

	if err := inferIngress(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferIngress error: %v", err)
	}

	foundBackend := false
	foundTLS := false
	for _, e := range edges {
		if e.Source == "ingress/petshop/web" && e.Target == "service/petshop/api" && e.Label == "backend" {
			foundBackend = true
		}
		if e.Source == "ingress/petshop/web" && e.Target == "certificate/petshop/tls-cert" && e.Label == "tls" {
			foundTLS = true
		}
	}
	if !foundBackend || !foundTLS {
		t.Fatalf("expected backend and tls edges, got %#v", edges)
	}
}

func TestInferHTTPRouteBackendAndParentGateway(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"service/petshop/api": {Key: "service/petshop/api", Type: "service"},
		"gateway/petshop/gw":  {Key: "gateway/petshop/gw", Type: "gateway"},
	}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "route"},
		"spec": map[string]any{
			"rules":      []any{map[string]any{"backendRefs": []any{map[string]any{"name": "api"}}}},
			"parentRefs": []any{map[string]any{"kind": "Gateway", "name": "gw"}},
		},
	}}

	if err := inferHTTPRoute(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferHTTPRoute error: %v", err)
	}

	foundBackend := false
	foundRoute := false
	for _, e := range edges {
		if e.Source == "httproute/petshop/route" && e.Target == "service/petshop/api" && e.Label == "backend" {
			foundBackend = true
		}
		if e.Source == "gateway/petshop/gw" && e.Target == "httproute/petshop/route" && e.Label == "route" {
			foundRoute = true
		}
	}
	if !foundBackend || !foundRoute {
		t.Fatalf("expected backend and route edges, got %#v", edges)
	}
}

func TestInferGatewayTLSCertificateLink(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"certificate/petshop/gw-cert": {
			Key:  "certificate/petshop/gw-cert",
			Type: "certificate",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "name": "gw-cert"},
				"spec":     map[string]any{"secretName": "gw-secret"},
			},
		},
	}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "gw"},
		"spec": map[string]any{
			"listeners": []any{map[string]any{
				"tls": map[string]any{
					"certificateRefs": []any{map[string]any{"name": "gw-secret"}},
				},
			}},
		},
	}}

	if err := inferGateway(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferGateway error: %v", err)
	}

	found := false
	for _, e := range edges {
		if e.Source == "gateway/petshop/gw" && e.Target == "certificate/petshop/gw-cert" && e.Label == "tls" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected tls edge from gateway to certificate, got %#v", edges)
	}
}

func TestInferNetworkPolicyAppliesToPods(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"pod/petshop/api": {
			Key:  "pod/petshop/api",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "labels": map[string]any{"app": "api"}},
			},
		},
	}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "deny-all"},
		"spec":     map[string]any{"podSelector": map[string]any{"matchLabels": map[string]any{"app": "api"}}},
	}}

	if err := inferNetworkPolicy(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferNetworkPolicy error: %v", err)
	}

	found := false
	for _, e := range edges {
		if e.Source == "networkpolicy/petshop/deny-all" && e.Target == "pod/petshop/api" && e.Label == "applies-to" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected applies-to edge, got %#v", edges)
	}
}

func TestInferEndpointSlicesServiceAndPodLinks(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"service/petshop/api": {Key: "service/petshop/api", Type: "service"},
		"pod/petshop/api-0":   {Key: "pod/petshop/api-0", Type: "pod"},
	}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{
			"namespace": "petshop",
			"name":      "api-slice",
			"labels":    map[string]any{"kubernetes.io/service-name": "api"},
		},
		"endpoints": []any{map[string]any{"targetRef": map[string]any{"kind": "Pod", "name": "api-0"}}},
	}}

	if err := inferEndpointSlices(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferEndpointSlices error: %v", err)
	}

	foundSvc := false
	foundPod := false
	for _, e := range edges {
		if e.Source == "service/petshop/api" && e.Target == "endpointslice/petshop/api-slice" && e.Label == "routes-to" {
			foundSvc = true
		}
		if e.Source == "endpointslice/petshop/api-slice" && e.Target == "pod/petshop/api-0" && e.Label == "routes-to" {
			foundPod = true
		}
	}
	if !foundSvc || !foundPod {
		t.Fatalf("expected service->slice and slice->pod routes-to edges, got %#v", edges)
	}
}

func TestInferEndpointsServiceLink(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"service/petshop/api": {Key: "service/petshop/api", Type: "service"},
	}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "api"},
	}}

	if err := inferEndpoints(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferEndpoints error: %v", err)
	}
	found := false
	for _, e := range edges {
		if e.Source == "service/petshop/api" && e.Target == "endpoints/petshop/api" && e.Label == "routes-to" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected service routes-to endpoints edge, got %#v", edges)
	}
}

func TestInferCiliumNetworkPolicyAppliesToAndIngressEgressInference(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"pod/petshop/api-0": {
			Key:  "pod/petshop/api-0",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "labels": map[string]any{"app": "api"}},
			},
		},
		"service/petshop/api": {
			Key:  "service/petshop/api",
			Type: "service",
			Resource: map[string]any{
				"metadata": map[string]any{
					"namespace": "petshop",
					"labels":    map[string]any{"app": "api"},
				},
			},
		},
	}
	item := &kube.Resource{Key: "ciliumnetworkpolicy/petshop/allow-api", Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "allow-api"},
		"spec": map[string]any{
			"endpointSelector": map[string]any{"matchLabels": map[string]any{"app": "api"}},
			"ingress":          []any{map[string]any{"fromEndpoints": []any{map[string]any{"matchLabels": map[string]any{"app": "api"}}}}},
			"egress":           []any{map[string]any{"toEndpoints": []any{map[string]any{"matchLabels": map[string]any{"app": "api"}}}}},
		},
	}}

	if err := inferCiliumNetworkPolicy(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferCiliumNetworkPolicy error: %v", err)
	}

	foundApplies := false
	foundIngress := false
	foundEgress := false
	for _, e := range edges {
		if e.Source == item.Key && e.Target == "pod/petshop/api-0" && e.Label == "applies-to" {
			foundApplies = true
		}
		if e.Label == "inferred-ingress" && e.Target == "service/petshop/api" {
			foundIngress = true
		}
		if e.Label == "inferred-egress" && e.Target == "service/petshop/api" {
			foundEgress = true
		}
	}
	if !foundApplies || !foundIngress || !foundEgress {
		t.Fatalf("expected applies/inferred ingress/inferred egress edges, got %#v", edges)
	}
}

func TestInferCiliumNetworkPolicyAppliesToWithCiliumNamespaceSelector(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"pod/utv/api-0": {
			Key:  "pod/utv/api-0",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "utv", "labels": map[string]any{"app": "api"}},
			},
		},
		"pod/other/api-1": {
			Key:  "pod/other/api-1",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "other", "labels": map[string]any{"app": "api"}},
			},
		},
	}
	item := &kube.Resource{Key: "ciliumnetworkpolicy/utv/temp-egress", Resource: map[string]any{
		"metadata": map[string]any{"namespace": "utv", "name": "temp-egress"},
		"spec": map[string]any{
			"endpointSelector": map[string]any{"matchLabels": map[string]any{"k8s:io.kubernetes.pod.namespace": "utv"}},
			"egress":           []any{map[string]any{"toFQDNs": []any{map[string]any{"matchPattern": "**.utv.spk.no"}}}},
		},
	}}

	if err := inferCiliumNetworkPolicy(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferCiliumNetworkPolicy error: %v", err)
	}

	foundUTV := false
	foundOther := false
	for _, e := range edges {
		if e.Source == item.Key && e.Target == "pod/utv/api-0" && e.Label == "applies-to" {
			foundUTV = true
		}
		if e.Source == item.Key && e.Target == "pod/other/api-1" && e.Label == "applies-to" {
			foundOther = true
		}
	}
	if !foundUTV || foundOther {
		t.Fatalf("expected applies-to only for utv pod, got edges=%#v", edges)
	}
}

func TestInferCiliumNetworkPolicyEmptyMatchLabelsAppliesToNamespacePods(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"pod/petshop/api-0": {
			Key:  "pod/petshop/api-0",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "labels": map[string]any{"app": "api"}},
			},
		},
		"pod/management/api-1": {
			Key:  "pod/management/api-1",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "management", "labels": map[string]any{"app": "api"}},
			},
		},
	}
	item := &kube.Resource{Key: "ciliumnetworkpolicy/petshop/default", Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "default"},
		"spec": map[string]any{
			"endpointSelector": map[string]any{"matchLabels": map[string]any{}},
		},
	}}

	if err := inferCiliumNetworkPolicy(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferCiliumNetworkPolicy error: %v", err)
	}

	applies := 0
	for _, e := range edges {
		if e.Source == item.Key && e.Label == "applies-to" {
			applies++
			if e.Target != "pod/petshop/api-0" {
				t.Fatalf("unexpected applies-to target %q in edges=%#v", e.Target, edges)
			}
		}
	}
	if applies != 1 {
		t.Fatalf("expected one applies-to edge in namespace, got %d edges=%#v", applies, edges)
	}
}

func TestMatchesCiliumLabelsVariants(t *testing.T) {
	meta := map[string]any{
		"namespace": "petshop",
		"labels":    map[string]any{"app": "api", "team": "payments"},
	}
	if !matchesCiliumLabels(map[string]any{"app": "api"}, meta) {
		t.Fatalf("expected label match")
	}
	if !matchesCiliumLabels(map[string]any{"k8s:io.kubernetes.pod.namespace": "petshop"}, meta) {
		t.Fatalf("expected namespace-prefixed label match")
	}
	if matchesCiliumLabels(map[string]any{"app": "web"}, meta) {
		t.Fatalf("expected mismatch for wrong app")
	}
}

func TestInferCiliumClusterwideNetworkPolicyAppliesAcrossNamespaces(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"pod/petshop/api-0": {
			Key:  "pod/petshop/api-0",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "labels": map[string]any{"app": "api"}},
			},
		},
		"pod/management/api-1": {
			Key:  "pod/management/api-1",
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "management", "labels": map[string]any{"app": "api"}},
			},
		},
	}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"name": "allow-api-all-ns"},
		"spec":     map[string]any{"endpointSelector": map[string]any{"matchLabels": map[string]any{"app": "api"}}},
	}}

	if err := inferCiliumClusterwideNetworkPolicy(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferCiliumClusterwideNetworkPolicy error: %v", err)
	}

	policyKey := "ciliumclusterwidenetworkpolicy/allow-api-all-ns"
	if _, ok := nodes[policyKey]; !ok {
		t.Fatalf("expected clusterwide policy node to be added")
	}
	applies := 0
	for _, e := range edges {
		if e.Source == policyKey && e.Label == "applies-to" {
			applies++
		}
	}
	if applies != 2 {
		t.Fatalf("expected applies-to edges to two pods, got %d edges=%#v", applies, edges)
	}
}

func TestInferIngressClassLinksIngresses(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"ingress/petshop/web": {
			Key:  "ingress/petshop/web",
			Type: "ingress",
			Resource: map[string]any{
				"metadata": map[string]any{"namespace": "petshop", "name": "web"},
				"spec":     map[string]any{"ingressClassName": "nginx"},
			},
		},
	}
	item := &kube.Resource{Resource: map[string]any{"metadata": map[string]any{"name": "nginx"}}}

	if err := inferIngressClass(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferIngressClass error: %v", err)
	}
	if _, ok := nodes["ingressclass/nginx"]; !ok {
		t.Fatalf("expected ingressclass node to be added")
	}
	found := false
	for _, e := range edges {
		if e.Source == "ingress/petshop/web" && e.Target == "ingressclass/nginx" && e.Label == "class" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected class edge, got %#v", edges)
	}
}
