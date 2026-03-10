package kube

import (
	"context"
	"errors"
	"testing"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestClientIoC(t *testing.T) {
	fakeClientset := fake.NewSimpleClientset()
	client := NewClientWithClientset(fakeClientset, nil, nil, "test-context", "test-namespace")
	if client == nil {
		t.Fatal("Expected client to be created")
	}
	if client.GetClientset() != fakeClientset {
		t.Error("Retrieved clientset does not match injected clientset")
	}
	if ctx, err := client.GetContext(); err != nil {
		t.Fatalf("GetContext() error: %v", err)
	} else if ctx != "test-context" {
		t.Errorf("Expected context 'test-context', got '%s'", ctx)
	}
	if ns, err := client.GetNamespace(); err != nil {
		t.Fatalf("GetNamespace() error: %v", err)
	} else if ns != "test-namespace" {
		t.Errorf("Expected namespace 'test-namespace', got '%s'", ns)
	}
	if client.IsMockMode() {
		t.Error("Client should not be in mock mode")
	}
}

func TestSetClientset(t *testing.T) {
	fakeClientset1 := fake.NewSimpleClientset()
	client := NewClientWithClientset(fakeClientset1, nil, nil, "context1", "namespace1")
	if client.GetClientset() != fakeClientset1 {
		t.Error("Initial clientset mismatch")
	}
	fakeClientset2 := fake.NewSimpleClientset()
	client.SetClientset(fakeClientset2, nil, nil)
	if client.GetClientset() != fakeClientset2 {
		t.Error("Clientset was not replaced")
	}
	if ctx, _ := client.GetContext(); ctx != "context1" {
		t.Errorf("Context should not have changed, got '%s'", ctx)
	}
}

func TestMockClientImmutable(t *testing.T) {
	mockClient := NewMockClient(nil)
	if !mockClient.IsMockMode() {
		t.Error("Mock client should be in mock mode")
	}
	if mockClient.GetClientset() != nil {
		t.Error("Mock client should return nil clientset")
	}
	fakeClientset := fake.NewSimpleClientset()
	mockClient.SetClientset(fakeClientset, nil, nil)
	if mockClient.GetClientset() != nil {
		t.Error("Mock client should remain immutable")
	}
}

func TestCacheEnabledByDefault(t *testing.T) {
	client := NewClientWithClientset(fake.NewSimpleClientset(), nil, nil, "test-context", "default")
	if !client.cacheEnabled {
		t.Error("Cache should be enabled by default")
	}
	if !client.GetCacheStats()["enabled"].(bool) {
		t.Error("Cache stats should show enabled")
	}
}

func TestCacheBasicOperations(t *testing.T) {
	rc := newResourceCache()
	rc.set("test-key", "test-value", 100*time.Millisecond)
	if value, ok := rc.get("test-key"); !ok || value != "test-value" {
		t.Errorf("Expected cache hit with 'test-value', got %v, %v", value, ok)
	}
	if rc.size() != 1 {
		t.Errorf("Expected cache size 1, got %d", rc.size())
	}
	time.Sleep(150 * time.Millisecond)
	if _, ok := rc.get("test-key"); ok {
		t.Error("Expected cache miss after expiration")
	}
	rc.clear()
	rc.set("key1", "value1", time.Second)
	rc.set("key2", "value2", time.Second)
	if rc.size() != 2 {
		t.Errorf("Expected cache size 2, got %d", rc.size())
	}
	rc.clear()
	if rc.size() != 0 {
		t.Errorf("Expected cache size 0 after clear, got %d", rc.size())
	}
}

func TestCacheKeyGeneration(t *testing.T) {
	client := NewClientWithClientset(fake.NewSimpleClientset(), nil, nil, "test-context", "default")
	tests := []struct {
		name, resourceType, namespace, expected string
		opts                                    metav1.ListOptions
	}{
		{"basic key", "pods", "default", "pods:default:*", metav1.ListOptions{}},
		{"with label selector", "deployments", "kube-system", "deployments:kube-system:app=nginx", metav1.ListOptions{LabelSelector: "app=nginx"}},
		{"cluster scoped", "nodes", "", "nodes:cluster:*", metav1.ListOptions{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if key := client.getCacheKey(tt.resourceType, tt.namespace, tt.opts); key != tt.expected {
				t.Errorf("Expected key %q, got %q", tt.expected, key)
			}
		})
	}
}

func TestStartStopSync(t *testing.T) {
	client := NewClientWithClientset(fake.NewSimpleClientset(), nil, nil, "test-context", "default")
	if err := client.StartSync(100*time.Millisecond, []string{"default"}); err != nil {
		t.Fatalf("Failed to start sync: %v", err)
	}
	if !client.cacheEnabled {
		t.Error("Cache should be enabled after StartSync")
	}
	time.Sleep(150 * time.Millisecond)
	if lastSync := client.GetCacheStats()["lastSync"].(time.Time); lastSync.IsZero() {
		t.Error("Expected lastSync to be set after sync")
	}
	client.StopSync()
	if client.syncCancel != nil {
		t.Error("Expected syncCancel to be nil after StopSync")
	}
}

func TestCachedGetReadThrough(t *testing.T) {
	client := NewClientWithClientset(fake.NewSimpleClientset(), nil, nil, "test-context", "default")
	client.cacheEnabled = true
	ctx, opts := context.Background(), metav1.ListOptions{}
	if _, err := client.GetPods("default", ctx, opts); err != nil {
		t.Fatalf("GetPods failed: %v", err)
	}
	cacheKey := client.getCacheKey("pods", "default", opts)
	cached, ok := client.cache.get(cacheKey)
	if !ok {
		t.Error("Expected pods to be cached")
	}
	if pods2, err := client.GetPods("default", ctx, opts); err != nil {
		t.Fatalf("Second GetPods failed: %v", err)
	} else if cached != pods2 {
		t.Error("Expected second call to return cached value")
	}
}

func TestCacheStats(t *testing.T) {
	stats := NewClientWithClientset(fake.NewSimpleClientset(), nil, nil, "test-context", "default").GetCacheStats()
	if _, hasEnabled := stats["enabled"]; !hasEnabled {
		t.Error("Cache stats missing 'enabled'")
	}
	if _, hasSize := stats["size"]; !hasSize {
		t.Error("Cache stats missing 'size'")
	}
	if _, hasTTL := stats["ttl"]; !hasTTL {
		t.Error("Cache stats missing 'ttl'")
	}
	if _, hasSyncInterval := stats["syncInterval"]; !hasSyncInterval {
		t.Error("Cache stats missing 'syncInterval'")
	}
	if _, hasLastSync := stats["lastSync"]; !hasLastSync {
		t.Error("Cache stats missing 'lastSync'")
	}
	if !stats["enabled"].(bool) {
		t.Error("Cache should be enabled by default")
	}
	if stats["size"].(int) != 0 {
		t.Errorf("Expected initial cache size 0, got %d", stats["size"])
	}
	if stats["ttl"].(time.Duration) != 30*time.Second {
		t.Errorf("Expected default TTL 30s, got %v", stats["ttl"])
	}
}

func TestRetryWithBackoff(t *testing.T) {
	client := NewClientWithClientset(fake.NewSimpleClientset(), nil, nil, "test-context", "default")
	client.maxRetries = 3
	client.initialBackoff = 10 * time.Millisecond
	client.maxBackoff = 100 * time.Millisecond

	t.Run("immediate success", func(t *testing.T) {
		attempts := 0
		result, err := retryWithBackoff(client, func() (string, error) {
			attempts++
			return "success", nil
		})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "success" {
			t.Errorf("Expected 'success', got %q", result)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("eventual success after retries", func(t *testing.T) {
		attempts := 0
		result, err := retryWithBackoff(client, func() (string, error) {
			attempts++
			if attempts < 3 {

				return "", kerrors.NewTimeoutError("timeout", 1)
			}
			return "success", nil
		})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "success" {
			t.Errorf("Expected 'success', got %q", result)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		attempts := 0
		result, err := retryWithBackoff(client, func() (string, error) {
			attempts++
			return "", kerrors.NewServiceUnavailable("persistent error")
		})
		if err == nil {
			t.Error("Expected error after max retries")
		}
		if result != "" {
			t.Errorf("Expected empty result, got %q", result)
		}
		if attempts != client.maxRetries+1 {
			t.Errorf("Expected %d attempts, got %d", client.maxRetries+1, attempts)
		}
	})

	t.Run("non-retryable error", func(t *testing.T) {
		attempts := 0
		nonRetryableErr := errors.New("permissions denied")
		result, err := retryWithBackoff(client, func() (string, error) {
			attempts++
			return "", nonRetryableErr
		})
		if err != nonRetryableErr {
			t.Errorf("Expected non-retryable error, got %v", err)
		}
		if result != "" {
			t.Errorf("Expected empty result, got %q", result)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt (no retry), got %d", attempts)
		}
	})
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil error", nil, false},
		{"timeout error", &mockRetryableError{message: "timeout"}, true},
		{"server timeout", &mockRetryableError{message: "server timeout"}, true},
		{"service unavailable", &mockRetryableError{message: "service unavailable"}, true},
		{"too many requests", &mockRetryableError{message: "too many requests"}, true},
		{"internal error", &mockRetryableError{message: "internal error"}, true},
		{"regular error", errors.New("regular error"), false},
		{"not found", createK8sError(404, "not found"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryable(tt.err); got != tt.retryable {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, got, tt.retryable)
			}
		})
	}
}

type mockRetryableError struct {
	message string
}

func (e *mockRetryableError) Error() string {
	return e.message
}

func (e *mockRetryableError) Status() metav1.Status {
	var reason metav1.StatusReason
	switch e.message {
	case "timeout":
		reason = metav1.StatusReasonTimeout
	case "server timeout":
		reason = metav1.StatusReasonServerTimeout
	case "service unavailable":
		reason = metav1.StatusReasonServiceUnavailable
	case "too many requests":
		reason = metav1.StatusReasonTooManyRequests
	case "internal error":
		reason = metav1.StatusReasonInternalError
	default:
		reason = metav1.StatusReasonUnknown
	}
	return metav1.Status{Reason: reason}
}

func createK8sError(code int32, message string) error {
	return &apiStatusError{
		ErrStatus: metav1.Status{
			Code:    code,
			Message: message,
		},
	}
}

type apiStatusError struct {
	ErrStatus metav1.Status
}

func (e *apiStatusError) Error() string {
	return e.ErrStatus.Message
}

func (e *apiStatusError) Status() metav1.Status {
	return e.ErrStatus
}

func TestCacheStatistics(t *testing.T) {
	client := NewClientWithClientset(fake.NewSimpleClientset(), nil, nil, "test-context", "default")
	client.cacheEnabled = true
	ctx, opts := context.Background(), metav1.ListOptions{}

	stats := client.GetCacheStats()
	if stats["calls"].(int64) != 0 || stats["hits"].(int64) != 0 || stats["misses"].(int64) != 0 {
		t.Error("Expected all cache stats to be 0 initially")
	}

	if _, err := client.GetPods("default", ctx, opts); err != nil {
		t.Fatalf("GetPods failed: %v", err)
	}

	stats = client.GetCacheStats()
	if stats["calls"].(int64) != 1 {
		t.Errorf("Expected 1 cache call, got %d", stats["calls"].(int64))
	}
	if stats["misses"].(int64) != 1 {
		t.Errorf("Expected 1 cache miss, got %d", stats["misses"].(int64))
	}
	if stats["hits"].(int64) != 0 {
		t.Errorf("Expected 0 cache hits, got %d", stats["hits"].(int64))
	}

	if _, err := client.GetPods("default", ctx, opts); err != nil {
		t.Fatalf("Second GetPods failed: %v", err)
	}

	stats = client.GetCacheStats()
	if stats["calls"].(int64) != 2 {
		t.Errorf("Expected 2 cache calls, got %d", stats["calls"].(int64))
	}
	if stats["hits"].(int64) != 1 {
		t.Errorf("Expected 1 cache hit, got %d", stats["hits"].(int64))
	}
	if stats["misses"].(int64) != 1 {
		t.Errorf("Expected 1 cache miss, got %d", stats["misses"].(int64))
	}

	hitRate := stats["hitRate"].(float64)
	expectedHitRate := 50.0
	if hitRate != expectedHitRate {
		t.Errorf("Expected hit rate %.1f%%, got %.1f%%", expectedHitRate, hitRate)
	}

	if _, err := client.GetPods("default", ctx, opts); err != nil {
		t.Fatalf("Third GetPods failed: %v", err)
	}

	stats = client.GetCacheStats()
	if stats["calls"].(int64) != 3 {
		t.Errorf("Expected 3 cache calls, got %d", stats["calls"].(int64))
	}
	if stats["hits"].(int64) != 2 {
		t.Errorf("Expected 2 cache hits, got %d", stats["hits"].(int64))
	}

	hitRate = stats["hitRate"].(float64)
	expectedHitRate = 66.66666666666666
	if hitRate < 66.6 || hitRate > 66.7 {
		t.Errorf("Expected hit rate around %.1f%%, got %.1f%%", expectedHitRate, hitRate)
	}
}

func TestCacheStatisticsInMockMode(t *testing.T) {
	client := NewMockClient(nil)
	ctx, opts := context.Background(), metav1.ListOptions{}

	stats := client.GetCacheStats()
	if stats["calls"].(int64) != 0 || stats["hits"].(int64) != 0 || stats["misses"].(int64) != 0 {
		t.Error("Expected all cache stats to be 0 initially")
	}

	if _, err := client.GetPods("default", ctx, opts); err != nil {
		t.Fatalf("GetPods failed: %v", err)
	}

	stats = client.GetCacheStats()
	if stats["calls"].(int64) != 1 {
		t.Errorf("Expected 1 cache call, got %d", stats["calls"].(int64))
	}
	if stats["misses"].(int64) != 1 {
		t.Errorf("Expected 1 cache miss, got %d", stats["misses"].(int64))
	}
	if stats["hits"].(int64) != 0 {
		t.Errorf("Expected 0 cache hits, got %d", stats["hits"].(int64))
	}

	if _, err := client.GetPods("default", ctx, opts); err != nil {
		t.Fatalf("Second GetPods failed: %v", err)
	}

	stats = client.GetCacheStats()
	if stats["calls"].(int64) != 2 {
		t.Errorf("Expected 2 cache calls, got %d", stats["calls"].(int64))
	}
	if stats["hits"].(int64) != 1 {
		t.Errorf("Expected 1 cache hit, got %d", stats["hits"].(int64))
	}
	if stats["misses"].(int64) != 1 {
		t.Errorf("Expected 1 cache miss, got %d", stats["misses"].(int64))
	}

	hitRate := stats["hitRate"].(float64)
	expectedHitRate := 50.0
	if hitRate != expectedHitRate {
		t.Errorf("Expected hit rate %.1f%%, got %.1f%%", expectedHitRate, hitRate)
	}
}
