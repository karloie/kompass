package tree

import (
	"fmt"
	"sort"
	"strings"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

func expandResourceFromVolume(parentKey, namespace string, idx int, volume map[string]any, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) *kube.Tree {
	volumeSources := []struct {
		field   string
		nameKey string
		resType string
	}{
		{"persistentVolumeClaim", "claimName", "persistentvolumeclaim"},
		{"secret", "secretName", "secret"},
		{"configMap", "name", "configmap"},
	}

	for _, vs := range volumeSources {
		if source, ok := graph.M(volume).MapOk(vs.field); ok {
			if name, ok := source.StringOk(vs.nameKey); ok {
				resourceKey := BuildResourceKeyRef(vs.resType, namespace, name)
				if _, exists := nodeMap[resourceKey]; exists && state.CanTraverse(resourceKey) {
					return buildTreeNode(resourceKey, graphChildren, state, nodeMap)
				}
			}
		}
	}

	if csi, ok := graph.M(volume).MapOk("csi"); ok {
		if driver, ok := csi.StringOk("driver"); ok && driver != "" {
			return buildVolumeFallbackNode(
				fmt.Sprintf("%s/volume/%d", parentKey, idx),
				namespace,
				"csi",
				driver,
				volume,
				nodeMap,
				true,
			)
		}
	}

	return nil
}

func expandVolumesAsResources(parentKey, namespace string, volumes []any, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	nodes := make([]*kube.Tree, 0)
	for idx, vol := range volumes {
		if volMap, ok := vol.(map[string]any); ok {
			if node := expandResourceFromVolume(parentKey, namespace, idx, volMap, graphChildren, state, nodeMap); node != nil {
				nodes = append(nodes, node)
			}
		}
	}
	return nodes
}

func buildStorageNode(parentKey string, storageChildren []*kube.Tree) *kube.Tree {
	return buildSectionNode(parentKey, "storage", storageChildren)
}

func buildSecretsNode(parentKey string, secretChildren []*kube.Tree) *kube.Tree {
	return buildSectionNode(parentKey, "secrets", secretChildren)
}

func buildConfigMapsNode(parentKey string, configMapChildren []*kube.Tree) *kube.Tree {
	return buildSectionNode(parentKey, "configmaps", configMapChildren)
}

func buildSectionNode(parentKey, sectionType string, sectionChildren []*kube.Tree) *kube.Tree {
	if len(sectionChildren) == 0 {
		return nil
	}
	node := NewTree(parentKey+"/"+sectionType, sectionType, nil)
	node.Children = sectionChildren
	sortChildren(node.Children)
	return node
}

func sortConfigMapChildren(children []*kube.Tree) {
	sortChildrenByPriority(children, map[string]int{"env": 0, "mount": 1})
}

func sortPersistentVolumeClaimChildren(children []*kube.Tree) {
	sortChildrenByPriority(children, map[string]int{"mount": 0, "persistentvolume": 1})
}

func secretStoreSignature(driver, spc, nodePublishSecretRef, fallback string) string {
	signature := driver + "|" + spc + "|" + nodePublishSecretRef
	if signature == "||" {
		return fallback
	}
	return signature
}

func attachSyncedSecretsToSecretStores(namespace string, secretStoreNodes []*kube.Tree, envUsageBySecretKey map[string][]map[string]any, state *treeBuildState, nodeMap map[string]kube.Resource) {
	for _, storeNode := range secretStoreNodes {
		if storeNode == nil || storeNode.Type != "secretstore" {
			continue
		}

		spcName, _ := storeNode.Meta["secretProviderClass"].(string)
		if spcName == "" {
			continue
		}

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

				secretKey := BuildResourceKeyRef("secret", namespace, secretName)
				if _, exists := nodeMap[secretKey]; !exists {
					continue
				}

				var secretNode *kube.Tree
				for _, child := range storeNode.Children {
					if child.Type == "secret" && child.Key == secretKey {
						secretNode = child
						break
					}
				}
				if secretNode == nil {
					secretNode = NewTree(secretKey, "secret", nil)
					storeNode.Children = append(storeNode.Children, secretNode)
				}

				for envIdx, envUsage := range envUsageBySecretKey[secretKey] {
					secretNode.Children = append(secretNode.Children, NewTree(secretKey+"/env/"+fmt.Sprintf("%d", envIdx), "env", envUsage))
				}
				sortChildren(secretNode.Children)

				if state != nil {
					state.MarkSeen(secretKey)
				}
			}
		}

		sortSecretStoreChildren(storeNode.Children)
	}
}

func sortSecretStoreChildren(children []*kube.Tree) {
	sortChildrenByPriority(children, map[string]int{"secretproviderclass": 0, "secret": 1, "mount": 2})
}

func sortChildrenByPriority(children []*kube.Tree, priority map[string]int) {
	sort.Slice(children, func(i, j int) bool {
		rankI, okI := priority[children[i].Type]
		rankJ, okJ := priority[children[j].Type]
		if !okI {
			rankI = 99
		}
		if !okJ {
			rankJ = 99
		}
		if rankI != rankJ {
			return rankI < rankJ
		}

		nameI := ""
		nameJ := ""
		if name, ok := children[i].Meta["name"].(string); ok {
			nameI = name
		}
		if name, ok := children[j].Meta["name"].(string); ok {
			nameJ = name
		}
		if nameI != nameJ {
			return nameI < nameJ
		}

		return children[i].Key < children[j].Key
	})
}

func secretKeysShownUnderSecretStores(secretStoreNodes []*kube.Tree) map[string]bool {
	keys := make(map[string]bool)
	for _, storeNode := range secretStoreNodes {
		if storeNode == nil || storeNode.Type != "secretstore" {
			continue
		}
		for _, child := range storeNode.Children {
			if child != nil && child.Type == "secret" && child.Key != "" {
				keys[child.Key] = true
			}
		}
	}
	return keys
}

func filterRedundantTopLevelSecrets(secretNodes []*kube.Tree, secretsShownByStore map[string]bool) []*kube.Tree {
	if len(secretNodes) == 0 || len(secretsShownByStore) == 0 {
		return secretNodes
	}

	filtered := make([]*kube.Tree, 0, len(secretNodes))
	for _, node := range secretNodes {
		if node == nil {
			continue
		}
		if node.Type == "secret" && secretsShownByStore[node.Key] {
			continue
		}
		filtered = append(filtered, node)
	}
	return filtered
}

func splitVolumeResourceNodes(volumeNodes []*kube.Tree) (storageNodes []*kube.Tree, secretNodes []*kube.Tree, configMapNodes []*kube.Tree) {
	seenSecretStore := make(map[string]bool)
	storageNodes = make([]*kube.Tree, 0, len(volumeNodes))
	secretNodes = make([]*kube.Tree, 0)
	configMapNodes = make([]*kube.Tree, 0)

	for _, node := range volumeNodes {
		if node == nil {
			continue
		}

		if node.Type == "configmap" {
			configMapNodes = append(configMapNodes, node)
			continue
		}

		if node.Type == "secret" {
			secretNodes = append(secretNodes, node)
			continue
		}

		if node.Type != "secretstore" {
			storageNodes = append(storageNodes, node)
			continue
		}

		driver, _ := node.Meta["driver"].(string)
		spc, _ := node.Meta["secretProviderClass"].(string)
		nps, _ := node.Meta["nodePublishSecretRef"].(string)
		signature := secretStoreSignature(driver, spc, nps, node.Key)
		if seenSecretStore[signature] {
			continue
		}
		seenSecretStore[signature] = true
		secretNodes = append(secretNodes, node)
	}

	return storageNodes, secretNodes, configMapNodes
}

func collectSecretStoreUsageBySignature(containers []any, volumes []any) map[string][]map[string]any {
	usageBySignature := make(map[string][]map[string]any)

	type secretStoreInfo struct {
		signature            string
		volumeName           string
		driver               string
		secretProviderClass  string
		nodePublishSecretRef string
	}

	storesByVolumeName := make(map[string]secretStoreInfo)
	for _, v := range volumes {
		volMap, ok := v.(map[string]any)
		if !ok {
			continue
		}
		volName, _ := volMap["name"].(string)
		if volName == "" {
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

		spc := ""
		nps := ""
		if attrs, ok := csi.Raw()["volumeAttributes"].(map[string]any); ok {
			if value, ok := attrs["secretProviderClass"].(string); ok {
				spc = value
			}
		}
		if nodePublishSecretRef, ok := csi.Raw()["nodePublishSecretRef"].(map[string]any); ok {
			if value, ok := nodePublishSecretRef["name"].(string); ok {
				nps = value
			}
		}

		storesByVolumeName[volName] = secretStoreInfo{
			signature:            secretStoreSignature(driver, spc, nps, volName),
			volumeName:           volName,
			driver:               driver,
			secretProviderClass:  spc,
			nodePublishSecretRef: nps,
		}
	}

	for _, c := range containers {
		containerMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		volumeMounts, _ := containerMap["volumeMounts"].([]any)
		for _, vm := range volumeMounts {
			vmMap, ok := vm.(map[string]any)
			if !ok {
				continue
			}
			mountVolumeName, _ := vmMap["name"].(string)
			mountPath, _ := vmMap["mountPath"].(string)
			if mountVolumeName == "" || mountPath == "" {
				continue
			}
			store, ok := storesByVolumeName[mountVolumeName]
			if !ok {
				continue
			}

			meta := map[string]any{
				"mount":  mountPath,
				"mode":   "csi",
				"volume": store.volumeName,
			}
			if readOnly, ok := vmMap["readOnly"].(bool); ok && readOnly {
				meta["readOnly"] = true
			}
			if store.driver != "" {
				meta["driver"] = store.driver
			}
			if store.secretProviderClass != "" {
				meta["secretProviderClass"] = store.secretProviderClass
			}
			if store.nodePublishSecretRef != "" {
				meta["nodePublishSecretRef"] = store.nodePublishSecretRef
			}

			usageBySignature[store.signature] = append(usageBySignature[store.signature], meta)
		}
	}

	return usageBySignature
}

func collectSecretEnvUsageBySecretKey(namespace string, containers []any) map[string][]map[string]any {
	usageBySecretKey := make(map[string][]map[string]any)
	seen := make(map[string]bool)

	for _, c := range containers {
		containerMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		envVars, _ := containerMap["env"].([]any)
		for _, e := range envVars {
			envMap, ok := e.(map[string]any)
			if !ok {
				continue
			}
			envName, _ := envMap["name"].(string)
			if envName == "" {
				continue
			}
			valueFrom, ok := envMap["valueFrom"].(map[string]any)
			if !ok {
				continue
			}
			secretKeyRef, ok := valueFrom["secretKeyRef"].(map[string]any)
			if !ok {
				continue
			}
			secretName, _ := secretKeyRef["name"].(string)
			if secretName == "" {
				continue
			}
			secretKey := BuildResourceKeyRef("secret", namespace, secretName)
			entryKey := secretKey + "|" + envName
			if seen[entryKey] {
				continue
			}
			seen[entryKey] = true

			meta := map[string]any{
				"name":  envName,
				"value": "<SECRET>",
			}
			if keyName, ok := secretKeyRef["key"].(string); ok && keyName != "" {
				meta["key"] = keyName
			}

			usageBySecretKey[secretKey] = append(usageBySecretKey[secretKey], meta)
		}
	}

	return usageBySecretKey
}

func attachSecretStoreUsage(secretStoreNodes []*kube.Tree, usageBySignature map[string][]map[string]any) {
	for _, storeNode := range secretStoreNodes {
		if storeNode == nil || storeNode.Type != "secretstore" {
			continue
		}

		driver, _ := storeNode.Meta["driver"].(string)
		spc, _ := storeNode.Meta["secretProviderClass"].(string)
		nps, _ := storeNode.Meta["nodePublishSecretRef"].(string)
		signature := secretStoreSignature(driver, spc, nps, storeNode.Key)

		for idx, usage := range usageBySignature[signature] {
			storeNode.Children = append(storeNode.Children, NewTree(storeNode.Key+"/mount/"+fmt.Sprintf("%d", idx), "mount", usage))
		}

		sortSecretStoreChildren(storeNode.Children)
	}
}

func mergeUniqueNodesByKey(primary []*kube.Tree, secondary []*kube.Tree) []*kube.Tree {
	if len(primary) == 0 {
		return secondary
	}
	if len(secondary) == 0 {
		return primary
	}

	seen := make(map[string]bool, len(primary)+len(secondary))
	merged := make([]*kube.Tree, 0, len(primary)+len(secondary))
	for _, n := range primary {
		if n == nil || n.Key == "" || seen[n.Key] {
			continue
		}
		seen[n.Key] = true
		merged = append(merged, n)
	}
	for _, n := range secondary {
		if n == nil || n.Key == "" || seen[n.Key] {
			continue
		}
		seen[n.Key] = true
		merged = append(merged, n)
	}
	return merged
}

func expandEnvSecretsAsResources(namespace string, containers []any, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	secretKeys := collectEnvSecretKeys(namespace, containers)

	nodes := make([]*kube.Tree, 0, len(secretKeys))
	for secretKey := range secretKeys {
		if _, exists := nodeMap[secretKey]; !exists {
			continue
		}
		if !state.CanTraverse(secretKey) {
			continue
		}
		if node := buildTreeNode(secretKey, graphChildren, state, nodeMap); node != nil {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

func collectEnvSecretKeys(namespace string, containers []any) map[string]bool {
	secretKeys := make(map[string]bool)

	for _, c := range containers {
		containerMap, ok := c.(map[string]any)
		if !ok {
			continue
		}

		if envVars, ok := containerMap["env"].([]any); ok {
			for _, ev := range envVars {
				envMap, ok := ev.(map[string]any)
				if !ok {
					continue
				}
				if valueFrom, ok := graph.M(envMap).MapOk("valueFrom"); ok {
					if secretKeyRef, ok := valueFrom.MapOk("secretKeyRef"); ok {
						if secretName, ok := secretKeyRef.StringOk("name"); ok && secretName != "" {
							secretKeys[BuildResourceKeyRef("secret", namespace, secretName)] = true
						}
					}
				}
			}
		}

		if envFrom, ok := containerMap["envFrom"].([]any); ok {
			for _, ef := range envFrom {
				envFromMap, ok := ef.(map[string]any)
				if !ok {
					continue
				}
				if secretRef, ok := graph.M(envFromMap).MapOk("secretRef"); ok {
					if secretName, ok := secretRef.StringOk("name"); ok && secretName != "" {
						secretKeys[BuildResourceKeyRef("secret", namespace, secretName)] = true
					}
				}
			}
		}
	}

	return secretKeys
}

func collectEnvConfigMapKeys(namespace string, containers []any) map[string]bool {
	configMapKeys := make(map[string]bool)

	for _, c := range containers {
		containerMap, ok := c.(map[string]any)
		if !ok {
			continue
		}

		if envVars, ok := containerMap["env"].([]any); ok {
			for _, ev := range envVars {
				envMap, ok := ev.(map[string]any)
				if !ok {
					continue
				}
				if valueFrom, ok := graph.M(envMap).MapOk("valueFrom"); ok {
					if cmKeyRef, ok := valueFrom.MapOk("configMapKeyRef"); ok {
						if cmName, ok := cmKeyRef.StringOk("name"); ok && cmName != "" {
							configMapKeys[BuildResourceKeyRef("configmap", namespace, cmName)] = true
						}
					}
				}
			}
		}

		if envFrom, ok := containerMap["envFrom"].([]any); ok {
			for _, ef := range envFrom {
				envFromMap, ok := ef.(map[string]any)
				if !ok {
					continue
				}
				if cmRef, ok := graph.M(envFromMap).MapOk("configMapRef"); ok {
					if cmName, ok := cmRef.StringOk("name"); ok && cmName != "" {
						configMapKeys[BuildResourceKeyRef("configmap", namespace, cmName)] = true
					}
				}
			}
		}
	}

	return configMapKeys
}

func collectVolumeConfigMapKeys(namespace string, volumes []any) map[string]bool {
	configMapKeys := make(map[string]bool)

	for _, v := range volumes {
		volMap, ok := v.(map[string]any)
		if !ok {
			continue
		}

		if configMap, ok := graph.M(volMap).MapOk("configMap"); ok {
			if name, ok := configMap.StringOk("name"); ok && name != "" {
				configMapKeys[BuildResourceKeyRef("configmap", namespace, name)] = true
			}
		}

		if projected, ok := graph.M(volMap).MapOk("projected"); ok {
			if sources, ok := projected.Raw()["sources"].([]any); ok {
				for _, source := range sources {
					sourceMap, ok := source.(map[string]any)
					if !ok {
						continue
					}
					if cm, ok := graph.M(sourceMap).MapOk("configMap"); ok {
						if name, ok := cm.StringOk("name"); ok && name != "" {
							configMapKeys[BuildResourceKeyRef("configmap", namespace, name)] = true
						}
					}
				}
			}
		}
	}

	return configMapKeys
}

func collectConfigMapUsageByKey(namespace string, containers []any, volumes []any) map[string][]map[string]any {
	usageByKey := make(map[string][]map[string]any)

	volumeMountsByName := make(map[string][]string)
	for _, c := range containers {
		containerMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		mounts, _ := containerMap["volumeMounts"].([]any)
		for _, m := range mounts {
			mountMap, ok := m.(map[string]any)
			if !ok {
				continue
			}
			name, _ := mountMap["name"].(string)
			mountPath, _ := mountMap["mountPath"].(string)
			if name == "" || mountPath == "" {
				continue
			}
			volumeMountsByName[name] = append(volumeMountsByName[name], mountPath)
		}
	}

	for _, v := range volumes {
		volMap, ok := v.(map[string]any)
		if !ok {
			continue
		}
		volName, _ := volMap["name"].(string)
		mountPaths := volumeMountsByName[volName]

		if configMap, ok := graph.M(volMap).MapOk("configMap"); ok {
			if cmName, ok := configMap.StringOk("name"); ok && cmName != "" {
				cmKey := BuildResourceKeyRef("configmap", namespace, cmName)
				for _, mountPath := range mountPaths {
					usageByKey[cmKey] = append(usageByKey[cmKey], map[string]any{
						"mount":  mountPath,
						"volume": volName,
						"mode":   "volume",
					})
				}
			}
		}

		if projected, ok := graph.M(volMap).MapOk("projected"); ok {
			if sources, ok := projected.Raw()["sources"].([]any); ok {
				for _, source := range sources {
					sourceMap, ok := source.(map[string]any)
					if !ok {
						continue
					}
					cmSource, ok := graph.M(sourceMap).MapOk("configMap")
					if !ok {
						continue
					}
					cmName, ok := cmSource.StringOk("name")
					if !ok || cmName == "" {
						continue
					}

					items := make([]any, 0)
					if rawItems, ok := cmSource.Raw()["items"].([]any); ok {
						for _, rawItem := range rawItems {
							itemMap, ok := rawItem.(map[string]any)
							if !ok {
								continue
							}
							key, _ := itemMap["key"].(string)
							path, _ := itemMap["path"].(string)
							if key != "" && path != "" {
								items = append(items, key+"->"+path)
							}
						}
					}

					cmKey := BuildResourceKeyRef("configmap", namespace, cmName)
					for _, mountPath := range mountPaths {
						meta := map[string]any{
							"mount":  mountPath,
							"volume": volName,
							"mode":   "projected",
						}
						if len(items) > 0 {
							meta["items"] = items
						}
						usageByKey[cmKey] = append(usageByKey[cmKey], meta)
					}
				}
			}
		}
	}

	return usageByKey
}

func collectConfigMapEnvUsageByKey(namespace string, containers []any) map[string][]map[string]any {
	usageByKey := make(map[string][]map[string]any)
	seen := make(map[string]bool)

	for _, c := range containers {
		containerMap, ok := c.(map[string]any)
		if !ok {
			continue
		}

		envVars, _ := containerMap["env"].([]any)
		for _, e := range envVars {
			envMap, ok := e.(map[string]any)
			if !ok {
				continue
			}
			envName, _ := envMap["name"].(string)
			if envName == "" {
				continue
			}
			valueFrom, ok := envMap["valueFrom"].(map[string]any)
			if !ok {
				continue
			}
			cmKeyRef, ok := valueFrom["configMapKeyRef"].(map[string]any)
			if !ok {
				continue
			}
			cmName, _ := cmKeyRef["name"].(string)
			if cmName == "" {
				continue
			}

			cmKey := BuildResourceKeyRef("configmap", namespace, cmName)
			entryKey := cmKey + "|" + envName
			if seen[entryKey] {
				continue
			}
			seen[entryKey] = true

			meta := map[string]any{
				"name": envName,
			}
			if keyName, ok := cmKeyRef["key"].(string); ok && keyName != "" {
				meta["key"] = keyName
			}
			usageByKey[cmKey] = append(usageByKey[cmKey], meta)
		}
	}

	return usageByKey
}

func collectPersistentVolumeClaimUsageByKey(namespace string, containers []any, volumes []any) map[string][]map[string]any {
	usageByKey := make(map[string][]map[string]any)

	volumeMountsByName := make(map[string][]string)
	for _, c := range containers {
		containerMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		mounts, _ := containerMap["volumeMounts"].([]any)
		for _, m := range mounts {
			mountMap, ok := m.(map[string]any)
			if !ok {
				continue
			}
			name, _ := mountMap["name"].(string)
			mountPath, _ := mountMap["mountPath"].(string)
			if name == "" || mountPath == "" {
				continue
			}
			volumeMountsByName[name] = append(volumeMountsByName[name], mountPath)
		}
	}

	for _, v := range volumes {
		volMap, ok := v.(map[string]any)
		if !ok {
			continue
		}
		volName, _ := volMap["name"].(string)
		mountPaths := volumeMountsByName[volName]
		if len(mountPaths) == 0 {
			continue
		}

		pvc, ok := graph.M(volMap).MapOk("persistentVolumeClaim")
		if !ok {
			continue
		}
		claimName, ok := pvc.StringOk("claimName")
		if !ok || claimName == "" {
			continue
		}

		pvcKey := BuildResourceKeyRef("persistentvolumeclaim", namespace, claimName)
		for _, mountPath := range mountPaths {
			usageByKey[pvcKey] = append(usageByKey[pvcKey], map[string]any{
				"mount":  mountPath,
				"volume": volName,
				"mode":   "volume",
			})
		}
	}

	return usageByKey
}

func attachPersistentVolumeClaimUsage(storageNodes []*kube.Tree, usageByKey map[string][]map[string]any) {
	for _, storageNode := range storageNodes {
		if storageNode == nil || storageNode.Type != "persistentvolumeclaim" {
			continue
		}

		for idx, usage := range usageByKey[storageNode.Key] {
			storageNode.Children = append(storageNode.Children, NewTree(storageNode.Key+"/mount/"+fmt.Sprintf("%d", idx), "mount", usage))
		}

		sortPersistentVolumeClaimChildren(storageNode.Children)
	}
}

func expandConfigMapsAsResources(namespace string, containers []any, volumes []any, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	configMapKeys := collectEnvConfigMapKeys(namespace, containers)
	for key := range collectVolumeConfigMapKeys(namespace, volumes) {
		configMapKeys[key] = true
	}
	usageByKey := collectConfigMapUsageByKey(namespace, containers, volumes)
	envUsageByKey := collectConfigMapEnvUsageByKey(namespace, containers)

	nodes := make([]*kube.Tree, 0, len(configMapKeys))
	for cmKey := range configMapKeys {
		if _, exists := nodeMap[cmKey]; !exists {
			continue
		}
		if !state.CanTraverse(cmKey) {
			continue
		}
		cmNode := NewTree(cmKey, "configmap", nil)
		for envIdx, envUsage := range envUsageByKey[cmKey] {
			cmNode.Children = append(cmNode.Children, NewTree(cmKey+"/env/"+fmt.Sprintf("%d", envIdx), "env", envUsage))
		}
		for idx, usage := range usageByKey[cmKey] {
			cmNode.Children = append(cmNode.Children, NewTree(cmKey+"/mount/"+fmt.Sprintf("%d", idx), "mount", usage))
		}
		sortConfigMapChildren(cmNode.Children)
		nodes = append(nodes, cmNode)
		state.MarkSeen(cmKey)
	}

	return nodes
}

func buildPodSpecChildren(specKey, namespace string, spec map[string]any, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) ([]*kube.Tree, *kube.Tree) {
	if spec == nil {
		return nil, nil
	}

	builder := NewChildrenBuilder()
	containers, _ := spec["containers"].([]any)
	volumes, _ := spec["volumes"].([]any)

	var podSecurityContextNode *kube.Tree
	if securityContext, ok := spec["securityContext"].(map[string]any); ok {
		podSecurityContextNode = buildPodSecurityContextNode(specKey, securityContext)
	}

	if len(containers) == 1 {
		if containerMap, ok := containers[0].(map[string]any); ok {
			builder.Extend(buildContainerChildren(specKey, namespace, 0, containerMap, nil, volumes, graphChildren, state, nodeMap))
		}
	} else {
		for idx, c := range containers {
			if containerMap, ok := c.(map[string]any); ok {
				builder.Add(buildContainerNode(specKey, namespace, idx, containerMap, nil, volumes, graphChildren, state, nodeMap))
			}
		}
	}

	volumeNodes := expandVolumesAsResources(specKey, namespace, volumes, graphChildren, state, nodeMap)
	storageVolumeNodes, secretStoreNodes, volumeConfigMapNodes := splitVolumeResourceNodes(volumeNodes)
	attachPersistentVolumeClaimUsage(storageVolumeNodes, collectPersistentVolumeClaimUsageByKey(namespace, containers, volumes))
	attachSecretStoreUsage(secretStoreNodes, collectSecretStoreUsageBySignature(containers, volumes))
	attachSyncedSecretsToSecretStores(namespace, secretStoreNodes, collectSecretEnvUsageBySecretKey(namespace, containers), state, nodeMap)

	secretChildren := expandEnvSecretsAsResources(namespace, containers, graphChildren, state, nodeMap)
	secretChildren = filterRedundantTopLevelSecrets(secretChildren, secretKeysShownUnderSecretStores(secretStoreNodes))
	secretChildren = append(secretChildren, secretStoreNodes...)

	configMapChildren := expandConfigMapsAsResources(namespace, containers, volumes, graphChildren, state, nodeMap)
	configMapChildren = mergeUniqueNodesByKey(configMapChildren, volumeConfigMapNodes)

	builder.Add(buildSecretsNode(specKey, secretChildren))
	builder.Add(buildConfigMapsNode(specKey, configMapChildren))
	builder.Add(buildStorageNode(specKey, storageVolumeNodes))

	return builder.Build(), podSecurityContextNode
}

func buildPodTemplateChildren(templateKey string, namespace string, templateSpec map[string]any, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	if templateSpec == nil {
		return nil
	}

	builder := NewChildrenBuilder()
	specChildren, podSecurityContextNode := buildPodSpecChildren(templateKey, namespace, templateSpec, graphChildren, state, nodeMap)
	builder.Add(podSecurityContextNode)
	builder.Extend(specChildren)

	return builder.Build()
}

func buildPodWithSimplifiedContainers(podKey string, pod kube.Resource) *kube.Tree {
	podNode := buildSimplifiedPodNode(podKey, pod)
	if podNode == nil {
		return nil
	}
	spec := graph.M(pod.AsMap()).Map("spec").Raw()
	status := graph.M(pod.AsMap()).Map("status").Raw()
	podNode.Children = buildRuntimeContainerChildren(podKey, spec, status)
	return podNode
}

func buildSimplifiedPodNode(podKey string, pod kube.Resource) *kube.Tree {
	metadata := graph.M(pod.AsMap()).Map("metadata").Raw()
	status := graph.M(pod.AsMap()).Map("status").Raw()
	spec := graph.M(pod.AsMap()).Map("spec").Raw()

	nodeMetadata := map[string]any{}

	if metadata != nil {
		if name := graph.M(metadata).String("name"); name != "" {
			nodeMetadata["name"] = name
		}
	}

	if status != nil {
		if phase := graph.M(status).String("phase"); phase != "" {
			nodeMetadata["phase"] = phase
		}
		if podIP := graph.M(status).String("podIP"); podIP != "" {
			nodeMetadata["podIP"] = podIP
		}
		if hostIP := graph.M(status).String("hostIP"); hostIP != "" {
			nodeMetadata["hostIP"] = hostIP
		}
	}

	if spec != nil {
		if nodeName := graph.M(spec).String("nodeName"); nodeName != "" {
			nodeMetadata["nodeName"] = nodeName
		}
	}

	return NewTree(podKey, "pod", nodeMetadata)
}

func buildPodChildren(podKey string, pod kube.Resource, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	spec := graph.M(pod.AsMap()).Map("spec").Raw()
	metadata := graph.M(pod.AsMap()).Map("metadata").Raw()
	status := graph.M(pod.AsMap()).Map("status").Raw()
	namespace := graph.M(metadata).String("namespace")

	if spec == nil {
		return nil
	}

	builder := NewChildrenBuilder()
	specKey := podKey + "/spec"
	specNode := NewTree(specKey, "spec", map[string]any{})
	specChildren, podSecurityContextNode := buildPodSpecChildren(specKey, namespace, spec, graphChildren, state, nodeMap)
	if podSecurityContextNode != nil {
		podSecurityContextNode.Key = podKey + "/podsecuritycontext"
	}
	specNode.Children = specChildren
	builder.Add(podSecurityContextNode)
	builder.Extend(buildRuntimeContainerChildren(podKey, spec, status))
	builder.Add(specNode)

	excludedChildTypes := map[string]bool{
		"ciliumnetworkpolicy":            true,
		"ciliumclusterwidenetworkpolicy": true,
	}
	builder.Extend(appendFilteredGraphChildren(nil, podKey, excludedChildTypes, graphChildren, state, nodeMap))

	return builder.Build()
}

func buildRuntimeContainerChildren(podKey string, spec map[string]any, status map[string]any) []*kube.Tree {
	if spec == nil {
		return nil
	}

	containerStatuses := indexContainerStatuses(status)

	runtimeChildren := make([]*kube.Tree, 0)
	if containers, ok := spec["containers"].([]any); ok {
		for idx, c := range containers {
			containerMap, ok := c.(map[string]any)
			if !ok {
				continue
			}

			containerName, _ := containerMap["name"].(string)
			containerStatus := containerStatuses[containerName]
			if containerNode := buildRuntimeContainerNode(podKey, idx, containerMap, containerStatus); containerNode != nil {
				runtimeChildren = append(runtimeChildren, containerNode)
			}
		}
	}

	return runtimeChildren
}

func indexContainerStatuses(status map[string]any) map[string]map[string]any {
	containerStatuses := make(map[string]map[string]any)
	if status == nil {
		return containerStatuses
	}
	if statuses, ok := status["containerStatuses"].([]any); ok {
		for _, s := range statuses {
			if statusMap, ok := s.(map[string]any); ok {
				if name, ok := statusMap["name"].(string); ok {
					containerStatuses[name] = statusMap
				}
			}
		}
	}
	return containerStatuses
}

func buildRuntimeContainerNode(podKey string, idx int, containerSpec map[string]any, containerStatus map[string]any) *kube.Tree {
	containerName, _ := containerSpec["name"].(string)
	containerKey := fmt.Sprintf("%s/container/%d", podKey, idx)

	metadata := map[string]any{"name": containerName}
	applyContainerStatusMetadata(metadata, containerStatus)

	containerNode := NewTree(containerKey, "container", metadata)
	if imageMeta, ok := runtimeImageMetadata(containerSpec, containerStatus); ok {
		containerNode.Children = append(containerNode.Children, NewTree(containerKey+"/image", "image", imageMeta))
	}
	if resourcesNode := buildRuntimeResourcesNode(containerKey, containerSpec, containerStatus); resourcesNode != nil {
		containerNode.Children = append(containerNode.Children, resourcesNode)
	}

	if containerStatus != nil {
		hasLivenessProbe := containerSpec["livenessProbe"] != nil
		hasReadinessProbe := containerSpec["readinessProbe"] != nil
		hasStartupProbe := containerSpec["startupProbe"] != nil

		if hasLivenessProbe {
			probeStatus := "passing"
			if state, ok := containerStatus["state"].(map[string]any); ok {
				if _, running := state["running"]; !running {
					probeStatus = "unknown"
				}
			}
			metadata["livenessStatus"] = probeStatus
		}

		if hasReadinessProbe {
			probeStatus := "not-ready"
			if ready, ok := containerStatus["ready"].(bool); ok && ready {
				probeStatus = "ready"
			}
			metadata["readinessStatus"] = probeStatus
		}

		if hasStartupProbe {
			probeStatus := "not-started"
			if started, ok := containerStatus["started"].(bool); ok && started {
				probeStatus = "started"
			}
			metadata["startupStatus"] = probeStatus
		}
	}

	sortChildren(containerNode.Children)
	return containerNode
}

func buildContainerChildren(parentKey string, namespace string, idx int, containerSpec map[string]any, containerStatus map[string]any, volumes []any, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	containerKey := fmt.Sprintf("%s/container/%d", parentKey, idx)
	var children []*kube.Tree

	if image, ok := containerSpec["image"].(string); ok {
		imageMetadata := map[string]any{"name": image}
		if pullPolicy, ok := containerSpec["imagePullPolicy"].(string); ok && pullPolicy != "" {
			imageMetadata["pullPolicy"] = pullPolicy
		}
		imageNode := NewTree(containerKey+"/image", "image", imageMetadata)
		children = append(children, imageNode)
	}

	if ports, ok := containerSpec["ports"].([]any); ok && len(ports) > 0 {
		portsNode := buildPortsNode(containerKey, ports)
		if portsNode != nil {
			children = append(children, portsNode)
		}
	}

	if envVars, ok := containerSpec["env"].([]any); ok && len(envVars) > 0 {
		volumeMounts, _ := containerSpec["volumeMounts"].([]any)
		envarsNode := buildEnvarsNode(containerKey, namespace, envVars, volumeMounts, volumes, nodeMap)
		if envarsNode != nil {
			children = append(children, envarsNode)
		}
	}

	if volumeMounts, ok := containerSpec["volumeMounts"].([]any); ok && len(volumeMounts) > 0 {
		mountsNode := buildMountsNode(containerKey, namespace, volumeMounts, volumes, graphChildren, state, nodeMap)
		if mountsNode != nil {
			children = append(children, mountsNode)
		}
	}

	if resources, ok := containerSpec["resources"].(map[string]any); ok && len(resources) > 0 {
		resourcesNode := buildResourcesNode(containerKey, resources)
		if resourcesNode != nil {
			children = append(children, resourcesNode)
		}
	}

	if livenessProbe, ok := containerSpec["livenessProbe"].(map[string]any); ok {
		probeNode := buildProbeNode(containerKey, "livenessprobe", livenessProbe, containerStatus)
		if probeNode != nil {
			children = append(children, probeNode)
		}
	}

	if readinessProbe, ok := containerSpec["readinessProbe"].(map[string]any); ok {
		probeNode := buildProbeNode(containerKey, "readinessprobe", readinessProbe, containerStatus)
		if probeNode != nil {
			children = append(children, probeNode)
		}
	}

	if startupProbe, ok := containerSpec["startupProbe"].(map[string]any); ok {
		probeNode := buildProbeNode(containerKey, "startupprobe", startupProbe, containerStatus)
		if probeNode != nil {
			children = append(children, probeNode)
		}
	}

	if securityContext, ok := containerSpec["securityContext"].(map[string]any); ok {
		securityContextNode := buildSecurityContextNode(containerKey, securityContext)
		if securityContextNode != nil {
			children = append(children, securityContextNode)
		}
	}

	sortChildren(children)

	return children
}

func buildContainerNode(podKey string, namespace string, idx int, containerSpec map[string]any, containerStatus map[string]any, volumes []any, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) *kube.Tree {
	containerName, _ := containerSpec["name"].(string)
	containerKey := fmt.Sprintf("%s/container/%d", podKey, idx)

	metadata := map[string]any{
		"name": containerName,
	}
	applyContainerStatusMetadata(metadata, containerStatus)

	containerNode := NewTree(containerKey, "container", metadata)

	containerNode.Children = buildContainerChildren(podKey, namespace, idx, containerSpec, containerStatus, volumes, graphChildren, state, nodeMap)

	return containerNode
}

func applyContainerStatusMetadata(metadata map[string]any, containerStatus map[string]any) {
	if metadata == nil || containerStatus == nil {
		return
	}

	if restartCount, ok := containerStatus["restartCount"].(float64); ok && restartCount > 0 {
		metadata["restarts"] = fmt.Sprintf("%.0f", restartCount)
	}

	stateMap, ok := containerStatus["state"].(map[string]any)
	if !ok {
		return
	}

	if waiting, ok := stateMap["waiting"].(map[string]any); ok {
		metadata["state"] = "Waiting"
		if reason, ok := waiting["reason"].(string); ok && reason != "" {
			metadata["reason"] = reason
		}
		return
	}

	if running, ok := stateMap["running"].(map[string]any); ok {
		if startedAt, ok := running["startedAt"].(string); ok && startedAt != "" {
			metadata["state"] = "Running"
		}
		return
	}

	if terminated, ok := stateMap["terminated"].(map[string]any); ok {
		metadata["state"] = "Terminated"
		if exitCode, ok := terminated["exitCode"].(float64); ok {
			metadata["exitCode"] = fmt.Sprintf("%.0f", exitCode)
		}
		if reason, ok := terminated["reason"].(string); ok && reason != "" {
			metadata["reason"] = reason
		}
	}
}

func runtimeImageMetadata(containerSpec map[string]any, containerStatus map[string]any) (map[string]any, bool) {
	rawImage := ""
	rawImageID := ""
	if containerStatus != nil {
		rawImage, _ = containerStatus["image"].(string)
		rawImageID, _ = containerStatus["imageID"].(string)
	}
	if rawImage == "" && containerSpec != nil {
		rawImage, _ = containerSpec["image"].(string)
	}

	image := normalizeRuntimeImageRef(rawImage)
	imageID := normalizeRuntimeImageRef(rawImageID)

	if imageID != "" {
		return map[string]any{"name": imageID}, true
	}

	if image == "" {
		return nil, false
	}

	return map[string]any{"name": image}, true
}

func buildRuntimeResourcesNode(containerKey string, containerSpec map[string]any, containerStatus map[string]any) *kube.Tree {
	runtimeResources := map[string]any{}

	mergeResources := func(resources map[string]any) {
		if resources == nil {
			return
		}
		if limits, ok := resources["limits"].(map[string]any); ok && len(limits) > 0 {
			mergedLimits := map[string]any{}
			if existing, ok := runtimeResources["limits"].(map[string]any); ok {
				for k, v := range existing {
					mergedLimits[k] = v
				}
			}
			for k, v := range limits {
				if _, exists := mergedLimits[k]; !exists {
					mergedLimits[k] = v
				}
			}
			if len(mergedLimits) > 0 {
				runtimeResources["limits"] = mergedLimits
			}
		}
		if requests, ok := resources["requests"].(map[string]any); ok && len(requests) > 0 {
			mergedRequests := map[string]any{}
			if existing, ok := runtimeResources["requests"].(map[string]any); ok {
				for k, v := range existing {
					mergedRequests[k] = v
				}
			}
			for k, v := range requests {
				if _, exists := mergedRequests[k]; !exists {
					mergedRequests[k] = v
				}
			}
			if len(mergedRequests) > 0 {
				runtimeResources["requests"] = mergedRequests
			}
		}
	}

	if resources, ok := containerStatus["resources"].(map[string]any); ok {
		mergeResources(resources)
	}

	if specResources, ok := containerSpec["resources"].(map[string]any); ok {
		mergeResources(specResources)
	}

	if allocated, ok := containerStatus["allocatedResources"].(map[string]any); ok && len(allocated) > 0 {
		runtimeResources["allocated"] = allocated
	}

	if len(runtimeResources) == 0 {
		return nil
	}

	return NewTree(containerKey+"/resources", "resources", runtimeResources)
}

func normalizeRuntimeImageRef(ref string) string {
	if ref == "" {
		return ""
	}
	for _, prefix := range []string{"docker-pullable://", "docker://", "containerd://", "cri-o://"} {
		if strings.HasPrefix(ref, prefix) {
			return strings.TrimPrefix(ref, prefix)
		}
	}
	return ref
}

func buildSecurityContextNode(containerKey string, securityContext map[string]any) *kube.Tree {
	securityContextKey := containerKey + "/securityContext"
	metadata := map[string]any{}

	if runAsUser, ok := graph.M(securityContext).IntOk("runAsUser"); ok {
		metadata["runAsUser"] = runAsUser
	}
	if runAsGroup, ok := graph.M(securityContext).IntOk("runAsGroup"); ok {
		metadata["runAsGroup"] = runAsGroup
	}
	if runAsNonRoot, ok := securityContext["runAsNonRoot"].(bool); ok {
		metadata["runAsNonRoot"] = runAsNonRoot
	}
	if privileged, ok := securityContext["privileged"].(bool); ok {
		metadata["privileged"] = privileged
	}
	if allowPrivilegeEscalation, ok := securityContext["allowPrivilegeEscalation"].(bool); ok {
		metadata["allowPrivilegeEscalation"] = allowPrivilegeEscalation
	}
	if readOnlyRootFilesystem, ok := securityContext["readOnlyRootFilesystem"].(bool); ok {
		metadata["readOnlyRootFilesystem"] = readOnlyRootFilesystem
	}
	if procMount, ok := securityContext["procMount"].(string); ok && procMount != "Default" {
		metadata["procMount"] = procMount
	}

	if capabilities, ok := securityContext["capabilities"].(map[string]any); ok {
		if add, ok := capabilities["add"].([]any); ok && len(add) > 0 {
			addStrs := []string{}
			for _, cap := range add {
				if capStr, ok := cap.(string); ok {
					addStrs = append(addStrs, capStr)
				}
			}
			if len(addStrs) > 0 {
				metadata["capabilitiesAdd"] = addStrs
			}
		}
		if drop, ok := capabilities["drop"].([]any); ok && len(drop) > 0 {
			dropStrs := []string{}
			for _, cap := range drop {
				if capStr, ok := cap.(string); ok {
					dropStrs = append(dropStrs, capStr)
				}
			}
			if len(dropStrs) > 0 {
				metadata["capabilitiesDrop"] = dropStrs
			}
		}
	}

	if seccompProfile, ok := securityContext["seccompProfile"].(map[string]any); ok {
		if profileType, ok := seccompProfile["type"].(string); ok {
			metadata["seccompProfile"] = profileType
			if profileType == "Localhost" {
				if localhostProfile, ok := seccompProfile["localhostProfile"].(string); ok {
					metadata["seccompLocalhostProfile"] = localhostProfile
				}
			}
		}
	}

	if seLinuxOptions, ok := securityContext["seLinuxOptions"].(map[string]any); ok {
		if level, ok := seLinuxOptions["level"].(string); ok {
			metadata["seLinuxLevel"] = level
		}
		if role, ok := seLinuxOptions["role"].(string); ok {
			metadata["seLinuxRole"] = role
		}
		if seLinuxType, ok := seLinuxOptions["type"].(string); ok {
			metadata["seLinuxType"] = seLinuxType
		}
		if user, ok := seLinuxOptions["user"].(string); ok {
			metadata["seLinuxUser"] = user
		}
	}

	if windowsOptions, ok := securityContext["windowsOptions"].(map[string]any); ok {
		if gmsaCredentialSpec, ok := windowsOptions["gmsaCredentialSpec"].(string); ok {
			metadata["windowsGmsaCredentialSpec"] = gmsaCredentialSpec
		}
		if gmsaCredentialSpecName, ok := windowsOptions["gmsaCredentialSpecName"].(string); ok {
			metadata["windowsGmsaCredentialSpecName"] = gmsaCredentialSpecName
		}
		if runAsUserName, ok := windowsOptions["runAsUserName"].(string); ok {
			metadata["windowsRunAsUserName"] = runAsUserName
		}
		if hostProcess, ok := windowsOptions["hostProcess"].(bool); ok {
			metadata["windowsHostProcess"] = hostProcess
		}
	}

	if len(metadata) == 0 {
		return nil
	}

	return NewTree(securityContextKey, "securitycontext", metadata)
}

func buildPodSecurityContextNode(podKey string, securityContext map[string]any) *kube.Tree {
	securityContextKey := podKey + "/securityContext"
	metadata := map[string]any{}

	if runAsUser, ok := graph.M(securityContext).IntOk("runAsUser"); ok {
		metadata["runAsUser"] = runAsUser
	}
	if runAsGroup, ok := graph.M(securityContext).IntOk("runAsGroup"); ok {
		metadata["runAsGroup"] = runAsGroup
	}
	if runAsNonRoot, ok := securityContext["runAsNonRoot"].(bool); ok {
		metadata["runAsNonRoot"] = runAsNonRoot
	}
	if fsGroup, ok := graph.M(securityContext).IntOk("fsGroup"); ok {
		metadata["fsGroup"] = fsGroup
	}
	if fsGroupChangePolicy, ok := securityContext["fsGroupChangePolicy"].(string); ok {
		metadata["fsGroupChangePolicy"] = fsGroupChangePolicy
	}
	if sysctls, ok := securityContext["sysctls"].([]any); ok && len(sysctls) > 0 {
		sysctlStrs := []string{}
		for _, sysctl := range sysctls {
			if sysctlMap, ok := sysctl.(map[string]any); ok {
				if name, ok := sysctlMap["name"].(string); ok {
					if value, ok := sysctlMap["value"].(string); ok {
						sysctlStrs = append(sysctlStrs, name+"="+value)
					}
				}
			}
		}
		if len(sysctlStrs) > 0 {
			metadata["sysctls"] = sysctlStrs
		}
	}

	if supplementalGroups, ok := securityContext["supplementalGroups"].([]any); ok && len(supplementalGroups) > 0 {
		groups := []int{}
		for _, g := range supplementalGroups {
			if gid, ok := graph.M(map[string]any{"v": g}).IntOk("v"); ok {
				groups = append(groups, gid)
			}
		}
		if len(groups) > 0 {
			metadata["supplementalGroups"] = groups
		}
	}

	if seccompProfile, ok := securityContext["seccompProfile"].(map[string]any); ok {
		if profileType, ok := seccompProfile["type"].(string); ok {
			metadata["seccompProfile"] = profileType
			if profileType == "Localhost" {
				if localhostProfile, ok := seccompProfile["localhostProfile"].(string); ok {
					metadata["seccompLocalhostProfile"] = localhostProfile
				}
			}
		}
	}

	if seLinuxOptions, ok := securityContext["seLinuxOptions"].(map[string]any); ok {
		if level, ok := seLinuxOptions["level"].(string); ok {
			metadata["seLinuxLevel"] = level
		}
		if role, ok := seLinuxOptions["role"].(string); ok {
			metadata["seLinuxRole"] = role
		}
		if seLinuxType, ok := seLinuxOptions["type"].(string); ok {
			metadata["seLinuxType"] = seLinuxType
		}
		if user, ok := seLinuxOptions["user"].(string); ok {
			metadata["seLinuxUser"] = user
		}
	}

	if windowsOptions, ok := securityContext["windowsOptions"].(map[string]any); ok {
		if gmsaCredentialSpec, ok := windowsOptions["gmsaCredentialSpec"].(string); ok {
			metadata["windowsGmsaCredentialSpec"] = gmsaCredentialSpec
		}
		if gmsaCredentialSpecName, ok := windowsOptions["gmsaCredentialSpecName"].(string); ok {
			metadata["windowsGmsaCredentialSpecName"] = gmsaCredentialSpecName
		}
		if runAsUserName, ok := windowsOptions["runAsUserName"].(string); ok {
			metadata["windowsRunAsUserName"] = runAsUserName
		}
		if hostProcess, ok := windowsOptions["hostProcess"].(bool); ok {
			metadata["windowsHostProcess"] = hostProcess
		}
	}

	if len(metadata) == 0 {
		return nil
	}

	return NewTree(securityContextKey, "podsecuritycontext", metadata)
}

func buildPortsNode(containerKey string, ports []any) *kube.Tree {
	portsKey := containerKey + "/ports"

	portsNode := NewTree(portsKey, "ports", nil)

	for idx, p := range ports {
		if portMap, ok := p.(map[string]any); ok {
			portKey := fmt.Sprintf("%s/port/%d", portsKey, idx)

			metadata := map[string]any{}

			if containerPort, ok := graph.M(portMap).IntOk("containerPort"); ok {
				metadata["containerPort"] = containerPort
			}
			if protocol, ok := graph.M(portMap).StringOk("protocol"); ok {
				metadata["protocol"] = protocol
			} else {
				metadata["protocol"] = "TCP"
			}
			if name, ok := graph.M(portMap).StringOk("name"); ok {
				metadata["name"] = name
			}
			if hostPort, ok := graph.M(portMap).IntOk("hostPort"); ok {
				metadata["hostPort"] = hostPort
			}

			portNode := NewTree(portKey, "port", metadata)
			portsNode.Children = append(portsNode.Children, portNode)
		}
	}

	sortChildren(portsNode.Children)

	return portsNode
}

func buildResourcesNode(containerKey string, resources map[string]any) *kube.Tree {
	resourcesKey := containerKey + "/resources"

	metadata := map[string]any{}

	if limits, ok := resources["limits"].(map[string]any); ok && len(limits) > 0 {
		limitsMap := make(map[string]any)
		for k, v := range limits {
			if str, ok := v.(string); ok {
				limitsMap[k] = str
			}
		}
		if len(limitsMap) > 0 {
			metadata["limits"] = limitsMap
		}
	}

	if requests, ok := resources["requests"].(map[string]any); ok && len(requests) > 0 {
		requestsMap := make(map[string]any)
		for k, v := range requests {
			if str, ok := v.(string); ok {
				requestsMap[k] = str
			}
		}
		if len(requestsMap) > 0 {
			metadata["requests"] = requestsMap
		}
	}

	if len(metadata) == 0 {
		return nil
	}

	return NewTree(resourcesKey, "resources", metadata)
}

func buildProbeNode(containerKey string, probeType string, probe map[string]any, containerStatus map[string]any) *kube.Tree {
	probeKey := containerKey + "/" + probeType

	metadata := map[string]any{}

	if containerStatus != nil {
		if probeType == "readinessprobe" {
			if ready, ok := containerStatus["ready"].(bool); ok {
				if ready {
					metadata["status"] = "ready"
				} else {
					metadata["status"] = "not-ready"
				}
			}
		} else if probeType == "startupprobe" {
			if started, ok := containerStatus["started"].(bool); ok {
				if started {
					metadata["status"] = "started"
				} else {
					metadata["status"] = "not-started"
				}
			}
		} else if probeType == "livenessprobe" {
			if state, ok := containerStatus["state"].(map[string]any); ok {
				if _, running := state["running"]; running {
					metadata["status"] = "passing"
				} else if terminated, ok := state["terminated"].(map[string]any); ok {
					if reason, ok := terminated["reason"].(string); ok && reason != "" {
						metadata["status"] = "failed"
					}
				}
			}
		}
	}

	if httpGet, ok := graph.M(probe).MapOk("httpGet"); ok {
		metadata["type"] = "httpGet"
		if path, ok := httpGet.StringOk("path"); ok {
			metadata["path"] = path
		}
		if port, ok := httpGet.Raw()["port"]; ok {
			metadata["port"] = port
		}
		if scheme, ok := httpGet.StringOk("scheme"); ok && scheme != "HTTP" {
			metadata["scheme"] = scheme
		}
		if host, ok := httpGet.StringOk("host"); ok && host != "" {
			metadata["host"] = host
		}
		if httpHeaders, ok := httpGet.Raw()["httpHeaders"].([]any); ok && len(httpHeaders) > 0 {
			headers := []string{}
			for _, h := range httpHeaders {
				if hMap, ok := h.(map[string]any); ok {
					if name, ok := hMap["name"].(string); ok {
						if value, ok := hMap["value"].(string); ok {
							headers = append(headers, name+": "+value)
						}
					}
				}
			}
			if len(headers) > 0 {
				metadata["httpHeaders"] = headers
			}
		}
	} else if exec, ok := graph.M(probe).MapOk("exec"); ok {
		metadata["type"] = "exec"
		if command, ok := exec.Raw()["command"].([]any); ok && len(command) > 0 {
			cmdStrs := []string{}
			for _, c := range command {
				if str, ok := c.(string); ok {
					cmdStrs = append(cmdStrs, str)
				}
			}
			if len(cmdStrs) > 0 {
				metadata["command"] = cmdStrs
			}
		}
	} else if tcpSocket, ok := graph.M(probe).MapOk("tcpSocket"); ok {
		metadata["type"] = "tcpSocket"
		if port, ok := tcpSocket.Raw()["port"]; ok {
			metadata["port"] = port
		}
	} else if grpc, ok := graph.M(probe).MapOk("grpc"); ok {
		metadata["type"] = "grpc"
		if port, ok := grpc.IntOk("port"); ok {
			metadata["port"] = port
		}
		if service, ok := grpc.StringOk("service"); ok {
			metadata["service"] = service
		}
	}

	if initialDelay, ok := graph.M(probe).IntOk("initialDelaySeconds"); ok && initialDelay > 0 {
		metadata["initialDelaySeconds"] = initialDelay
	}
	if period, ok := graph.M(probe).IntOk("periodSeconds"); ok && period != 10 {
		metadata["periodSeconds"] = period
	}
	if timeout, ok := graph.M(probe).IntOk("timeoutSeconds"); ok && timeout != 1 {
		metadata["timeoutSeconds"] = timeout
	}
	if successThreshold, ok := graph.M(probe).IntOk("successThreshold"); ok && successThreshold != 1 {
		metadata["successThreshold"] = successThreshold
	}
	if failureThreshold, ok := graph.M(probe).IntOk("failureThreshold"); ok && failureThreshold != 3 {
		metadata["failureThreshold"] = failureThreshold
	}
	if terminationGracePeriod, ok := graph.M(probe).IntOk("terminationGracePeriodSeconds"); ok {
		metadata["terminationGracePeriodSeconds"] = terminationGracePeriod
	}

	if len(metadata) == 0 {
		return nil
	}

	return NewTree(probeKey, probeType, metadata)
}
