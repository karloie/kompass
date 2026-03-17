package tree

import (
	"fmt"
	"sort"
	"time"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
	jp "github.com/ohler55/ojg/jp"
)

type MetadataRule struct {
	Type      string
	Field     string
	Path      string
	Format    string
	Condition string
	expr      jp.Expr
}

var MetadataRules []MetadataRule

func init() {
	rules := []MetadataRule{

		{Type: "service", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "service", Field: "ports", Path: "$.spec.ports", Format: "portArray"},
		{Type: "service", Field: "type", Path: "$.spec.type", Format: "string", Condition: "notEmpty"},
		{Type: "service", Field: "clusterIP", Path: "$.spec.clusterIP", Format: "stringNotNone"},
		{Type: "service", Field: "selector", Path: "$.spec.selector", Format: "selectorMap"},
		{Type: "service", Field: "externalIPs", Path: "$.spec.externalIPs", Format: "stringArray", Condition: "notEmpty"},
		{Type: "service", Field: "loadBalancerIP", Path: "$.status.loadBalancer.ingress", Format: "lbIngress", Condition: "notEmpty"},

		{Type: "pod", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "pod", Field: "nodeName", Path: "$.spec.nodeName", Format: "string", Condition: "notEmpty"},
		{Type: "pod", Field: "phase", Path: "$.status.phase", Format: "string", Condition: "notEmpty"},
		{Type: "pod", Field: "podIP", Path: "$.status.podIP", Format: "string", Condition: "notEmpty"},
		{Type: "pod", Field: "qosClass", Path: "$.status.qosClass", Format: "string", Condition: "notEmpty"},
		{Type: "pod", Field: "startTime", Path: "$.status.startTime", Format: "age"},
		{Type: "pod", Field: "restarts", Path: "$.status.containerStatuses", Format: "restarts"},
		{Type: "pod", Field: "status", Path: "$", Format: "podStatus"},

		{Type: "secret", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "secret", Field: "type", Path: "$.type", Format: "string", Condition: "notEmpty"},
		{Type: "secret", Field: "keys", Path: "$", Format: "secretKeys"},

		{Type: "secretproviderclass", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "secretproviderclass", Field: "provider", Path: "$.spec.provider", Format: "string", Condition: "notEmpty"},
		{Type: "secretproviderclass", Field: "secretObjects", Path: "$.spec.secretObjects", Format: "secretObjectCount", Condition: "notEmpty"},

		{Type: "configmap", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "configmap", Field: "keys", Path: "$", Format: "configMapKeys"},

		{Type: "certificate", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "certificate", Field: "dnsNames", Path: "$.spec.dnsNames", Format: "stringArray", Condition: "notEmpty"},
		{Type: "certificate", Field: "conditions", Path: "$.status.conditions", Format: "certConditions"},
		{Type: "certificate", Field: "notAfter", Path: "$.status.notAfter", Format: "certExpiry"},

		{Type: "ciliumnetworkpolicy", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "ciliumnetworkpolicy", Field: "ingress", Path: "$.spec.enableDefaultDeny.ingress", Format: "bool"},
		{Type: "ciliumnetworkpolicy", Field: "egress", Path: "$.spec.enableDefaultDeny.egress", Format: "bool"},
		{Type: "ciliumclusterwidenetworkpolicy", Field: "ingress", Path: "$.spec.enableDefaultDeny.ingress", Format: "bool"},
		{Type: "ciliumclusterwidenetworkpolicy", Field: "egress", Path: "$.spec.enableDefaultDeny.egress", Format: "bool"},

		{Type: "persistentvolumeclaim", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "persistentvolumeclaim", Field: "accessModes", Path: "$.spec.accessModes", Format: "stringArray", Condition: "notEmpty"},
		{Type: "persistentvolumeclaim", Field: "storage", Path: "$.spec.resources.requests.storage", Format: "string"},
		{Type: "persistentvolumeclaim", Field: "storageClass", Path: "$.spec.storageClassName", Format: "string", Condition: "notEmpty"},
		{Type: "persistentvolumeclaim", Field: "volumeName", Path: "$.spec.volumeName", Format: "string", Condition: "notEmpty"},
		{Type: "persistentvolumeclaim", Field: "phase", Path: "$.status.phase", Format: "string", Condition: "notEmpty"},
		{Type: "persistentvolumeclaim", Field: "capacity", Path: "$.status.capacity.storage", Format: "string"},

		{Type: "gateway", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "gateway", Field: "gatewayClass", Path: "$.spec.gatewayClassName", Format: "string", Condition: "notEmpty"},
		{Type: "gateway", Field: "listeners", Path: "$.spec.listeners", Format: "gatewayListeners", Condition: "notEmpty"},

		{Type: "deployment", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "deployment", Field: "replicas", Path: "$.spec.replicas", Format: "int"},
		{Type: "deployment", Field: "strategy", Path: "$.spec.strategy.type", Format: "strategyType"},
		{Type: "deployment", Field: "replicaCounts", Path: "$.status", Format: "replicaCounts"},
		{Type: "deployment", Field: "conditions", Path: "$.status.conditions", Format: "deploymentConditions"},

		{Type: "statefulset", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "statefulset", Field: "replicas", Path: "$.spec.replicas", Format: "int"},
		{Type: "statefulset", Field: "serviceName", Path: "$.spec.serviceName", Format: "string", Condition: "notEmpty"},
		{Type: "statefulset", Field: "updateStrategy", Path: "$.spec.updateStrategy.type", Format: "strategyType"},
		{Type: "statefulset", Field: "replicaCounts", Path: "$.status", Format: "replicaCounts"},
		{Type: "statefulset", Field: "status", Path: "$", Format: "statefulsetStatus"},

		{Type: "daemonset", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "daemonset", Field: "updateStrategy", Path: "$.spec.updateStrategy.type", Format: "strategyType"},
		{Type: "daemonset", Field: "daemonCounts", Path: "$.status", Format: "daemonCounts"},
		{Type: "daemonset", Field: "status", Path: "$", Format: "daemonsetStatus"},

		{Type: "replicaset", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "replicaset", Field: "replicas", Path: "$.spec.replicas", Format: "int"},
		{Type: "replicaset", Field: "replicaCounts", Path: "$.status", Format: "replicaCounts"},

		{Type: "job", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "job", Field: "cronjob", Path: "$.metadata.ownerReferences", Format: "jobOwnerCronJob", Condition: "notEmpty"},
		{Type: "job", Field: "completions", Path: "$.spec.completions", Format: "int"},
		{Type: "job", Field: "parallelism", Path: "$.spec.parallelism", Format: "int"},
		{Type: "job", Field: "jobCounts", Path: "$.status", Format: "jobCounts"},
		{Type: "job", Field: "conditions", Path: "$.status.conditions", Format: "jobConditions"},
		{Type: "job", Field: "duration", Path: "$.status", Format: "jobDuration"},

		{Type: "cronjob", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "cronjob", Field: "schedule", Path: "$.spec.schedule", Format: "string", Condition: "notEmpty"},
		{Type: "cronjob", Field: "suspended", Path: "$.spec.suspend", Format: "bool"},
		{Type: "cronjob", Field: "lastSchedule", Path: "$.status.lastScheduleTime", Format: "age"},

		{Type: "horizontalpodautoscaler", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "horizontalpodautoscaler", Field: "hpaConfig", Path: "$.spec", Format: "hpaConfig"},
		{Type: "horizontalpodautoscaler", Field: "hpaStatus", Path: "$.status", Format: "hpaStatus"},

		{Type: "httproute", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "httproute", Field: "hostnames", Path: "$.spec.hostnames", Format: "stringArray", Condition: "notEmpty"},
		{Type: "httproute", Field: "paths", Path: "$.spec.rules", Format: "httproutePaths"},
		{Type: "httproute", Field: "service", Path: "$.spec.rules", Format: "httprouteService"},

		{Type: "ingress", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "ingress", Field: "ingressClass", Path: "$.spec.ingressClassName", Format: "string", Condition: "notEmpty"},
		{Type: "ingress", Field: "hosts", Path: "$.spec.rules", Format: "ingressHosts"},
		{Type: "ingress", Field: "tls", Path: "$.spec.tls", Format: "hasTLS"},

		{Type: "issuer", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "issuer", Field: "age", Path: "$.metadata.creationTimestamp", Format: "age"},
		{Type: "issuer", Field: "status", Path: "$.status.conditions", Format: "issuerStatus"},
		{Type: "issuer", Field: "readyReason", Path: "$.status.conditions", Format: "issuerReadyReason"},
		{Type: "issuer", Field: "type", Path: "$.spec", Format: "issuerType"},
		{Type: "issuer", Field: "issuerConfig", Path: "$.spec", Format: "issuerConfig"},

		{Type: "clusterissuer", Field: "age", Path: "$.metadata.creationTimestamp", Format: "age"},
		{Type: "clusterissuer", Field: "status", Path: "$.status.conditions", Format: "issuerStatus"},
		{Type: "clusterissuer", Field: "readyReason", Path: "$.status.conditions", Format: "issuerReadyReason"},
		{Type: "clusterissuer", Field: "type", Path: "$.spec", Format: "issuerType"},
		{Type: "clusterissuer", Field: "issuerConfig", Path: "$.spec", Format: "issuerConfig"},

		{Type: "persistentvolume", Field: "capacity", Path: "$.spec.capacity.storage", Format: "string"},
		{Type: "persistentvolume", Field: "accessModes", Path: "$.spec.accessModes", Format: "stringArray", Condition: "notEmpty"},
		{Type: "persistentvolume", Field: "storageClass", Path: "$.spec.storageClassName", Format: "string", Condition: "notEmpty"},
		{Type: "persistentvolume", Field: "reclaimPolicy", Path: "$.spec.persistentVolumeReclaimPolicy", Format: "string", Condition: "notEmpty"},
		{Type: "persistentvolume", Field: "volumeMode", Path: "$.spec.volumeMode", Format: "string", Condition: "notEmpty"},
		{Type: "persistentvolume", Field: "phase", Path: "$.status.phase", Format: "string", Condition: "notEmpty"},

		{Type: "storageclass", Field: "provisioner", Path: "$.provisioner", Format: "string", Condition: "notEmpty"},
		{Type: "storageclass", Field: "reclaimPolicy", Path: "$.reclaimPolicy", Format: "string", Condition: "notEmpty"},
		{Type: "storageclass", Field: "volumeBindingMode", Path: "$.volumeBindingMode", Format: "string", Condition: "notEmpty"},
		{Type: "storageclass", Field: "allowVolumeExpansion", Path: "$.allowVolumeExpansion", Format: "bool"},

		{Type: "volumeattachment", Field: "attacher", Path: "$.spec.attacher", Format: "string", Condition: "notEmpty"},
		{Type: "volumeattachment", Field: "nodeName", Path: "$.spec.nodeName", Format: "string", Condition: "notEmpty"},
		{Type: "volumeattachment", Field: "pvName", Path: "$.spec.source.persistentVolumeName", Format: "string", Condition: "notEmpty"},
		{Type: "volumeattachment", Field: "attached", Path: "$.status.attached", Format: "bool"},

		{Type: "node", Field: "cpu", Path: "$.status.allocatable.cpu", Format: "string", Condition: "notEmpty"},
		{Type: "node", Field: "memory", Path: "$.status.allocatable.memory", Format: "string", Condition: "notEmpty"},
		{Type: "node", Field: "nodeInfo", Path: "$.status.nodeInfo", Format: "nodeInfo"},
		{Type: "node", Field: "conditions", Path: "$.status.conditions", Format: "nodeConditions"},

		{Type: "namespace", Field: "phase", Path: "$.status.phase", Format: "string", Condition: "notEmpty"},

		{Type: "networkpolicy", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "networkpolicy", Field: "policyTypes", Path: "$.spec.policyTypes", Format: "stringArray", Condition: "notEmpty"},

		{Type: "endpointslice", Field: "namespace", Path: "$.metadata.namespace", Format: "string"},
		{Type: "endpointslice", Field: "addressType", Path: "$.addressType", Format: "string", Condition: "notEmpty"},
		{Type: "endpointslice", Field: "firstPort", Path: "$.ports", Format: "endpointFirstPort"},
	}

	MetadataRules = make([]MetadataRule, 0, len(rules))
	for _, rule := range rules {
		compiled := MetadataRule{
			Type:      rule.Type,
			Field:     rule.Field,
			Path:      rule.Path,
			Format:    rule.Format,
			Condition: rule.Condition,
			expr:      jp.MustParseString(rule.Path),
		}
		MetadataRules = append(MetadataRules, compiled)
	}
}

func ApplyMetadataRules(resource kube.Resource, nodeMap map[string]*kube.Resource) map[string]any {
	metadata := make(map[string]any)
	data := resource.AsMap()

	if meta, ok := data["metadata"].(map[string]any); ok {
		if name, ok := meta["name"].(string); ok {
			metadata["name"] = name
		}
	}
	if kind, ok := data["kind"].(string); ok && kind != "" {
		metadata["kind"] = kind
	}

	for _, rule := range MetadataRules {
		if rule.Type != resource.Type {
			continue
		}

		results := rule.expr.Get(data)

		if len(results) == 0 {
			if rule.Format == "strategyType" {
				metadata[rule.Field] = "RollingUpdate"
			}
			continue
		}

		value := results[0]

		if rule.Condition == "notEmpty" {
			if isEmpty(value) {
				continue
			}
		}

		formattedValue := formatMetadataValue(value, rule.Format, data, resource)
		if formattedValue != nil {
			metadata[rule.Field] = formattedValue
		}
	}

	return metadata
}

func isEmpty(value any) bool {
	if value == nil {
		return true
	}
	switch v := value.(type) {
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	}
	return false
}

func formatMetadataValue(value any, format string, fullResource map[string]any, resource kube.Resource) any {
	switch format {
	case "string":
		if s, ok := value.(string); ok {
			return s
		}
		return nil

	case "stringNotNone":
		if s, ok := value.(string); ok && s != "" && s != "None" {
			return s
		}
		return nil

	case "int":
		if f, ok := value.(float64); ok {
			return int(f)
		}
		return nil

	case "bool":
		if b, ok := value.(bool); ok {
			return b
		}
		return nil

	case "stringArray":
		if arr, ok := value.([]any); ok {
			strs := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					strs = append(strs, s)
				}
			}
			if len(strs) > 0 {
				return strs
			}
		}
		return nil

	case "firstString":
		if arr, ok := value.([]any); ok && len(arr) > 0 {
			if s, ok := arr[0].(string); ok {
				return s
			}
		}
		return nil

	case "portArray":
		return formatPorts(value)

	case "selectorMap":
		return formatSelector(value)

	case "lbIngress":
		return formatLoadBalancerIngress(value)

	case "age":
		return formatAge(value)

	case "restarts":
		return formatRestarts(value)

	case "podStatus":
		return formatPodStatus(fullResource)

	case "secretKeys":
		return countSecretKeys(fullResource)

	case "secretObjectCount":
		return countSecretObjects(value)

	case "configMapKeys":
		return countConfigMapKeys(fullResource)

	case "certExpiry":
		return formatCertExpiry(value, fullResource)

	case "certConditions":
		return formatCertConditions(value)

	case "strategyType":
		return formatStrategyType(value)

	case "replicaCounts":
		return formatReplicaCounts(value)

	case "deploymentConditions":
		return formatDeploymentConditions(value)

	case "statefulsetStatus":
		return formatStatefulSetStatus(fullResource)

	case "daemonCounts":
		return formatDaemonCounts(value)

	case "daemonsetStatus":
		return formatDaemonSetStatus(fullResource)

	case "jobCounts":
		return formatJobCounts(value)

	case "jobConditions":
		return formatJobConditions(value)

	case "jobDuration":
		return formatJobDuration(value)

	case "jobOwnerCronJob":
		return formatJobOwnerCronJob(value)

	case "hpaConfig":
		return formatHPAConfig(value)

	case "hpaStatus":
		return formatHPAStatus(value)

	case "httproutePaths":
		return formatHTTPRoutePaths(value)

	case "httprouteService":
		return formatHTTPRouteService(value, fullResource)

	case "ingressHosts":
		return formatIngressHosts(value)

	case "issuerConfig":
		return formatIssuerConfig(value)

	case "issuerStatus":
		return formatIssuerStatus(value)

	case "issuerReadyReason":
		return formatIssuerReadyReason(value)

	case "issuerType":
		return formatIssuerType(value)

	case "nodeInfo":
		return formatNodeInfo(value)

	case "nodeConditions":
		return formatNodeConditions(value)

	case "endpointPorts":
		return formatEndpointPorts(value)

	case "endpointFirstPort":
		return formatEndpointFirstPort(value)

	case "gatewayListeners":
		return formatGatewayListeners(value)

	case "hasTLS":
		return formatHasTLS(value)

	default:
		return value
	}
}

func formatPorts(value any) any {
	ports, ok := value.([]any)
	if !ok || len(ports) == 0 {
		return nil
	}

	portStrs := []string{}
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
	if len(portStrs) > 0 {
		return portStrs
	}
	return nil
}

func formatSelector(value any) any {
	selector, ok := value.(map[string]any)
	if !ok || len(selector) == 0 {
		return nil
	}

	selectorStrs := []string{}
	for k, v := range selector {
		if vStr, ok := v.(string); ok {
			selectorStrs = append(selectorStrs, k+"="+vStr)
		}
	}
	if len(selectorStrs) > 0 {
		sort.Strings(selectorStrs)
		return selectorStrs
	}
	return nil
}

func formatLoadBalancerIngress(value any) any {
	ingress, ok := value.([]any)
	if !ok || len(ingress) == 0 {
		return nil
	}

	var lbAddrs []string
	for _, ing := range ingress {
		if ingMap, ok := ing.(map[string]any); ok {
			if ip, ok := ingMap["ip"].(string); ok && ip != "" {
				lbAddrs = append(lbAddrs, ip)
			} else if hostname, ok := ingMap["hostname"].(string); ok && hostname != "" {
				lbAddrs = append(lbAddrs, hostname)
			}
		}
	}
	if len(lbAddrs) > 0 {
		return lbAddrs
	}
	return nil
}

func formatAge(value any) any {
	startTime, ok := value.(string)
	if !ok || startTime == "" {
		return nil
	}

	if parsedTime, err := time.Parse(time.RFC3339, startTime); err == nil {
		age := time.Since(parsedTime)
		if age.Hours() >= 24 {
			return fmt.Sprintf("%.0fd", age.Hours()/24)
		} else if age.Hours() >= 1 {
			return fmt.Sprintf("%.0fh", age.Hours())
		} else if age.Minutes() >= 1 {
			return fmt.Sprintf("%.0fm", age.Minutes())
		} else {
			return fmt.Sprintf("%.0fs", age.Seconds())
		}
	}
	return nil
}

func formatRestarts(value any) any {
	containerStatuses, ok := value.([]any)
	if !ok {
		return nil
	}

	var totalRestarts int
	for _, cs := range containerStatuses {
		if csMap, ok := cs.(map[string]any); ok {
			if restartCount, ok := csMap["restartCount"].(float64); ok {
				totalRestarts += int(restartCount)
			}
		}
	}
	if totalRestarts > 0 {
		return totalRestarts
	}
	return nil
}

func formatPodStatus(fullResource map[string]any) any {
	status, ok := fullResource["status"].(map[string]any)
	if !ok {
		return nil
	}

	phase, _ := status["phase"].(string)
	if phase == "" {
		return nil
	}

	if phase == "Failed" || phase == "Unknown" {
		return phase
	}

	if phase == "Pending" {

		if containerStatuses, ok := status["containerStatuses"].([]any); ok && len(containerStatuses) > 0 {
			for _, cs := range containerStatuses {
				if csMap, ok := cs.(map[string]any); ok {
					if state, ok := csMap["state"].(map[string]any); ok {
						if waiting, ok := state["waiting"].(map[string]any); ok {
							if reason, ok := waiting["reason"].(string); ok && reason != "" {
								if reason == "CrashLoopBackOff" || reason == "ImagePullBackOff" || reason == "ErrImagePull" {
									return reason
								}
							}
						}
					}
				}
			}
		}
		return "Pending"
	}

	if phase == "Running" {

		if conditions, ok := status["conditions"].([]any); ok {
			for _, c := range conditions {
				if condMap, ok := c.(map[string]any); ok {
					if condType, ok := condMap["type"].(string); ok && condType == "Ready" {
						if statusVal, ok := condMap["status"].(string); ok {
							if statusVal != "True" {
								return "NotReady"
							}
							return "Running"
						}
					}
				}
			}
		}
		return "Running"
	}

	if phase == "Succeeded" {
		return "Succeeded"
	}

	return phase
}

func countSecretKeys(fullResource map[string]any) any {
	count := 0
	if data, ok := fullResource["data"].(map[string]any); ok {
		count += len(data)
	}
	if binaryData, ok := fullResource["binaryData"].(map[string]any); ok {
		count += len(binaryData)
	}
	if count > 0 {
		return count
	}
	return nil
}

func countConfigMapKeys(fullResource map[string]any) any {
	count := 0
	if data, ok := fullResource["data"].(map[string]any); ok {
		count += len(data)
	}
	if binaryData, ok := fullResource["binaryData"].(map[string]any); ok {
		count += len(binaryData)
	}
	if count > 0 {
		return count
	}
	return nil
}

func countSecretObjects(value any) any {
	secretObjects, ok := value.([]any)
	if !ok {
		return nil
	}
	if len(secretObjects) == 0 {
		return nil
	}
	return len(secretObjects)
}

func formatCertConditions(value any) any {
	conditions, ok := value.([]any)
	if !ok || len(conditions) == 0 {
		return nil
	}

	result := make(map[string]any)
	for _, cond := range conditions {
		if condMap, ok := cond.(map[string]any); ok {
			condType, _ := condMap["type"].(string)
			if condType == "Ready" {
				if condStatus, ok := condMap["status"].(string); ok && condStatus != "" {
					result["ready"] = condStatus
				}
				if reason, ok := condMap["reason"].(string); ok && reason != "" {
					result["readyReason"] = reason
				}
				break
			}
		}
	}

	if len(result) > 0 {
		return result
	}
	return nil
}

func formatCertExpiry(value any, fullResource map[string]any) any {
	notAfter, ok := value.(string)
	if !ok || notAfter == "" {
		return nil
	}

	expTime, err := time.Parse(time.RFC3339, notAfter)
	if err != nil {
		return nil
	}

	daysUntilExpiry := time.Until(expTime).Hours() / 24

	status, _ := fullResource["status"].(map[string]any)
	var ready string
	if status != nil {
		if conditions, ok := status["conditions"].([]any); ok {
			for _, c := range conditions {
				if condMap, ok := c.(map[string]any); ok {
					if condType, _ := condMap["type"].(string); condType == "Ready" {
						ready, _ = condMap["status"].(string)
						break
					}
				}
			}
		}
	}

	result := make(map[string]any)
	days := int(daysUntilExpiry)

	if daysUntilExpiry < 0 {

		daysExpired := int(-daysUntilExpiry)
		result["expiresIn"] = fmt.Sprintf("-%dd", daysExpired)
		if daysExpired == 0 {
			result["status"] = "Expired Today"
		} else if daysExpired == 1 {
			result["status"] = "Expired 1d Ago"
		} else {
			result["status"] = fmt.Sprintf("Expired %dd Ago", daysExpired)
		}
	} else if daysUntilExpiry < 30 {

		result["expiresIn"] = fmt.Sprintf("%dd", days)
		if days == 0 {
			result["status"] = "Expires Today"
		} else if days == 1 {
			result["status"] = "Expires In 1d"
		} else {
			result["status"] = fmt.Sprintf("Expires In %dd", days)
		}
	} else {

		result["expiresIn"] = fmt.Sprintf("%dd", days)
		result["status"] = fmt.Sprintf("Expires In %dd", days)
	}

	if ready != "" && ready != "True" {
		if statusText, hasStatus := result["status"].(string); hasStatus && statusText != "" {
			result["status"] = "NotReady, " + statusText
		} else {
			result["status"] = "NotReady"
		}
	}

	return result
}

func formatGatewayListeners(value any) any {
	listeners, ok := value.([]any)
	if !ok || len(listeners) == 0 {
		return nil
	}

	listenerInfos := []string{}
	for _, listener := range listeners {
		if listenerMap, ok := listener.(map[string]any); ok {
			var port string
			var protocol string

			if p, ok := graph.M(listenerMap).IntOk("port"); ok {
				port = fmt.Sprintf("%d", p)
			}
			if proto, ok := listenerMap["protocol"].(string); ok {
				protocol = proto
			}

			if port != "" {

				if protocol != "" {
					listenerInfos = append(listenerInfos, port+"/"+protocol)
				} else {
					listenerInfos = append(listenerInfos, port)
				}
			}
		}
	}

	if len(listenerInfos) > 0 {
		return listenerInfos
	}
	return nil
}

func formatStrategyType(value any) any {
	if s, ok := value.(string); ok && s != "" {
		return s
	}
	return "RollingUpdate"
}

func formatReplicaCounts(value any) any {
	status, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	result := make(map[string]any)
	if current, ok := graph.M(status).IntOk("replicas"); ok {
		result["current"] = current
	}
	if ready, ok := graph.M(status).IntOk("readyReplicas"); ok {
		result["ready"] = ready
	}
	if available, ok := graph.M(status).IntOk("availableReplicas"); ok {
		result["available"] = available
	}
	if updated, ok := graph.M(status).IntOk("updatedReplicas"); ok {
		result["updated"] = updated
	}

	return result
}

func formatDeploymentConditions(value any) any {
	conditions, ok := value.([]any)
	if !ok {
		return nil
	}

	for _, c := range conditions {
		if condMap, ok := c.(map[string]any); ok {
			condType, _ := condMap["type"].(string)
			statusVal, _ := condMap["status"].(string)

			if condType == "ReplicaFailure" && statusVal == "True" {
				return "ReplicaFailure"
			} else if condType == "Progressing" && statusVal == "False" {
				if reason, ok := condMap["reason"].(string); ok && reason == "ProgressDeadlineExceeded" {
					return "ProgressDeadlineExceeded"
				}
			} else if condType == "Available" && statusVal == "False" {
				return "NotAvailable"
			} else if condType == "Available" && statusVal == "True" {
				return "Available"
			}
		}
	}
	return nil
}

func formatStatefulSetStatus(fullResource map[string]any) any {
	spec, _ := fullResource["spec"].(map[string]any)
	status, _ := fullResource["status"].(map[string]any)

	if spec == nil || status == nil {
		return nil
	}

	desiredReplicas, _ := graph.M(spec).IntOk("replicas")
	readyReplicas, _ := graph.M(status).IntOk("readyReplicas")

	if readyReplicas < desiredReplicas {
		return "NotReady"
	} else if readyReplicas == desiredReplicas && desiredReplicas > 0 {
		return "Ready"
	}
	return nil
}

func formatDaemonCounts(value any) any {
	status, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	result := make(map[string]any)
	if desired, ok := graph.M(status).IntOk("desiredNumberScheduled"); ok {
		result["desired"] = desired
	}
	if current, ok := graph.M(status).IntOk("currentNumberScheduled"); ok {
		result["current"] = current
	}
	if ready, ok := graph.M(status).IntOk("numberReady"); ok {
		result["ready"] = ready
	}
	if available, ok := graph.M(status).IntOk("numberAvailable"); ok {
		result["available"] = available
	}
	if updated, ok := graph.M(status).IntOk("updatedNumberScheduled"); ok {
		result["updated"] = updated
	}

	return result
}

func formatDaemonSetStatus(fullResource map[string]any) any {
	status, _ := fullResource["status"].(map[string]any)
	if status == nil {
		return nil
	}

	desired, _ := graph.M(status).IntOk("desiredNumberScheduled")
	ready, _ := graph.M(status).IntOk("numberReady")

	if ready < desired {
		return "NotReady"
	} else if ready == desired && desired > 0 {
		return "Ready"
	}
	return nil
}

func formatJobCounts(value any) any {
	status, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	result := make(map[string]any)
	succeeded := 0
	failed := 0
	active := 0

	if s, ok := status["succeeded"].(float64); ok {
		succeeded = int(s)
	}
	if f, ok := status["failed"].(float64); ok {
		failed = int(f)
	}
	if a, ok := status["active"].(float64); ok {
		active = int(a)
	}

	if succeeded > 0 || failed > 0 || active > 0 {
		result["succeeded"] = succeeded
		result["failed"] = failed
		result["active"] = active
		return result
	}
	return nil
}

func formatJobConditions(value any) any {
	conditions, ok := value.([]any)
	if !ok {
		return nil
	}

	for _, c := range conditions {
		if condMap, ok := c.(map[string]any); ok {
			condType, _ := condMap["type"].(string)
			statusVal, _ := condMap["status"].(string)

			if statusVal == "True" {
				if condType == "Complete" {
					return "Complete"
				} else if condType == "Failed" {
					return "Failed"
				} else if condType == "Suspended" {
					return "Suspended"
				}
			}
		}
	}

	return nil
}

func formatJobDuration(value any) any {
	status, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	startTime, _ := status["startTime"].(string)
	completionTime, _ := status["completionTime"].(string)

	if startTime != "" && completionTime != "" {
		start, err1 := time.Parse(time.RFC3339, startTime)
		end, err2 := time.Parse(time.RFC3339, completionTime)
		if err1 == nil && err2 == nil {
			duration := end.Sub(start)
			if duration.Hours() >= 1 {
				return fmt.Sprintf("%.0fh", duration.Hours())
			} else if duration.Minutes() >= 1 {
				return fmt.Sprintf("%.0fm", duration.Minutes())
			} else {
				return fmt.Sprintf("%.0fs", duration.Seconds())
			}
		}
	}
	return nil
}

func formatJobOwnerCronJob(value any) any {
	owners, ok := value.([]any)
	if !ok {
		return nil
	}

	for _, owner := range owners {
		ownerMap, ok := owner.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := ownerMap["kind"].(string)
		if kind != "CronJob" {
			continue
		}
		if name, _ := ownerMap["name"].(string); name != "" {
			return name
		}
	}

	return nil
}

func formatHPAConfig(value any) any {
	spec, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	result := make(map[string]any)
	if minReplicas, ok := spec["minReplicas"].(float64); ok {
		result["minReplicas"] = int(minReplicas)
	}
	if maxReplicas, ok := spec["maxReplicas"].(float64); ok {
		result["maxReplicas"] = int(maxReplicas)
	}

	if len(result) > 0 {
		return result
	}
	return nil
}

func formatHPAStatus(value any) any {
	status, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	result := make(map[string]any)
	if currentReplicas, ok := status["currentReplicas"].(float64); ok {
		result["currentReplicas"] = int(currentReplicas)
	}
	if desiredReplicas, ok := status["desiredReplicas"].(float64); ok {
		result["desiredReplicas"] = int(desiredReplicas)
	}

	if len(result) > 0 {
		return result
	}
	return nil
}

func formatHTTPRoutePaths(value any) any {
	rules, ok := value.([]any)
	if !ok || len(rules) == 0 {
		return nil
	}

	paths := []string{}
	for _, rule := range rules {
		if ruleMap, ok := rule.(map[string]any); ok {
			if matches, ok := ruleMap["matches"].([]any); ok {
				for _, match := range matches {
					if matchMap, ok := match.(map[string]any); ok {
						if path, ok := matchMap["path"].(map[string]any); ok {
							pathType, _ := path["type"].(string)
							pathValue, _ := path["value"].(string)
							if pathValue != "" {
								if pathType == "PathPrefix" {
									paths = append(paths, pathValue+"*")
								} else {
									paths = append(paths, pathValue)
								}
							}
						}
					}
				}
			}
		}
	}

	if len(paths) > 0 {
		return paths
	}
	return nil
}

func formatHTTPRouteService(value any, fullResource map[string]any) any {
	rules, ok := value.([]any)
	if !ok || len(rules) == 0 {
		return nil
	}

	for _, rule := range rules {
		if ruleMap, ok := rule.(map[string]any); ok {
			if backendRefs, ok := ruleMap["backendRefs"].([]any); ok && len(backendRefs) > 0 {
				if backendMap, ok := backendRefs[0].(map[string]any); ok {
					if serviceName, ok := backendMap["name"].(string); ok && serviceName != "" {

						if port, ok := backendMap["port"].(float64); ok {
							return fmt.Sprintf("%s:%d/TCP (ClusterIP)", serviceName, int(port))
						}
						return serviceName
					}
				}
			}
		}
	}
	return nil
}

func formatIngressHosts(value any) any {
	rules, ok := value.([]any)
	if !ok || len(rules) == 0 {
		return nil
	}

	hosts := []string{}
	for _, rule := range rules {
		if ruleMap, ok := rule.(map[string]any); ok {
			if host, ok := ruleMap["host"].(string); ok && host != "" {
				hosts = append(hosts, host)
			}
		}
	}

	if len(hosts) > 0 {
		return hosts
	}
	return nil
}

func formatIssuerConfig(value any) any {
	spec, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	result := make(map[string]any)

	if acme, ok := spec["acme"].(map[string]any); ok {
		if email, ok := acme["email"].(string); ok && email != "" {
			result["email"] = email
		}
		if server, ok := acme["server"].(string); ok && server != "" {
			result["server"] = server
		}
	}

	if ca, ok := spec["ca"].(map[string]any); ok {
		if secretName, ok := ca["secretName"].(string); ok && secretName != "" {
			result["caSecret"] = secretName
		}
	}

	if vault, ok := spec["vault"].(map[string]any); ok {
		if path, ok := vault["path"].(string); ok && path != "" {
			result["vaultPath"] = path
		}
	}

	if len(result) > 0 {
		return result
	}
	return nil
}

func formatIssuerStatus(value any) any {
	conditions, ok := value.([]any)
	if !ok || len(conditions) == 0 {
		return nil
	}

	for _, cond := range conditions {
		condMap, ok := cond.(map[string]any)
		if !ok {
			continue
		}
		if condType, _ := condMap["type"].(string); condType != "Ready" {
			continue
		}
		status, _ := condMap["status"].(string)
		switch status {
		case "True":
			return "Ready"
		case "False":
			return "NotReady"
		case "Unknown":
			return "Unknown"
		default:
			return nil
		}
	}

	return nil
}

func formatIssuerReadyReason(value any) any {
	conditions, ok := value.([]any)
	if !ok || len(conditions) == 0 {
		return nil
	}

	for _, cond := range conditions {
		condMap, ok := cond.(map[string]any)
		if !ok {
			continue
		}
		if condType, _ := condMap["type"].(string); condType != "Ready" {
			continue
		}
		reason, _ := condMap["reason"].(string)
		if reason != "" {
			return reason
		}
		return nil
	}

	return nil
}

func formatIssuerType(value any) any {
	spec, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	types := []string{"acme", "ca", "vault", "selfSigned", "venafi"}
	for _, t := range types {
		if _, exists := spec[t]; exists {
			if t == "selfSigned" {
				return "self-signed"
			}
			return t
		}
	}

	return nil
}

func formatNodeInfo(value any) any {
	nodeInfo, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	result := make(map[string]any)
	if osImage, ok := nodeInfo["osImage"].(string); ok && osImage != "" {
		result["osImage"] = osImage
	}
	if kernelVersion, ok := nodeInfo["kernelVersion"].(string); ok && kernelVersion != "" {
		result["kernelVersion"] = kernelVersion
	}
	if kubeletVersion, ok := nodeInfo["kubeletVersion"].(string); ok && kubeletVersion != "" {
		result["kubeletVersion"] = kubeletVersion
	}

	if len(result) > 0 {
		return result
	}
	return nil
}

func formatNodeConditions(value any) any {
	conditions, ok := value.([]any)
	if !ok {
		return nil
	}

	for _, c := range conditions {
		if condMap, ok := c.(map[string]any); ok {
			condType, _ := condMap["type"].(string)
			statusVal, _ := condMap["status"].(string)

			if condType == "Ready" {
				if statusVal == "True" {
					return "Ready"
				} else {
					return "NotReady"
				}
			}
		}
	}
	return nil
}

func formatEndpointPorts(value any) any {
	ports, ok := value.([]any)
	if !ok || len(ports) == 0 {
		return nil
	}

	portStrs := []string{}
	for _, p := range ports {
		if portMap, ok := p.(map[string]any); ok {
			var portNum int
			var protocol string

			if pn, ok := portMap["port"].(float64); ok {
				portNum = int(pn)
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

	if len(portStrs) > 0 {
		return portStrs
	}
	return nil
}

func formatEndpointFirstPort(value any) any {
	ports, ok := value.([]any)
	if !ok || len(ports) == 0 {
		return nil
	}

	result := make(map[string]any)
	if portMap, ok := ports[0].(map[string]any); ok {
		if name, ok := portMap["name"].(string); ok && name != "" {
			result["portName"] = name
		}
		if portNum, ok := graph.M(portMap).IntOk("port"); ok && portNum > 0 {
			result["port"] = portNum
		}
		if protocol, ok := portMap["protocol"].(string); ok && protocol != "" {
			result["protocol"] = protocol
		} else {
			result["protocol"] = "TCP"
		}
	}

	if len(result) > 0 {
		return result
	}
	return nil
}

func formatHasTLS(value any) any {
	if tls, ok := value.([]any); ok && len(tls) > 0 {
		return true
	}
	return nil
}
