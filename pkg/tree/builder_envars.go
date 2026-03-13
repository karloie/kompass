package tree

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

func buildEnvarsNode(containerKey string, namespace string, envVars []any, volumeMounts []any, volumes []any, nodeMap map[string]kube.Resource) *kube.Tree {
	envarsKey := containerKey + "/environment"

	envarsNode := NewTree(envarsKey, "environment", nil)
	secretStoresBySecret := mapSecretStoresBySyncedSecret(namespace, volumes, nodeMap)
	secretStoreMounts := mapSecretStoreMountPaths(volumeMounts, volumes)

	for idx, ev := range envVars {
		if envMap, ok := ev.(map[string]any); ok {
			envNode := buildEnvNode(envarsKey, namespace, idx, envMap, nodeMap, secretStoresBySecret, secretStoreMounts)
			if envNode != nil {
				envarsNode.Children = append(envarsNode.Children, envNode)
			}
		}
	}

	sortChildren(envarsNode.Children)

	return envarsNode
}

func buildEnvNode(envarsKey string, namespace string, idx int, envMap map[string]any, nodeMap map[string]kube.Resource, secretStoresBySecret map[string][]string, secretStoreMounts map[string]string) *kube.Tree {
	envName, _ := envMap["name"].(string)
	envKey := fmt.Sprintf("%s/env/%d", envarsKey, idx)

	metadata := map[string]any{
		"name": envName,
	}

	var referencedResource *kube.Tree
	var sourceType string

	if valueFrom, ok := graph.M(envMap).MapOk("valueFrom"); ok {
		if secretKeyRef, ok := valueFrom.MapOk("secretKeyRef"); ok {
			if secretName, ok := secretKeyRef.StringOk("name"); ok {
				sourceType = "secret"
				metadata["source"] = secretName
				if stores := secretStoresBySecret[secretName]; len(stores) > 0 {
					if len(stores) == 1 {
						metadata["secretStore"] = stores[0]
					} else {
						storeValues := make([]any, 0, len(stores))
						for _, s := range stores {
							storeValues = append(storeValues, s)
						}
						metadata["secretStores"] = storeValues
					}
				}

				if key, ok := secretKeyRef.StringOk("key"); ok {
					metadata["key"] = key
					if value, ok := lookupSecretDataValue(nodeMap, namespace, secretName, key); ok {
						metadata["value"] = value
					}
				}

				referencedResource = newSecretReferenceNode(nodeMap, namespace, secretName)
			}
		} else if configMapKeyRef, ok := valueFrom.MapOk("configMapKeyRef"); ok {
			if configMapName, ok := configMapKeyRef.StringOk("name"); ok {
				sourceType = "configmap"
				metadata["source"] = configMapName
				metadata["configMap"] = configMapName
				if key, ok := configMapKeyRef.StringOk("key"); ok {
					metadata["key"] = key

					cmKey := BuildResourceKeyRef("configmap", namespace, configMapName)
					if cmResource, exists := nodeMap[cmKey]; exists {
						if cmData, ok := graph.M(cmResource.AsMap()).MapOk("data"); ok {
							if value, ok := cmData[key].(string); ok {
								metadata["value"] = value
							}
						}
					}
				}

				cmKey := BuildResourceKeyRef("configmap", namespace, configMapName)
				if _, exists := nodeMap[cmKey]; exists {
					referencedResource = NewTree(cmKey, "configmap", nil)
				}
			}
		} else if fieldRef, ok := valueFrom.MapOk("fieldRef"); ok {
			if fieldPath, ok := fieldRef.StringOk("fieldPath"); ok {
				sourceType = "field"
				metadata["value"] = "fieldRef " + fieldPath
			}
		} else if resourceFieldRef, ok := valueFrom.MapOk("resourceFieldRef"); ok {
			if resource, ok := resourceFieldRef.StringOk("resource"); ok {
				sourceType = "resource"
				metadata["value"] = "resourceFieldRef " + resource
			}
		}
	} else if value, ok := envMap["value"].(string); ok {
		sourceType = "literal"
		metadata["value"] = value
		if spc, secretName, ok := resolveSecretStorePath(value, secretStoreMounts); ok {
			metadata["secretStore"] = spc
			metadata["secretName"] = secretName
		}
	} else {
		sourceType = "literal"
		metadata["value"] = ""
	}

	if sourceType != "" {
		metadata["sourceType"] = sourceType
	}

	envNode := NewTree(envKey, "env", metadata)

	if referencedResource != nil && sourceType != "secret" && sourceType != "configmap" {
		envNode.Children = append(envNode.Children, referencedResource)
	}

	return envNode
}

func mapSecretStoreMountPaths(volumeMounts []any, volumes []any) map[string]string {
	volumeByName := make(map[string]map[string]any)
	for _, vol := range volumes {
		volMap, ok := vol.(map[string]any)
		if !ok {
			continue
		}
		name, _ := volMap["name"].(string)
		if name != "" {
			volumeByName[name] = volMap
		}
	}

	mounts := make(map[string]string)
	for _, vm := range volumeMounts {
		vmMap, ok := vm.(map[string]any)
		if !ok {
			continue
		}
		mountPath, _ := vmMap["mountPath"].(string)
		volumeName, _ := vmMap["name"].(string)
		if mountPath == "" || volumeName == "" {
			continue
		}

		volMap, exists := volumeByName[volumeName]
		if !exists {
			continue
		}
		csi, ok := graph.M(volMap).MapOk("csi")
		if !ok {
			continue
		}
		driver, _ := csi.StringOk("driver")
		if driver != "secrets-store.csi.k8s.io" {
			continue
		}
		attrs, ok := csi.Raw()["volumeAttributes"].(map[string]any)
		if !ok {
			continue
		}
		spc, _ := attrs["secretProviderClass"].(string)
		if spc == "" {
			continue
		}

		mounts[mountPath] = spc
	}

	return mounts
}

func resolveSecretStorePath(value string, secretStoreMounts map[string]string) (secretStore, secretName string, ok bool) {
	if value == "" || len(secretStoreMounts) == 0 {
		return "", "", false
	}

	for mountPath, spc := range secretStoreMounts {
		if !strings.HasPrefix(value, mountPath) {
			continue
		}
		remainder := strings.TrimPrefix(value, mountPath)
		if remainder == "" || remainder == "/" {
			continue
		}
		if !strings.HasPrefix(remainder, "/") {
			continue
		}
		secretName = path.Base(value)
		if secretName == "" || secretName == "/" || secretName == "." {
			continue
		}
		return spc, secretName, true
	}

	return "", "", false
}

func mapSecretStoresBySyncedSecret(namespace string, volumes []any, nodeMap map[string]kube.Resource) map[string][]string {
	storesBySecret := map[string][]string{}

	spcNames := map[string]bool{}
	for _, vol := range volumes {
		volMap, ok := vol.(map[string]any)
		if !ok {
			continue
		}
		if csi, ok := graph.M(volMap).MapOk("csi"); ok {
			if attrs, ok := csi.Raw()["volumeAttributes"].(map[string]any); ok {
				if spc, ok := attrs["secretProviderClass"].(string); ok && spc != "" {
					spcNames[spc] = true
				}
			}
		}
	}

	for spcName := range spcNames {
		spcKey := BuildResourceKeyRef("secretproviderclass", namespace, spcName)
		spcResource, exists := nodeMap[spcKey]
		if !exists {
			continue
		}

		if secretObjects, ok := graph.M(spcResource.AsMap()).Path("spec").Raw()["secretObjects"].([]any); ok {
			for _, so := range secretObjects {
				soMap, ok := so.(map[string]any)
				if !ok {
					continue
				}
				secretName, _ := soMap["secretName"].(string)
				if secretName == "" {
					continue
				}
				storesBySecret[secretName] = append(storesBySecret[secretName], spcName)
			}
		}
	}

	for secretName, stores := range storesBySecret {
		sort.Strings(stores)
		compact := stores[:0]
		for i, s := range stores {
			if i == 0 || s != stores[i-1] {
				compact = append(compact, s)
			}
		}
		storesBySecret[secretName] = compact
	}

	return storesBySecret
}
