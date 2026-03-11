package kube

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCacheEnabledByDefault(t *testing.T) {
	client := NewClientWithClientset(fake.NewSimpleClientset(), nil, nil, "test-context", "default")
	if !client.cacheEnabled {
		t.Error("Cache should be enabled by default")
	}
	if !client.GetStats()["enabled"].(bool) {
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
	waitForCondition(t, 500*time.Millisecond, 20*time.Millisecond, func() bool {
		_, ok := rc.get("test-key")
		return !ok
	}, "Expected cache miss after expiration")
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
	waitForCondition(t, 2*time.Second, 25*time.Millisecond, func() bool {
		lastSync := client.GetStats()["lastSync"].(time.Time)
		return !lastSync.IsZero()
	}, "Expected lastSync to be set after sync")
	client.StopSync()
	if client.syncCancel != nil {
		t.Error("Expected syncCancel to be nil after StopSync")
	}
	client.StopSync()
}

func TestStartSyncAlreadyRunning(t *testing.T) {
	client := NewClientWithClientset(fake.NewSimpleClientset(), nil, nil, "test-context", "default")
	if err := client.StartSync(200*time.Millisecond, []string{"default"}); err != nil {
		t.Fatalf("first StartSync failed: %v", err)
	}
	defer client.StopSync()

	if err := client.StartSync(200*time.Millisecond, []string{"default"}); err == nil {
		t.Fatal("expected sync already running error")
	}
}

func TestStartSyncInMockModeNoop(t *testing.T) {
	client := NewMockClient(nil)
	if err := client.StartSync(200*time.Millisecond, []string{"default"}); err != nil {
		t.Fatalf("expected StartSync to no-op in mock mode, got %v", err)
	}
	if client.syncCancel != nil {
		t.Fatal("expected no syncCancel to be set in mock mode")
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
	stats := NewClientWithClientset(fake.NewSimpleClientset(), nil, nil, "test-context", "default").GetStats()
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

func TestCacheStatistics(t *testing.T) {
	client := NewClientWithClientset(fake.NewSimpleClientset(), nil, nil, "test-context", "default")
	client.cacheEnabled = true
	ctx, opts := context.Background(), metav1.ListOptions{}

	stats := client.GetStats()
	if stats["calls"].(int64) != 0 || stats["hits"].(int64) != 0 || stats["misses"].(int64) != 0 {
		t.Error("Expected all cache stats to be 0 initially")
	}

	if _, err := client.GetPods("default", ctx, opts); err != nil {
		t.Fatalf("GetPods failed: %v", err)
	}

	stats = client.GetStats()
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

	stats = client.GetStats()
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

	stats = client.GetStats()
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

	stats := client.GetStats()
	if stats["calls"].(int64) != 0 || stats["hits"].(int64) != 0 || stats["misses"].(int64) != 0 {
		t.Error("Expected all cache stats to be 0 initially")
	}

	if _, err := client.GetPods("default", ctx, opts); err != nil {
		t.Fatalf("GetPods failed: %v", err)
	}

	stats = client.GetStats()
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

	stats = client.GetStats()
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

func TestClientClearCache(t *testing.T) {
	client := NewClientWithClientset(fake.NewSimpleClientset(), nil, nil, "test-context", "default")
	client.cache.set("pods:default:*", "cached", time.Second)

	if size := client.cache.size(); size != 1 {
		t.Fatalf("expected cache size 1 before clear, got %d", size)
	}

	client.ClearCache()

	if size := client.cache.size(); size != 0 {
		t.Fatalf("expected cache size 0 after clear, got %d", size)
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, step time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(step)
	}
	t.Fatal(msg)
}
