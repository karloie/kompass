package tree

import (
	"fmt"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

func buildEnvarsNode(containerKey string, namespace string, envVars []any, containerSpec map[string]any, nodeMap map[string]kube.Resource) *kube.GraphTree {
	envarsKey := containerKey + "/envars"

	envarsNode := NewGraphTree(envarsKey, "envars", nil)

	for idx, ev := range envVars {
		if envMap, ok := ev.(map[string]any); ok {
			envNode := buildEnvNode(envarsKey, namespace, idx, envMap, nodeMap)
			if envNode != nil {
				envarsNode.Children = append(envarsNode.Children, envNode)
			}
		}
	}

	return envarsNode
}

func buildEnvNode(envarsKey string, namespace string, idx int, envMap map[string]any, nodeMap map[string]kube.Resource) *kube.GraphTree {
	envName, _ := envMap["name"].(string)
	envKey := fmt.Sprintf("%s/env/%d", envarsKey, idx)

	metadata := map[string]any{
		"name": envName,
	}

	var referencedResource *kube.GraphTree
	var sourceType string

	if valueFrom, ok := graph.M(envMap).MapOk("valueFrom"); ok {
		if secretKeyRef, ok := valueFrom.MapOk("secretKeyRef"); ok {
			if secretName, ok := secretKeyRef.StringOk("name"); ok {
				sourceType = "secret"
				metadata["source"] = secretName

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
					referencedResource = NewGraphTree(cmKey, "configmap", nil)
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
	} else {
		sourceType = "literal"
		metadata["value"] = ""
	}

	if sourceType != "" {
		metadata["sourceType"] = sourceType
	}

	envNode := NewGraphTree(envKey, "env", metadata)

	if referencedResource != nil {
		envNode.Children = append(envNode.Children, referencedResource)
	}

	return envNode
}
