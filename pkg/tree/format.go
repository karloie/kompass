package tree

import (
	"fmt"
	"sort"
	"strings"

	"github.com/karloie/kompass/pkg/graph"
)

const (
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorGray   = "\033[90m"
	colorLight  = "\033[37m"
	colorReset  = "\033[0m"
)

func formatNodeName(nodeType string, meta map[string]any, resource map[string]any, plain bool, parentMeta map[string]any) string {
	var display string

	prefix := ""
	{

		emojiType := nodeType
		if displayPrefix, ok := meta["displayPrefix"].(string); ok && displayPrefix != "" {
			emojiType = displayPrefix
		}

		emoji := graph.GetResourceEmoji(emojiType)

		if nodeType == "replicaset" {
			if orphaned, ok := meta["orphaned"].(bool); ok && orphaned {
				emoji = "⚠️"
			}
		}
		prefix = emoji + " "
	}

	parentType, _ := parentMeta["__nodeType"].(string)
	displayType := resolveDisplayType(nodeType, parentType)

	hasDisplayPrefix := false
	if displayPrefix, ok := meta["displayPrefix"].(string); ok && displayPrefix != "" {
		displayType = displayPrefix + " " + displayType
		hasDisplayPrefix = true
	}

	if name, ok := meta["name"].(string); ok && name != "" {

		if nodeType == "env" && !hasDisplayPrefix {
			display = prefix + name
		} else if hasDisplayPrefix {
			display = prefix + displayType + " " + name
		} else {
			display = prefix + displayType + " " + name
		}

		if nodeType == "env" {
			if value, hasValue := meta["value"].(string); hasValue {

				if !strings.HasPrefix(value, "fieldRef ") && !strings.HasPrefix(value, "resourceFieldRef ") {
					if plain {
						display = prefix + name + "=" + truncateMultiline(value)
					} else {
						display = prefix + colorLight + name + colorReset + colorGray + "=" + colorReset + truncateMultiline(value)
					}
				}
			}
		}
	} else {

		if nodeType == "mount" {
			if mountPath, ok := meta["mount"].(string); ok && mountPath != "" {
				switch parentType {
				case "secretstore":
					display = prefix + "mounted in container " + mountPath
				case "configmap":
					display = prefix + "mounted as files " + mountPath
				case "persistentvolumeclaim":
					display = prefix + "mounted in container " + mountPath
				default:
					display = prefix + mountPath
				}
			} else {
				display = prefix + displayType
			}
		} else if hasDisplayPrefix && nodeType == "label" {
			if key, keyOk := meta["key"].(string); keyOk {
				if value, valueOk := meta["value"].(string); valueOk {
					display = prefix + displayType + " " + key + "=" + value
				} else {
					display = prefix + displayType
				}
			} else {
				display = prefix + displayType
			}
		} else if hasDisplayPrefix {
			display = prefix + displayType
		} else {
			display = prefix + displayType
		}
	}

	hiddenFields := buildHiddenFields(nodeType, hasDisplayPrefix, parentType)

	if nodeType == "env" && parentType == "configmap" {
		if name, ok := meta["name"].(string); ok && name != "" {
			if plain {
				display = prefix + "used as env " + name
			} else {
				display = prefix + "used as env " + colorLight + name + colorReset
			}
		}
	}

	parentNamespace := ""
	if parentMeta != nil {
		if ns, ok := parentMeta["namespace"].(string); ok {
			parentNamespace = ns
		}
	}

	labels := collectMetadataLabels(meta, parentMeta, parentNamespace, hiddenFields, nodeType, hasDisplayPrefix)
	display = appendProbeStatus(display, nodeType, meta, plain)
	display = appendOperatorBadge(display, meta, plain)

	statusValue := deriveStatusValue(nodeType, meta)
	if statusValue != "" && !isProbeNode(nodeType) {
		display = appendNodeStatus(display, nodeType, statusValue, meta, plain)
	}

	if len(labels) > 0 {
		display += renderMetadataLabels(labels, plain)
	}

	return display
}

func appendOperatorBadge(display string, meta map[string]any, plain bool) string {
	if meta == nil {
		return display
	}
	if prefix, ok := meta["displayPrefix"].(string); !ok || prefix != "operator" {
		return display
	}
	if plain {
		return display + " [OPERATOR]"
	}
	return display + " [" + colorYellow + "OPERATOR" + colorReset + "]"
}

func isProbeNode(nodeType string) bool {
	return nodeType == "livenessprobe" || nodeType == "readinessprobe" || nodeType == "startupprobe"
}

func appendProbeStatus(display, nodeType string, meta map[string]any, plain bool) string {
	if !isProbeNode(nodeType) {
		return display
	}

	status, ok := meta["status"].(string)
	if !ok {
		return display
	}

	statusDisplay, isGood := probeStatusStyle(status)
	if plain {
		return display + " [" + statusDisplay + "]"
	}
	if isGood {
		return display + " [" + colorGreen + statusDisplay + colorReset + "]"
	}
	return display + " [" + colorRed + statusDisplay + colorReset + "]"
}

func probeStatusStyle(status string) (string, bool) {
	switch status {
	case "ready":
		return "READY", true
	case "not-ready":
		return "NOT READY", false
	case "started":
		return "STARTED", true
	case "not-started":
		return "NOT STARTED", false
	case "passing":
		return "PASSING", true
	case "failed":
		return "FAILED", false
	default:
		return strings.ToUpper(status), false
	}
}

func deriveStatusValue(nodeType string, meta map[string]any) string {
	if status, ok := meta["status"].(string); ok && status != "" {
		return status
	}
	if nodeType == "container" {
		if state, ok := meta["state"].(string); ok && state != "" {
			return state
		}
	}
	if nodeType == "pod" {
		if phase, ok := meta["phase"].(string); ok && phase != "" {
			return phase
		}
	}
	if nodeType == "endpoint" || nodeType == "address" {
		if ready, ok := meta["ready"].(bool); ok {
			if ready {
				return "ready"
			}
			return "not-ready"
		}
	}
	return ""
}

func appendNodeStatus(display, nodeType, statusValue string, meta map[string]any, plain bool) string {
	statusUpper := strings.ToUpper(statusValue)

	if nodeType == "container" {
		return display + formatContainerStatus(statusUpper, meta, plain)
	}

	if nodeType == "certificate" {
		return display + formatCertificateStatus(statusUpper, plain)
	}

	return display + formatGenericStatus(statusUpper, plain)
}

func formatContainerStatus(statusUpper string, meta map[string]any, plain bool) string {
	stateGood := map[string]bool{"RUNNING": true, "SUCCEEDED": true, "COMPLETE": true, "ACTIVE": true, "AVAILABLE": true, "BOUND": true, "READY": true}
	stateWarn := map[string]bool{"SCHEDULED": true, "PENDING": true, "UNKNOWN": true, "SUSPENDED": true}

	tokens := []string{statusUpper}
	tokenGood := []bool{stateGood[statusUpper]}
	tokenWarn := []bool{stateWarn[statusUpper]}

	appendProbe := func(label, raw string) {
		upper := strings.ToUpper(raw)
		tokens = append(tokens, label+"="+upper)
		good := upper == "PASSING" || upper == "READY" || upper == "STARTED"
		warn := upper == "UNKNOWN"
		tokenGood = append(tokenGood, good)
		tokenWarn = append(tokenWarn, warn)
	}

	if v, ok := meta["livenessStatus"].(string); ok && v != "" {
		appendProbe("LIVENESS", v)
	}
	if v, ok := meta["readinessStatus"].(string); ok && v != "" {
		appendProbe("READINESS", v)
	}
	if v, ok := meta["startupStatus"].(string); ok && v != "" {
		appendProbe("STARTUP", v)
	}

	if plain {
		return " [" + strings.Join(tokens, ", ") + "]"
	}

	parts := make([]string, 0, len(tokens))
	for i, token := range tokens {
		if tokenGood[i] {
			parts = append(parts, colorGreen+token+colorReset)
		} else if tokenWarn[i] {
			parts = append(parts, colorYellow+token+colorReset)
		} else {
			parts = append(parts, colorRed+token+colorReset)
		}
	}

	return " [" + strings.Join(parts, ", ") + "]"
}

func formatCertificateStatus(statusUpper string, plain bool) string {
	if plain {
		return " [" + statusUpper + "]"
	}

	switch {
	case strings.HasPrefix(statusUpper, "EXPIRES IN "):
		return " [" + colorGreen + statusUpper + colorReset + "]"
	case strings.HasPrefix(statusUpper, "EXPIRES TODAY"):
		return " [" + colorYellow + statusUpper + colorReset + "]"
	case strings.HasPrefix(statusUpper, "EXPIRED"):
		return " [" + colorRed + statusUpper + colorReset + "]"
	default:
		return " [" + colorRed + statusUpper + colorReset + "]"
	}
}

func formatGenericStatus(statusUpper string, plain bool) string {
	if plain {
		return " [" + statusUpper + "]"
	}

	goodStatuses := map[string]bool{
		"RUNNING":   true,
		"SUCCEEDED": true,
		"COMPLETE":  true,
		"ACTIVE":    true,
		"AVAILABLE": true,
		"BOUND":     true,
		"READY":     true,
	}
	suspiciousStatuses := map[string]bool{
		"SCHEDULED": true,
		"PENDING":   true,
		"UNKNOWN":   true,
		"SUSPENDED": true,
	}

	if goodStatuses[statusUpper] {
		return " [" + colorGreen + statusUpper + colorReset + "]"
	}
	if suspiciousStatuses[statusUpper] {
		return " [" + colorYellow + statusUpper + colorReset + "]"
	}
	return " [" + colorRed + statusUpper + colorReset + "]"
}

func resolveDisplayType(nodeType, parentType string) string {
	switch {
	case strings.HasPrefix(nodeType, "env-") || strings.HasPrefix(nodeType, "envfrom-"):
		return "env"
	case strings.HasPrefix(nodeType, "mount-"):
		return "mount"
	case nodeType == "configmaps":
		return "config"
	case nodeType == "configmap" && parentType == "configmaps":
		return "source configmap"
	case nodeType == "secretstore":
		return "external secret source"
	case nodeType == "secretproviderclass" && parentType == "secretstore":
		return "provider config"
	case nodeType == "secret" && parentType == "secretstore":
		return "synced secret"
	case nodeType == "mount" && parentType == "secretstore":
		return "mounted in container"
	case nodeType == "storage":
		return "data volumes"
	case nodeType == "persistentvolumeclaim" && parentType == "storage":
		return "claim"
	case nodeType == "persistentvolume" && parentType == "persistentvolumeclaim":
		return "backing volume"
	case nodeType == "storageclass" && parentType == "persistentvolume":
		return "storage class"
	case nodeType == "volumeattachment" && parentType == "persistentvolume":
		return "attachment"
	default:
		return nodeType
	}
}

func buildHiddenFields(nodeType string, hasDisplayPrefix bool, parentType string) map[string]bool {
	hiddenFields := map[string]bool{
		"annotations":       true,
		"__nodeType":        true,
		"count":             true,
		"creationTimestamp": true,
		"displayPrefix":     true,
		"index":             true,
		"kind":              true,
		"labels":            true,
		"managedFields":     true,
		"name":              true,
		"orphaned":          true,
		"ownerReferences":   true,
		"policyType":        true,
		"livenessStatus":    true,
		"readinessStatus":   true,
		"startupStatus":     true,
		"resourceVersion":   true,
		"ruleType":          true,
		"source":            true,
		"sourceType":        true,
		"status":            true,
		"targetKind":        true,
		"uid":               true,
		"value":             true,
		"volumeType":        true,
	}

	if nodeType == "livenessprobe" || nodeType == "readinessprobe" || nodeType == "startupprobe" {
		hiddenFields["status"] = true
	}
	if nodeType == "label" && hasDisplayPrefix {
		hiddenFields["key"] = true
		hiddenFields["value"] = true
	}
	if nodeType == "env" && parentType == "configmap" {
		hiddenFields["value"] = true
	}
	if nodeType == "mount" {
		hiddenFields["mount"] = true
	}
	if nodeType == "certificate" {
		hiddenFields["expiresIn"] = true
	}
	if nodeType == "container" {
		hiddenFields["state"] = true
	}
	if nodeType == "pod" {
		hiddenFields["phase"] = true
	}
	if nodeType == "endpoint" || nodeType == "address" {
		hiddenFields["ready"] = true
	}

	return hiddenFields
}

func collectMetadataLabels(meta map[string]any, parentMeta map[string]any, parentNamespace string, hiddenFields map[string]bool, nodeType string, hasDisplayPrefix bool) []string {
	labels := []string{}
	for key, value := range meta {
		if hiddenFields[key] {
			if nodeType == "label" && hasDisplayPrefix && (key == "key" || key == "value") {
				continue
			}
			if !(key == "value" && nodeType != "env") {
				continue
			}
		}

		if key == "namespace" && parentNamespace != "" {
			if strVal, ok := value.(string); ok && strVal == parentNamespace {
				continue
			}
		}

		if key == "cronjob" && parentMeta != nil {
			parentName, hasParentName := parentMeta["name"].(string)
			_, isCronJob := parentMeta["schedule"]
			cronJobName, hasCronJobName := value.(string)
			if hasParentName && hasCronJobName && parentName == cronJobName && isCronJob {
				continue
			}
		}

		if strVal := metadataValueString(value); strVal != "" {
			labels = append(labels, key+"="+strVal)
		}
	}
	return labels
}

func metadataValueString(value any) string {
	switch v := value.(type) {
	case string:
		if v != "" {
			return truncateMultiline(v)
		}
		return ""
	case int, int32, int64:
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%v", v)
	case map[string]any:
		if len(v) == 0 {
			return ""
		}
		parts := []string{}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			valStr := metadataValueString(v[k])
			if valStr != "" {
				parts = append(parts, k+":"+valStr)
			}
		}
		return strings.Join(parts, " ")
	case []any:
		if len(v) == 0 {
			return ""
		}
		items := []string{}
		for _, item := range v {
			itemStr := metadataValueString(item)
			if itemStr != "" {
				items = append(items, itemStr)
			}
		}
		return strings.Join(items, ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func renderMetadataLabels(labels []string, plain bool) string {
	sort.Strings(labels)
	if plain {
		return " {" + strings.Join(labels, ", ") + "}"
	}

	colored := make([]string, 0, len(labels))
	for _, label := range labels {
		parts := strings.SplitN(label, "=", 2)
		if len(parts) != 2 {
			colored = append(colored, colorGray+label+colorReset)
			continue
		}
		colored = append(colored, colorDim+colorGray+parts[0]+colorReset+colorLight+"="+colorReset+colorBold+colorGray+parts[1]+colorReset)
	}

	return " {" + strings.Join(colored, ", ") + "}"
}

func truncateMultiline(s string) string {
	if idx := strings.IndexAny(s, "\n\r"); idx >= 0 {
		firstLine := s[:idx]

		if len(firstLine) > 80 {
			firstLine = firstLine[:77] + "..."
		}
		return firstLine + "..."
	}
	return s
}
