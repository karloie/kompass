package graph

import (
	"reflect"
	"sort"
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/mock"
)

func TestMAccessors(t *testing.T) {
	m := M{
		"s":    "value",
		"i":    7,
		"f":    3.9,
		"b":    true,
		"map":  map[string]any{"k": "v"},
		"list": []any{map[string]any{"a": 1}, "skip"},
	}

	if got := m.String("s"); got != "value" {
		t.Fatalf("String mismatch: got %q", got)
	}
	if _, ok := m.StringOk("s"); !ok {
		t.Fatalf("StringOk expected true")
	}
	if got := m.Int("i"); got != 7 {
		t.Fatalf("Int mismatch for int: got %d", got)
	}
	if got := m.Int("f"); got != 3 {
		t.Fatalf("Int mismatch for float64: got %d", got)
	}
	if got := m.BoolOr("b", false); !got {
		t.Fatalf("BoolOr expected true")
	}
	if got := m.BoolOr("missing", true); !got {
		t.Fatalf("BoolOr default expected true")
	}
	if !m.Exists("s") || m.Exists("missing") {
		t.Fatalf("Exists mismatch")
	}

	mv := m.Map("map")
	if mv == nil || mv.String("k") != "v" {
		t.Fatalf("Map accessor mismatch")
	}
	if ms := m.MapSlice("list"); len(ms) != 1 || ms[0].Int("a") != 1 {
		t.Fatalf("MapSlice mismatch: %#v", ms)
	}
	if raw := m.Raw(); raw["s"] != "value" {
		t.Fatalf("Raw mismatch")
	}
}

func TestPathAndNilSafety(t *testing.T) {
	m := M{"a": map[string]any{"b": map[string]any{"c": "ok"}}}
	if got := m.Path("a", "b").String("c"); got != "ok" {
		t.Fatalf("Path mismatch: got %q", got)
	}
	if got := m.Path("a", "x"); got != nil {
		t.Fatalf("expected nil path for missing segment")
	}

	var nilM M
	if nilM.String("x") != "" || nilM.Int("x") != 0 || nilM.BoolOr("x", true) != true {
		t.Fatalf("nil M accessors did not return defaults")
	}
}

func TestKeyAndResourceHelpers(t *testing.T) {
	r := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "api"},
		"spec":     map[string]any{"replicas": 2},
	}}

	if got := Key("deployment", "petshop", "api"); got != "deployment/petshop/api" {
		t.Fatalf("Key mismatch: %q", got)
	}
	if got := Key("clusterissuer", "", "letsencrypt"); got != "clusterissuer/letsencrypt" {
		t.Fatalf("cluster key mismatch: %q", got)
	}
	if got := ResourceKey(r, "deployment"); got != "deployment/petshop/api" {
		t.Fatalf("ResourceKey mismatch: %q", got)
	}
	if ns := ExtractNamespace(ResourceMeta(r)); ns != "petshop" {
		t.Fatalf("ExtractNamespace mismatch: %q", ns)
	}
	ns, name := ExtractNamespacedName(r)
	if ns != "petshop" || name != "api" {
		t.Fatalf("ExtractNamespacedName mismatch: %s/%s", ns, name)
	}
	if ResourceSpec(r).Int("replicas") != 2 {
		t.Fatalf("ResourceSpec mismatch")
	}
}

func TestEdgeAndNodeHelpers(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{}
	item := &kube.Resource{Resource: map[string]any{"metadata": map[string]any{"namespace": "petshop", "name": "api"}}}

	key := addNode(&edges, item, &nodes, "deployment")
	if key == "" {
		t.Fatalf("expected node key")
	}
	if _, ok := nodes[key]; !ok {
		t.Fatalf("node not inserted")
	}

	addEdgeIfNodeExists(&edges, nodes, key, key, "owns")
	addEdgeIfNodeExists(&edges, nodes, key, "missing/key", "owns")
	if len(edges) != 1 || edges[0].Label != "owns" {
		t.Fatalf("unexpected edges: %#v", edges)
	}
}

func TestSelectorAndPatternHelpers(t *testing.T) {
	cases := []struct {
		key     string
		pattern string
		want    bool
	}{
		{"pod/petshop/api", "pod/petshop/api", true},
		{"pod/petshop/api", "*/petshop/*", true},
		{"service/petshop/api", "*/petshop/ap*", true},
		{"service/petshop/api", "*/management/*", false},
		{"pod/petshop/api", "petshop/*", true},
	}
	for _, tc := range cases {
		if got := matchesPattern(tc.key, tc.pattern); got != tc.want {
			t.Fatalf("matchesPattern(%q,%q)=%v want %v", tc.key, tc.pattern, got, tc.want)
		}
	}

	if !matchesSegment("api-service", "api*") || matchesSegment("api", "web*") {
		t.Fatalf("matchesSegment behavior mismatch")
	}

	if got := normalizeSelector("", "petshop"); got != "pod/petshop/*" {
		t.Fatalf("normalize empty mismatch: %q", got)
	}
	if got := normalizeSelector("*", "petshop"); got != "*/petshop/*" {
		t.Fatalf("normalize wildcard mismatch: %q", got)
	}
	if got := normalizeSelector("api", "petshop"); got != "*/petshop/api" {
		t.Fatalf("normalize name mismatch: %q", got)
	}
	if got := normalizeSelector("ns/api", "petshop"); got != "*/ns/api" {
		t.Fatalf("normalize ns/name mismatch: %q", got)
	}
}

func TestParseAndExpandSelectors(t *testing.T) {
	if got := ParseSelectors(" a , , b "); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("ParseSelectors mismatch: %#v", got)
	}

	nodeMap := map[string]kube.Resource{
		"pod/petshop/api":        {Key: "pod/petshop/api", Type: "pod"},
		"service/petshop/api":    {Key: "service/petshop/api", Type: "service"},
		"pod/management/webhook": {Key: "pod/management/webhook", Type: "pod"},
	}
	keys, err := expandSelectors([]string{"*/petshop/*", "pod/management/webhook"}, "petshop", nodeMap)
	if err != nil {
		t.Fatalf("expandSelectors err: %v", err)
	}
	sort.Strings(keys)
	want := []string{"pod/management/webhook", "pod/petshop/api", "service/petshop/api"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("expandSelectors mismatch: got %#v want %#v", keys, want)
	}
}

func TestLabelAndSelectorExtractionHelpers(t *testing.T) {
	meta := map[string]any{"labels": map[string]any{"app": "api", "tier": "backend"}}
	if !matchesLabels(map[string]any{"app": "api"}, meta) {
		t.Fatalf("matchesLabels expected true")
	}
	if matchesLabels(map[string]any{"app": "web"}, meta) {
		t.Fatalf("matchesLabels expected false")
	}

	labels := stripLabelKey(map[string]any{"a": 1, "b": 2}, "a")
	if _, ok := labels["a"]; ok || labels["b"] != 2 {
		t.Fatalf("stripLabelKey mismatch: %#v", labels)
	}

	sel := extractSelector(map[string]any{"selector": map[string]any{"app": "api"}})
	if sel["app"] != "api" {
		t.Fatalf("extractSelector selector mismatch: %#v", sel)
	}
	podSel := extractSelector(map[string]any{"podSelector": map[string]any{"matchLabels": map[string]any{"app": "api"}}})
	if podSel["app"] != "api" {
		t.Fatalf("extractSelector podSelector mismatch: %#v", podSel)
	}
}

func TestNodeIterationAndOwnerHelpers(t *testing.T) {
	nodes := map[string]kube.Resource{
		"pod/petshop/api": {Type: "pod", Resource: map[string]any{"metadata": map[string]any{"namespace": "petshop", "labels": map[string]any{"app": "api"}}}},
		"pod/petshop/db":  {Type: "pod", Resource: map[string]any{"metadata": map[string]any{"namespace": "petshop", "labels": map[string]any{"app": "db"}}}},
	}

	count := 0
	forEachNodeOfType(nodes, "pod", func(kube.Resource) { count++ })
	if count != 2 {
		t.Fatalf("forEachNodeOfType mismatch: %d", count)
	}

	matched := 0
	forEachPodMatchingSelector(nodes, "petshop", map[string]any{"app": "api"}, func(kube.Resource) { matched++ })
	if matched != 1 {
		t.Fatalf("forEachPodMatchingSelector mismatch: %d", matched)
	}

	meta := M{"ownerReferences": []any{map[string]any{"kind": "Deployment", "name": "api"}}}
	if !hasOwnerKind(meta, "Deployment") || hasOwnerKind(meta, "StatefulSet") {
		t.Fatalf("hasOwnerKind mismatch")
	}
}

func TestParseNamespaces(t *testing.T) {
	provider := kube.NewMockClient(mock.GenerateMock())
	ns := parseNamespaces([]string{"*/petshop/*", "*/management/*"}, "petshop", provider)
	if !ns["petshop"] || !ns["management"] {
		t.Fatalf("expected explicit namespaces to be present: %#v", ns)
	}

	nsEmpty := parseNamespaces(nil, "petshop", provider)
	if !nsEmpty["petshop"] {
		t.Fatalf("expected default namespace when selectors are empty")
	}

	errProvider := kube.NewMockClient(mock.GenerateMock(), kube.MockConfig{
		Methods: map[string]kube.MockMethodBehavior{
			"GetNamespaces": {ReturnError: true},
		},
	})
	nsFallback := parseNamespaces([]string{"*/*/*"}, "petshop", errProvider)
	if !nsFallback["petshop"] {
		t.Fatalf("expected fallback to default namespace on wildcard load error: %#v", nsFallback)
	}
}
