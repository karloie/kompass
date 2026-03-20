package kube

import (
	"context"
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestBuildResourceKey(t *testing.T) {
	if got := buildResourceKey("pod", "petshop", "api-0"); got != "pod/petshop/api-0" {
		t.Fatalf("unexpected namespaced key: %q", got)
	}
	if got := buildResourceKey("node", "", "worker-1"); got != "node/worker-1" {
		t.Fatalf("unexpected cluster key: %q", got)
	}
	if got := buildResourceKey("pod", "petshop", ""); got != "" {
		t.Fatalf("expected empty key for missing name, got %q", got)
	}
}

func TestNewResource(t *testing.T) {
	res, ok := newResource("service", "petshop", "api", map[string]any{"x": 1})
	if !ok {
		t.Fatalf("expected resource to be created")
	}
	if res.Key != "service/petshop/api" || res.Type != "service" {
		t.Fatalf("unexpected resource: %#v", res)
	}

	_, ok = newResource("service", "petshop", "", map[string]any{})
	if ok {
		t.Fatalf("expected resource creation to fail with empty name")
	}
}

func TestNamespacedLoad(t *testing.T) {
	loader := namespacedLoad[corev1.ConfigMap, *corev1.ConfigMap, *corev1.ConfigMapList](
		"configmap",
		func(_ Provider, _ string, _ context.Context, _ metav1.ListOptions) (*corev1.ConfigMapList, error) {
			return &corev1.ConfigMapList{Items: []corev1.ConfigMap{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "petshop", Name: "cfg"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: "petshop", Name: ""}},
			}}, nil
		},
		func(l *corev1.ConfigMapList) []corev1.ConfigMap { return l.Items },
	)

	out, err := loader(nil, "petshop", context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one valid resource, got %#v", out)
	}
	if out[0].Key != "configmap/petshop/cfg" {
		t.Fatalf("unexpected key: %q", out[0].Key)
	}
}

func TestClusterLoad(t *testing.T) {
	loader := clusterLoad[corev1.Node, *corev1.Node, *corev1.NodeList](
		"node",
		func(_ Provider, _ context.Context, _ metav1.ListOptions) (*corev1.NodeList, error) {
			return &corev1.NodeList{Items: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: ""}},
			}}, nil
		},
		func(l *corev1.NodeList) []corev1.Node { return l.Items },
	)

	out, err := loader(nil, "", context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].Key != "node/worker-1" {
		t.Fatalf("unexpected cluster resources: %#v", out)
	}
}

func TestNamespacedLoadSkipsForbidden(t *testing.T) {
	loader := namespacedLoad[corev1.ConfigMap, *corev1.ConfigMap, *corev1.ConfigMapList](
		"configmap",
		func(_ Provider, _ string, _ context.Context, _ metav1.ListOptions) (*corev1.ConfigMapList, error) {
			return nil, apierrors.NewForbidden(schema.GroupResource{Group: "", Resource: "configmaps"}, "", nil)
		},
		func(l *corev1.ConfigMapList) []corev1.ConfigMap { return l.Items },
	)

	out, err := loader(nil, "petshop", context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("expected forbidden to be skipped, got %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty result, got %#v", out)
	}
}

func TestClusterLoadSkipsForbidden(t *testing.T) {
	loader := clusterLoad[corev1.Node, *corev1.Node, *corev1.NodeList](
		"node",
		func(_ Provider, _ context.Context, _ metav1.ListOptions) (*corev1.NodeList, error) {
			return nil, apierrors.NewForbidden(schema.GroupResource{Group: "", Resource: "nodes"}, "", nil)
		},
		func(l *corev1.NodeList) []corev1.Node { return l.Items },
	)

	out, err := loader(nil, "", context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("expected forbidden to be skipped, got %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty result, got %#v", out)
	}
}

func TestConditionBasedLoad(t *testing.T) {
	t.Run("provider missing", func(t *testing.T) {
		loader := conditionBasedLoad(
			"certificate",
			func(Provider) (any, bool) { return nil, false },
			func(any, string, context.Context, metav1.ListOptions) ([]map[string]any, error) { return nil, nil },
			true,
		)
		out, err := loader(nil, "petshop", context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("expected nil error when provider missing, got err=%v", err)
		}
		if len(out) != 0 {
			t.Fatalf("expected empty result when provider missing, got %#v", out)
		}
	})

	t.Run("getter error", func(t *testing.T) {
		loader := conditionBasedLoad(
			"certificate",
			func(Provider) (any, bool) { return struct{}{}, true },
			func(any, string, context.Context, metav1.ListOptions) ([]map[string]any, error) {
				return nil, errors.New("boom")
			},
			true,
		)
		_, err := loader(nil, "petshop", context.Background(), metav1.ListOptions{})
		if err == nil || err.Error() != "boom" {
			t.Fatalf("expected boom error, got %v", err)
		}
	})

	t.Run("getter forbidden", func(t *testing.T) {
		loader := conditionBasedLoad(
			"certificate",
			func(Provider) (any, bool) { return struct{}{}, true },
			func(any, string, context.Context, metav1.ListOptions) ([]map[string]any, error) {
				return nil, apierrors.NewForbidden(schema.GroupResource{Group: "cert-manager.io", Resource: "certificates"}, "", nil)
			},
			true,
		)
		out, err := loader(nil, "petshop", context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("expected forbidden to be skipped, got %v", err)
		}
		if len(out) != 0 {
			t.Fatalf("expected empty result, got %#v", out)
		}
	})

	t.Run("filters invalid metadata", func(t *testing.T) {
		loader := conditionBasedLoad(
			"certificate",
			func(Provider) (any, bool) { return struct{}{}, true },
			func(any, string, context.Context, metav1.ListOptions) ([]map[string]any, error) {
				return []map[string]any{
					{"metadata": map[string]any{"namespace": "petshop", "name": "ok"}},
					{"metadata": map[string]any{"namespace": "petshop", "name": ""}},
					{"metadata": map[string]any{"name": "missing-ns"}},
				}, nil
			},
			true,
		)
		out, err := loader(nil, "petshop", context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) != 1 || out[0].Key != "certificate/petshop/ok" {
			t.Fatalf("unexpected filtered result: %#v", out)
		}
	})
}

func TestWorkloadLoad(t *testing.T) {
	loader := workloadLoad("deployment", func(_ Provider, _ string, _ context.Context, _ metav1.ListOptions) (any, error) {
		return &appsv1.DeploymentList{Items: []appsv1.Deployment{
			{ObjectMeta: metav1.ObjectMeta{Namespace: "petshop", Name: "api"}},
			{ObjectMeta: metav1.ObjectMeta{Namespace: "petshop", Name: ""}},
		}}, nil
	})

	out, err := loader(nil, "petshop", context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].Key != "deployment/petshop/api" {
		t.Fatalf("unexpected workload output: %#v", out)
	}
}

func TestWorkloadLoadSkipsForbidden(t *testing.T) {
	loader := workloadLoad("deployment", func(_ Provider, _ string, _ context.Context, _ metav1.ListOptions) (any, error) {
		return nil, apierrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "deployments"}, "", nil)
	})

	out, err := loader(nil, "petshop", context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("expected forbidden to be skipped, got %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty result, got %#v", out)
	}
}

func TestGetLoader(t *testing.T) {
	if getLoader("configmap") == nil {
		t.Fatalf("expected configmap loader to exist")
	}
	if getLoader("does-not-exist") != nil {
		t.Fatalf("expected unknown loader to be nil")
	}
}

func TestLoadSecretRedactsValues(t *testing.T) {
	model := NewModel()
	model.Secrets = []*corev1.Secret{{
		ObjectMeta: metav1.ObjectMeta{Namespace: "petshop", Name: "db-creds"},
		Data:       map[string][]byte{"password": []byte("super-secret")},
		StringData: map[string]string{"token": "raw-token"},
	}}
	provider := NewMockClient(model)

	out, err := LoadSecret(provider, "petshop", context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one secret, got %#v", out)
	}
	res := out[0]
	if res.Key != "secret/petshop/db-creds" {
		t.Fatalf("unexpected secret key: %q", res.Key)
	}
	m := res.AsMap()
	if m["keyCount"] != 2 {
		t.Fatalf("expected keyCount=2, got %#v", m["keyCount"])
	}
	data, _ := m["data"].(map[string]any)
	if data["password"] != "<SECRET>" || data["token"] != "<SECRET>" {
		t.Fatalf("expected redacted data map, got %#v", data)
	}
}

func TestLoadPod(t *testing.T) {
	model := NewModel()
	model.Pods = []*corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Namespace: "petshop", Name: "api-0"}}}
	provider := NewMockClient(model)

	out, err := LoadPod(provider, "petshop", context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].Key != "pod/petshop/api-0" {
		t.Fatalf("unexpected pod load result: %#v", out)
	}
}

func TestLoadService(t *testing.T) {
	model := NewModel()
	model.Services = []*corev1.Service{{ObjectMeta: metav1.ObjectMeta{Namespace: "petshop", Name: "api"}}}
	provider := NewMockClient(model)

	out, err := LoadService(provider, "petshop", context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].Key != "service/petshop/api" {
		t.Fatalf("unexpected service load result: %#v", out)
	}
}

func TestLoadNode(t *testing.T) {
	model := NewModel()
	model.Nodes = []*corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}}}
	provider := NewMockClient(model)

	out, err := LoadNode(provider, "", context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].Key != "node/worker-1" {
		t.Fatalf("unexpected node load result: %#v", out)
	}
}

func TestLoadJob(t *testing.T) {
	model := NewModel()
	model.Jobs = []*batchv1.Job{{ObjectMeta: metav1.ObjectMeta{Namespace: "petshop", Name: "batch-1"}}}
	provider := NewMockClient(model)

	out, err := LoadJob(provider, "petshop", context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].Key != "job/petshop/batch-1" {
		t.Fatalf("unexpected job load result: %#v", out)
	}
}

func TestLoadNetworkPolicy(t *testing.T) {
	model := NewModel()
	model.NetworkPolicies = []*networkingv1.NetworkPolicy{{ObjectMeta: metav1.ObjectMeta{Namespace: "petshop", Name: "deny-all"}}}
	provider := NewMockClient(model)

	out, err := LoadNetworkPolicy(provider, "petshop", context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].Key != "networkpolicy/petshop/deny-all" {
		t.Fatalf("unexpected networkpolicy load result: %#v", out)
	}
}

func TestConditionProviderLoadersReturnNilWhenUnsupported(t *testing.T) {
	provider := NewMockClient(NewModel())

	cases := []struct {
		name   string
		loader ResourceLoader
	}{
		{name: "cilium-namespaced", loader: LoadCiliumNetworkPolicy},
		{name: "cilium-clusterwide", loader: LoadCiliumClusterwideNetworkPolicy},
		{name: "certificate", loader: LoadCertificate},
		{name: "issuer", loader: LoadIssuer},
		{name: "clusterissuer", loader: LoadClusterIssuer},
		{name: "httproute", loader: LoadHTTPRoute},
		{name: "gateway", loader: LoadGateway},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.loader(provider, "petshop", context.Background(), metav1.ListOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("expected empty output when no resources are available, got %#v", out)
			}
		})
	}
}
