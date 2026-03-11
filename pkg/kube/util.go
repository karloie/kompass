package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func toMap(obj interface{}) map[string]any {
	b, err := json.Marshal(obj)
	if err != nil {
		return map[string]any{"marshalError": err.Error()}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{"unmarshalError": err.Error()}
	}
	trimVerboseMetadata(m)
	return m
}

func trimVerboseMetadata(obj map[string]any) {
	if obj == nil {
		return
	}

	metadata, ok := obj["metadata"].(map[string]any)
	if !ok {
		return
	}

	// managedFields can be extremely large and is not used in graph/tree inference.
	delete(metadata, "managedFields")

	if annotations, ok := metadata["annotations"].(map[string]any); ok {
		delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
		if len(annotations) == 0 {
			delete(metadata, "annotations")
		}
	}
}

func (r GraphRequest) Selectors() []string {
	if r.KeySelector == "" {
		return nil
	}
	selectors := []string{}
	for _, s := range strings.Split(r.KeySelector, ",") {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			selectors = append(selectors, trimmed)
		}
	}
	return selectors
}

func (r GraphRequest) DefaultNamespace() string {
	selectors := r.Selectors()
	if len(selectors) == 0 {
		return ""
	}
	parts := strings.Split(selectors[0], "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func NewModel() *InMemoryModel {
	return &InMemoryModel{PodLogs: map[string]string{}}
}

func derefSlice[T any](in []*T) []T {
	out := make([]T, 0, len(in))
	for _, v := range in {
		if v != nil {
			out = append(out, *v)
		}
	}
	return out
}

func mockList[M any, L any](config MockConfig, method string, empty L, items []M, wrap func([]M) L) (L, error) {
	slog.Debug("provider mock call", "method", method, "items", len(items))
	if config.AllError {
		slog.Debug("provider mock call failed", "method", method, "error", "mock error")
		return empty, fmt.Errorf("mock error for %s", method)
	}
	if config.AllEmpty {
		slog.Debug("provider mock call returned empty", "method", method, "reason", "all_empty")
		return empty, nil
	}
	if mb, ok := config.Methods[method]; ok {
		if mb.ReturnError {
			slog.Debug("provider mock call failed", "method", method, "error", mb.ErrorMessage)
			if mb.ErrorMessage == "" {
				return empty, fmt.Errorf("mock error for %s", method)
			}
			return empty, fmt.Errorf("%s", mb.ErrorMessage)
		}
		if mb.ReturnEmpty {
			slog.Debug("provider mock call returned empty", "method", method, "reason", "method_override")
			return empty, nil
		}
	}
	slog.Debug("provider mock call succeeded", "method", method, "items", len(items))
	return wrap(items), nil
}

func mockMapList(config MockConfig, method string, items []map[string]any) ([]map[string]any, error) {
	slog.Debug("provider mock call", "method", method, "items", len(items))
	if config.AllError {
		slog.Debug("provider mock call failed", "method", method, "error", "mock error")
		return []map[string]any{}, fmt.Errorf("mock error for %s", method)
	}
	slog.Debug("provider mock call succeeded", "method", method, "items", len(items))
	return items, nil
}

func listDynamicResourceObjects(dc dynamic.Interface, gvr schema.GroupVersionResource, namespace string, namespaced bool, ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error) {
	start := time.Now()
	slog.Debug("provider cluster call", "resource", gvr.Resource, "group", gvr.Group, "version", gvr.Version, "namespace", namespace, "selector", opts.LabelSelector)
	var (
		list *unstructured.UnstructuredList
		err  error
	)

	if namespaced {
		list, err = dc.Resource(gvr).Namespace(namespace).List(ctx, opts)
	} else {
		list, err = dc.Resource(gvr).List(ctx, opts)
	}

	if err != nil {
		slog.Debug("provider cluster call failed", "resource", gvr.Resource, "namespace", namespace, "selector", opts.LabelSelector, "duration", time.Since(start).String(), "error", err)
		return []map[string]any{}, nil
	}

	result := make([]map[string]any, 0, len(list.Items))
	for _, item := range list.Items {
		trimVerboseMetadata(item.Object)
		result = append(result, item.Object)
	}
	slog.Debug("provider cluster call succeeded", "resource", gvr.Resource, "namespace", namespace, "selector", opts.LabelSelector, "items", len(result), "duration", time.Since(start).String())
	return result, nil
}

func getCacheKey(resourceType, namespace string, opts metav1.ListOptions) string {
	if namespace == "" {
		namespace = "cluster"
	}
	selector := "*"
	if opts.LabelSelector != "" {
		selector = opts.LabelSelector
	}
	return fmt.Sprintf("%s:%s:%s", resourceType, namespace, selector)
}

func cachedGet[T any](c *Client, resourceType, namespace string, opts metav1.ListOptions, loadFunc func() (T, error)) (T, error) {
	var empty T
	if !c.cacheEnabled {
		slog.Debug("provider call", "provider", map[bool]string{true: "mock", false: "cluster"}[c.mockMode], "resource", resourceType, "namespace", namespace, "selector", opts.LabelSelector, "cached", false)
		return retryWithBackoff(c, loadFunc)
	}

	c.cacheMutex.Lock()
	c.cacheCalls++
	c.cacheMutex.Unlock()

	key := getCacheKey(resourceType, namespace, opts)
	if cached, ok := c.cache.get(key); ok {
		if val, ok := cached.(T); ok {

			c.cacheMutex.Lock()
			c.cacheHits++
			c.cacheMutex.Unlock()
			slog.Debug("provider cache hit", "provider", map[bool]string{true: "mock", false: "cluster"}[c.mockMode], "resource", resourceType, "namespace", namespace, "selector", opts.LabelSelector)
			return val, nil
		}
	}

	c.cacheMutex.Lock()
	c.cacheMisses++
	c.cacheMutex.Unlock()

	start := time.Now()
	slog.Debug("provider call", "provider", map[bool]string{true: "mock", false: "cluster"}[c.mockMode], "resource", resourceType, "namespace", namespace, "selector", opts.LabelSelector, "cached", true)
	result, err := retryWithBackoff(c, loadFunc)
	if err != nil {
		slog.Debug("provider call failed", "provider", map[bool]string{true: "mock", false: "cluster"}[c.mockMode], "resource", resourceType, "namespace", namespace, "selector", opts.LabelSelector, "duration", time.Since(start).String(), "error", err)
		return empty, err
	}
	c.cache.set(key, result, c.cacheTTL)
	slog.Debug("provider call succeeded", "provider", map[bool]string{true: "mock", false: "cluster"}[c.mockMode], "resource", resourceType, "namespace", namespace, "selector", opts.LabelSelector, "duration", time.Since(start).String())
	return result, nil
}

func retryWithBackoff[T any](c *Client, fn func() (T, error)) (T, error) {
	var empty T
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if c.rateLimiter != nil && !c.mockMode {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := c.rateLimiter.Wait(ctx)
			cancel()
			if err != nil {
				return empty, fmt.Errorf("rate limiter timeout: %w", err)
			}
		}
		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !isRetryable(err) {
			return empty, err
		}
		if attempt == c.maxRetries {
			break
		}
		backoff := c.initialBackoff * time.Duration(math.Pow(2, float64(attempt)))
		if backoff > c.maxBackoff {
			backoff = c.maxBackoff
		}
		time.Sleep(backoff)
	}
	return empty, fmt.Errorf("max retries exceeded (%d attempts): %w", c.maxRetries+1, lastErr)
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.IsTimeout(err) ||
		errors.IsServerTimeout(err) ||
		errors.IsServiceUnavailable(err) ||
		errors.IsInternalError(err) ||
		errors.IsTooManyRequests(err) {
		return true
	}
	if errors.IsUnexpectedServerError(err) {
		return true
	}
	return false
}
