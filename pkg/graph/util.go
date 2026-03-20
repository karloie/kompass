package graph

import (
	"context"
	"strings"

	kube "github.com/karloie/kompass/pkg/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type M map[string]any

func (m M) Path(keys ...string) M {
	current := map[string]any(m)
	for _, key := range keys {
		if current == nil {
			return nil
		}
		if next, ok := current[key].(map[string]any); ok {
			current = next
		} else {
			return nil
		}
	}
	return M(current)
}

func (m M) String(key string) string {
	if m == nil {
		return ""
	}
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

func (m M) StringOk(key string) (string, bool) {
	if m == nil {
		return "", false
	}
	s, ok := m[key].(string)
	return s, ok
}

func (m M) Map(key string) M {
	if m == nil {
		return nil
	}
	if mv, ok := m[key].(map[string]any); ok {
		return M(mv)
	}
	return nil
}

func (m M) MapOk(key string) (M, bool) {
	if m == nil {
		return nil, false
	}
	if mv, ok := m[key].(map[string]any); ok {
		return M(mv), true
	}
	return nil, false
}

func (m M) Int(key string) int {
	if m == nil {
		return 0
	}
	if f, ok := m[key].(float64); ok {
		return int(f)
	}
	if i, ok := m[key].(int); ok {
		return i
	}
	return 0
}

func (m M) IntOk(key string) (int, bool) {
	if m == nil {
		return 0, false
	}
	if f, ok := m[key].(float64); ok {
		return int(f), true
	}
	i, ok := m[key].(int)
	return i, ok
}

func (m M) Slice(key string) []any {
	if m == nil {
		return nil
	}
	if s, ok := m[key].([]any); ok {
		return s
	}
	return nil
}

func (m M) MapSlice(key string) []M {
	if m == nil {
		return nil
	}
	slice := m.Slice(key)
	if slice == nil {
		return nil
	}
	result := make([]M, 0, len(slice))
	for _, item := range slice {
		if mapItem, ok := item.(map[string]any); ok {
			result = append(result, M(mapItem))
		}
	}
	return result
}

func (m M) Raw() map[string]any {
	return map[string]any(m)
}

func ResourceMeta(r *kube.Resource) M {
	return M(r.AsMap()).Map("metadata")
}

func Key(resourceType, namespace, name string) string {
	if name == "" {
		return ""
	}
	if namespace == "" {
		return resourceType + "/" + name
	}
	return resourceType + "/" + namespace + "/" + name
}

func addEdge(edges *[]kube.ResourceEdge, sourceKey, targetKey, label string) {
	*edges = append(*edges, kube.ResourceEdge{Source: sourceKey, Target: targetKey, Label: label})
}

func addNode(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, kind string) string {
	meta := M(item.AsMap()).Map("metadata")
	if meta == nil {
		return ""
	}
	key := Key(kind, meta.String("namespace"), meta.String("name"))
	if key == "" {
		return ""
	}
	res := *item
	res.Type, res.Key = kind, key
	(*nodes)[key] = res
	return key
}

func addEdgeIfNodeExists(edges *[]kube.ResourceEdge, nodes map[string]kube.Resource, sourceKey, targetKey, label string) {
	if sourceKey == "" || targetKey == "" {
		return
	}
	if _, exists := nodes[targetKey]; !exists {
		return
	}
	addEdge(edges, sourceKey, targetKey, label)
}

func forEachNodeOfType(nodes map[string]kube.Resource, resourceType string, fn func(kube.Resource)) {
	for _, node := range nodes {
		if node.Type == resourceType {
			fn(node)
		}
	}
}

func forEachPodMatchingSelector(nodes map[string]kube.Resource, namespace string, selector map[string]any, fn func(kube.Resource)) {
	if len(selector) == 0 {
		return
	}
	forEachNodeOfType(nodes, "pod", func(node kube.Resource) {
		meta := M(node.AsMap()).Map("metadata").Raw()
		if meta == nil {
			return
		}
		if namespace != "" && M(meta).String("namespace") != namespace {
			return
		}
		if matchesLabels(selector, meta) {
			fn(node)
		}
	})
}

func forEachPodMatchingCiliumSelector(nodes map[string]kube.Resource, namespace string, selector map[string]any, fn func(kube.Resource)) {
	forEachNodeOfType(nodes, "pod", func(node kube.Resource) {
		meta := M(node.AsMap()).Map("metadata").Raw()
		if meta == nil {
			return
		}
		if namespace != "" && M(meta).String("namespace") != namespace {
			return
		}
		if matchesCiliumLabels(selector, meta) {
			fn(node)
		}
	})
}

func hasOwnerKind(meta M, ownerKind string) bool {
	for _, owner := range extractOwnerReferences(meta) {
		if M(owner).String("kind") == ownerKind {
			return true
		}
	}
	return false
}

func stripLabelKey(labels map[string]any, key string) map[string]any {
	if len(labels) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(labels))
	for k, v := range labels {
		if k != key {
			out[k] = v
		}
	}
	return out
}

func ExtractMetaSpec(item *kube.Resource) (meta, spec M) {
	r := M(item.AsMap())
	return r.Map("metadata"), r.Map("spec")
}

func ExtractNamespace(meta M) string {
	if meta == nil {
		return ""
	}
	return meta.String("namespace")
}

func ExtractNamespacedName(item *kube.Resource) (namespace, name string) {
	meta := ResourceMeta(item).Raw()
	return M(meta).String("namespace"), M(meta).String("name")
}

func matchesLabels(selector map[string]any, meta map[string]any) bool {
	labels, ok := meta["labels"].(map[string]any)
	if !ok {
		return false
	}
	for k, v := range selector {
		selVal, ok := v.(string)
		labelVal, ok2 := labels[k].(string)
		if !ok || !ok2 || labelVal != selVal {
			return false
		}
	}
	return true
}

func matchesPattern(key, pattern string) bool {
	keyParts := strings.Split(key, "/")
	patternParts := strings.Split(pattern, "/")
	if len(patternParts) == 2 && len(keyParts) == 3 {
		return matchesSegment(keyParts[1], patternParts[0]) && matchesSegment(keyParts[2], patternParts[1])
	}
	if len(keyParts) != len(patternParts) {
		return false
	}
	for i, p := range patternParts {
		if !matchesSegment(keyParts[i], p) {
			return false
		}
	}
	return true
}

func matchesSegment(segment, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return segment == pattern
	}
	parts := strings.Split(pattern, "*")
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(segment[pos:], part)
		if idx == -1 || (i == 0 && !strings.HasPrefix(pattern, "*") && idx != 0) {
			return false
		}
		pos += idx + len(part)
	}
	return strings.HasSuffix(pattern, "*") || pos == len(segment)
}

func ParseSelectors(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	normalized := strings.NewReplacer(",", " ", "|", " ").Replace(s)
	tokens := strings.Fields(normalized)
	var result []string
	for _, token := range tokens {
		if strings.EqualFold(token, "or") {
			continue
		}
		result = append(result, token)
	}
	return result
}

func normalizeSelector(selector string, defaultNamespace string) string {
	if strings.Count(selector, "/") == 2 {
		return selector
	}
	if selector == "" {
		return "pod/" + defaultNamespace + "/*"
	}
	if selector == "*" {
		return "*/" + defaultNamespace + "/*"
	}
	parts := strings.Split(selector, "/")
	switch len(parts) {
	case 1:
		return "*/" + defaultNamespace + "/" + parts[0]
	case 2:
		return "*/" + parts[0] + "/" + parts[1]
	default:
		return selector
	}
}

func extractSelector(spec map[string]any) map[string]any {
	if spec == nil {
		return nil
	}
	if sel, ok := spec["selector"].(map[string]any); ok && len(sel) > 0 {
		return sel
	}
	if podSel, ok := spec["podSelector"].(map[string]any); ok {
		if ml, ok := podSel["matchLabels"].(map[string]any); ok && len(ml) > 0 {
			return ml
		}
	}
	return nil
}

func expandSelectors(selectors []string, defaultNamespace string, nodeMap map[string]kube.Resource) ([]string, error) {
	var result []string
	if len(selectors) == 0 {
		selectors = []string{"*/" + defaultNamespace + "/*"}
	}
	for _, selector := range selectors {
		selector = normalizeSelector(selector, defaultNamespace)
		if strings.Contains(selector, "*") {
			for key := range nodeMap {
				if matchesPattern(key, selector) {
					result = append(result, key)
				}
			}
		} else if _, exists := nodeMap[selector]; exists {
			result = append(result, selector)
		}
	}
	return result, nil
}

func parseNamespaces(selectors []string, defaultNamespace string, provider kube.Provider) map[string]bool {
	namespaces := map[string]bool{}
	if len(selectors) == 0 {
		namespaces[defaultNamespace] = true
		return namespaces
	}
	hasWildcard := false
	for _, selector := range selectors {
		selector = normalizeSelector(selector, defaultNamespace)
		parts := strings.Split(selector, "/")
		if len(parts) != 3 {
			continue
		}
		if parts[1] == "*" {
			hasWildcard = true
		} else {
			namespaces[parts[1]] = true
		}
	}
	if hasWildcard {
		if nsList, err := provider.GetNamespaces(context.Background(), metav1.ListOptions{}); err == nil && nsList != nil {
			for _, ns := range nsList.Items {
				namespaces[ns.Name] = true
			}
		} else {
			namespaces[defaultNamespace] = true
		}
	}
	return namespaces
}
