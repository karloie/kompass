package tree

import (
	"sort"
	"strings"
	"time"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

var cronJobFilteredChildTypes = map[string]bool{
	"job": true,
}

func appendHoistedChildren(children []*kube.Tree, hoistedKeys map[string]bool, targetType string, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	for hoistedKey := range hoistedKeys {
		if getTypeFromKey(hoistedKey, nodeMap) != targetType {
			continue
		}
		state.Unsuppress(hoistedKey)
		node := buildTreeNode(hoistedKey, graphChildren, state, nodeMap)
		if node != nil {
			children = append(children, node)
		}
	}
	return children
}

func extractTemplateSpecAndLabels(spec map[string]any) (map[string]any, map[string]any) {
	if spec == nil {
		return nil, nil
	}
	template := graph.M(spec).Map("template")
	if template == nil {
		return nil, nil
	}
	templateSpec := template.Map("spec").Raw()
	var podLabels map[string]any
	if templateMeta := template.Map("metadata"); templateMeta != nil {
		podLabels = templateMeta.Map("labels").Raw()
	}
	return templateSpec, podLabels
}

func workloadPodKeys(workloadKey string, workloadType string, graphChildren map[string][]string, nodeMap map[string]kube.Resource) []string {
	podKeys := make([]string, 0)
	if workloadType == "deployment" {
		for _, rsKey := range childKeysOfType(workloadKey, "replicaset", graphChildren, nodeMap) {
			podKeys = append(podKeys, childKeysOfType(rsKey, "pod", graphChildren, nodeMap)...)
		}
		return podKeys
	}
	return childKeysOfType(workloadKey, "pod", graphChildren, nodeMap)
}

func collectHoistedWorkloadKeys(podKeys []string, namespace string, podLabels map[string]any, graphChildren map[string][]string, nodeMap map[string]kube.Resource) map[string]bool {
	hoistedKeys := make(map[string]bool)
	for _, podKey := range podKeys {
		for _, childKey := range graphChildren[podKey] {
			childType := getTypeFromKey(childKey, nodeMap)
			matcher, exists := workloadHoistMatchers[childType]
			if exists && matcher(childKey, namespace, podLabels, nodeMap) {
				hoistedKeys[childKey] = true
			}
		}
	}
	return hoistedKeys
}

func appendReplicasetChildren(children []*kube.Tree, workloadKey string, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	for _, rsKey := range childKeysOfType(workloadKey, "replicaset", graphChildren, nodeMap) {
		rsNode := buildTreeNode(rsKey, graphChildren, state, nodeMap)
		if rsNode != nil {
			children = append(children, rsNode)
		}
	}
	return children
}

func isReplicaSetOwnedByDeployment(metadata map[string]any) bool {
	if metadata == nil {
		return false
	}
	owners := graph.M(metadata).Slice("ownerReferences")
	for _, owner := range owners {
		if ownerMap, ok := owner.(map[string]any); ok {
			if graph.M(ownerMap).String("kind") == "Deployment" {
				return true
			}
		}
	}
	return false
}

func buildWorkloadChildren(workloadKey string, workload kube.Resource, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	children := []*kube.Tree{}

	metadata := graph.M(workload.AsMap()).Map("metadata").Raw()
	spec := graph.M(workload.AsMap()).Map("spec").Raw()

	if spec == nil || metadata == nil {
		return children
	}

	namespace := graph.M(metadata).String("namespace")
	templateSpec, podLabels := extractTemplateSpecAndLabels(spec)
	podKeys := workloadPodKeys(workloadKey, workload.Type, graphChildren, nodeMap)
	hoistedKeys := collectHoistedWorkloadKeys(podKeys, namespace, podLabels, graphChildren, nodeMap)
	markSuppressedSet(state, hoistedKeys)
	markSuppressedKeys(state, podKeys)

	if workload.Type == "deployment" {
		if templateSpec != nil {
			specKey := workloadKey + "/spec"
			specNode := NewTree(specKey, "spec", map[string]any{})
			specNode.Children = buildPodTemplateChildren(specKey, namespace, templateSpec, graphChildren, state, nodeMap)
			children = append(children, specNode)
		}
		children = appendReplicasetChildren(children, workloadKey, graphChildren, state, nodeMap)
	} else if workload.Type == "daemonset" || workload.Type == "statefulset" {
		if templateSpec != nil {
			specKey := workloadKey + "/spec"
			specNode := NewTree(specKey, "spec", map[string]any{})
			specNode.Children = buildPodTemplateChildren(specKey, namespace, templateSpec, graphChildren, state, nodeMap)
			children = append(children, specNode)
		}

		for _, podKey := range podKeys {
			if podRes, exists := nodeMap[podKey]; exists {
				simplifiedPod := buildSimplifiedPodNode(podKey, podRes)
				if simplifiedPod != nil {
					children = append(children, simplifiedPod)
				}
			}
		}
	}

	children = appendHoistedChildren(children, hoistedKeys, "ciliumnetworkpolicy", graphChildren, state, nodeMap)
	children = appendHoistedChildren(children, hoistedKeys, "service", graphChildren, state, nodeMap)
	children = appendFilteredGraphChildren(children, workloadKey, workloadFilteredChildTypes, graphChildren, state, nodeMap)

	sortChildren(children)
	return children
}

func buildReplicaSetChildren(rsKey string, rs kube.Resource, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	children := []*kube.Tree{}

	metadata := graph.M(rs.AsMap()).Map("metadata").Raw()
	spec := graph.M(rs.AsMap()).Map("spec").Raw()
	namespace := graph.M(metadata).String("namespace")
	ownedByDeployment := isReplicaSetOwnedByDeployment(metadata)
	templateSpec, _ := extractTemplateSpecAndLabels(spec)

	if ownedByDeployment {
	} else {
		if templateSpec != nil {
			templateChildren := buildPodTemplateChildren(rsKey, namespace, templateSpec, graphChildren, state, nodeMap)
			children = append(children, templateChildren...)
		}
	}

	podKeys := childKeysOfType(rsKey, "pod", graphChildren, nodeMap)

	for _, podKey := range podKeys {
		if podRes, exists := nodeMap[podKey]; exists {
			var podNode *kube.Tree
			if ownedByDeployment {
				podNode = buildPodWithSimplifiedContainers(podKey, podRes)
			} else {
				podNode = buildSimplifiedPodNode(podKey, podRes)
			}
			if podNode != nil {
				children = append(children, podNode)
			}
		}
	}

	children = appendFilteredGraphChildren(children, rsKey, replicaSetFilteredChildTypes, graphChildren, state, nodeMap)

	sortChildren(children)
	return children
}

func buildJobChildren(jobKey string, job kube.Resource, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	children := []*kube.Tree{}
	children = appendFilteredGraphChildren(children, jobKey, jobFilteredChildTypes, graphChildren, state, nodeMap)
	sortChildren(children)
	return children
}

func buildCronJobChildren(cronJobKey string, cronJob kube.Resource, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	children := []*kube.Tree{}

	jobKeys := childKeysOfType(cronJobKey, "job", graphChildren, nodeMap)
	focusJobKey := selectCronJobFocusJob(jobKeys, nodeMap)

	for _, jobKey := range jobKeys {
		if !state.CanTraverse(jobKey) {
			continue
		}

		_, exists := nodeMap[jobKey]
		if !exists {
			continue
		}

		if jobKey == focusJobKey {
			jobNode := buildTreeNode(jobKey, graphChildren, state, nodeMap)
			if jobNode != nil {
				children = append(children, jobNode)
			}
			continue
		}

		state.MarkSeen(jobKey)
		jobNode := NewTree(jobKey, "job", map[string]any{})
		for _, podKey := range childKeysOfType(jobKey, "pod", graphChildren, nodeMap) {
			if !state.CanTraverse(podKey) {
				continue
			}
			podRes, exists := nodeMap[podKey]
			if !exists {
				continue
			}
			podLeaf := buildSimplifiedPodNode(podKey, podRes)
			if podLeaf != nil {
				jobNode.Children = append(jobNode.Children, podLeaf)
				state.MarkSeen(podKey)
			}
		}
		sortChildren(jobNode.Children)
		children = append(children, jobNode)
	}

	children = appendFilteredGraphChildren(children, cronJobKey, cronJobFilteredChildTypes, graphChildren, state, nodeMap)
	sortChildren(children)
	return children
}

func selectCronJobFocusJob(jobKeys []string, nodeMap map[string]kube.Resource) string {
	if len(jobKeys) == 0 {
		return ""
	}

	sortedKeys := append([]string(nil), jobKeys...)
	sort.Strings(sortedKeys)

	focusJobKey := ""
	focusTime := time.Time{}
	for _, jobKey := range sortedKeys {
		jobRes, exists := nodeMap[jobKey]
		if !exists || !isActiveJob(jobRes) {
			continue
		}
		jobTime := jobSortTimestamp(jobRes)
		if focusJobKey == "" || jobTime.After(focusTime) || (jobTime.Equal(focusTime) && jobKey > focusJobKey) {
			focusJobKey = jobKey
			focusTime = jobTime
		}
	}
	if focusJobKey != "" {
		return focusJobKey
	}

	for _, jobKey := range sortedKeys {
		jobRes, exists := nodeMap[jobKey]
		if !exists {
			continue
		}
		jobTime := jobSortTimestamp(jobRes)
		if focusJobKey == "" || jobTime.After(focusTime) || (jobTime.Equal(focusTime) && jobKey > focusJobKey) {
			focusJobKey = jobKey
			focusTime = jobTime
		}
	}

	if focusJobKey == "" {
		return sortedKeys[len(sortedKeys)-1]
	}

	return focusJobKey
}

func isActiveJob(job kube.Resource) bool {
	status := graph.M(job.AsMap()).Map("status")
	if status == nil {
		return false
	}
	if active, ok := status.IntOk("active"); ok {
		return active > 0
	}
	return false
}

func jobSortTimestamp(job kube.Resource) time.Time {
	if startTime := graph.M(job.AsMap()).Map("status").String("startTime"); startTime != "" {
		if ts, err := time.Parse(time.RFC3339, startTime); err == nil {
			return ts
		}
	}
	if creationTime := graph.M(job.AsMap()).Map("metadata").String("creationTimestamp"); creationTime != "" {
		if ts, err := time.Parse(time.RFC3339, creationTime); err == nil {
			return ts
		}
	}
	return time.Time{}
}

func resourceMatchesSelector(resourceKey string, workloadNamespace string, podLabels map[string]any, nodeMap map[string]kube.Resource, selectorPaths []string) bool {
	resource, exists := nodeMap[resourceKey]
	if !exists || podLabels == nil {
		return false
	}

	if meta := graph.M(resource.AsMap()).Map("metadata"); meta != nil {
		if meta.String("namespace") != workloadNamespace {
			return false
		}
	} else {
		return false
	}

	spec := graph.M(resource.AsMap()).Map("spec")
	if spec == nil {
		return false
	}

	var selector graph.M
	for _, path := range selectorPaths {
		parts := strings.Split(path, ".")
		current := spec
		for _, part := range parts {
			if current == nil {
				break
			}
			current = current.Map(part)
		}
		if current != nil {
			selector = current
			break
		}
	}

	if selector == nil {
		return false
	}

	for key, selectorValue := range selector.Raw() {
		if podValue, exists := podLabels[key]; !exists || podValue != selectorValue {
			return false
		}
	}

	return true
}

func policyAppliesToWorkload(policyKey string, workloadNamespace string, podLabels map[string]any, nodeMap map[string]kube.Resource) bool {
	return resourceMatchesSelector(policyKey, workloadNamespace, podLabels, nodeMap, []string{"matchLabels", "endpointSelector.matchLabels"})
}

func serviceSelectsWorkload(serviceKey string, workloadNamespace string, podLabels map[string]any, nodeMap map[string]kube.Resource) bool {
	return resourceMatchesSelector(serviceKey, workloadNamespace, podLabels, nodeMap, []string{"selector"})
}
