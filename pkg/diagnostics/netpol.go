package diagnostics

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

// RunNetpolAnalysis fetches pod and networkpolicies via kubectl and renders
// a human-readable verdict.
var RunNetpolAnalysis = func(target PodTarget, context string) (string, error) {
	if target.Name == "" || target.Namespace == "" {
		return "(no pod info available)", nil
	}
	args := []string{"get", "pod", target.Name, "-n", target.Namespace, "-o", "json"}
	if strings.TrimSpace(context) != "" {
		args = append(args, "--context", strings.TrimSpace(context))
	}
	podOut, err := exec.Command("kubectl", args...).CombinedOutput()
	if err != nil {
		return "error fetching pod: " + strings.TrimSpace(string(podOut)), err
	}
	var podRaw map[string]any
	if err := json.Unmarshal(podOut, &podRaw); err != nil {
		return "error parsing pod JSON: " + err.Error(), err
	}

	npArgs := []string{"get", "networkpolicy", "-n", target.Namespace, "-o", "json"}
	if strings.TrimSpace(context) != "" {
		npArgs = append(npArgs, "--context", strings.TrimSpace(context))
	}
	npOut, _ := exec.Command("kubectl", npArgs...).CombinedOutput()

	nodes := map[string]kube.Resource{}
	podKey := "pod/" + target.Namespace + "/" + target.Name
	nodes[podKey] = kube.Resource{Key: podKey, Type: "pod", Resource: podRaw}

	var npList map[string]any
	if err := json.Unmarshal(npOut, &npList); err == nil {
		if items, ok := npList["items"].([]any); ok {
			for _, item := range items {
				if m, ok := item.(map[string]any); ok {
					ns, _ := nestedString(m, "metadata", "namespace")
					name, _ := nestedString(m, "metadata", "name")
					if ns != "" && name != "" {
						k := "networkpolicy/" + ns + "/" + name
						nodes[k] = kube.Resource{Key: k, Type: "networkpolicy", Resource: m}
					}
				}
			}
		}
	}

	podResource := nodes[podKey]
	verdict := graph.AnalyzePodNetworkPolicies(nodes, podResource)
	return graph.FormatNetPolVerdict(verdict), nil
}

// AnalyzePodNetworkPoliciesFromResources evaluates netpol using in-memory
// resources loaded by the selector tree, avoiding shell calls when possible.
func AnalyzePodNetworkPoliciesFromResources(target PodTarget, resources map[string]*kube.Resource) (string, bool) {
	if target.Name == "" || target.Namespace == "" {
		return "", false
	}
	if len(resources) == 0 {
		return "", false
	}

	podKey := "pod/" + target.Namespace + "/" + target.Name
	podPtr, ok := resources[podKey]
	if !ok || podPtr == nil {
		return "", false
	}
	podObj := podPtr.AsMap()
	meta, ok := podObj["metadata"].(map[string]any)
	if !ok {
		return "", false
	}
	podName, _ := meta["name"].(string)
	podNS, _ := meta["namespace"].(string)
	if strings.TrimSpace(podName) == "" || strings.TrimSpace(podNS) == "" {
		return "", false
	}

	nodes := make(map[string]kube.Resource, len(resources))
	for key, res := range resources {
		if res == nil {
			continue
		}
		if res.Type != "networkpolicy" && key != podKey {
			continue
		}
		nodes[key] = *res
	}
	if len(nodes) == 0 {
		return "", false
	}

	verdict := graph.AnalyzePodNetworkPolicies(nodes, *podPtr)
	return graph.FormatNetPolVerdict(verdict), true
}

func nestedString(m map[string]any, keys ...string) (string, bool) {
	cur := m
	for i, k := range keys {
		if i == len(keys)-1 {
			s, ok := cur[k].(string)
			return s, ok
		}
		next, ok := cur[k].(map[string]any)
		if !ok {
			return "", false
		}
		cur = next
	}
	return "", false
}

type defaultNetpolProvider struct{}

func (defaultNetpolProvider) AnalyzePod(target PodTarget, context string, resources map[string]*kube.Resource) (string, error) {
	if analysis, ok := AnalyzePodNetworkPoliciesFromResources(target, resources); ok {
		return analysis, nil
	}
	slog.Warn("netpol provider fallback", "provider", "kubectl", "namespace", target.Namespace, "name", target.Name, "reason", "in-memory analysis unavailable")
	return RunNetpolAnalysis(target, context)
}

func ResolveNetpolProvider(p NetpolProvider) NetpolProvider {
	if p != nil {
		return p
	}
	return defaultNetpolProvider{}
}

func (t PodTarget) String() string {
	return fmt.Sprintf("%s/%s", t.Namespace, t.Name)
}