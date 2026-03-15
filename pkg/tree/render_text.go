package tree

import (
	"fmt"
	"strings"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

// RenderText renders all trees as a plain-text string. header is the first line
// (context/namespace info); cache stats are appended if present in the result.
func RenderText(result *kube.Response, header string, plain bool) string {
	var sb strings.Builder
	nodeMap := result.NodeMap()
	if result.Metadata != nil && result.Metadata.CacheCalls > 0 {
		header += fmt.Sprintf(", Cache: %d calls | %d hits | %d misses | %.1f%% hit rate",
			result.Metadata.CacheCalls, result.Metadata.CacheHits, result.Metadata.CacheMisses, result.Metadata.CacheHitRate)
	}
	sb.WriteString(header)
	if !strings.HasSuffix(header, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	for i := range result.Trees {
		sb.WriteString(RenderTree(&result.Trees[i], nodeMap, plain))
		if i < len(result.Trees)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

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
		childIsLast := i == numChildren-1
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
			if _, hasNodeType := childParentMeta["__nodeType"]; !hasNodeType {
				cloned := make(map[string]any, len(childParentMeta)+1)
				for k, v := range childParentMeta {
					cloned[k] = v
				}
				cloned["__nodeType"] = treeNode.Type
				childParentMeta = cloned
			}
			if _, hasNamespace := childParentMeta["namespace"]; !hasNamespace && parentMeta != nil {
				if ns, ok := parentMeta["namespace"].(string); ok {

					childParentMeta = make(map[string]any)
					for k, v := range meta {
						childParentMeta[k] = v
					}
					childParentMeta["__nodeType"] = treeNode.Type
					childParentMeta["namespace"] = ns
				}
			}
		} else if parentMeta != nil {

			if ns, ok := parentMeta["namespace"].(string); ok {
				childParentMeta = map[string]any{"namespace": ns, "__nodeType": treeNode.Type}
			}
		} else {
			childParentMeta = map[string]any{"__nodeType": treeNode.Type}
		}
		renderTreeNode(sb, child, childPrefix, childIsLast, false, nodeMap, visited, plain, childParentMeta)
	}
}
