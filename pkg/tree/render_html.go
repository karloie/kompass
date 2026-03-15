package tree

import (
	"bytes"
	_ "embed"
	"fmt"
	"html"
	"html/template"
	"strings"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

//go:embed templates/tree.html.tmpl
var treeHTMLTemplateSource string

//go:embed templates/tree.js
var treeHTMLScriptSource string

var treeHTMLTemplate = template.Must(template.New("tree-html").Parse(treeHTMLTemplateSource))

type treeHTMLView struct {
	Context   string
	Namespace string
	Header    string
	TreeHTML  template.HTML
	Script    template.JS
}

// RenderHTML renders all trees as a self-contained HTML document.
func RenderHTML(result *kube.Response, context_, namespace, configPath string, selectors []string) string {
	if result == nil {
		result = &kube.Response{}
	}

	var treeHTML strings.Builder
	treeHTML.WriteString(`<ul id="tree-root" class="tree">`)
	for i := range result.Trees {
		renderTreeHTMLNode(&treeHTML, &result.Trees[i], result.Nodes, nil)
	}
	treeHTML.WriteString(`</ul>`)

	header := fmt.Sprintf("🌍 Context: %s, Namespace: %s, Selectors: %v, Config: %s", context_, namespace, selectors, configPath)
	var out bytes.Buffer
	view := treeHTMLView{
		Context:   context_,
		Namespace: namespace,
		Header:    header,
		TreeHTML:  template.HTML(treeHTML.String()),
		Script:    template.JS(treeHTMLScriptSource),
	}
	if err := treeHTMLTemplate.Execute(&out, view); err != nil {
		return "<html><body><p>failed to render tree html</p></body></html>"
	}

	return out.String()
}

func renderTreeHTMLNode(sb *strings.Builder, treeNode *kube.Tree, nodeMap map[string]*kube.Resource, parentMeta map[string]any) {
	if treeNode == nil {
		return
	}

	meta := treeNode.Meta
	if len(meta) == 0 {
		if resource, ok := nodeMap[treeNode.Key]; ok {
			meta = extractMetadataFromResource(*resource, nodeMap)
		}
	}

	var label string
	if len(meta) > 0 {
		label = formatNodeName(treeNode.Type, meta, nil, true, parentMeta)
	} else {
		label = graph.GetResourceEmoji(treeNode.Type) + " " + treeNode.Type
	}

	sb.WriteString(`<li data-label="`)
	sb.WriteString(html.EscapeString(label))
	sb.WriteString(`" data-user-collapsed="false"><div class="row">`)
	if len(treeNode.Children) > 0 {
		sb.WriteString(`<button type="button" class="toggle" aria-label="Toggle branch">▼</button>`)
	} else {
		sb.WriteString(`<button type="button" class="toggle" disabled aria-hidden="true"></button>`)
	}
	sb.WriteString(`<span class="node" data-label="`)
	sb.WriteString(html.EscapeString(label))
	sb.WriteString(`">`)
	sb.WriteString(html.EscapeString(label))
	sb.WriteString(`</span></div>`)

	if len(treeNode.Children) > 0 {
		sb.WriteString(`<ul>`)
		for _, child := range treeNode.Children {
			renderTreeHTMLNode(sb, child, nodeMap, meta)
		}
		sb.WriteString(`</ul>`)
	}

	sb.WriteString(`</li>`)
}
