package graph

import (
	"strings"
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func netpolPod(namespace, name string, labels map[string]string) kube.Resource {
	labelMap := make(map[string]any, len(labels))
	for k, v := range labels {
		labelMap[k] = v
	}
	return kube.Resource{
		Key:  "pod/" + namespace + "/" + name,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{
				"namespace": namespace,
				"name":      name,
				"labels":    labelMap,
			},
		},
	}
}

func netpolPolicy(namespace, name string, podSelector map[string]any, ingress, egress []any, policyTypes []string) kube.Resource {
	spec := map[string]any{
		"podSelector": podSelector,
	}
	if ingress != nil {
		spec["ingress"] = ingress
	}
	if egress != nil {
		spec["egress"] = egress
	}
	if policyTypes != nil {
		ptAny := make([]any, len(policyTypes))
		for i, pt := range policyTypes {
			ptAny[i] = pt
		}
		spec["policyTypes"] = ptAny
	}
	return kube.Resource{
		Key:  "networkpolicy/" + namespace + "/" + name,
		Type: "networkpolicy",
		Resource: map[string]any{
			"metadata": map[string]any{"namespace": namespace, "name": name},
			"spec":     spec,
		},
	}
}

func TestNoNetworkPoliciesIngressAndEgressOpen(t *testing.T) {
	pod := netpolPod("petshop", "api", map[string]string{"app": "api"})
	nodes := map[string]kube.Resource{"pod/petshop/api": pod}

	v := AnalyzePodNetworkPolicies(nodes, pod)
	if v.IngressRestricted {
		t.Fatalf("expected ingress open when no policies, got restricted")
	}
	if v.EgressRestricted {
		t.Fatalf("expected egress open when no policies, got restricted")
	}

	text := FormatNetPolVerdict(v)
	if !strings.Contains(text, "INGRESS: OPEN") {
		t.Fatalf("expected INGRESS: OPEN in output, got:\n%s", text)
	}
	if !strings.Contains(text, "EGRESS: OPEN") {
		t.Fatalf("expected EGRESS: OPEN in output, got:\n%s", text)
	}
}

func TestPolicyMatchesOnlySelectedPods(t *testing.T) {
	podA := netpolPod("petshop", "api", map[string]string{"app": "api"})
	podB := netpolPod("petshop", "web", map[string]string{"app": "web"})
	policy := netpolPolicy("petshop", "deny-api-ingress",
		map[string]any{"matchLabels": map[string]any{"app": "api"}},
		[]any{},
		nil,
		[]string{"Ingress"},
	)
	nodes := map[string]kube.Resource{
		podA.Key:   podA,
		podB.Key:   podB,
		policy.Key: policy,
	}

	vA := AnalyzePodNetworkPolicies(nodes, podA)
	if !vA.IngressRestricted {
		t.Fatalf("expected api pod ingress restricted")
	}
	if vA.EgressRestricted {
		t.Fatalf("expected api pod egress open")
	}

	vB := AnalyzePodNetworkPolicies(nodes, podB)
	if vB.IngressRestricted {
		t.Fatalf("expected web pod ingress open (policy does not select it)")
	}
}

func TestEmptyPodSelectorMatchesAllPods(t *testing.T) {
	pod := netpolPod("petshop", "api", map[string]string{"app": "api"})
	policy := netpolPolicy("petshop", "deny-all",
		map[string]any{},
		[]any{},
		nil,
		[]string{"Ingress"},
	)
	nodes := map[string]kube.Resource{pod.Key: pod, policy.Key: policy}

	v := AnalyzePodNetworkPolicies(nodes, pod)
	if !v.IngressRestricted {
		t.Fatalf("expected ingress restricted (empty podSelector matches all)")
	}
}

func TestPolicyDoesNotApplyAcrossNamespaces(t *testing.T) {
	pod := netpolPod("petshop", "api", map[string]string{"app": "api"})
	policy := netpolPolicy("other", "deny-all",
		map[string]any{},
		[]any{},
		nil,
		[]string{"Ingress"},
	)
	nodes := map[string]kube.Resource{pod.Key: pod, policy.Key: policy}

	v := AnalyzePodNetworkPolicies(nodes, pod)
	if v.IngressRestricted {
		t.Fatalf("expected ingress open, policy is in different namespace")
	}
}

func TestIngressPolicyWithFromPodSelectorAndPort(t *testing.T) {
	pod := netpolPod("petshop", "api", map[string]string{"app": "api"})
	policy := netpolPolicy("petshop", "allow-frontend",
		map[string]any{"matchLabels": map[string]any{"app": "api"}},
		[]any{
			map[string]any{
				"from": []any{
					map[string]any{"podSelector": map[string]any{"matchLabels": map[string]any{"role": "frontend"}}},
				},
				"ports": []any{
					map[string]any{"protocol": "TCP", "port": float64(8080)},
				},
			},
		},
		nil,
		[]string{"Ingress"},
	)
	nodes := map[string]kube.Resource{pod.Key: pod, policy.Key: policy}

	v := AnalyzePodNetworkPolicies(nodes, pod)
	if !v.IngressRestricted {
		t.Fatalf("expected ingress restricted")
	}
	if len(v.IngressPolicies) != 1 {
		t.Fatalf("expected 1 ingress policy entry, got %d", len(v.IngressPolicies))
	}
	p := v.IngressPolicies[0]
	if p.PolicyName != "allow-frontend" {
		t.Fatalf("expected policy name allow-frontend, got %q", p.PolicyName)
	}
	if len(p.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(p.Rules))
	}
	r := p.Rules[0]
	if !strings.Contains(r.PeerDesc, "role=frontend") {
		t.Fatalf("expected peer desc to contain role=frontend, got %q", r.PeerDesc)
	}
	if len(r.Ports) != 1 || r.Ports[0] != "8080/TCP" {
		t.Fatalf("expected port 8080/TCP, got %v", r.Ports)
	}

	text := FormatNetPolVerdict(v)
	if !strings.Contains(text, "INGRESS: RESTRICTED") {
		t.Fatalf("expected INGRESS: RESTRICTED in output, got:\n%s", text)
	}
	if !strings.Contains(text, "allow-frontend") {
		t.Fatalf("expected policy name in output, got:\n%s", text)
	}
	if !strings.Contains(text, "role=frontend") {
		t.Fatalf("expected peer label in output, got:\n%s", text)
	}
	if !strings.Contains(text, "8080/TCP") {
		t.Fatalf("expected port in output, got:\n%s", text)
	}
	if !strings.Contains(text, "default deny") {
		t.Fatalf("expected default deny note in output, got:\n%s", text)
	}
}

func TestEgressOnlyPolicy(t *testing.T) {
	pod := netpolPod("petshop", "api", map[string]string{"app": "api"})
	policy := netpolPolicy("petshop", "allow-dns-egress",
		map[string]any{"matchLabels": map[string]any{"app": "api"}},
		nil,
		[]any{
			map[string]any{
				"to": []any{
					map[string]any{"podSelector": map[string]any{"matchLabels": map[string]any{"k8s-app": "kube-dns"}}},
				},
				"ports": []any{
					map[string]any{"protocol": "UDP", "port": float64(53)},
				},
			},
		},
		[]string{"Egress"},
	)
	nodes := map[string]kube.Resource{pod.Key: pod, policy.Key: policy}

	v := AnalyzePodNetworkPolicies(nodes, pod)
	if v.IngressRestricted {
		t.Fatalf("expected ingress open for egress-only policy")
	}
	if !v.EgressRestricted {
		t.Fatalf("expected egress restricted")
	}
	if len(v.EgressPolicies) != 1 || v.EgressPolicies[0].PolicyName != "allow-dns-egress" {
		t.Fatalf("expected egress policy allow-dns-egress, got %+v", v.EgressPolicies)
	}
	r := v.EgressPolicies[0].Rules[0]
	if !strings.Contains(r.PeerDesc, "k8s-app=kube-dns") {
		t.Fatalf("expected kube-dns peer in egress rule, got %q", r.PeerDesc)
	}
}

func TestMatchExpressionInSelector(t *testing.T) {
	pod := netpolPod("petshop", "api", map[string]string{"tier": "backend"})
	policy := netpolPolicy("petshop", "in-expr-policy",
		map[string]any{
			"matchExpressions": []any{
				map[string]any{"key": "tier", "operator": "In", "values": []any{"backend", "db"}},
			},
		},
		[]any{},
		nil,
		[]string{"Ingress"},
	)
	nodes := map[string]kube.Resource{pod.Key: pod, policy.Key: policy}

	v := AnalyzePodNetworkPolicies(nodes, pod)
	if !v.IngressRestricted {
		t.Fatalf("expected policy to match pod via matchExpressions In")
	}
}

func TestMatchExpressionNotInRejects(t *testing.T) {
	pod := netpolPod("petshop", "api", map[string]string{"tier": "backend"})
	policy := netpolPolicy("petshop", "notin-policy",
		map[string]any{
			"matchExpressions": []any{
				map[string]any{"key": "tier", "operator": "NotIn", "values": []any{"backend", "db"}},
			},
		},
		[]any{},
		nil,
		[]string{"Ingress"},
	)
	nodes := map[string]kube.Resource{pod.Key: pod, policy.Key: policy}

	v := AnalyzePodNetworkPolicies(nodes, pod)
	if v.IngressRestricted {
		t.Fatalf("expected policy NOT to match pod via matchExpressions NotIn")
	}
}

func TestIPBlockPeer(t *testing.T) {
	pod := netpolPod("petshop", "api", map[string]string{"app": "api"})
	policy := netpolPolicy("petshop", "allow-external",
		map[string]any{"matchLabels": map[string]any{"app": "api"}},
		[]any{
			map[string]any{
				"from": []any{
					map[string]any{
						"ipBlock": map[string]any{
							"cidr":   "192.168.0.0/16",
							"except": []any{"192.168.1.0/24"},
						},
					},
				},
			},
		},
		nil,
		[]string{"Ingress"},
	)
	nodes := map[string]kube.Resource{pod.Key: pod, policy.Key: policy}

	v := AnalyzePodNetworkPolicies(nodes, pod)
	if !v.IngressRestricted {
		t.Fatalf("expected ingress restricted")
	}
	text := FormatNetPolVerdict(v)
	if !strings.Contains(text, "cidr:192.168.0.0/16") {
		t.Fatalf("expected cidr in output, got:\n%s", text)
	}
	if !strings.Contains(text, "192.168.1.0/24") {
		t.Fatalf("expected except cidr in output, got:\n%s", text)
	}
}
