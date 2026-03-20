package graph

import (
	"context"
	"os"
	"sort"
	"strings"

	kube "github.com/karloie/kompass/pkg/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func isWorkloadType(resourceType string) bool {
	switch resourceType {
	case "deployment", "replicaset", "statefulset", "daemonset", "job", "cronjob", "pod":
		return true
	default:
		return false
	}
}

func findWorkloadRoot(key string, keyType string, nodeMap map[string]kube.Resource) string {

	switch keyType {
	case "deployment", "statefulset", "daemonset":
		return key
	}

	resource, exists := nodeMap[key]
	if !exists {
		return ""
	}

	parts := strings.Split(key, "/")
	if len(parts) < 3 {
		return ""
	}
	namespace := parts[1]

	meta := M(resource.AsMap()).Map("metadata")
	if meta == nil {

		if keyType == "pod" || keyType == "replicaset" {
			return key
		}
		return ""
	}

	owners := meta.Slice("ownerReferences")
	if len(owners) == 0 {

		if keyType == "pod" || keyType == "replicaset" {
			return key
		}
		return ""
	}

	for _, ownerAny := range owners {
		owner, ok := ownerAny.(map[string]any)
		if !ok {
			continue
		}

		kind := M(owner).String("kind")
		name := M(owner).String("name")

		if kind == "" || name == "" {
			continue
		}

		var ownerType string
		switch kind {
		case "Deployment":
			ownerType = "deployment"
		case "StatefulSet":
			ownerType = "statefulset"
		case "DaemonSet":
			ownerType = "daemonset"
		case "ReplicaSet":
			ownerType = "replicaset"
		case "Job":
			ownerType = "job"
		case "CronJob":
			ownerType = "cronjob"
		default:
			continue
		}

		ownerKey := ownerType + "/" + namespace + "/" + name

		if ownerType == "replicaset" {
			if deploymentKey := findWorkloadRoot(ownerKey, ownerType, nodeMap); deploymentKey != "" {
				return deploymentKey
			}
		}

		return ownerKey
	}

	if keyType == "pod" || keyType == "replicaset" {
		return key
	}
	return ""
}

// BuildGraphs constructs a graph of Kubernetes resources and their relationships.
// It loads resources matching the request selectors, analyzes their dependencies,
// and returns a Response containing nodes, edges, and connected components.
func BuildGraphs(provider kube.Provider, req kube.Request) (*kube.Response, error) {
	selectors := req.NormalizedSelectors()
	defaultNamespace := req.DefaultNamespace()

	if defaultNamespace == "" {
		if ns, err := provider.GetNamespace(); err == nil && ns != "" {
			defaultNamespace = ns
		}
	}

	nodeMap, edges := map[string]kube.Resource{}, []kube.ResourceEdge{}
	namespaces := parseNamespaces(selectors, defaultNamespace, provider)
	hasRoutesOrIngress := false
	hasGateways := false

	for _, loader := range kube.Loaders {
		if loader == nil {
			continue
		}
		for ns := range namespaces {
			resources, err := loader(provider, ns, context.Background(), metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			for _, r := range resources {
				if _, exists := nodeMap[r.Key]; !exists {
					nodeMap[r.Key] = r
					if r.Type == "httproute" || r.Type == "ingress" || r.Type == "ingress-route" {
						hasRoutesOrIngress = true
					}
					if r.Type == "gateway" {
						hasGateways = true
					}
				}
			}
		}
	}

	if hasRoutesOrIngress {
		if gatewayLoader, ok := kube.Loaders["gateway"]; ok && gatewayLoader != nil {
			for ns := range collectHTTPRouteGatewayNamespaces(nodeMap) {
				if ns == "" || namespaces[ns] {
					continue
				}
				if resources, err := gatewayLoader(provider, ns, context.Background(), metav1.ListOptions{}); err == nil {
					for _, r := range resources {
						if _, exists := nodeMap[r.Key]; !exists {
							nodeMap[r.Key] = r
							if r.Type == "gateway" {
								hasGateways = true
							}
						}
					}
				}
			}
		}

		loadedExtraCertNamespaces := make(map[string]bool)
		if certLoader, ok := kube.Loaders["certificate"]; ok && certLoader != nil {
			certNamespaces := []string{"management", "cert-manager", "gateway-system", "istio-system"}
			if envNs := os.Getenv("KOMPASS_CERT_NAMESPACES"); envNs != "" {
				certNamespaces = strings.Split(envNs, ",")
			}
			for _, ns := range certNamespaces {
				if ns = strings.TrimSpace(ns); ns == "" || namespaces[ns] {
					continue
				}
				loadedExtraCertNamespaces[ns] = true
				if resources, err := certLoader(provider, ns, context.Background(), metav1.ListOptions{}); err == nil {
					for _, r := range resources {
						if _, exists := nodeMap[r.Key]; !exists {
							nodeMap[r.Key] = r
						}
					}
				}
			}

			for ns := range collectGatewayCertificateNamespaces(nodeMap) {
				if ns == "" || namespaces[ns] || loadedExtraCertNamespaces[ns] {
					continue
				}
				loadedExtraCertNamespaces[ns] = true
				if resources, err := certLoader(provider, ns, context.Background(), metav1.ListOptions{}); err == nil {
					for _, r := range resources {
						if _, exists := nodeMap[r.Key]; !exists {
							nodeMap[r.Key] = r
						}
					}
				}
			}
		}

		if issuerLoader, ok := kube.Loaders["issuer"]; ok && issuerLoader != nil {
			for ns := range loadedExtraCertNamespaces {
				if resources, err := issuerLoader(provider, ns, context.Background(), metav1.ListOptions{}); err == nil {
					for _, r := range resources {
						if _, exists := nodeMap[r.Key]; !exists {
							nodeMap[r.Key] = r
						}
					}
				}
			}
		}

		if clusterIssuerLoader, ok := kube.Loaders["clusterissuer"]; ok && clusterIssuerLoader != nil {
			if resources, err := clusterIssuerLoader(provider, "", context.Background(), metav1.ListOptions{}); err == nil {
				for _, r := range resources {
					if _, exists := nodeMap[r.Key]; !exists {
						nodeMap[r.Key] = r
					}
				}
			}
		}
	}

	if hasGateways {
		if httpRouteLoader, ok := kube.Loaders["httproute"]; ok && httpRouteLoader != nil {
			// Fetch all HTTPRoutes cluster-wide in one call instead of iterating every namespace.
			if resources, err := httpRouteLoader(provider, "", context.Background(), metav1.ListOptions{}); err == nil {
				for _, r := range resources {
					if _, exists := nodeMap[r.Key]; !exists {
						nodeMap[r.Key] = r
						hasRoutesOrIngress = true
					}
				}
			}
		}
	}

	keys, err := expandSelectors(selectors, defaultNamespace, nodeMap)
	if err != nil {
		return nil, err
	}

	nodesByType := make(map[string][]kube.Resource, len(kube.Loaders))
	for _, n := range nodeMap {
		nodesByType[n.Type] = append(nodesByType[n.Type], n)
	}

	for typ, handler := range handlers {
		if handler != nil {
			for i := range nodesByType[typ] {
				if err := handler(&edges, &nodesByType[typ][i], &nodeMap, provider); err != nil {
					return nil, err
				}
			}
		}
	}

	return buildGraphs(keys, edges, nodeMap), nil
}

func collectHTTPRouteGatewayNamespaces(nodeMap map[string]kube.Resource) map[string]bool {
	namespaces := map[string]bool{}
	for _, n := range nodeMap {
		if n.Type != "httproute" {
			continue
		}
		resourceMap := n.AsMap()
		meta := M(resourceMap).Map("metadata").Raw()
		routeNamespace := M(meta).String("namespace")
		spec := M(resourceMap).Map("spec").Raw()
		for _, parentRef := range M(spec).MapSlice("parentRefs") {
			kind := parentRef.String("kind")
			if kind != "" && !strings.EqualFold(kind, "Gateway") {
				continue
			}
			gatewayNamespace := parentRef.String("namespace")
			if gatewayNamespace == "" {
				gatewayNamespace = routeNamespace
			}
			if gatewayNamespace != "" {
				namespaces[gatewayNamespace] = true
			}
		}
	}
	return namespaces
}

func collectGatewayCertificateNamespaces(nodeMap map[string]kube.Resource) map[string]bool {
	namespaces := map[string]bool{}
	for _, n := range nodeMap {
		if n.Type != "gateway" {
			continue
		}
		resourceMap := n.AsMap()
		meta := M(resourceMap).Map("metadata").Raw()
		gatewayNamespace := M(meta).String("namespace")
		spec := M(resourceMap).Map("spec").Raw()
		for _, listener := range M(spec).MapSlice("listeners") {
			tls, ok := listener.MapOk("tls")
			if !ok {
				continue
			}
			for _, certRef := range tls.MapSlice("certificateRefs") {
				certNamespace := certRef.String("namespace")
				if certNamespace == "" {
					certNamespace = gatewayNamespace
				}
				if certNamespace != "" {
					namespaces[certNamespace] = true
				}
			}
		}
	}
	return namespaces
}

func buildGraphs(keys []string, edges []kube.ResourceEdge, nodeMap map[string]kube.Resource) *kube.Response {
	neighbors := make(map[string][]string)
	for _, e := range edges {
		if e.Source != "" && e.Target != "" {
			neighbors[e.Source] = append(neighbors[e.Source], e.Target)
			if targetType := strings.Split(e.Target, "/")[0]; targetType != "node" && targetType != "storageclass" && targetType != "namespace" {
				neighbors[e.Target] = append(neighbors[e.Target], e.Source)
			}
		}
	}

	matchedKeySet := make(map[string]bool)
	for _, key := range keys {
		matchedKeySet[key] = true
	}

	workloadRoots := make(map[string]bool)

	for _, key := range keys {
		keyType := strings.Split(key, "/")[0]

		workloadKey := findWorkloadRoot(key, keyType, nodeMap)
		if workloadKey != "" {
			workloadRoots[workloadKey] = true
		}

		if keyType != "pod" {
			for _, neighbor := range neighbors[key] {
				neighborType := strings.Split(neighbor, "/")[0]
				if neighborType == "pod" {
					workloadKey := findWorkloadRoot(neighbor, neighborType, nodeMap)
					if workloadKey != "" {
						workloadRoots[workloadKey] = true
					}
				}
			}
		}
	}

	var workloadKeys []string
	for workloadKey := range workloadRoots {
		workloadKeys = append(workloadKeys, workloadKey)
	}
	sort.Strings(workloadKeys)

	coveredKeys := make(map[string]bool)
	var components []kube.Component

	for _, rootKey := range workloadKeys {
		if rootKey == "" {
			continue
		}

		visited, queue := make(map[string]bool), []string{rootKey}
		visited[rootKey] = true
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, nb := range neighbors[cur] {
				if !visited[nb] {
					visited[nb] = true
					queue = append(queue, nb)
				}
			}
		}

		for visitedKey := range visited {
			if matchedKeySet[visitedKey] {
				coveredKeys[visitedKey] = true
			}
		}

		components = append(components, buildComponent(rootKey, visited))
	}

	for _, key := range keys {
		keyType := strings.Split(key, "/")[0]

		if isWorkloadType(keyType) {
			continue
		}
		if coveredKeys[key] {
			continue
		}

		visited, queue := make(map[string]bool), []string{key}
		visited[key] = true
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, nb := range neighbors[cur] {
				if !visited[nb] {
					visited[nb] = true
					queue = append(queue, nb)
				}
			}
		}

		components = append(components, buildComponent(key, visited))
	}

	inferredRootTypes := map[string]bool{
		"gateway":     true,
		"certificate": true,
	}

	inferredPriority := map[string]int{
		"gateway":       3,
		"certificate":   2,
		"clusterissuer": 1,
	}

	var inferredRoots []string
	for _, node := range nodeMap {
		if !inferredRootTypes[node.Type] {
			continue
		}
		if matchedKeySet[node.Key] {
			continue
		}
		inferredRoots = append(inferredRoots, node.Key)
	}

	sort.Slice(inferredRoots, func(i, j int) bool {
		typeI := strings.Split(inferredRoots[i], "/")[0]
		typeJ := strings.Split(inferredRoots[j], "/")[0]
		priI := inferredPriority[typeI]
		priJ := inferredPriority[typeJ]
		if priI != priJ {
			return priI > priJ
		}
		return inferredRoots[i] < inferredRoots[j]
	})

	inferredVisited := make(map[string]bool)
	for _, rootKey := range inferredRoots {
		if inferredVisited[rootKey] {
			continue
		}

		visited, queue := make(map[string]bool), []string{rootKey}
		visited[rootKey] = true
		inferredVisited[rootKey] = true

		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, nb := range neighbors[cur] {
				if !visited[nb] {
					visited[nb] = true

					if inferredRootTypes[strings.Split(nb, "/")[0]] && !matchedKeySet[nb] {
						inferredVisited[nb] = true
					}
					queue = append(queue, nb)
				}
			}
		}

		components = append(components, buildComponent(rootKey, visited))
	}

	sort.Slice(components, func(i, j int) bool {
		typeI, namespaceI, nameI := graphIDParts(components[i].Root)
		typeJ, namespaceJ, nameJ := graphIDParts(components[j].Root)

		isWorkloadI := isWorkloadType(typeI)
		isWorkloadJ := isWorkloadType(typeJ)

		isInferredI := typeI == "certificate" || typeI == "gateway" || typeI == "clusterissuer"
		isInferredJ := typeJ == "certificate" || typeJ == "gateway" || typeJ == "clusterissuer"

		if isWorkloadI && !isWorkloadJ {
			return true
		}
		if !isWorkloadI && isWorkloadJ {
			return false
		}

		if !isWorkloadI && !isWorkloadJ {
			if !isInferredI && isInferredJ {
				return true
			}
			if isInferredI && !isInferredJ {
				return false
			}
		}

		if nameI != nameJ {
			return nameI < nameJ
		}
		if typeI != typeJ {
			return typeI < typeJ
		}
		if namespaceI != namespaceJ {
			return namespaceI < namespaceJ
		}
		return components[i].Root < components[j].Root
	})

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		if edges[i].Target != edges[j].Target {
			return edges[i].Target < edges[j].Target
		}
		return edges[i].Label < edges[j].Label
	})

	nodeKeys := make([]string, 0, len(nodeMap))
	for key := range nodeMap {
		nodeKeys = append(nodeKeys, key)
	}
	sort.Strings(nodeKeys)

	responseNodes := make([]kube.Resource, 0, len(nodeKeys))
	for _, key := range nodeKeys {
		node := nodeMap[key]
		node.Discovered = !matchedKeySet[key]
		responseNodes = append(responseNodes, node)
	}

	return &kube.Response{Nodes: responseNodes, Edges: edges, Components: components}
}

func graphIDParts(id string) (resourceType, namespace, name string) {
	parts := strings.SplitN(id, "/", 3)
	if len(parts) > 0 {
		resourceType = parts[0]
	}
	if len(parts) > 1 {
		namespace = parts[1]
	}
	if len(parts) > 2 {
		name = parts[2]
	}
	return resourceType, namespace, name
}

func buildComponent(rootKey string, visited map[string]bool) kube.Component {
	nodeKeys := make([]string, 0, len(visited))
	for key := range visited {
		nodeKeys = append(nodeKeys, key)
	}
	sort.Strings(nodeKeys)

	return kube.Component{
		ID:       rootKey,
		Root:     rootKey,
		NodeKeys: nodeKeys,
	}
}
