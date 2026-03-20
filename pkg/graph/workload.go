package graph

import kube "github.com/karloie/kompass/pkg/kube"

var workloadOwners = map[string][]struct{ targetType, ownerKind string }{
	"deployment":  {{"replicaset", "Deployment"}},
	"replicaset":  {{"pod", "ReplicaSet"}},
	"statefulset": {{"pod", "StatefulSet"}},
	"daemonset":   {{"pod", "DaemonSet"}},
	"job":         {{"pod", "Job"}},
	"cronjob":     {{"job", "CronJob"}},
}

func inferWorkloadOwner(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, srcKey, targetType, ownerKind string) {
	itemName := M(item.AsMap()).Path("metadata").String("name")
	for _, n := range *nodes {
		if n.Type == targetType {
			for _, owner := range extractOwnerReferences(M(n.AsMap()).Map("metadata")) {
				if M(owner).String("kind") == ownerKind && M(owner).String("name") == itemName {
					*edges = append(*edges, kube.ResourceEdge{Source: n.Key, Target: srcKey, Label: "managed-by"})
				}
			}
		}
	}
}

func extractOwnerReferences(meta M) []map[string]any {
	var result []map[string]any
	if meta == nil {
		return result
	}
	owners := meta.Slice("ownerReferences")
	if owners == nil {
		return result
	}
	for _, v := range owners {
		if m, ok := v.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

func inferWorkload(kind string) func(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
	return func(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
		if key := addNode(edges, item, nodes, kind); key != "" {
			for _, owner := range workloadOwners[kind] {
				inferWorkloadOwner(edges, item, nodes, key, owner.targetType, owner.ownerKind)
			}
		}
		return nil
	}
}

func inferHorizontalPodAutoscaler(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
	key := addNode(edges, item, nodes, "horizontalpodautoscaler")
	if key == "" {
		return nil
	}

	meta, spec := ExtractMetaSpec(item)
	if spec == nil {
		return nil
	}

	if scaleTargetRef := spec.Map("scaleTargetRef"); scaleTargetRef != nil {
		targetKind := scaleTargetRef.String("kind")
		targetName := scaleTargetRef.String("name")
		if targetKind != "" && targetName != "" {
			namespace := ExtractNamespace(meta)
			var targetKey string
			switch targetKind {
			case "Deployment":
				targetKey = "deployment/" + namespace + "/" + targetName
			case "StatefulSet":
				targetKey = "statefulset/" + namespace + "/" + targetName
			case "DaemonSet":
				targetKey = "daemonset/" + namespace + "/" + targetName
			}
			if targetKey != "" {
				addEdge(edges, key, targetKey, "scales")
			}
		}
	}
	return nil
}

func inferReplicaSet(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
	key := addNode(edges, item, nodes, "replicaset")
	if key == "" {
		return nil
	}

	ownedPodCount := 0
	for _, owner := range workloadOwners["replicaset"] {
		before := len(*edges)
		inferWorkloadOwner(edges, item, nodes, key, owner.targetType, owner.ownerKind)
		ownedPodCount += len(*edges) - before
	}

	if ownedPodCount == 0 {
		spec := M(item.AsMap()).Map("spec")
		if spec != nil {
			if selector := spec.Map("selector"); selector != nil {
				if matchLabels := selector.Map("matchLabels"); matchLabels != nil {
					strippedSelector := stripLabelKey(matchLabels.Raw(), "pod-template-hash")
					for _, n := range *nodes {
						if n.Type == "pod" {
							podMeta := M(n.AsMap()).Map("metadata")
							if podMeta != nil {
								if hasOwnerKind(podMeta, "ReplicaSet") {
									continue
								}
								podLabels := podMeta.Map("labels")
								if podLabels != nil {
									strippedPodLabels := stripLabelKey(podLabels.Raw(), "pod-template-hash")
									if matchesLabels(strippedSelector, map[string]any{"labels": strippedPodLabels}) {
										addEdge(edges, n.Key, key, "selector-match")
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return nil
}

func selectorsMatch(pdbSelector, workloadSelector map[string]any) bool {

	for k, v1 := range pdbSelector {
		v2, exists := workloadSelector[k]
		if !exists || v1 != v2 {
			return false
		}
	}
	return true
}

func inferPodDisruptionBudget(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
	meta, spec := ExtractMetaSpec(item)
	namespace, name := ExtractNamespace(meta), M(meta).String("name")
	if namespace == "" || name == "" {
		return nil
	}

	key := "poddisruptionbudget/" + namespace + "/" + name
	addNode(edges, item, nodes, "poddisruptionbudget")

	if spec != nil {
		selector := spec.Map("selector")
		if selector != nil {
			matchLabels := selector.Map("matchLabels")
			if matchLabels != nil {
				workloadTypes := []string{"deployment", "statefulset", "daemonset"}
				for _, workloadType := range workloadTypes {
					forEachNodeOfType(*nodes, workloadType, func(n kube.Resource) {
						workloadSpec := M(n.AsMap()).Map("spec")
						if workloadSpec == nil {
							return
						}
						workloadSelector := workloadSpec.Map("selector")
						if workloadSelector == nil {
							return
						}
						workloadMatchLabels := workloadSelector.Map("matchLabels")
						if workloadMatchLabels != nil && selectorsMatch(matchLabels.Raw(), workloadMatchLabels.Raw()) {
							addEdge(edges, n.Key, key, "protected-by")
						}
					})
				}
			}
		}
	}

	return nil
}
