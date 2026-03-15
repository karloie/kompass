package kube

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

type resourceCache struct {
	entries map[string]*cacheEntry
	mu      sync.RWMutex
}

func newResourceCache() *resourceCache {
	return &resourceCache{entries: make(map[string]*cacheEntry)}
}

func (rc *resourceCache) get(key string) (interface{}, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if entry, ok := rc.entries[key]; ok && time.Now().Before(entry.expiresAt) {
		return entry.data, true
	}
	return nil, false
}

func (rc *resourceCache) set(key string, data interface{}, ttl time.Duration) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.entries[key] = &cacheEntry{data: data, expiresAt: time.Now().Add(ttl)}
}

func (rc *resourceCache) clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.entries = make(map[string]*cacheEntry)
}

func (rc *resourceCache) size() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return len(rc.entries)
}

func (c *Client) StartSync(interval time.Duration, namespaces []string) error {
	if c.mockMode {
		return nil
	}
	if c.syncCancel != nil {
		return fmt.Errorf("sync already running")
	}
	c.cacheEnabled = true
	c.syncInterval = interval
	c.syncNamespaces = namespaces
	c.syncCtx, c.syncCancel = context.WithCancel(context.Background())
	go c.syncLoop()
	return nil
}

func (c *Client) StopSync() {
	if c.syncCancel != nil {
		c.syncCancel()
		c.syncCancel = nil
	}
}

func (c *Client) ClearCache() {
	if c.cache != nil {
		c.cache.clear()
	}
}

func (c *Client) GetResponseMeta() *Metadata {
	c.lastSyncMutex.RLock()
	lastSyncTime := c.lastSyncTime
	c.lastSyncMutex.RUnlock()

	c.cacheMutex.RLock()
	calls, hits, misses := c.cacheCalls, c.cacheHits, c.cacheMisses
	c.cacheMutex.RUnlock()

	hitRate := 0.0
	if calls > 0 {
		hitRate = float64(hits) / float64(calls) * 100
	}

	return &Metadata{
		CacheEnabled:      c.cacheEnabled,
		CacheSize:         c.cache.size(),
		CacheLastSync:     lastSyncTime,
		CacheSyncInterval: c.syncInterval,
		CacheTTL:          c.cacheTTL,
		CacheCalls:        calls,
		CacheHits:         hits,
		CacheMisses:       misses,
		CacheHitRate:      hitRate,
	}
}

func (c *Client) syncLoop() {
	ticker := time.NewTicker(c.syncInterval)
	defer ticker.Stop()
	c.performSync()
	for {
		select {
		case <-c.syncCtx.Done():
			return
		case <-ticker.C:
			c.performSync()
		}
	}
}

func (c *Client) performSync() {
	if len(c.syncNamespaces) == 0 {
		return
	}
	ctx, opts := context.Background(), metav1.ListOptions{}
	var wg sync.WaitGroup

	for _, ns := range c.syncNamespaces {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()

			c.GetPods(n, ctx, opts)
			c.GetDeployments(n, ctx, opts)
			c.GetReplicaSets(n, ctx, opts)
			c.GetStatefulSets(n, ctx, opts)
			c.GetDaemonSets(n, ctx, opts)
			c.GetServices(n, ctx, opts)
			c.GetConfigMaps(n, ctx, opts)
			c.GetSecrets(n, ctx, opts)
			c.GetServiceAccounts(n, ctx, opts)
			c.GetEndpoints(n, ctx, opts)
			c.GetEndpointSlices(n, ctx, opts)
			c.GetIngresses(n, ctx, opts)
			c.GetNetworkPolicies(n, ctx, opts)
			c.GetPersistentVolumeClaims(n, ctx, opts)
			c.GetJobs(n, ctx, opts)
			c.GetCronJobs(n, ctx, opts)
			c.GetRoleBindings(n, ctx, opts)
			c.GetRoles(n, ctx, opts)
			c.GetHorizontalPodAutoscalers(n, ctx, opts)
		}(ns)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		c.GetNodes(ctx, opts)
		c.GetNamespaces(ctx, opts)
		c.GetPersistentVolumes(ctx, opts)
		c.GetStorageClasses(ctx, opts)
		c.GetClusterRoles(ctx, opts)
		c.GetClusterRoleBindings(ctx, opts)
		c.GetPriorityClasses(ctx, opts)
		c.GetCSIDrivers(ctx, opts)
		c.GetCSINodes(ctx, opts)
		c.GetVolumeAttachments(ctx, opts)
	}()

	wg.Wait()
	c.lastSyncMutex.Lock()
	c.lastSyncTime = time.Now()
	c.lastSyncMutex.Unlock()
}
