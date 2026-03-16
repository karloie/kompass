package tree

import (
	"fmt"
	"strings"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

// enrichTreeMeta walks a built tree and populates empty Meta maps with
// structured resource metadata. This is the data the text renderer extracted
// in its own second pass; here we expose it directly on the tree object so
// web clients have display labels without any ASCII-tree formatting.
func enrichTreeMeta(node *kube.Tree, nodeMap map[string]*kube.Resource) {
	if node == nil {
		return
	}
	if len(node.Meta) == 0 {
		if resource, ok := nodeMap[node.Key]; ok {
			node.Meta = extractMetadataFromResource(*resource, nodeMap)
		}
	}
	for _, child := range node.Children {
		enrichTreeMeta(child, nodeMap)
	}
}

func extractMetadataFromResource(resource kube.Resource, nodeMap map[string]*kube.Resource) map[string]any {

	metadata := ApplyMetadataRules(resource, nodeMap)

	if resource.Type == "certificate" {

		if expiryData, ok := metadata["notAfter"].(map[string]any); ok {
			for k, v := range expiryData {
				metadata[k] = v
			}
			delete(metadata, "notAfter")
		}

		if conditionsData, ok := metadata["conditions"].(map[string]any); ok {
			for k, v := range conditionsData {
				metadata[k] = v
			}
			delete(metadata, "conditions")
		}
	}

	if replicaCounts, ok := metadata["replicaCounts"].(map[string]any); ok {
		for k, v := range replicaCounts {
			metadata[k] = v
		}
		delete(metadata, "replicaCounts")
	}

	if daemonCounts, ok := metadata["daemonCounts"].(map[string]any); ok {
		for k, v := range daemonCounts {
			metadata[k] = v
		}
		delete(metadata, "daemonCounts")
	}

	if jobCounts, ok := metadata["jobCounts"].(map[string]any); ok {
		for k, v := range jobCounts {
			metadata[k] = v
		}
		delete(metadata, "jobCounts")
	}

	if hpaConfig, ok := metadata["hpaConfig"].(map[string]any); ok {
		for k, v := range hpaConfig {
			metadata[k] = v
		}
		delete(metadata, "hpaConfig")
	}
	if hpaStatus, ok := metadata["hpaStatus"].(map[string]any); ok {
		for k, v := range hpaStatus {
			metadata[k] = v
		}
		delete(metadata, "hpaStatus")
	}

	if issuerConfig, ok := metadata["issuerConfig"].(map[string]any); ok {
		for k, v := range issuerConfig {
			metadata[k] = v
		}
		delete(metadata, "issuerConfig")
	}

	if nodeInfo, ok := metadata["nodeInfo"].(map[string]any); ok {
		for k, v := range nodeInfo {
			metadata[k] = v
		}
		delete(metadata, "nodeInfo")
	}

	if resource.Type == "deployment" {
		if conditions, ok := metadata["conditions"].(string); ok {
			metadata["status"] = conditions
			delete(metadata, "conditions")
		}
	}
	if resource.Type == "job" {
		if conditions, ok := metadata["conditions"].(string); ok {
			metadata["status"] = conditions
			delete(metadata, "conditions")
		}

		if _, hasStatus := metadata["status"]; !hasStatus {
			if active, ok := metadata["active"].(int); ok && active > 0 {
				metadata["status"] = "Active"
			}
		}
	}

	if resource.Type == "node" {
		if conditions, ok := metadata["conditions"].(string); ok {
			metadata["status"] = conditions
			delete(metadata, "conditions")
		}
	}

	if resource.Type == "endpointslice" {
		if firstPort, ok := metadata["firstPort"].(map[string]any); ok {
			for k, v := range firstPort {
				metadata[k] = v
			}
			delete(metadata, "firstPort")
		}
	}

	if resource.Type == "httproute" {
		if serviceStr, ok := metadata["service"].(string); ok {

			namespace := ""
			if ns, ok := metadata["namespace"].(string); ok {
				namespace = ns
			}
			if namespace != "" {
				serviceKey := "service/" + namespace + "/" + serviceStr
				if svcResource, ok := nodeMap[serviceKey]; ok {
					svcType := "ClusterIP"
					var portStrs []string
					if spec, ok := svcResource.AsMap()["spec"].(map[string]any); ok {
						if t, ok := spec["type"].(string); ok && t != "" {
							svcType = t
						}

						if ports, ok := spec["ports"].([]any); ok && len(ports) > 0 {
							for _, p := range ports {
								if portMap, ok := p.(map[string]any); ok {
									var portNum int
									var protocol string
									if pn, ok := graph.M(portMap).IntOk("port"); ok {
										portNum = pn
									}
									if proto, ok := portMap["protocol"].(string); ok && proto != "" {
										protocol = proto
									} else {
										protocol = "TCP"
									}
									if portNum > 0 {
										portStrs = append(portStrs, fmt.Sprintf("%d/%s", portNum, protocol))
									}
								}
							}
						}
					}
					if len(portStrs) > 0 {
						metadata["service"] = serviceStr + ":" + strings.Join(portStrs, ",") + " (" + svcType + ")"
					} else {
						metadata["service"] = serviceStr + " (" + svcType + ")"
					}
				}
			}
		}
	}

	legacyExtractMetadata(resource, metadata, nodeMap)

	if _, hasStatus := metadata["status"]; !hasStatus {
		if status := extractRuntimeStatus(resource.AsMap(), resource.Type); status != "" {
			metadata["status"] = status
		}
	}

	return metadata
}

func extractRuntimeStatus(resourceMap map[string]any, resourceType string) string {
	if resourceMap == nil {
		return ""
	}

	spec, _ := resourceMap["spec"].(map[string]any)
	status, _ := resourceMap["status"].(map[string]any)

	switch resourceType {
	case "deployment":
		specReplicas := graph.M(spec).Int("replicas")
		unavailable := graph.M(status).Int("unavailableReplicas")
		available := graph.M(status).Int("availableReplicas")
		ready := graph.M(status).Int("readyReplicas")
		total := graph.M(status).Int("replicas")

		if unavailable > 0 {
			return "Degraded"
		}
		if specReplicas > 0 && available == specReplicas && ready == specReplicas {
			return "Ready"
		}
		if total > 0 {
			return "Progressing"
		}

	case "replicaset":
		specReplicas := graph.M(spec).Int("replicas")
		ready := graph.M(status).Int("readyReplicas")
		total := graph.M(status).Int("replicas")

		if total > 0 && ready == total && total == specReplicas {
			return "Ready"
		}
		if total > 0 {
			return "Progressing"
		}

	case "statefulset":
		specReplicas := graph.M(spec).Int("replicas")
		ready := graph.M(status).Int("readyReplicas")

		if specReplicas > 0 && ready == specReplicas {
			return "Ready"
		}
		if ready > 0 {
			return "Progressing"
		}

	case "daemonset":
		desired := graph.M(status).Int("desiredNumberScheduled")
		ready := graph.M(status).Int("numberReady")

		if desired > 0 && ready == desired {
			return "Ready"
		}
		if ready > 0 {
			return "Progressing"
		}

	case "persistentvolumeclaim":
		if phase, ok := status["phase"].(string); ok {
			return phase
		}

	case "ingress":
		if lb, ok := status["loadBalancer"].(map[string]any); ok {
			if ingress, ok := lb["ingress"].([]any); ok && len(ingress) > 0 {
				return "Active"
			}
		}

	case "horizontalpodautoscaler":
		current := graph.M(status).Int("currentReplicas")
		desired := graph.M(status).Int("desiredReplicas")

		if current == desired {
			return "Stable"
		}
		if current < desired {
			return "Scaling Up"
		}
		if current > desired {
			return "Scaling Down"
		}

	case "cronjob":
		if suspend, ok := spec["suspend"].(bool); ok && suspend {
			return "Suspended"
		}
		if active, ok := status["active"].([]any); ok && len(active) > 0 {
			return "Active"
		}
		if last, ok := status["lastScheduleTime"]; ok && last != nil {
			if s, ok := last.(string); !ok || s != "" {
				return "Scheduled"
			}
		}

	case "poddisruptionbudget":
		if graph.M(status).Int("currentHealthy") >= graph.M(status).Int("desiredHealthy") {
			return "Healthy"
		}
		return "Disrupted"

	case "certificate":
		return extractConditionStatus(resourceMap, "Ready", "Ready", "NotReady")

	case "gateway":
		return extractConditionStatus(resourceMap, "Programmed", "Programmed", "NotProgrammed")

	case "issuer", "clusterissuer":
		return extractConditionStatus(resourceMap, "Ready", "Ready", "NotReady")
	}

	return ""
}

func extractConditionStatus(resource map[string]any, condType, trueVal, falseVal string) string {
	statusMap, ok := resource["status"].(map[string]any)
	if !ok {
		return ""
	}
	conditions, ok := statusMap["conditions"].([]any)
	if !ok {
		return ""
	}
	for _, cond := range conditions {
		condMap, ok := cond.(map[string]any)
		if !ok {
			continue
		}
		if ct, _ := condMap["type"].(string); ct == condType {
			if cs, _ := condMap["status"].(string); cs == "True" {
				return trueVal
			}
			return falseVal
		}
	}
	return ""
}

func legacyExtractMetadata(resource kube.Resource, metadata map[string]any, nodeMap map[string]*kube.Resource) {

	ruledTypes := map[string]bool{
		"service":                 true,
		"pod":                     true,
		"secret":                  true,
		"configmap":               true,
		"certificate":             true,
		"persistentvolumeclaim":   true,
		"gateway":                 true,
		"ciliumnetworkpolicy":     true,
		"deployment":              true,
		"statefulset":             true,
		"daemonset":               true,
		"replicaset":              true,
		"job":                     true,
		"cronjob":                 true,
		"horizontalpodautoscaler": true,
		"httproute":               true,
		"ingress":                 true,
		"issuer":                  true,
		"clusterissuer":           true,
		"persistentvolume":        true,
		"storageclass":            true,
		"volumeattachment":        true,
		"node":                    true,
		"namespace":               true,
		"networkpolicy":           true,
		"endpointslice":           true,
	}
	if ruledTypes[resource.Type] {
		return
	}

	if meta, ok := resource.AsMap()["metadata"].(map[string]any); ok {
		if _, hasNamespace := metadata["namespace"]; !hasNamespace {
			if namespace, ok := meta["namespace"].(string); ok {
				metadata["namespace"] = namespace
			}
		}
	}
}
