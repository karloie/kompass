package tree

import (
	"fmt"
	"sort"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

func buildMountsNode(containerKey string, namespace string, volumeMounts []any, volumes []any, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) *kube.Tree {
	mountsKey := containerKey + "/mounts"

	mountsNode := NewTree(mountsKey, "mounts", nil)

	for idx, vm := range volumeMounts {
		if vmMap, ok := vm.(map[string]any); ok {
			mountNode := buildMountNode(mountsKey, namespace, idx, vmMap, volumes, graphChildren, state, nodeMap)
			if mountNode != nil {
				mountsNode.Children = append(mountsNode.Children, mountNode)
			}
		}
	}

	sort.Slice(mountsNode.Children, func(i, j int) bool {
		mountI, _ := mountsNode.Children[i].Meta["mount"].(string)
		mountJ, _ := mountsNode.Children[j].Meta["mount"].(string)
		if mountI != mountJ {
			return mountI < mountJ
		}
		volumeI, _ := mountsNode.Children[i].Meta["volume"].(string)
		volumeJ, _ := mountsNode.Children[j].Meta["volume"].(string)
		if volumeI != volumeJ {
			return volumeI < volumeJ
		}
		return mountsNode.Children[i].Key < mountsNode.Children[j].Key
	})

	return mountsNode
}

func buildMountNode(mountsKey string, namespace string, idx int, vmMap map[string]any, volumes []any, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) *kube.Tree {
	mountName, _ := vmMap["name"].(string)
	mountPath, _ := vmMap["mountPath"].(string)
	mountKey := fmt.Sprintf("%s/mount/%d", mountsKey, idx)

	metadata := map[string]any{
		"mount": mountPath,
	}

	if readOnly, ok := vmMap["readOnly"].(bool); ok && readOnly {
		metadata["readOnly"] = readOnly
	}
	if subPath, ok := vmMap["subPath"].(string); ok && subPath != "" {
		metadata["subPath"] = subPath
	}

	var volumeDef map[string]any
	for _, vol := range volumes {
		if volMap, ok := vol.(map[string]any); ok {
			if volName, _ := volMap["name"].(string); volName == mountName {
				volumeDef = volMap
				break
			}
		}
	}

	if volumeDef != nil {
		volumeType, volumeSource, _ := extractVolumeInfo(volumeDef, namespace)

		if volumeSource != "" {
			metadata["volume"] = volumeSource
		} else {
			metadata["volume"] = mountName
		}

		if volumeType != "" {
			metadata["volumeType"] = volumeType
		}

		if volumeType == "csi" {
			if csi, ok := graph.M(volumeDef).MapOk("csi"); ok {
				if nps, ok := csi.Raw()["nodePublishSecretRef"].(map[string]any); ok {
					if secretName, ok := nps["name"].(string); ok && secretName != "" {
						metadata["nodePublishSecretRef"] = secretName
					}
				}
				if attrs, ok := csi.Raw()["volumeAttributes"].(map[string]any); ok {
					if spc, ok := attrs["secretProviderClass"].(string); ok && spc != "" {
						metadata["secretProviderClass"] = spc
					}
				}
			}
		}
	} else {
		metadata["volume"] = mountName
	}

	mountNode := NewTree(mountKey, "mount", metadata)

	return mountNode
}

func buildVolumeFallbackNode(mountKey, namespace, volumeType, volumeSource string, volumeDef map[string]any, nodeMap map[string]kube.Resource, includeReferences bool) *kube.Tree {
	if volumeSource == "" {
		return nil
	}
	if volumeType == "emptyDir" {
		return nil
	}

	meta := map[string]any{"name": volumeSource}
	if volumeType != "" {
		meta["sourceType"] = volumeType
	}

	if volumeType == "csi" {
		if csi, ok := graph.M(volumeDef).MapOk("csi"); ok {
			nodeType := "storage"
			if driver, ok := csi.StringOk("driver"); ok && driver != "" {
				meta["driver"] = driver
				if driver == "secrets-store.csi.k8s.io" {
					nodeType = "secretstore"
				}
			}

			fallbackNode := NewTree(mountKey+"/volume", nodeType, meta)
			if !includeReferences {
				if nps, ok := csi.Raw()["nodePublishSecretRef"].(map[string]any); ok {
					if secretName, ok := nps["name"].(string); ok && secretName != "" {
						meta["nodePublishSecretRef"] = secretName
					}
				}
				if attrs, ok := csi.Raw()["volumeAttributes"].(map[string]any); ok {
					if spc, ok := attrs["secretProviderClass"].(string); ok && spc != "" {
						meta["secretProviderClass"] = spc
					}
				}
				return fallbackNode
			}

			if nps, ok := csi.Raw()["nodePublishSecretRef"].(map[string]any); ok {
				if secretName, ok := nps["name"].(string); ok && secretName != "" {
					meta["nodePublishSecretRef"] = secretName
					if secretRefNode := newSecretReferenceNode(nodeMap, namespace, secretName); secretRefNode != nil {
						fallbackNode.Children = append(fallbackNode.Children, secretRefNode)
					}
				}
			}
			if attrs, ok := csi.Raw()["volumeAttributes"].(map[string]any); ok {
				if spc, ok := attrs["secretProviderClass"].(string); ok && spc != "" {
					meta["secretProviderClass"] = spc
					if spcRefNode := newSecretProviderClassReferenceNode(nodeMap, namespace, spc); spcRefNode != nil {
						fallbackNode.Children = append(fallbackNode.Children, spcRefNode)
					}
				}
			}

			return fallbackNode
		}
	}

	return NewTree(mountKey+"/volume", "storage", meta)
}

func extractVolumeInfo(volMap map[string]any, namespace string) (volumeType, volumeSource, resourceKey string) {
	if secretType, secretSource, secretKey, ok := extractSecretVolumeInfo(volMap, namespace); ok {
		return secretType, secretSource, secretKey
	} else if configMap, ok := graph.M(volMap).MapOk("configMap"); ok {
		if name, ok := configMap.StringOk("name"); ok {
			return "configmap", name, BuildResourceKeyRef("configmap", namespace, name)
		}
	} else if pvc, ok := graph.M(volMap).MapOk("persistentVolumeClaim"); ok {
		if name, ok := pvc.StringOk("claimName"); ok {
			return "persistentvolumeclaim", name, BuildResourceKeyRef("persistentvolumeclaim", namespace, name)
		}
	} else if _, ok := volMap["emptyDir"]; ok {
		return "emptyDir", "emptyDir", ""
	} else if hostPath, ok := graph.M(volMap).MapOk("hostPath"); ok {
		if path, ok := hostPath.StringOk("path"); ok {
			return "hostPath", path, ""
		}
	} else if _, ok := volMap["projected"]; ok {
		return "projected", "projected", ""
	} else if _, ok := volMap["downwardAPI"]; ok {
		return "downwardAPI", "downwardAPI", ""
	} else if csi, ok := graph.M(volMap).MapOk("csi"); ok {
		if driver, ok := csi.StringOk("driver"); ok {
			return "csi", driver, ""
		}
	}
	return "", "", ""
}
