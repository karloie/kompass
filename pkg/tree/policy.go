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
func FilterOwnedJobRoots(result *kube.Graphs) {
	if result == nil || len(result.Graphs) < 2 {
		return
	}

	rootIDs := make(map[string]bool, len(result.Graphs))
	for _, g := range result.Graphs {
		rootIDs[g.ID] = true
	}

	filtered := make([]kube.Graph, 0, len(result.Graphs))
	for _, g := range result.Graphs {
		if ParseResourceKeyRef(g.ID).Type != "job" {
			filtered = append(filtered, g)
			continue
		}

		rootNode := rootNodeForGraph(result, &g)
		if rootNode == nil {
			filtered = append(filtered, g)
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

		filtered = append(filtered, g)
	}

	result.Graphs = filtered
}

func rootNodeForGraph(result *kube.Graphs, g *kube.Graph) *kube.Resource {
	if result != nil && result.Nodes != nil {
		if node := result.Nodes[g.ID]; node != nil {
			return node
		}
	}
	return nil
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
