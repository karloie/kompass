package tree

import (
	"sort"

	kube "github.com/karloie/kompass/pkg/kube"
)

func appendGraphChildren(parentKey string, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.GraphTree {
	builder := NewChildrenBuilder()
	for _, childKey := range graphChildren[parentKey] {
		if state.CanTraverse(childKey) {
			builder.Add(buildTreeNode(childKey, graphChildren, state, nodeMap))
		}
	}
	return builder.Build()
}

func childKeysOfType(parentKey string, childType string, graphChildren map[string][]string, nodeMap map[string]kube.Resource) []string {
	keys := make([]string, 0)
	for _, childKey := range graphChildren[parentKey] {
		if getTypeFromKey(childKey, nodeMap) == childType {
			keys = append(keys, childKey)
		}
	}
	return keys
}

func appendFilteredGraphChildren(children []*kube.GraphTree, parentKey string, excludedTypes map[string]bool, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.GraphTree {
	for _, childKey := range graphChildren[parentKey] {
		if !state.CanTraverse(childKey) {
			continue
		}
		if excludedTypes[getTypeFromKey(childKey, nodeMap)] {
			continue
		}
		childNode := buildTreeNode(childKey, graphChildren, state, nodeMap)
		if childNode != nil {
			children = append(children, childNode)
		}
	}
	return children
}

func buildTreeAdjacency(edges []kube.ResourceEdge, nodeMap map[string]kube.Resource) map[string][]string {
	children := make(map[string][]string)
	for _, edge := range edges {
		if edge.Source == "" || edge.Target == "" {
			continue
		}
		children[edge.Source] = append(children[edge.Source], edge.Target)
		sourceType := getTypeFromKey(edge.Source, nodeMap)
		targetType := getTypeFromKey(edge.Target, nodeMap)
		if shouldAddReverseEdge(sourceType, targetType) {
			children[edge.Target] = append(children[edge.Target], edge.Source)
		}
	}
	return children
}

func normalizeChildrenMap(children map[string][]string) {
	for key, childKeys := range children {
		if len(childKeys) <= 1 {
			continue
		}
		sorted := append([]string(nil), childKeys...)
		sort.Strings(sorted)
		w := 0
		for _, childKey := range sorted {
			if w == 0 || childKey != sorted[w-1] {
				sorted[w] = childKey
				w++
			}
		}
		children[key] = sorted[:w]
	}
}

func getTypeFromKey(key string, nodeMap map[string]kube.Resource) string {
	if resource, ok := nodeMap[key]; ok {
		return resource.Type
	}
	return ParseResourceKeyRef(key).Type
}
