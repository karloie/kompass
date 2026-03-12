package kube

import (
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestConfigureRequestTimeout(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		if err := configureRequestTimeout(nil); err != nil {
			t.Fatalf("expected nil error for nil config, got %v", err)
		}
	})

	t.Run("preserve existing timeout", func(t *testing.T) {
		cfg := &rest.Config{Timeout: 3 * time.Second}
		t.Setenv(requestTimeoutEnvVar, "99")
		if err := configureRequestTimeout(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Timeout != 3*time.Second {
			t.Fatalf("expected existing timeout to be preserved, got %v", cfg.Timeout)
		}
	})

	t.Run("default timeout when env empty", func(t *testing.T) {
		cfg := &rest.Config{}
		t.Setenv(requestTimeoutEnvVar, "")
		if err := configureRequestTimeout(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Timeout != defaultRequestTimeout {
			t.Fatalf("expected default timeout %v, got %v", defaultRequestTimeout, cfg.Timeout)
		}
	})

	t.Run("integer seconds env", func(t *testing.T) {
		cfg := &rest.Config{}
		t.Setenv(requestTimeoutEnvVar, "20")
		if err := configureRequestTimeout(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Timeout != 20*time.Second {
			t.Fatalf("expected 20s timeout, got %v", cfg.Timeout)
		}
	})

	t.Run("duration env", func(t *testing.T) {
		cfg := &rest.Config{}
		t.Setenv(requestTimeoutEnvVar, "1500ms")
		if err := configureRequestTimeout(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Timeout != 1500*time.Millisecond {
			t.Fatalf("expected 1500ms timeout, got %v", cfg.Timeout)
		}
	})

	t.Run("invalid env format", func(t *testing.T) {
		cfg := &rest.Config{}
		t.Setenv(requestTimeoutEnvVar, "not-a-duration")
		if err := configureRequestTimeout(cfg); err == nil {
			t.Fatalf("expected error for invalid timeout env")
		}
	})

	t.Run("zero seconds integer", func(t *testing.T) {
		cfg := &rest.Config{}
		t.Setenv(requestTimeoutEnvVar, "0")
		if err := configureRequestTimeout(cfg); err == nil {
			t.Fatalf("expected error for zero integer timeout")
		}
	})

	t.Run("negative duration", func(t *testing.T) {
		cfg := &rest.Config{}
		t.Setenv(requestTimeoutEnvVar, "-5s")
		if err := configureRequestTimeout(cfg); err == nil {
			t.Fatalf("expected error for negative duration")
		}
	})
}

func TestWarningHandlerHandleWarningHeader(t *testing.T) {
	var h warningHandler
	h.HandleWarningHeader(299, "apiserver", "Endpoints is deprecated in v1.33+")
	h.HandleWarningHeader(299, "apiserver", "some other warning")
}

func TestResourceAsMap(t *testing.T) {
	t.Run("nil resource", func(t *testing.T) {
		r := &Resource{}
		if got := r.AsMap(); got != nil {
			t.Fatalf("expected nil map, got %#v", got)
		}
	})

	t.Run("map passthrough", func(t *testing.T) {
		m := map[string]any{"k": "v"}
		r := &Resource{Resource: m}
		got := r.AsMap()
		if got["k"] != "v" {
			t.Fatalf("expected passthrough map, got %#v", got)
		}
	})

	t.Run("struct marshal", func(t *testing.T) {
		type sample struct {
			Name string `json:"name"`
		}
		r := &Resource{Resource: sample{Name: "pod-1"}}
		got := r.AsMap()
		if got["name"] != "pod-1" {
			t.Fatalf("expected marshaled name pod-1, got %#v", got)
		}
	})

	t.Run("marshal error", func(t *testing.T) {
		type badPayload struct {
			Bad func() `json:"bad"`
		}
		r := &Resource{Resource: badPayload{Bad: func() {}}}
		got := r.AsMap()
		if _, ok := got["marshalError"]; !ok {
			t.Fatalf("expected marshalError key, got %#v", got)
		}
	})
}

func TestNewMockClientDefaultsAndAccessors(t *testing.T) {
	cfg := MockConfig{AllEmpty: true}
	c := NewMockClient(nil, cfg)
	if !c.IsMockMode() {
		t.Fatalf("expected mock mode")
	}
	if c.GetMockModel() == nil {
		t.Fatalf("expected mock model to be initialized")
	}
	if !c.mockConfig.AllEmpty {
		t.Fatalf("expected mock config to be applied")
	}
}

func TestNewClientWithClientsetDefaultsAndHostKubeconfig(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	c := NewClientWithClientset(clientset, nil, nil, "", "")
	if c.context != "injected-cluster" {
		t.Fatalf("expected default injected context, got %q", c.context)
	}
	if c.namespace != "default" {
		t.Fatalf("expected default namespace, got %q", c.namespace)
	}
	if c.kubeconfig != "injected" {
		t.Fatalf("expected injected kubeconfig marker, got %q", c.kubeconfig)
	}

	cfg := &rest.Config{Host: "https://example"}
	c2 := NewClientWithClientset(clientset, nil, cfg, "ctx", "ns")
	if c2.kubeconfig != "https://example" {
		t.Fatalf("expected kubeconfig from config host, got %q", c2.kubeconfig)
	}
	if c2.context != "ctx" || c2.namespace != "ns" {
		t.Fatalf("expected explicit context/namespace to be preserved, got %q/%q", c2.context, c2.namespace)
	}
}
