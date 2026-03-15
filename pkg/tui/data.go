package tui

import (
	"fmt"
	"sort"
	"strings"

	kube "github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/tree"
)

func flattenTrees(trees *kube.Response) []Row {
	if trees == nil {
		return nil
	}

	nodeMap := trees.NodeMap()
	rows := make([]Row, 0, 128)
	for i := range trees.Trees {
		root := &trees.Trees[i]
		coloredRendered := strings.TrimRight(tree.RenderTree(root, nodeMap, false), "\n")
		plainRendered := strings.TrimRight(tree.RenderTree(root, nodeMap, true), "\n")
		coloredRows := []string{}
		plainRows := []string{}
		if coloredRendered != "" {
			coloredRows = strings.Split(coloredRendered, "\n")
		}
		if plainRendered != "" {
			plainRows = strings.Split(plainRendered, "\n")
		}
		rowIndex := 0
		flattenNode(&rows, root, 0, nodeMap, coloredRows, plainRows, &rowIndex, nil)
		if i < len(trees.Trees)-1 {
			rows = append(rows, Row{Separator: true})
		}
	}
	return rows
}

func filterResponseTrees(source *kube.Response, matcher queryMatcher) *kube.Response {
	if source == nil {
		return nil
	}

	filtered := &kube.Response{
		APIVersion: source.APIVersion,
		Request:    source.Request,
		Nodes:      source.Nodes,
		Edges:      source.Edges,
		Components: source.Components,
		Metadata:   source.Metadata,
		Trees:      make([]kube.Tree, 0, len(source.Trees)),
	}

	for i := range source.Trees {
		node := &source.Trees[i]
		if next := filterTreeNode(node, matcher); next != nil {
			filtered.Trees = append(filtered.Trees, *next)
		}
	}

	return filtered
}

func filterTreeNode(node *kube.Tree, matcher queryMatcher) *kube.Tree {
	if node == nil {
		return nil
	}

	children := make([]*kube.Tree, 0, len(node.Children))
	for _, child := range node.Children {
		if next := filterTreeNode(child, matcher); next != nil {
			children = append(children, next)
		}
	}

	matchesSelf := matcher.test(treeNodeSearchText(node))
	if !matchesSelf && len(children) == 0 {
		return nil
	}

	meta := make(map[string]any, len(node.Meta))
	for k, v := range node.Meta {
		meta[k] = v
	}

	return &kube.Tree{
		Key:      node.Key,
		Type:     node.Type,
		Icon:     node.Icon,
		Meta:     meta,
		Children: children,
	}
}

func treeNodeSearchText(node *kube.Tree) string {
	if node == nil {
		return ""
	}
	parts := []string{node.Type, node.Key}
	for k, v := range node.Meta {
		if k == "orphaned" {
			parts = append(parts, "single", fmt.Sprint(v))
			continue
		}
		parts = append(parts, k, fmt.Sprint(v))
	}
	return strings.Join(parts, " ")
}

func flattenNode(rows *[]Row, n *kube.Tree, depth int, nodeMap map[string]*kube.Resource, coloredRows, plainRows []string, rowIndex *int, parentMeta map[string]any) {
	if n == nil {
		return
	}
	meta := tree.ResolveNodeMetadata(n, nodeMap)
	if meta == nil {
		meta = map[string]any{}
	}
	name := stringMeta(meta, "name", n.Key)
	status := stringMeta(meta, "status", "")
	rowText := name
	plainRowText := name
	if rowIndex != nil {
		if *rowIndex < len(coloredRows) {
			rowText = coloredRows[*rowIndex]
		}
		if *rowIndex < len(plainRows) {
			plainRowText = plainRows[*rowIndex]
		}
		*rowIndex++
	}
	row := Row{Key: n.Key, Type: n.Type, Name: name, Text: rowText, Plain: plainRowText, PlainText: plainRowText, Status: status, Metadata: meta, Depth: depth}
	searchLabel := tree.RenderNodeLabel(n, nodeMap, true, parentMeta)
	row.SearchText = strings.Join([]string{n.Key, tree.BuildNodeSearchText(n.Type, searchLabel, meta)}, " ")
	*rows = append(*rows, row)
	childParentMeta := tree.BuildChildParentMeta(n.Type, meta, parentMeta)
	for _, c := range n.Children {
		flattenNode(rows, c, depth+1, nodeMap, coloredRows, plainRows, rowIndex, childParentMeta)
	}
}

func singleRows(rows []Row) []Row {
	out := make([]Row, 0)
	for _, r := range rows {
		if isSingle, ok := r.Metadata["orphaned"].(bool); ok && isSingle {
			out = append(out, r)
		}
	}
	return out
}

func stringMeta(meta map[string]any, key, fallback string) string {
	if v, ok := meta[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

func stringMetadata(meta map[string]any) string {
	if len(meta) == 0 {
		return ""
	}
	keys := make([]string, 0, len(meta))
	for k := range meta {
		if k == "name" || k == "status" || k == "orphaned" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ""
	}
	parts := make([]string, 0, minInt(3, len(keys)))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, meta[k]))
		if len(parts) == 3 {
			break
		}
	}
	return strings.Join(parts, " ")
}
