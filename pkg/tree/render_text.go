package tree

import (
	"fmt"
	"strings"

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

	meta := ResolveNodeMetadata(treeNode, nodeMap)
	displayKey = RenderNodeLabel(treeNode, nodeMap, plain, parentMeta)

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

		childParentMeta := BuildChildParentMeta(treeNode.Type, meta, parentMeta)
		renderTreeNode(sb, child, childPrefix, childIsLast, false, nodeMap, visited, plain, childParentMeta)
	}
}
