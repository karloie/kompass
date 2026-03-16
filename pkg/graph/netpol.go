package graph

import (
	"fmt"
	"sort"
	"strings"

	kube "github.com/karloie/kompass/pkg/kube"
)

// NetPolVerdict holds the ingress/egress network policy analysis for one pod.
type NetPolVerdict struct {
	PodName           string
	Namespace         string
	Labels            map[string]string
	IngressRestricted bool
	EgressRestricted  bool
	IngressPolicies   []NetPolPolicyEntry
	EgressPolicies    []NetPolPolicyEntry
}

// NetPolPolicyEntry is one NetworkPolicy that applies to the pod in one direction.
type NetPolPolicyEntry struct {
	PolicyName string
	Rules      []NetPolPeerRule
}

// NetPolPeerRule is one from/to block within a NetworkPolicy rule.
type NetPolPeerRule struct {
	PeerDesc string
	Ports    []string
}

// AnalyzePodNetworkPolicies evaluates all NetworkPolicy objects in nodes that apply
// to pod, and returns a verdict describing what ingress/egress is restricted.
func AnalyzePodNetworkPolicies(nodes map[string]kube.Resource, pod kube.Resource) NetPolVerdict {
	meta := M(pod.AsMap()).Map("metadata")
	v := NetPolVerdict{
		PodName:   meta.String("name"),
		Namespace: meta.String("namespace"),
		Labels:    extractStringMap(M(pod.AsMap()).Path("metadata", "labels")),
	}

	for _, node := range nodes {
		if node.Type != "networkpolicy" {
			continue
		}
		npMeta := M(node.AsMap()).Map("metadata")
		if npMeta.String("namespace") != v.Namespace {
			continue
		}
		spec := M(node.AsMap()).Map("spec")
		if spec == nil {
			continue
		}

		// Check whether this policy selects our pod.
		podSel := spec.Map("podSelector")
		if !podLabelSelectorMatches(podSel, v.Labels) {
			continue
		}

		policyName := npMeta.String("name")
		policyTypes := extractPolicyTypes(spec)

		if policyTypes["Ingress"] {
			v.IngressRestricted = true
			entry := NetPolPolicyEntry{PolicyName: policyName}
			for _, rule := range spec.MapSlice("ingress") {
				entry.Rules = append(entry.Rules,
					parsePeerRules(rule.MapSlice("from"), rule.MapSlice("ports"))...)
			}
			v.IngressPolicies = append(v.IngressPolicies, entry)
		}
		if policyTypes["Egress"] {
			v.EgressRestricted = true
			entry := NetPolPolicyEntry{PolicyName: policyName}
			for _, rule := range spec.MapSlice("egress") {
				entry.Rules = append(entry.Rules,
					parsePeerRules(rule.MapSlice("to"), rule.MapSlice("ports"))...)
			}
			v.EgressPolicies = append(v.EgressPolicies, entry)
		}
	}
	return v
}

// FormatNetPolVerdict renders a verdict as a human-readable multi-line string.
func FormatNetPolVerdict(v NetPolVerdict) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("NetworkPolicy analysis: %s/%s\n", v.Namespace, v.PodName))
	if len(v.Labels) > 0 {
		sb.WriteString("Labels: " + strings.Join(sortedLabels(v.Labels), ", ") + "\n")
	}

	sb.WriteString("\n")
	writeDirectionBlock(&sb, "INGRESS", v.IngressRestricted, v.IngressPolicies, "ingress")
	sb.WriteString("\n")
	writeDirectionBlock(&sb, "EGRESS", v.EgressRestricted, v.EgressPolicies, "egress")

	return strings.TrimRight(sb.String(), "\n")
}

// ─── internal helpers ─────────────────────────────────────────────────────────

func writeDirectionBlock(sb *strings.Builder, dir string, restricted bool, policies []NetPolPolicyEntry, noun string) {
	if !restricted {
		sb.WriteString(dir + ": OPEN\n")
		sb.WriteString("  Result: all " + noun + " traffic is allowed.\n")
		sb.WriteString("  Reason: no NetworkPolicy currently restricts " + noun + " for this pod.\n")
		return
	}
	n := len(policies)
	sb.WriteString(fmt.Sprintf("%s: RESTRICTED  (%d %s)\n", dir, n, policyWord(n)))
	sb.WriteString("  Result: only traffic matching the allow rules below is permitted.\n")
	sb.WriteString("  Anything not matching those rules is denied by default.\n")
	for _, p := range policies {
		sb.WriteString("\n  ▸ " + p.PolicyName + "\n")
		if len(p.Rules) == 0 {
			sb.WriteString("      No allow rules are defined here, so this policy allows no " + noun + " traffic.\n")
		}
		for _, r := range p.Rules {
			line := "      ✅ " + r.PeerDesc
			if len(r.Ports) == 1 {
				line += "  (port " + r.Ports[0] + ")"
			} else if len(r.Ports) > 1 {
				line += "  (ports " + strings.Join(r.Ports, ", ") + ")"
			}
			sb.WriteString(line + "\n")
		}
	}
	sb.WriteString("\n  🚫 default deny: all other " + noun + " traffic is blocked\n")
}

// podLabelSelectorMatches returns true when an empty/nil selector (= match all) or when
// matchLabels + matchExpressions both match the given labels.
func podLabelSelectorMatches(sel M, labels map[string]string) bool {
	if sel == nil {
		return true
	}

	// matchLabels
	if ml, ok := sel["matchLabels"].(map[string]any); ok {
		for k, v := range ml {
			want, _ := v.(string)
			if labels[k] != want {
				return false
			}
		}
	}

	// matchExpressions
	exprs, _ := sel["matchExpressions"].([]any)
	for _, exprAny := range exprs {
		expr, _ := exprAny.(map[string]any)
		if expr == nil {
			continue
		}
		key, _ := expr["key"].(string)
		op, _ := expr["operator"].(string)
		var vals []string
		for _, vi := range toStringSlice(expr["values"]) {
			vals = append(vals, vi)
		}
		labelVal, has := labels[key]
		switch op {
		case "In":
			if !stringIn(labelVal, vals) {
				return false
			}
		case "NotIn":
			if stringIn(labelVal, vals) {
				return false
			}
		case "Exists":
			if !has {
				return false
			}
		case "DoesNotExist":
			if has {
				return false
			}
		}
	}
	return true
}

// extractPolicyTypes reads policyTypes from spec, inferring presence from ingress/egress fields
// when the explicit array is absent (Kubernetes does this too).
func extractPolicyTypes(spec M) map[string]bool {
	types := map[string]bool{}
	if pts, ok := spec["policyTypes"].([]any); ok {
		for _, pt := range pts {
			if s, ok := pt.(string); ok {
				types[s] = true
			}
		}
		return types
	}
	// infer from presence of ingress/egress keys
	if _, ok := spec["ingress"]; ok {
		types["Ingress"] = true
	}
	if _, ok := spec["egress"]; ok {
		types["Egress"] = true
	}
	if len(types) == 0 {
		// bare podSelector only → implies Ingress by default
		types["Ingress"] = true
	}
	return types
}

func parsePeerRules(peers []M, ports []M) []NetPolPeerRule {
	portDescs := descPorts(ports)

	if len(peers) == 0 {
		return []NetPolPeerRule{{PeerDesc: "allow all peers", Ports: portDescs}}
	}

	var rules []NetPolPeerRule
	for _, peer := range peers {
		rules = append(rules, NetPolPeerRule{
			PeerDesc: descPeer(peer),
			Ports:    portDescs,
		})
	}
	return rules
}

func descPeer(peer M) string {
	if peer == nil {
		return "all peers"
	}

	// IP block is standalone — return early.
	if ipBlock, ok := peer["ipBlock"].(map[string]any); ok {
		cidr, _ := ipBlock["cidr"].(string)
		if cidr == "" {
			cidr = "0.0.0.0/0"
		}
		desc := "traffic from cidr:" + cidr
		if excepts, ok := ipBlock["except"].([]any); ok && len(excepts) > 0 {
			desc += " (except " + strings.Join(toStringSlice(excepts), ", ") + ")"
		}
		return desc
	}

	var podPart, nsPart string
	if ps, ok := peer["podSelector"].(map[string]any); ok {
		sel := descLabelSelector(M(ps))
		if sel == "{}" {
			podPart = "all pods"
		} else {
			podPart = "pods with labels matching " + sel
		}
	}
	if ns, ok := peer["namespaceSelector"].(map[string]any); ok {
		sel := descLabelSelector(M(ns))
		if sel == "{}" {
			nsPart = "any namespace"
		} else {
			nsPart = "namespaces with labels matching " + sel
		}
	}

	switch {
	case podPart != "" && nsPart != "":
		return podPart + " in " + nsPart
	case podPart != "":
		return podPart
	case nsPart != "":
		return "pods in " + nsPart
	default:
		return "all peers"
	}
}

func descLabelSelector(sel M) string {
	if sel == nil {
		return "{}"
	}
	ml, _ := sel["matchLabels"].(map[string]any)
	exprs, _ := sel["matchExpressions"].([]any)
	if len(ml) == 0 && len(exprs) == 0 {
		return "{}"
	}
	return descLabelSelectorSafe(sel)
}

func descLabelSelectorSafe(sel M) string {
	if sel == nil {
		return "{}"
	}
	var parts []string
	if ml, ok := sel["matchLabels"].(map[string]any); ok {
		for k, v := range ml {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
	}
	if exprs, ok := sel["matchExpressions"].([]any); ok {
		for _, exprAny := range exprs {
			if expr, ok := exprAny.(map[string]any); ok {
				key, _ := expr["key"].(string)
				op, _ := expr["operator"].(string)
				vals := toStringSlice(expr["values"])
				switch op {
				case "In":
					parts = append(parts, fmt.Sprintf("%s in (%s)", key, strings.Join(vals, ",")))
				case "NotIn":
					parts = append(parts, fmt.Sprintf("%s notin (%s)", key, strings.Join(vals, ",")))
				case "Exists":
					parts = append(parts, key)
				case "DoesNotExist":
					parts = append(parts, "!"+key)
				}
			}
		}
	}
	if len(parts) == 0 {
		return "{}"
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func descPorts(ports []M) []string {
	if len(ports) == 0 {
		return nil
	}
	var out []string
	for _, p := range ports {
		proto, _ := p["protocol"].(string)
		if proto == "" {
			proto = "TCP"
		}
		portStr := ""
		switch v := p["port"].(type) {
		case string:
			portStr = v
		case float64:
			portStr = fmt.Sprintf("%d", int(v))
		case int:
			portStr = fmt.Sprintf("%d", v)
		}
		if portStr != "" {
			out = append(out, portStr+"/"+proto)
		} else {
			out = append(out, proto)
		}
	}
	return out
}

func extractStringMap(m M) map[string]string {
	out := map[string]string{}
	if m == nil {
		return out
	}
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

func sortedLabels(labels map[string]string) []string {
	out := make([]string, 0, len(labels))
	for k, v := range labels {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

func policyWord(n int) string {
	if n == 1 {
		return "policy"
	}
	return "policies"
}

func stringIn(s string, list []string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func toStringSlice(v any) []string {
	slice, _ := v.([]any)
	out := make([]string, 0, len(slice))
	for _, item := range slice {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
