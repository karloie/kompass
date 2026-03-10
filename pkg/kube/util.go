package kube

import (
	"context"
	"encoding/json"
	"fmt"
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
	return m
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
	if config.AllError {
		return empty, fmt.Errorf("mock error for %s", method)
	}
	if config.AllEmpty {
		return empty, nil
	}
	if mb, ok := config.Methods[method]; ok {
		if mb.ReturnError {
			if mb.ErrorMessage == "" {
				return empty, fmt.Errorf("mock error for %s", method)
			}
			return empty, fmt.Errorf("%s", mb.ErrorMessage)
		}
		if mb.ReturnEmpty {
			return empty, nil
		}
	}
	return wrap(items), nil
}

func mockMapList(config MockConfig, method string, items []map[string]any) ([]map[string]any, error) {
	if config.AllError {
		return []map[string]any{}, fmt.Errorf("mock error for %s", method)
	}
	return items, nil
}

func listDynamicResourceObjects(dc dynamic.Interface, gvr schema.GroupVersionResource, namespace string, namespaced bool, ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error) {
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

		return []map[string]any{}, nil
	}

	result := make([]map[string]any, 0, len(list.Items))
	for _, item := range list.Items {
		result = append(result, item.Object)
	}
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
			return val, nil
		}
	}

	c.cacheMutex.Lock()
	c.cacheMisses++
	c.cacheMutex.Unlock()

	result, err := retryWithBackoff(c, loadFunc)
	if err != nil {
		return empty, err
	}
	c.cache.set(key, result, c.cacheTTL)
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
