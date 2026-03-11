package tree

import (
	"fmt"

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

	var referencedResource *kube.Tree

	if volumeDef != nil {
		volumeType, volumeSource, resourceKey := extractVolumeInfo(volumeDef, namespace)

		if volumeSource != "" {
			metadata["volume"] = volumeSource
		} else {
			metadata["volume"] = mountName
		}

		if volumeType != "" {
			metadata["volumeType"] = volumeType
		}

		if volumeType == "secret" || volumeType == "configmap" || volumeType == "persistentvolumeclaim" {
			if resource, exists := nodeMap[resourceKey]; exists {
				referencedResource = NewTree(resourceKey, resource.Type, map[string]any{})
			}
		}
	} else {
		metadata["volume"] = mountName
	}

	mountNode := NewTree(mountKey, "mount", metadata)

	if referencedResource != nil {
		mountNode.Children = append(mountNode.Children, referencedResource)
	}

	return mountNode
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
