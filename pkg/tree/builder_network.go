package tree

import (
	"fmt"
	"sort"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

func buildEndpointSliceChildren(endpointSliceKey string, endpointSlice kube.Resource, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	children := []*kube.Tree{}

	endpoints, ok := endpointSlice.AsMap()["endpoints"].([]any)
	if !ok || len(endpoints) == 0 {
		return appendGraphChildren(endpointSliceKey, graphChildren, state, nodeMap)
	}

	for idx, ep := range endpoints {
		if epMap, ok := ep.(map[string]any); ok {
			epKey := fmt.Sprintf("%s/endpoint/%d", endpointSliceKey, idx)

			metadata := map[string]any{}

			if addresses, ok := epMap["addresses"].([]any); ok && len(addresses) > 0 {
				addrStrs := []string{}
				for _, addr := range addresses {
					if addrStr, ok := addr.(string); ok {
						addrStrs = append(addrStrs, addrStr)
					}
				}
				if len(addrStrs) > 0 {
					if len(addrStrs) == 1 {
						metadata["address"] = addrStrs[0]
					} else {
						metadata["addresses"] = addrStrs
					}
				}
			}

			if conditions, ok := epMap["conditions"].(map[string]any); ok {
				if ready, ok := conditions["ready"].(bool); ok {
					metadata["ready"] = ready
				}
				if serving, ok := conditions["serving"].(bool); ok && serving {
					metadata["serving"] = serving
				}
				if terminating, ok := conditions["terminating"].(bool); ok && terminating {
					metadata["terminating"] = terminating
				}
			}

			if nodeName, ok := epMap["nodeName"].(string); ok && nodeName != "" {
				metadata["nodeName"] = nodeName
			}

			if targetRef, ok := epMap["targetRef"].(map[string]any); ok {
				if kind, ok := targetRef["kind"].(string); ok {
					metadata["targetKind"] = kind
				}
				if name, ok := targetRef["name"].(string); ok {
					metadata["targetName"] = name
				}
			}

			epNode := NewTree(epKey, "endpoint", metadata)
			children = append(children, epNode)
		}
	}
	sortChildren(children)
	return children
}

func buildServiceChildren(serviceKey string, service kube.Resource, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	children := []*kube.Tree{}

	for _, childKey := range graphChildren[serviceKey] {
		if !state.CanTraverse(childKey) {
			continue
		}

		childResource, exists := nodeMap[childKey]
		if !exists {
			continue
		}

		if childResource.Type == "pod" {
			if !childResource.Discovered {
				continue
			}

			leafNode := NewTree(childKey, childResource.Type, map[string]any{})
			children = append(children, leafNode)
			state.MarkSeen(childKey)
			continue
		}

		childNode := buildTreeNode(childKey, graphChildren, state, nodeMap)
		if childNode != nil {
			children = append(children, childNode)
		}
	}

	sortChildren(children)
	return children
}

func buildServiceAccountChildren(serviceAccountKey string, serviceAccount kube.Resource, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	children := []*kube.Tree{}

	for _, childKey := range graphChildren[serviceAccountKey] {
		if !state.CanTraverse(childKey) {
			continue
		}

		childResource, exists := nodeMap[childKey]
		if !exists {
			continue
		}

		if childResource.Type == "pod" {
			if !childResource.Discovered {
				continue
			}

			leafNode := NewTree(childKey, childResource.Type, map[string]any{})
			children = append(children, leafNode)
			state.MarkSeen(childKey)
			continue
		}

		childNode := buildTreeNode(childKey, graphChildren, state, nodeMap)
		if childNode != nil {
			children = append(children, childNode)
		}
	}

	sortChildren(children)
	return children
}

func buildEndpointsChildren(endpointsKey string, endpoints kube.Resource, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	children := []*kube.Tree{}

	subsets, ok := endpoints.AsMap()["subsets"].([]any)
	if !ok || len(subsets) == 0 {
		return appendGraphChildren(endpointsKey, graphChildren, state, nodeMap)
	}

	for subsetIdx, subset := range subsets {
		if subsetMap, ok := subset.(map[string]any); ok {
			subsetKey := fmt.Sprintf("%s/subset/%d", endpointsKey, subsetIdx)
			subsetMetadata := map[string]any{}

			if ports, ok := subsetMap["ports"].([]any); ok && len(ports) > 0 {
				portStrs := []string{}
				for _, p := range ports {
					if portMap, ok := p.(map[string]any); ok {
						var portNum int
						var protocol string
						var name string

						if pn, ok := graph.M(portMap).IntOk("port"); ok {
							portNum = pn
						}
						if proto, ok := portMap["protocol"].(string); ok && proto != "" {
							protocol = proto
						} else {
							protocol = "TCP"
						}
						if n, ok := portMap["name"].(string); ok && n != "" {
							name = n
						}

						if portNum > 0 {
							if name != "" {
								portStrs = append(portStrs, fmt.Sprintf("%s:%d/%s", name, portNum, protocol))
							} else {
								portStrs = append(portStrs, fmt.Sprintf("%d/%s", portNum, protocol))
							}
						}
					}
				}
				if len(portStrs) > 0 {
					subsetMetadata["ports"] = portStrs
				}
			}

			subsetNode := NewTree(subsetKey, "subset", subsetMetadata)

			addrCounter := 0
			if addresses, ok := subsetMap["addresses"].([]any); ok && len(addresses) > 0 {
				for _, addr := range addresses {
					if addrMap, ok := addr.(map[string]any); ok {
						addrKey := fmt.Sprintf("%s/address/%d", subsetKey, addrCounter)
						addrCounter++
						addrMetadata := map[string]any{
							"ready": true,
						}

						if ip, ok := addrMap["ip"].(string); ok && ip != "" {
							addrMetadata["ip"] = ip
						}
						if nodeName, ok := addrMap["nodeName"].(string); ok && nodeName != "" {
							addrMetadata["nodeName"] = nodeName
						}
						if hostname, ok := addrMap["hostname"].(string); ok && hostname != "" {
							addrMetadata["hostname"] = hostname
						}

						if targetRef, ok := addrMap["targetRef"].(map[string]any); ok {
							if kind, ok := targetRef["kind"].(string); ok {
								addrMetadata["targetKind"] = kind
							}
							if name, ok := targetRef["name"].(string); ok {
								addrMetadata["targetName"] = name
							}
						}

						addrNode := NewTree(addrKey, "address", addrMetadata)
						subsetNode.Children = append(subsetNode.Children, addrNode)
					}
				}
			}

			if notReadyAddresses, ok := subsetMap["notReadyAddresses"].([]any); ok && len(notReadyAddresses) > 0 {
				for _, addr := range notReadyAddresses {
					if addrMap, ok := addr.(map[string]any); ok {
						addrKey := fmt.Sprintf("%s/address/%d", subsetKey, addrCounter)
						addrCounter++
						addrMetadata := map[string]any{
							"ready": false,
						}

						if ip, ok := addrMap["ip"].(string); ok && ip != "" {
							addrMetadata["ip"] = ip
						}
						if nodeName, ok := addrMap["nodeName"].(string); ok && nodeName != "" {
							addrMetadata["nodeName"] = nodeName
						}
						if hostname, ok := addrMap["hostname"].(string); ok && hostname != "" {
							addrMetadata["hostname"] = hostname
						}

						if targetRef, ok := addrMap["targetRef"].(map[string]any); ok {
							if kind, ok := targetRef["kind"].(string); ok {
								addrMetadata["targetKind"] = kind
							}
							if name, ok := targetRef["name"].(string); ok {
								addrMetadata["targetName"] = name
							}
						}

						addrNode := NewTree(addrKey, "address", addrMetadata)
						subsetNode.Children = append(subsetNode.Children, addrNode)
					}
				}
			}

			children = append(children, subsetNode)
		}
	}

	children = append(children, appendGraphChildren(endpointsKey, graphChildren, state, nodeMap)...)
	sortChildren(children)
	return children
}

func buildCiliumNetworkPolicyChildren(policyKey string, policy kube.Resource, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	children := []*kube.Tree{}

	spec := graph.M(policy.AsMap()).Map("spec").Raw()
	if spec == nil {
		return appendGraphChildren(policyKey, graphChildren, state, nodeMap)
	}

	if endpointSelector, ok := spec["endpointSelector"].(map[string]any); ok {
		if matchLabels, ok := endpointSelector["matchLabels"].(map[string]any); ok && len(matchLabels) > 0 {
			labelKeys := make([]string, 0, len(matchLabels))
			for labelKey := range matchLabels {
				labelKeys = append(labelKeys, labelKey)
			}
			sort.Strings(labelKeys)
			for _, labelKey := range labelKeys {
				labelValue := matchLabels[labelKey]
				if labelStr, ok := labelValue.(string); ok {
					labelNode := NewTree(
						fmt.Sprintf("%s/endpointSelector/label/%s", policyKey, labelKey),
						"label",
						map[string]any{
							"displayPrefix": "endpointselector",
							"key":           labelKey,
							"value":         labelStr,
						},
					)
					children = append(children, labelNode)
				}
			}
		}
	}

	if ingress, ok := spec["ingress"].([]any); ok && len(ingress) > 0 {
		for idx, rule := range ingress {
			if ruleMap, ok := rule.(map[string]any); ok {
				ruleKey := fmt.Sprintf("%s/ingress/rule/%d", policyKey, idx)

				if fromEndpoints, ok := ruleMap["fromEndpoints"].([]any); ok {
					for epIdx, ep := range fromEndpoints {
						if epMap, ok := ep.(map[string]any); ok {
							if matchLabels, ok := epMap["matchLabels"].(map[string]any); ok {
								epKey := fmt.Sprintf("%s/fromEndpoint/%d", ruleKey, epIdx)
								epNode := NewTree(epKey, "fromendpoint", map[string]any{
									"displayPrefix": "cnp-ingress",
									"matchLabels":   matchLabels,
								})

								for _, childKey := range graphChildren[epKey] {
									if !state.IsSeen(childKey) {
										childResource, exists := nodeMap[childKey]
										if !exists {
											continue
										}
										if childResource.Type == "service" {
											childNode := NewTree(childKey, childResource.Type, map[string]any{})
											epNode.Children = append(epNode.Children, childNode)
										}
									}
								}

								children = append(children, epNode)
							}
						}
					}
				}

				if fromEntities, ok := ruleMap["fromEntities"].([]any); ok && len(fromEntities) > 0 {
					entities := []string{}
					for _, entity := range fromEntities {
						if entityStr, ok := entity.(string); ok {
							entities = append(entities, entityStr)
						}
					}
					if len(entities) > 0 {
						entitiesKey := fmt.Sprintf("%s/fromEntities", ruleKey)
						entitiesNode := NewTree(entitiesKey, "fromentities", map[string]any{
							"displayPrefix": "cnp-ingress",
							"entities":      entities,
						})
						children = append(children, entitiesNode)
					}
				}

				if toPorts, ok := ruleMap["toPorts"].([]any); ok {
					for portIdx, port := range toPorts {
						if portMap, ok := port.(map[string]any); ok {
							portKey := fmt.Sprintf("%s/toPort/%d", ruleKey, portIdx)
							portMetadata := map[string]any{"displayPrefix": "cnp-ingress"}

							if ports, ok := portMap["ports"].([]any); ok && len(ports) > 0 {
								portMetadata["ports"] = ports
							}

							portNode := NewTree(portKey, "toport", portMetadata)
							children = append(children, portNode)
						}
					}
				}
			}
		}
	}

	if egress, ok := spec["egress"].([]any); ok && len(egress) > 0 {
		for idx, rule := range egress {
			if ruleMap, ok := rule.(map[string]any); ok {
				ruleKey := fmt.Sprintf("%s/egress/rule/%d", policyKey, idx)

				if toEndpoints, ok := ruleMap["toEndpoints"].([]any); ok {
					for epIdx, ep := range toEndpoints {
						if epMap, ok := ep.(map[string]any); ok {
							if matchLabels, ok := epMap["matchLabels"].(map[string]any); ok {
								epKey := fmt.Sprintf("%s/toEndpoint/%d", ruleKey, epIdx)
								epNode := NewTree(epKey, "toendpoint", map[string]any{
									"displayPrefix": "cnp-egress",
									"matchLabels":   matchLabels,
								})

								for _, childKey := range graphChildren[epKey] {
									if !state.IsSeen(childKey) {
										childResource, exists := nodeMap[childKey]
										if !exists {
											continue
										}
										if childResource.Type == "service" {
											childNode := NewTree(childKey, childResource.Type, map[string]any{})
											epNode.Children = append(epNode.Children, childNode)
										}
									}
								}

								children = append(children, epNode)
							}
						}
					}
				}

				if toEntities, ok := ruleMap["toEntities"].([]any); ok && len(toEntities) > 0 {
					entities := []string{}
					for _, entity := range toEntities {
						if entityStr, ok := entity.(string); ok {
							entities = append(entities, entityStr)
						}
					}
					if len(entities) > 0 {
						entitiesKey := fmt.Sprintf("%s/toEntities", ruleKey)
						entitiesNode := NewTree(entitiesKey, "toentities", map[string]any{
							"displayPrefix": "cnp-egress",
							"entities":      entities,
						})
						children = append(children, entitiesNode)
					}
				}

				if toFQDNs, ok := ruleMap["toFQDNs"].([]any); ok && len(toFQDNs) > 0 {
					for fqdnIdx, fqdn := range toFQDNs {
						if fqdnMap, ok := fqdn.(map[string]any); ok {
							fqdnKey := fmt.Sprintf("%s/toFQDN/%d", ruleKey, fqdnIdx)
							fqdnMetadata := map[string]any{"displayPrefix": "cnp-egress"}

							if matchPattern, ok := fqdnMap["matchPattern"].(string); ok {
								fqdnMetadata["matchPattern"] = matchPattern
							}
							if matchName, ok := fqdnMap["matchName"].(string); ok {
								fqdnMetadata["matchName"] = matchName
							}

							fqdnNode := NewTree(fqdnKey, "tofqdn", fqdnMetadata)
							children = append(children, fqdnNode)
						}
					}
				}

				if toPorts, ok := ruleMap["toPorts"].([]any); ok {
					for portIdx, port := range toPorts {
						if portMap, ok := port.(map[string]any); ok {
							portKey := fmt.Sprintf("%s/toPort/%d", ruleKey, portIdx)
							portMetadata := map[string]any{"displayPrefix": "cnp-egress"}

							if ports, ok := portMap["ports"].([]any); ok && len(ports) > 0 {
								portMetadata["ports"] = ports
							}

							portNode := NewTree(portKey, "toport", portMetadata)
							children = append(children, portNode)
						}
					}
				}
			}
		}
	}

	for _, childKey := range graphChildren[policyKey] {
		if state.CanTraverse(childKey) {
			childResource, exists := nodeMap[childKey]
			if !exists {
				continue
			}

			if childResource.Type == "service" || childResource.Type == "pod" {
				if childResource.Type == "pod" && !childResource.Discovered {
					continue
				}

				leafMeta := map[string]any{}
				if childResource.Type == "pod" {
					if podName := graph.M(childResource.AsMap()).Map("metadata").String("name"); podName != "" {
						leafMeta["name"] = podName
					}
				}

				leafNode := NewTree(childKey, childResource.Type, leafMeta)
				children = append(children, leafNode)
				state.MarkSeen(childKey)
			} else {
				childNode := buildTreeNode(childKey, graphChildren, state, nodeMap)
				if childNode != nil {
					children = append(children, childNode)
				}
			}
		}
	}

	sortChildren(children)
	return children
}

func buildPersistentVolumeClaimChildren(pvcKey string, pvc kube.Resource, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	return appendGraphChildren(pvcKey, graphChildren, state, nodeMap)
}

func buildSecretChildren(secretKey string, secret kube.Resource, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	builder := NewChildrenBuilder()

	if immutable, ok := secret.AsMap()["immutable"].(bool); ok && immutable {
		builder.Add(NewTree(secretKey+"/immutable", "immutable", map[string]any{"value": true}))
	}

	builder.Extend(appendGraphChildren(secretKey, graphChildren, state, nodeMap))
	return builder.Build()
}

func buildCertificateChildren(certKey string, cert kube.Resource, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	return appendGraphChildren(certKey, graphChildren, state, nodeMap)
}

func buildHTTPRouteChildren(httpRouteKey string, httpRoute kube.Resource, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	builder := NewChildrenBuilder()
	for _, childKey := range graphChildren[httpRouteKey] {
		if state.CanTraverse(childKey) {
			childResource, exists := nodeMap[childKey]
			if !exists {
				continue
			}
			if childResource.Type == "service" {
				state.MarkSeen(childKey)
				continue
			}
			builder.Add(buildTreeNode(childKey, graphChildren, state, nodeMap))
		}
	}
	return builder.Build()
}
