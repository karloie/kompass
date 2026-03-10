package tree

import kube "github.com/karloie/kompass/pkg/kube"

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
