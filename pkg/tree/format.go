package tree

import (
	"fmt"
	"sort"
	"strings"

	"github.com/karloie/kompass/pkg/graph"
)

const (
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorGray   = "\033[90m"
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

	displayType := nodeType
	if strings.HasPrefix(nodeType, "env-") || strings.HasPrefix(nodeType, "envfrom-") {
		displayType = "env"
	} else if strings.HasPrefix(nodeType, "mount-") {
		displayType = "mount"
	}

	hasDisplayPrefix := false
	if displayPrefix, ok := meta["displayPrefix"].(string); ok && displayPrefix != "" {
		displayType = displayPrefix + ": " + displayType
		hasDisplayPrefix = true
	}

	if name, ok := meta["name"].(string); ok && name != "" {

		if hasDisplayPrefix {
			display = prefix + displayType + " " + name
		} else {
			display = prefix + displayType + ": " + name
		}

		if nodeType == "env" {
			if value, hasValue := meta["value"].(string); hasValue {

				if !strings.HasPrefix(value, "fieldRef ") && !strings.HasPrefix(value, "resourceFieldRef ") {
					display = prefix + displayType + ": " + name + "=" + truncateMultiline(value)
				}
			}
		}
	} else {

		if hasDisplayPrefix && nodeType == "label" {
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
			display = prefix + displayType + ":"
		}
	}

	hiddenFields := map[string]bool{
		"annotations":       true,
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

	parentNamespace := ""
	if parentMeta != nil {
		if ns, ok := parentMeta["namespace"].(string); ok {
			parentNamespace = ns
		}
	}

	var labels []string
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

		var strVal string
		switch v := value.(type) {
		case string:
			if v != "" {
				strVal = truncateMultiline(v)
			}
		case int, int32, int64:
			strVal = fmt.Sprintf("%v", v)
		case bool:
			strVal = fmt.Sprintf("%v", v)
		case map[string]any:

			if len(v) > 0 {
				parts := []string{}

				keys := make([]string, 0, len(v))
				for k := range v {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					mv := v[k]

					var valStr string
					switch mvTyped := mv.(type) {
					case string:
						valStr = truncateMultiline(mvTyped)
					case map[string]any:

						innerParts := []string{}
						for _, iv := range mvTyped {
							innerParts = append(innerParts, fmt.Sprintf("%v", iv))
						}
						valStr = strings.Join(innerParts, " ")
					case []any:

						itemStrs := []string{}
						for _, item := range mvTyped {
							var itemStr string
							if s, ok := item.(string); ok {
								itemStr = truncateMultiline(s)
							} else {
								itemStr = fmt.Sprintf("%v", item)
							}
							itemStrs = append(itemStrs, itemStr)
						}
						valStr = strings.Join(itemStrs, ",")
					default:
						valStr = fmt.Sprintf("%v", mv)
					}
					if valStr != "" {
						parts = append(parts, k+":"+valStr)
					}
				}
				strVal = strings.Join(parts, " ")
			}
		case []any:

			if len(v) > 0 {
				items := []string{}
				for _, item := range v {
					var itemStr string
					if s, ok := item.(string); ok {
						itemStr = truncateMultiline(s)
					} else {
						itemStr = fmt.Sprintf("%v", item)
					}
					items = append(items, itemStr)
				}
				strVal = strings.Join(items, ",")
			}
		default:

			strVal = fmt.Sprintf("%v", v)
		}

		if strVal != "" {
			labels = append(labels, key+"="+strVal)
		}
	}

	if nodeType == "livenessprobe" || nodeType == "readinessprobe" || nodeType == "startupprobe" {
		if status, ok := meta["status"].(string); ok {
			var statusDisplay string
			var isGood bool
			switch status {
			case "ready":
				statusDisplay = "READY"
				isGood = true
			case "not-ready":
				statusDisplay = "NOT READY"
				isGood = false
			case "started":
				statusDisplay = "STARTED"
				isGood = true
			case "not-started":
				statusDisplay = "NOT STARTED"
				isGood = false
			case "passing":
				statusDisplay = "PASSING"
				isGood = true
			case "failed":
				statusDisplay = "FAILED"
				isGood = false
			default:
				statusDisplay = strings.ToUpper(status)
				isGood = false
			}

			if !plain {
				if isGood {
					display += " [" + colorGreen + statusDisplay + colorReset + "]"
				} else {
					display += " [" + colorRed + statusDisplay + colorReset + "]"
				}
			} else {
				display += " [" + statusDisplay + "]"
			}
		}
	}

	if status, ok := meta["status"].(string); ok && status != "" {

		if nodeType != "livenessprobe" && nodeType != "readinessprobe" && nodeType != "startupprobe" {
			statusUpper := strings.ToUpper(status)
			if nodeType == "certificate" {
				if !plain {
					switch {
					case strings.HasPrefix(statusUpper, "EXPIRES IN "):
						display += " [" + colorGreen + statusUpper + colorReset + "]"
					case strings.HasPrefix(statusUpper, "EXPIRES TODAY"):
						display += " [" + colorYellow + statusUpper + colorReset + "]"
					case strings.HasPrefix(statusUpper, "EXPIRED"):
						display += " [" + colorRed + statusUpper + colorReset + "]"
					default:
						display += " [" + colorRed + statusUpper + colorReset + "]"
					}
				} else {
					display += " [" + statusUpper + "]"
				}
			} else {

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

				if !plain {
					if goodStatuses[statusUpper] {
						display += " [" + colorGreen + statusUpper + colorReset + "]"
					} else if suspiciousStatuses[statusUpper] {
						display += " [" + colorYellow + statusUpper + colorReset + "]"
					} else {

						display += " [" + colorRed + statusUpper + colorReset + "]"
					}
				} else {
					display += " [" + statusUpper + "]"
				}
			}
		}
	}

	if len(labels) > 0 {
		sort.Strings(labels)
		metadataText := " {" + strings.Join(labels, ", ") + "}"
		if plain {
			display += metadataText
		} else {
			display += colorGray + metadataText + colorReset
		}
	}

	return display
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
