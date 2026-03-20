package tree

import (
	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

func secretResourceKey(namespace, secretName string) string {
	return BuildResourceKeyRef("secret", namespace, secretName)
}

func newSecretReferenceNode(nodeMap map[string]kube.Resource, namespace, secretName string) *kube.Tree {
	secretKey := secretResourceKey(namespace, secretName)
	if _, exists := nodeMap[secretKey]; !exists {
		return nil
	}
	return newTree(secretKey, "secret", nil)
}

func secretProviderClassResourceKey(namespace, name string) string {
	return BuildResourceKeyRef("secretproviderclass", namespace, name)
}

func newSecretProviderClassReferenceNode(nodeMap map[string]kube.Resource, namespace, name string) *kube.Tree {
	key := secretProviderClassResourceKey(namespace, name)
	if _, exists := nodeMap[key]; !exists {
		return nil
	}
	return newTree(key, "secretproviderclass", nil)
}

func lookupSecretDataValue(nodeMap map[string]kube.Resource, namespace, secretName, key string) (string, bool) {
	secretKey := secretResourceKey(namespace, secretName)
	secretResource, exists := nodeMap[secretKey]
	if !exists {
		return "", false
	}

	secretData, ok := graph.M(secretResource.AsMap()).MapOk("data")
	if !ok {
		return "", false
	}

	value, ok := secretData[key].(string)
	if !ok {
		return "", false
	}

	return value, true
}

func extractSecretVolumeInfo(volMap map[string]any, namespace string) (volumeType, volumeSource, resourceKey string, ok bool) {
	secret, hasSecret := graph.M(volMap).MapOk("secret")
	if !hasSecret {
		return "", "", "", false
	}

	name, hasName := secret.StringOk("secretName")
	if !hasName {
		return "", "", "", false
	}

	return "secret", name, secretResourceKey(namespace, name), true
}
