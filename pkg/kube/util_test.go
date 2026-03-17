package kube

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestTrimVerboseMetadata(t *testing.T) {
	obj := map[string]any{
		"metadata": map[string]any{
			"managedFields": []any{"x"},
			"annotations": map[string]any{
				"kubectl.kubernetes.io/last-applied-configuration": "big-payload",
				"keep": "value",
			},
		},
	}

	trimVerboseMetadata(obj)
	meta := obj["metadata"].(map[string]any)
	if _, ok := meta["managedFields"]; ok {
		t.Fatalf("expected managedFields to be removed")
	}
	ann, ok := meta["annotations"].(map[string]any)
	if !ok {
		t.Fatalf("expected annotations to remain when non-empty")
	}
	if _, ok := ann["kubectl.kubernetes.io/last-applied-configuration"]; ok {
		t.Fatalf("expected last-applied annotation to be removed")
	}
	if ann["keep"] != "value" {
		t.Fatalf("expected keep annotation to be preserved")
	}
}

func TestTrimVerboseMetadataRemovesEmptyAnnotations(t *testing.T) {
	obj := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				"kubectl.kubernetes.io/last-applied-configuration": "only-item",
			},
		},
	}

	trimVerboseMetadata(obj)
	meta := obj["metadata"].(map[string]any)
	if _, ok := meta["annotations"]; ok {
		t.Fatalf("expected empty annotations map to be removed")
	}
}

func TestRedactSecretMap(t *testing.T) {
	obj := map[string]any{
		"data": map[string]any{
			"password": "c3VwZXItc2VjcmV0",
		},
		"stringData": map[string]any{
			"token": "raw-token",
		},
	}

	redactSecretMap(obj)

	if obj["keyCount"] != 2 {
		t.Fatalf("expected keyCount=2, got %#v", obj["keyCount"])
	}
	data, _ := obj["data"].(map[string]any)
	if data["password"] != "<SECRET>" || data["token"] != "<SECRET>" {
		t.Fatalf("expected redacted keys, got %#v", data)
	}
	if _, ok := obj["stringData"]; ok {
		t.Fatalf("expected stringData to be removed after redaction")
	}
}

func TestFetchResourceRedactsSecretValues(t *testing.T) {
	model := NewModel()
	model.Secrets = []*corev1.Secret{{
		ObjectMeta: metav1.ObjectMeta{Namespace: "petshop", Name: "db-creds"},
		Data:       map[string][]byte{"password": []byte("super-secret")},
		StringData: map[string]string{"token": "raw-token"},
	}}
	provider := NewMockClient(model)

	res, err := provider.FetchResource("secret", "petshop", "db-creds", context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := res.AsMap()
	if m["keyCount"] != 2 {
		t.Fatalf("expected keyCount=2, got %#v", m["keyCount"])
	}
	data, _ := m["data"].(map[string]any)
	if data["password"] != "<SECRET>" || data["token"] != "<SECRET>" {
		t.Fatalf("expected redacted data map, got %#v", data)
	}
	if _, ok := m["stringData"]; ok {
		t.Fatalf("expected stringData to be removed")
	}
	if got := model.Secrets[0].StringData["token"]; got != "raw-token" {
		t.Fatalf("expected mock model to remain unchanged, got %q", got)
	}
}

func TestToMap(t *testing.T) {
	in := map[string]any{
		"metadata": map[string]any{
			"name":          "pod-1",
			"managedFields": []any{"to-remove"},
		},
	}

	got := toMap(in)
	meta := got["metadata"].(map[string]any)
	if meta["name"] != "pod-1" {
		t.Fatalf("expected metadata.name to be preserved, got %#v", meta["name"])
	}
	if _, ok := meta["managedFields"]; ok {
		t.Fatalf("expected managedFields to be trimmed")
	}
}

func TestToMapMarshalError(t *testing.T) {
	got := toMap(map[string]any{"bad": func() {}})
	if _, ok := got["marshalError"]; !ok {
		t.Fatalf("expected marshalError key, got %#v", got)
	}
}

func TestRequestSelectorsAndDefaultNamespace(t *testing.T) {
	req := Request{Selectors: []string{" pod/petshop/api ", " */management/* ", "", "service/default/web "}}
	sel := req.NormalizedSelectors()
	if len(sel) != 3 {
		t.Fatalf("expected 3 selectors, got %#v", sel)
	}
	if sel[0] != "pod/petshop/api" {
		t.Fatalf("expected first selector trimmed, got %q", sel[0])
	}
	if req.DefaultNamespace() != "petshop" {
		t.Fatalf("expected default namespace petshop, got %q", req.DefaultNamespace())
	}

	empty := Request{}
	if empty.NormalizedSelectors() != nil {
		t.Fatalf("expected nil selectors for empty request")
	}
	if empty.DefaultNamespace() != "" {
		t.Fatalf("expected empty default namespace for empty request")
	}
}

func TestDerefSlice(t *testing.T) {
	a := 1
	b := 2
	in := []*int{&a, nil, &b}
	got := derefSlice(in)
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("unexpected deref result: %#v", got)
	}
}

func TestGetCacheKey(t *testing.T) {
	if got := getCacheKey("pods", "default", metav1.ListOptions{}); got != "pods:default:*" {
		t.Fatalf("unexpected key: %q", got)
	}
	if got := getCacheKey("nodes", "", metav1.ListOptions{}); got != "nodes:cluster:*" {
		t.Fatalf("unexpected cluster key: %q", got)
	}
	if got := getCacheKey("deployments", "kube-system", metav1.ListOptions{LabelSelector: "app=web"}); got != "deployments:kube-system:app=web" {
		t.Fatalf("unexpected selector key: %q", got)
	}
}

func TestMockListBehavior(t *testing.T) {
	items := []int{1, 2}
	wrap := func(v []int) []int { return v }
	empty := []int{}

	got, err := mockList(MockConfig{}, "GetPods", empty, items, wrap)
	if err != nil || len(got) != 2 {
		t.Fatalf("expected success list, got=%#v err=%v", got, err)
	}

	got, err = mockList(MockConfig{AllEmpty: true}, "GetPods", empty, items, wrap)
	if err != nil || len(got) != 0 {
		t.Fatalf("expected all-empty list, got=%#v err=%v", got, err)
	}

	_, err = mockList(MockConfig{AllError: true}, "GetPods", empty, items, wrap)
	if err == nil {
		t.Fatalf("expected all-error")
	}

	cfg := MockConfig{Methods: map[string]MockMethodBehavior{"GetPods": {ReturnError: true, ErrorMessage: "boom"}}}
	_, err = mockList(cfg, "GetPods", empty, items, wrap)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected method error boom, got %v", err)
	}
}

func TestMockMapListBehavior(t *testing.T) {
	items := []map[string]any{{"name": "a"}}

	got, err := mockMapList(MockConfig{}, "GetPods", items)
	if err != nil || len(got) != 1 {
		t.Fatalf("expected success map list, got=%#v err=%v", got, err)
	}

	got, err = mockMapList(MockConfig{AllError: true}, "GetPods", items)
	if err == nil || len(got) != 0 {
		t.Fatalf("expected all-error empty list, got=%#v err=%v", got, err)
	}
}

func TestIsRetryableUtility(t *testing.T) {
	if isRetryable(nil) {
		t.Fatalf("nil error is not retryable")
	}
	if !isRetryable(kerrors.NewServiceUnavailable("svc unavailable")) {
		t.Fatalf("service unavailable should be retryable")
	}
	if isRetryable(errors.New("permission denied")) {
		t.Fatalf("generic error should not be retryable")
	}
}

func TestNewModel(t *testing.T) {
	m := NewModel()
	if m == nil {
		t.Fatalf("expected model to be created")
	}
	if m.PodLogs == nil {
		t.Fatalf("expected PodLogs map to be initialized")
	}
	if len(m.PodLogs) != 0 {
		t.Fatalf("expected empty PodLogs map, got %#v", m.PodLogs)
	}
}

func TestListDynamicResourceObjects(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	scheme := runtime.NewScheme()
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"namespace":     "petshop",
			"name":          "api-0",
			"managedFields": []any{"x"},
		},
	}}
	dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{gvr: "PodList"},
		obj,
	)

	got, err := listDynamicResourceObjects(dc, gvr, "petshop", true, context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one object, got %#v", got)
	}
	meta, _ := got[0]["metadata"].(map[string]any)
	if meta["name"] != "api-0" {
		t.Fatalf("expected pod name api-0, got %#v", meta["name"])
	}
	if _, ok := meta["managedFields"]; ok {
		t.Fatalf("expected managedFields to be trimmed")
	}
}

func TestListDynamicResourceObjectsErrorPath(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: "PodList"},
	)

	dc.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("list failed")
	})

	got, err := listDynamicResourceObjects(dc, gvr, "petshop", true, context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("expected nil error on list failure path, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result on list failure path, got %#v", got)
	}
}
