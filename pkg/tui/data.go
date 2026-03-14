package tui

import (
	"fmt"
	"sort"
	"strings"

	kube "github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/tree"
)

func flattenTrees(trees *kube.Trees) []Row {
	rows := make([]Row, 0, 128)
	for _, root := range trees.Trees {
		coloredRendered := strings.TrimRight(tree.RenderTree(root, trees.Nodes, false), "\n")
		plainRendered := strings.TrimRight(tree.RenderTree(root, trees.Nodes, true), "\n")
		coloredRows := []string{}
		plainRows := []string{}
		if coloredRendered != "" {
			coloredRows = strings.Split(coloredRendered, "\n")
		}
		if plainRendered != "" {
			plainRows = strings.Split(plainRendered, "\n")
		}
		rowIndex := 0
		flattenNode(&rows, root, 0, coloredRows, plainRows, &rowIndex)
	}
	return rows
}

func flattenNode(rows *[]Row, n *kube.Tree, depth int, coloredRows, plainRows []string, rowIndex *int) {
	if n == nil {
		return
	}
	meta := map[string]any{}
	for k, v := range n.Meta {
		meta[k] = v
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
	*rows = append(*rows, Row{Key: n.Key, Type: n.Type, Name: name, Text: rowText, Plain: plainRowText, PlainText: plainRowText, Status: status, Metadata: meta, Depth: depth})
	for _, c := range n.Children {
		flattenNode(rows, c, depth+1, coloredRows, plainRows, rowIndex)
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
