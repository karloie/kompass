package kube

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) getCacheKey(resourceType, namespace string, opts metav1.ListOptions) string {
	return getCacheKey(resourceType, namespace, opts)
}
