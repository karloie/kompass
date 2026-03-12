package tree

import (
	"fmt"
	"strings"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

func RenderTree(treeNode *kube.Tree, nodeMap map[string]*kube.Resource, plain bool) string {
	var sb strings.Builder
	visited := make(map[string]bool)
	renderTreeNode(&sb, treeNode, "", true, true, nodeMap, visited, plain, nil)
	return sb.String()
}

func renderTreeNode(sb *strings.Builder, treeNode *kube.Tree, prefix string, isLast bool, isRoot bool, nodeMap map[string]*kube.Resource, visited map[string]bool, plain bool, parentMeta map[string]any) {
	if treeNode == nil {
		return
	}

	key := treeNode.Key

	if visited[key] {
		return
	}
	visited[key] = true
	defer func() { visited[key] = false }()

	var displayKey string

	meta := treeNode.Meta
	if len(meta) == 0 {

		if resource, ok := nodeMap[treeNode.Key]; ok {
			meta = extractMetadataFromResource(*resource, nodeMap)
		}
	}

	if len(meta) > 0 {
		displayKey = formatNodeName(treeNode.Type, meta, nil, plain, parentMeta)
	} else {
		emoji := graph.GetResourceEmoji(treeNode.Type) + " "
		displayKey = emoji + treeNode.Type
	}

	var treeBranch string
	if isRoot {
		treeBranch = ""
	} else if isLast {
		treeBranch = "└─ "
	} else {
		treeBranch = "├─ "
	}

	sb.WriteString(fmt.Sprintf("%s%s%s\n", prefix, treeBranch, displayKey))

	numChildren := len(treeNode.Children)
	for i, child := range treeNode.Children {
		childIsLast := (i == numChildren-1)
		var childPrefix string
		if isRoot {
			childPrefix = ""
		} else if isLast {
			childPrefix = prefix + "   "
		} else {
			childPrefix = prefix + "│  "
		}

		childParentMeta := meta
		if childParentMeta != nil {
			if _, hasNamespace := childParentMeta["namespace"]; !hasNamespace && parentMeta != nil {
				if ns, ok := parentMeta["namespace"].(string); ok {

					childParentMeta = make(map[string]any)
					for k, v := range meta {
						childParentMeta[k] = v
					}
					childParentMeta["namespace"] = ns
				}
			}
		} else if parentMeta != nil {

			if ns, ok := parentMeta["namespace"].(string); ok {
				childParentMeta = map[string]any{"namespace": ns}
			}
		}
		renderTreeNode(sb, child, childPrefix, childIsLast, false, nodeMap, visited, plain, childParentMeta)
	}
}
