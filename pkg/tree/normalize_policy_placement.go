package tree

import (
	"sort"

	kube "github.com/karloie/kompass/pkg/kube"
)

func isPolicyType(nodeType string) bool {
	return nodeType == "ciliumnetworkpolicy" || nodeType == "ciliumclusterwidenetworkpolicy"
}

func isDeclaredSpecType(nodeType string) bool {
	if isPolicyType(nodeType) {
		return true
	}
	return nodeType == "service" ||
		nodeType == "serviceaccount" ||
		nodeType == "networkpolicy"
}

func isWorkloadType(nodeType string) bool {
	return nodeType == "deployment" ||
		nodeType == "daemonset" ||
		nodeType == "statefulset" ||
		nodeType == "cronjob"
}

func normalizePolicyPlacement(root *kube.Tree) {
	if root == nil {
		return
	}
	normalizePolicyPlacementTree(root)
}

func normalizePolicyPlacementTree(node *kube.Tree) {
	if node == nil {
		return
	}

	if isWorkloadType(node.Type) {
		normalizeWorkloadPolicyPlacement(node)
	}

	for i := range node.Children {
		normalizePolicyPlacementTree(node.Children[i])
	}
}

func normalizeWorkloadPolicyPlacement(node *kube.Tree) {
	declared := map[string]*kube.Tree{}
	collectDeclaredSpecNodes(node.Children, declared)
	if len(declared) == 0 {
		return
	}

	node.Children = stripDeclaredSpecNodes(node.Children)
	specNode := findOrCreateSpecNode(node)
	if specNode == nil {
		return
	}

	specNode.Children = stripDeclaredSpecNodes(specNode.Children)

	orderedKeys := make([]string, 0, len(declared))
	for key := range declared {
		orderedKeys = append(orderedKeys, key)
	}
	sort.Strings(orderedKeys)
	for _, key := range orderedKeys {
		node := declared[key]
		if node != nil {
			node.Children = stripDeclaredSpecNodes(node.Children)
		}
		specNode.Children = append(specNode.Children, node)
	}

	sortChildren(specNode.Children)
	sortChildren(node.Children)

	if node.Type == "cronjob" {
		stripCronJobPodRedundantNodes(node)
	}
}

func collectDeclaredSpecNodes(children []*kube.Tree, declared map[string]*kube.Tree) {
	for i := range children {
		child := children[i]
		if child == nil {
			continue
		}
		if isDeclaredSpecType(child.Type) {
			mergeDeclaredSpecNode(declared, child)
		}
		collectDeclaredSpecNodes(child.Children, declared)
	}
}

func mergeDeclaredSpecNode(declared map[string]*kube.Tree, candidate *kube.Tree) {
	if candidate == nil || candidate.Key == "" {
		return
	}
	if existing, ok := declared[candidate.Key]; ok {
		if len(candidate.Children) > len(existing.Children) {
			declared[candidate.Key] = candidate
		}
		return
	}
	declared[candidate.Key] = candidate
}

func stripDeclaredSpecNodes(children []*kube.Tree) []*kube.Tree {
	filtered := make([]*kube.Tree, 0, len(children))
	for i := range children {
		child := children[i]
		if child == nil {
			continue
		}
		if isDeclaredSpecType(child.Type) {
			continue
		}
		child.Children = stripDeclaredSpecNodes(child.Children)
		filtered = append(filtered, child)
	}
	return filtered
}

func findOrCreateSpecNode(node *kube.Tree) *kube.Tree {
	for i := range node.Children {
		child := node.Children[i]
		if child != nil && child.Type == "spec" {
			return child
		}
	}

	specKey := node.Key + "/spec"
	specNode := newTree(specKey, "spec", map[string]any{})
	node.Children = append(node.Children, specNode)
	return specNode
}

// For pods inside a cronjob's job children: strip the pod-level spec node (the
// template-derived spec lives at the cronjob level) and strip endpoints/endpointslice
// (they are already nested under their service in the cronjob spec).
func stripCronJobPodRedundantNodes(cronJobNode *kube.Tree) {
	for _, jobNode := range cronJobNode.Children {
		if jobNode == nil || jobNode.Type != "job" {
			continue
		}
		for _, podNode := range jobNode.Children {
			if podNode == nil || podNode.Type != "pod" {
				continue
			}
			filtered := podNode.Children[:0]
			for _, child := range podNode.Children {
				if child == nil {
					continue
				}
				if child.Type == "spec" || child.Type == "endpoints" || child.Type == "endpointslice" || isDeclaredSpecType(child.Type) {
					continue
				}
				filtered = append(filtered, child)
			}
			podNode.Children = filtered
		}
	}
}
