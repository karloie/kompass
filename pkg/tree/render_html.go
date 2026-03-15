package tree

import (
	"bytes"
	_ "embed"
	"fmt"
	"html"
	"html/template"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

//go:embed templates/tree.html.tmpl
var treeHTMLTemplateSource string

//go:embed templates/tree.js
var treeHTMLScriptSource string

var treeHTMLTemplate = template.Must(template.New("tree-html").Parse(treeHTMLTemplateSource))

// BuildMode is overridden in release builds via ldflags.
var BuildMode = "dev"

const (
	treeTemplatePath = "pkg/tree/templates/tree.html.tmpl"
	treeScriptPath   = "pkg/tree/templates/tree.js"
)

type treeHTMLView struct {
	Context    string
	Namespace  string
	Namespaces []string
	StaticMode bool
	Header     template.HTML
	TreeHTML   template.HTML
	Script     template.JS
}

// RenderHTML renders all trees as a self-contained HTML document.
func RenderHTML(result *kube.Response, context_, namespace, configPath string, selectors []string, staticMode bool) string {
	if result == nil {
		result = &kube.Response{}
	}
	nodeMap := result.NodeMap()

	var treeHTML strings.Builder
	treeHTML.WriteString(`<ul id="tree-root" class="tree">`)
	for i := range result.Trees {
		renderTreeHTMLNode(&treeHTML, &result.Trees[i], nodeMap, nil)
	}
	treeHTML.WriteString(`</ul>`)

	brand := "🧭 <a href=\"https://github.com/karloie/kompass\" target=\"_blank\" rel=\"noopener noreferrer\">Kompass</a>"

	escapedSelectors := make([]string, len(selectors))
	for i, s := range selectors {
		escapedSelectors[i] = html.EscapeString(s)
	}
	header := template.HTML(fmt.Sprintf("%s: Context: %s, Namespace: %s, Selectors: %v, Config: %s",
		brand,
		html.EscapeString(context_),
		html.EscapeString(namespace),
		escapedSelectors,
		html.EscapeString(configPath)))
	namespaces := []string{}
	if !staticMode {
		namespaces = collectTreeNamespaces(result, namespace)
	}
	var out bytes.Buffer
	view := treeHTMLView{
		Context:    context_,
		Namespace:  namespace,
		Namespaces: namespaces,
		StaticMode: staticMode,
		Header:     header,
		TreeHTML:   template.HTML(treeHTML.String()),
		Script:     template.JS(loadTreeHTMLScript()),
	}
	if err := loadTreeHTMLTemplate().Execute(&out, view); err != nil {
		return "<html><body><p>failed to render tree html</p></body></html>"
	}

	return out.String()
}

func shouldUseRuntimeTemplateFiles() bool {
	return strings.ToLower(strings.TrimSpace(BuildMode)) != "release"
}

func loadTreeHTMLTemplate() *template.Template {
	if !shouldUseRuntimeTemplateFiles() {
		return treeHTMLTemplate
	}
	content, err := os.ReadFile(treeTemplatePath)
	if err != nil {
		return treeHTMLTemplate
	}
	tmpl, err := template.New("tree-html").Parse(string(content))
	if err != nil {
		return treeHTMLTemplate
	}
	return tmpl
}

func loadTreeHTMLScript() string {
	if !shouldUseRuntimeTemplateFiles() {
		return treeHTMLScriptSource
	}
	content, err := os.ReadFile(treeScriptPath)
	if err != nil {
		return treeHTMLScriptSource
	}
	return string(content)
}
func collectTreeNamespaces(result *kube.Response, currentNamespace string) []string {
	set := map[string]struct{}{}
	if currentNamespace != "" {
		set[currentNamespace] = struct{}{}
	}

	for key := range result.Nodes {
		ns := ParseResourceKeyRef(result.Nodes[key].Key).Namespace
		if ns != "" {
			set[ns] = struct{}{}
		}
	}

	namespaces := make([]string, 0, len(set))
	for ns := range set {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)
	return namespaces
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
	searchText := buildNodeSearchText(treeNode.Type, label, meta)

	sb.WriteString(`<li data-label="`)
	sb.WriteString(html.EscapeString(label))
	sb.WriteString(`" data-search="`)
	sb.WriteString(html.EscapeString(searchText))
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

func buildNodeSearchText(nodeType, label string, meta map[string]any) string {
	tokens := []string{nodeType, label}
	appendSearchTokens(&tokens, "", meta)
	return strings.Join(tokens, " ")
}

var noisyMetadataKeys = map[string]bool{
	"__nodetype":         true,
	"annotations":        true,
	"creationtimestamp":  true,
	"managedfields":      true,
	"ownerreferences":    true,
	"resourceversion":    true,
	"uid":                true,
	"lasttransitiontime": true,
	"containerid":        true,
}

var hashLikeToken = regexp.MustCompile(`^[a-f0-9]{24,}$`)

func appendSearchTokens(tokens *[]string, keyHint string, value any) {
	switch v := value.(type) {
	case nil:
		return
	case string:
		if shouldIndexToken(v) {
			*tokens = append(*tokens, v)
		}
	case bool:
		*tokens = append(*tokens, strconv.FormatBool(v))
	case int:
		*tokens = append(*tokens, strconv.Itoa(v))
	case int64:
		*tokens = append(*tokens, strconv.FormatInt(v, 10))
	case float64:
		*tokens = append(*tokens, strconv.FormatFloat(v, 'f', -1, 64))
	case []any:
		for _, item := range v {
			appendSearchTokens(tokens, keyHint, item)
		}
	case []string:
		for _, item := range v {
			appendSearchTokens(tokens, keyHint, item)
		}
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if isNoisyMetadataKey(key) {
				continue
			}
			if shouldIndexToken(key) {
				*tokens = append(*tokens, key)
			}
			appendSearchTokens(tokens, key, v[key])
		}
	default:
		raw := fmt.Sprint(v)
		if shouldIndexToken(raw) {
			*tokens = append(*tokens, raw)
		}
	}
}

func isNoisyMetadataKey(key string) bool {
	return noisyMetadataKeys[strings.ToLower(strings.TrimSpace(key))]
}

func shouldIndexToken(value string) bool {
	token := strings.TrimSpace(value)
	if token == "" {
		return false
	}
	if len(token) > 140 {
		return false
	}
	lower := strings.ToLower(token)
	if strings.Contains(lower, "sha256:") {
		return false
	}
	if hashLikeToken.MatchString(lower) {
		return false
	}
	return true
}
