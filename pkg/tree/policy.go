package tree

import (
	"strings"

	kube "github.com/karloie/kompass/pkg/kube"
)

var reverseEdgeBlockedTargetTypes = map[string]bool{
	"node":                  true,
	"storageclass":          true,
	"namespace":             true,
	"secret":                true,
	"configmap":             true,
	"persistentvolumeclaim": true,
}

var reverseEdgeBlockedPairs = map[string]map[string]bool{
	"gateway": {
		"httproute": true,
	},
}

var workloadFilteredChildTypes = map[string]bool{
	"replicaset":          true,
	"pod":                 true,
	"service":             true,
	"ciliumnetworkpolicy": true,
}

var replicaSetFilteredChildTypes = map[string]bool{
	"pod": true,
}

var jobFilteredChildTypes = map[string]bool{
	"cronjob": true,
}

var workloadHoistMatchers = map[string]func(childKey, namespace string, podLabels map[string]any, nodeMap map[string]kube.Resource) bool{
	"service":             serviceSelectsWorkload,
	"ciliumnetworkpolicy": policyAppliesToWorkload,
}

func shouldAddReverseEdge(sourceType, targetType string) bool {
	if reverseEdgeBlockedTargetTypes[targetType] {
		return false
	}
	if blockedTargets, exists := reverseEdgeBlockedPairs[sourceType]; exists && blockedTargets[targetType] {
		return false
	}
	return true
}

// FilterOwnedJobRoots removes Job graph roots when their owning CronJob is also a graph root.
func FilterOwnedJobRoots(result *kube.Response) {
	if result == nil || len(result.Components) < 2 {
		return
	}

	rootIDs := make(map[string]bool, len(result.Components))
	for _, component := range result.Components {
		rootIDs[component.Root] = true
	}

	filtered := make([]kube.Component, 0, len(result.Components))
	for _, component := range result.Components {
		if ParseResourceKeyRef(component.Root).Type != "job" {
			filtered = append(filtered, component)
			continue
		}

		rootNode := rootNodeForComponent(result, &component)
		if rootNode == nil {
			filtered = append(filtered, component)
			continue
		}

		namespace, ownerRefs := ownerRefsFromResource(rootNode)
		hasCronJobRoot := false
		for _, owner := range ownerRefs {
			if !strings.EqualFold(stringMapValue(owner, "kind"), "CronJob") {
				continue
			}
			ownerName := stringMapValue(owner, "name")
			if ownerName == "" {
				continue
			}
			cronJobKey := BuildResourceKeyRef("cronjob", namespace, ownerName)
			if rootIDs[cronJobKey] {
				hasCronJobRoot = true
				break
			}
		}

		if hasCronJobRoot {
			continue
		}

		filtered = append(filtered, component)
	}

	result.Components = filtered
}

// FilterOwnedSecretRoots removes Secret graph roots when their owning workload is also a graph root.
func FilterOwnedSecretRoots(result *kube.Response) {
	if result == nil || len(result.Components) < 2 {
		return
	}

	rootIDs := make(map[string]bool, len(result.Components))
	for _, component := range result.Components {
		rootIDs[component.Root] = true
	}

	filtered := make([]kube.Component, 0, len(result.Components))
	for _, component := range result.Components {
		if ParseResourceKeyRef(component.Root).Type != "secret" {
			filtered = append(filtered, component)
			continue
		}

		rootNode := rootNodeForComponent(result, &component)
		if rootNode == nil {
			filtered = append(filtered, component)
			continue
		}

		namespace, ownerRefs := ownerRefsFromResource(rootNode)
		removeSecretRoot := false
		for _, owner := range ownerRefs {
			ownerKind := strings.ToLower(stringMapValue(owner, "kind"))
			ownerName := stringMapValue(owner, "name")
			if ownerKind == "" || ownerName == "" {
				continue
			}

			if ownerOrAncestorIsRoot(ownerKind, namespace, ownerName, rootIDs, result.NodeMap(), map[string]bool{}) {
				removeSecretRoot = true
				break
			}
		}

		if removeSecretRoot {
			continue
		}

		filtered = append(filtered, component)
	}

	result.Components = filtered
}

func ownerOrAncestorIsRoot(ownerType, namespace, ownerName string, rootIDs map[string]bool, nodes map[string]*kube.Resource, visited map[string]bool) bool {
	ownerKey := BuildResourceKeyRef(ownerType, namespace, ownerName)
	if rootIDs[ownerKey] {
		return true
	}

	if visited[ownerKey] {
		return false
	}
	visited[ownerKey] = true

	ownerNode, ok := nodes[ownerKey]
	if !ok || ownerNode == nil {
		return false
	}

	ownerNamespace, ownerRefs := ownerRefsFromResource(ownerNode)
	if ownerNamespace == "" {
		ownerNamespace = namespace
	}

	for _, ref := range ownerRefs {
		nextType := strings.ToLower(stringMapValue(ref, "kind"))
		nextName := stringMapValue(ref, "name")
		if nextType == "" || nextName == "" {
			continue
		}
		if ownerOrAncestorIsRoot(nextType, ownerNamespace, nextName, rootIDs, nodes, visited) {
			return true
		}
	}

	return false
}

func rootNodeForComponent(result *kube.Response, component *kube.Component) *kube.Resource {
	if result == nil || component == nil {
		return nil
	}
	return result.Node(component.Root)
}

func ownerRefsFromResource(resource *kube.Resource) (string, []map[string]any) {
	if resource == nil {
		return "", nil
	}

	resourceMap := resource.AsMap()
	if resourceMap == nil {
		return "", nil
	}

	meta, _ := resourceMap["metadata"].(map[string]any)
	if meta == nil {
		return "", nil
	}

	namespace := stringMapValue(meta, "namespace")
	rawOwners, _ := meta["ownerReferences"].([]any)
	if len(rawOwners) == 0 {
		return namespace, nil
	}

	owners := make([]map[string]any, 0, len(rawOwners))
	for _, rawOwner := range rawOwners {
		owner, ok := rawOwner.(map[string]any)
		if !ok {
			continue
		}
		owners = append(owners, owner)
	}

	return namespace, owners
}

func stringMapValue(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}
