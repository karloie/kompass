package tree

import (
	"fmt"
	"strings"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

// FormatTreeHeader returns the common context/namespace/selectors/config summary.
func FormatTreeHeader(context, namespace, configPath string, selectors []string) string {
	return fmt.Sprintf("Kompass Context: %s, Namespace: %s, Selectors: %v, Config: %s", context, namespace, selectors, configPath)
}

// ResolveNodeMetadata returns the normalized metadata used for rendering/search.
// It applies metadata extraction rules when the tree node does not carry metadata.
func ResolveNodeMetadata(treeNode *kube.Tree, nodeMap map[string]*kube.Resource) map[string]any {
	if treeNode == nil {
		return nil
	}

	meta := treeNode.Meta
	if len(meta) == 0 {
		if resource, ok := nodeMap[treeNode.Key]; ok {
			meta = extractMetadataFromResource(*resource, nodeMap)
		}
	}

	if len(meta) == 0 {
		return nil
	}

	// Keep callers from mutating the original metadata map.
	cloned := make(map[string]any, len(meta))
	for k, v := range meta {
		cloned[k] = v
	}
	return cloned
}

func nodeIcon(treeNode *kube.Tree) string {
	if treeNode == nil {
		return ""
	}
	if icon := strings.TrimSpace(treeNode.Icon); icon != "" {
		return icon
	}
	return graph.GetResourceEmoji(treeNode.Type)
}

// RenderNodeLabel returns a single node label consistent across renderers.
func RenderNodeLabel(treeNode *kube.Tree, nodeMap map[string]*kube.Resource, plain bool, parentMeta map[string]any) string {
	if treeNode == nil {
		return ""
	}

	meta := ResolveNodeMetadata(treeNode, nodeMap)
	if len(meta) > 0 {
		return formatNodeName(treeNode.Type, meta, nil, plain, parentMeta)
	}

	icon := nodeIcon(treeNode)
	if icon == "" {
		return treeNode.Type
	}
	return icon + " " + treeNode.Type
}

// BuildNodeSearchText is the shared search token builder used by both TUI and HTML.
func BuildNodeSearchText(nodeType, label string, meta map[string]any) string {
	return buildNodeSearchText(nodeType, label, meta)
}

// BuildChildParentMeta carries parent context down the tree in a consistent way.
func BuildChildParentMeta(nodeType string, meta, parentMeta map[string]any) map[string]any {
	childParentMeta := meta
	if childParentMeta != nil {
		if _, hasNodeType := childParentMeta["__nodeType"]; !hasNodeType {
			cloned := make(map[string]any, len(childParentMeta)+1)
			for k, v := range childParentMeta {
				cloned[k] = v
			}
			cloned["__nodeType"] = nodeType
			childParentMeta = cloned
		}
		if _, hasNamespace := childParentMeta["namespace"]; !hasNamespace && parentMeta != nil {
			if ns, ok := parentMeta["namespace"].(string); ok {
				withNS := make(map[string]any, len(childParentMeta)+1)
				for k, v := range childParentMeta {
					withNS[k] = v
				}
				withNS["namespace"] = ns
				withNS["__nodeType"] = nodeType
				childParentMeta = withNS
			}
		}
		return childParentMeta
	}

	if parentMeta != nil {
		if ns, ok := parentMeta["namespace"].(string); ok {
			return map[string]any{"namespace": ns, "__nodeType": nodeType}
		}
	}

	return map[string]any{"__nodeType": nodeType}
}
