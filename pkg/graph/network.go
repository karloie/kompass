package graph

import (
	"fmt"
	"strings"

	kube "github.com/karloie/kompass/pkg/kube"
)

func inferIngress(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Kube) error {
	namespace, ingressName := ExtractNamespacedName(item)
	if namespace == "" || ingressName == "" {
		return nil
	}

	key := Key("ingress", namespace, ingressName)
	addNode(edges, item, nodes, "ingress")

	spec := M(item.AsMap()).Map("spec").Raw()
	if spec == nil {
		return nil
	}

	tlsSecrets := make(map[string]bool)
	for _, tlsMap := range M(spec).MapSlice("tls") {
		if secretName := tlsMap.String("secretName"); secretName != "" {
			tlsSecrets[secretName] = true
		}
	}

	forEachNodeOfType(*nodes, "certificate", func(n kube.Resource) {
		if certSpec := M(n.AsMap()).Map("spec").Raw(); certSpec != nil {
			if secretName := M(certSpec).String("secretName"); tlsSecrets[secretName] {
				if certNamespace := M(n.AsMap()).Map("metadata").String("namespace"); certNamespace == namespace {
					addEdge(edges, key, n.Key, "tls")
				}
			}
		}
	})

	if rules, ok := spec["rules"].([]any); ok {
		for _, r := range rules {
			if rule, ok := r.(map[string]any); ok {
				if http, ok := rule["http"].(map[string]any); ok {
					if paths, ok := http["paths"].([]any); ok {
						for _, p := range paths {
							if pathMap, ok := p.(map[string]any); ok {
								if backend, ok := pathMap["backend"].(map[string]any); ok {
									if service, ok := backend["service"].(map[string]any); ok {
										if svcName, ok := service["name"].(string); ok && svcName != "" {
											svcKey := Key("service", namespace, svcName)
											addEdgeIfNodeExists(edges, *nodes, key, svcKey, "backend")
										}
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

func inferHTTPRoute(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Kube) error {
	namespace, httpRouteName := ExtractNamespacedName(item)
	if namespace == "" || httpRouteName == "" {
		return nil
	}

	key := Key("httproute", namespace, httpRouteName)
	addNode(edges, item, nodes, "httproute")

	spec := M(item.AsMap()).Map("spec").Raw()
	if spec == nil {
		return nil
	}

	hostnameStrs := []string{}
	for _, h := range M(spec).Slice("hostnames") {
		if hostname, ok := h.(string); ok && hostname != "" {
			hostnameStrs = append(hostnameStrs, hostname)
		}
	}

	rules, ok := spec["rules"].([]any)
	if ok {
		for _, r := range rules {
			ruleMap, _ := r.(map[string]any)
			backendRefs, _ := ruleMap["backendRefs"].([]any)
			for _, b := range backendRefs {
				backendMap, _ := b.(map[string]any)
				svcName := M(backendMap).String("name")
				if svcName != "" {
					svcKey := Key("service", namespace, svcName)
					addEdgeIfNodeExists(edges, *nodes, key, svcKey, "backend")
				}
			}
		}
	}

	for _, parentMap := range M(spec).MapSlice("parentRefs") {
		parentKind := parentMap.String("kind")
		if parentKind == "" || parentKind == "Gateway" {
			parentName := parentMap.String("name")
			parentNamespace := parentMap.String("namespace")
			if parentNamespace == "" {
				parentNamespace = namespace
			}
			if parentName != "" {
				gatewayKey := Key("gateway", parentNamespace, parentName)
				addEdgeIfNodeExists(edges, *nodes, gatewayKey, key, "route")
			}
		}
	}
	return nil
}

func inferGateway(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Kube) error {
	namespace, name := ExtractNamespacedName(item)
	if name == "" || namespace == "" {
		return nil
	}

	key := Key("gateway", namespace, name)
	addNode(edges, item, nodes, "gateway")

	spec := M(item.AsMap()).Map("spec").Raw()
	if spec == nil {
		return nil
	}

	var certificateRefs []string
	for _, listenerMap := range M(spec).MapSlice("listeners") {
		if tls, ok := listenerMap.MapOk("tls"); ok {
			for _, certRefMap := range tls.MapSlice("certificateRefs") {
				certName := certRefMap.String("name")
				certNamespace := certRefMap.String("namespace")
				if certNamespace == "" {
					certNamespace = namespace
				}
				if certName != "" {
					certificateRefs = append(certificateRefs, certNamespace+"/"+certName)
				}
			}
		}
	}

	for _, secretRef := range certificateRefs {
		parts := strings.Split(secretRef, "/")
		if len(parts) != 2 {
			continue
		}
		secretNamespace, secretName := parts[0], parts[1]

		for _, n := range *nodes {
			if n.Type == "certificate" {
				certSpec := M(n.AsMap()).Map("spec").Raw()
				if certSpec != nil && M(certSpec).String("secretName") == secretName {
					certMeta := M(n.AsMap()).Map("metadata").Raw()
					certNamespace := M(certMeta).String("namespace")
					if certNamespace == secretNamespace {
						addEdge(edges, key, n.Key, "tls")
						break
					}
				}
			}
		}
	}

	return nil
}

func inferNetworkPolicy(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Kube) error {
	meta := M(item.AsMap()).Map("metadata").Raw()
	namespace, name := M(meta).String("namespace"), M(meta).String("name")
	if name == "" || namespace == "" {
		return nil
	}

	key := Key("networkpolicy", namespace, name)
	addNode(edges, item, nodes, "networkpolicy")

	spec := M(item.AsMap()).Map("spec").Raw()
	if spec == nil {
		return nil
	}

	selector := extractSelector(spec)
	if len(selector) == 0 {
		return nil
	}

	forEachPodMatchingSelector(*nodes, namespace, selector, func(n kube.Resource) {
		addEdge(edges, key, n.Key, "applies-to")
	})
	return nil
}

func inferEndpointSlices(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Kube) error {
	key := addNode(edges, item, nodes, "endpointslice")
	if key == "" {
		return nil
	}

	meta := M(item.AsMap()).Map("metadata").Raw()
	if meta == nil {
		return nil
	}

	namespace := M(meta).String("namespace")

	if labels, ok := meta["labels"].(map[string]any); ok {
		if serviceName, ok := labels["kubernetes.io/service-name"].(string); ok && serviceName != "" {
			svcKey := Key("service", namespace, serviceName)
			addEdgeIfNodeExists(edges, *nodes, svcKey, key, "routes-to")
		}
	}

	for _, epMap := range M(item.AsMap()).MapSlice("endpoints") {
		if targetRef, ok := epMap.MapOk("targetRef"); ok {
			if kind := targetRef.String("kind"); kind == "Pod" {
				if podName := targetRef.String("name"); podName != "" {
					podKey := Key("pod", namespace, podName)
					addEdgeIfNodeExists(edges, *nodes, key, podKey, "routes-to")
				}
			}
		}
	}

	return nil
}

func inferEndpoints(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Kube) error {
	key := addNode(edges, item, nodes, "endpoints")
	if key == "" {
		return nil
	}

	namespace, name := ExtractNamespacedName(item)
	svcKey := Key("service", namespace, name)
	addEdgeIfNodeExists(edges, *nodes, svcKey, key, "routes-to")
	return nil
}

func inferCiliumNetworkPolicy(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Kube) error {
	addNode(edges, item, nodes, "ciliumnetworkpolicy")

	meta := M(item.AsMap()).Map("metadata").Raw()
	namespace := M(meta).String("namespace")
	spec := M(item.AsMap()).Map("spec").Raw()
	if spec == nil {
		return nil
	}

	endpointSelector, _ := spec["endpointSelector"].(map[string]any)
	matchLabels, _ := endpointSelector["matchLabels"].(map[string]any)
	var targetPods []string
	if len(matchLabels) > 0 {
		forEachPodMatchingSelector(*nodes, namespace, matchLabels, func(n kube.Resource) {
			addEdge(edges, item.Key, n.Key, "applies-to")
			targetPods = append(targetPods, n.Key)
		})
	}

	if ingress, ok := spec["ingress"].([]any); ok {
		for ruleIdx, rule := range ingress {
			if ruleMap, ok := rule.(map[string]any); ok {

				if fromEndpoints, ok := ruleMap["fromEndpoints"].([]any); ok {
					for epIdx, endpoint := range fromEndpoints {
						if endpointMap, ok := endpoint.(map[string]any); ok {
							if ingressLabels, ok := endpointMap["matchLabels"].(map[string]any); ok {

								ruleKey := fmt.Sprintf("%s/ingress/rule/%d", item.Key, ruleIdx)
								epKey := fmt.Sprintf("%s/fromEndpoint/%d", ruleKey, epIdx)

								addEdge(edges, item.Key, epKey, "policy-ingress-rule")

								for _, n := range *nodes {
									if n.Type == "service" {

										resourceMeta := M(n.AsMap()).Map("metadata").Raw()
										if matchesCiliumLabels(ingressLabels, resourceMeta) {

											addEdge(edges, epKey, n.Key, "inferred-ingress")
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if egress, ok := spec["egress"].([]any); ok {
		for ruleIdx, rule := range egress {
			if ruleMap, ok := rule.(map[string]any); ok {

				if toEndpoints, ok := ruleMap["toEndpoints"].([]any); ok {
					for epIdx, endpoint := range toEndpoints {
						if endpointMap, ok := endpoint.(map[string]any); ok {
							if egressLabels, ok := endpointMap["matchLabels"].(map[string]any); ok {

								ruleKey := fmt.Sprintf("%s/egress/rule/%d", item.Key, ruleIdx)
								epKey := fmt.Sprintf("%s/toEndpoint/%d", ruleKey, epIdx)

								addEdge(edges, item.Key, epKey, "policy-egress-rule")

								for _, n := range *nodes {
									if n.Type == "service" {

										resourceMeta := M(n.AsMap()).Map("metadata").Raw()
										if matchesCiliumLabels(egressLabels, resourceMeta) {

											addEdge(edges, epKey, n.Key, "inferred-egress")
										}
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

func matchesCiliumLabels(selector map[string]any, meta map[string]any) bool {
	namespace := M(meta).String("namespace")

	var labels map[string]string
	if labelsAny, ok := meta["labels"].(map[string]any); ok {

		labels = make(map[string]string)
		for k, v := range labelsAny {
			if strVal, ok := v.(string); ok {
				labels[k] = strVal
			}
		}
	} else if labelsStr, ok := meta["labels"].(map[string]string); ok {
		labels = labelsStr
	}

	for k, v := range selector {
		selVal, ok := v.(string)
		if !ok {
			return false
		}

		labelKey := k
		if strings.HasPrefix(k, "k8s:") {
			labelKey = strings.TrimPrefix(k, "k8s:")

			if labelKey == "io.kubernetes.pod.namespace" {
				if namespace != selVal {
					return false
				}
				continue
			}
		}

		if labels == nil {
			return false
		}
		labelVal, ok := labels[labelKey]
		if !ok || labelVal != selVal {
			return false
		}
	}
	return true
}

func inferCiliumClusterwideNetworkPolicy(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Kube) error {

	meta := M(item.AsMap()).Map("metadata").Raw()
	name := M(meta).String("name")
	key := "ciliumclusterwidenetworkpolicy/" + name
	resource := *item
	resource.Type, resource.Key = "ciliumclusterwidenetworkpolicy", key
	(*nodes)[key] = resource

	spec := M(item.AsMap()).Map("spec").Raw()
	if spec == nil {
		return nil
	}

	endpointSelector, _ := spec["endpointSelector"].(map[string]any)
	matchLabels, _ := endpointSelector["matchLabels"].(map[string]any)
	if len(matchLabels) > 0 {
		forEachPodMatchingSelector(*nodes, "", matchLabels, func(n kube.Resource) {
			addEdge(edges, key, n.Key, "applies-to")
		})
	}

	return nil
}

func inferIngressClass(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Kube) error {
	_, name := ExtractNamespacedName(item)
	if name == "" {
		return nil
	}

	key := Key("ingressclass", "", name)
	addNode(edges, item, nodes, "ingressclass")

	for _, n := range *nodes {
		if n.Type == "ingress" {
			ingressSpec := M(n.AsMap()).Map("spec").Raw()
			if ingressSpec != nil {
				ingressClassName := M(ingressSpec).String("ingressClassName")
				if ingressClassName == name {
					addEdge(edges, n.Key, key, "class")
				}
			}
		}
	}

	return nil
}
