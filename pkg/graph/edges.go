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

func InferGraphs(provider kube.Kube, req kube.Request) (*kube.Graphs, error) {
	selectors := req.Selectors()
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

	for _, task := range ResourceTypes {
		if task.Loader == nil {
			continue
		}
		for ns := range namespaces {
			resources, err := task.Loader(provider, ns, context.Background(), metav1.ListOptions{})
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
		if certTask, ok := ResourceTypes["certificate"]; ok && certTask.Loader != nil {
			certNamespaces := []string{"management", "cert-manager", "gateway-system", "istio-system"}
			if envNs := os.Getenv("KOMPASS_CERT_NAMESPACES"); envNs != "" {
				certNamespaces = strings.Split(envNs, ",")
			}
			for _, ns := range certNamespaces {
				if ns = strings.TrimSpace(ns); ns == "" || namespaces[ns] {
					continue
				}
				if resources, err := certTask.Loader(provider, ns, context.Background(), metav1.ListOptions{}); err == nil {
					for _, r := range resources {
						if _, exists := nodeMap[r.Key]; !exists {
							nodeMap[r.Key] = r
						}
					}
				}
			}
		}

		if clusterIssuerTask, ok := ResourceTypes["clusterissuer"]; ok && clusterIssuerTask.Loader != nil {
			if resources, err := clusterIssuerTask.Loader(provider, "", context.Background(), metav1.ListOptions{}); err == nil {
				for _, r := range resources {
					if _, exists := nodeMap[r.Key]; !exists {
						nodeMap[r.Key] = r
					}
				}
			}
		}
	}

	if hasGateways {
		if httpRouteTask, ok := ResourceTypes["httproute"]; ok && httpRouteTask.Loader != nil {

			nsList, err := provider.GetNamespaces(context.Background(), metav1.ListOptions{})
			if err == nil && nsList != nil {
				for _, ns := range nsList.Items {
					nsName := ns.Name

					if namespaces[nsName] {
						continue
					}

					if resources, err := httpRouteTask.Loader(provider, nsName, context.Background(), metav1.ListOptions{}); err == nil {
						for _, r := range resources {
							if _, exists := nodeMap[r.Key]; !exists {
								nodeMap[r.Key] = r
								hasRoutesOrIngress = true
							}
						}
					}
				}
			}
		}
	}

	keys, err := expandSelectors(selectors, defaultNamespace, nodeMap)
	if err != nil {
		return nil, err
	}

	nodesByType := make(map[string][]kube.Resource, len(ResourceTypes))
	for _, n := range nodeMap {
		nodesByType[n.Type] = append(nodesByType[n.Type], n)
	}

	for typ, task := range ResourceTypes {
		if task.Handler != nil {
			for i := range nodesByType[typ] {
				if err := task.Handler(&edges, &nodesByType[typ][i], &nodeMap, provider); err != nil {
					return nil, err
				}
			}
		}
	}

	return buildGraphs(keys, edges, nodeMap), nil
}

func buildGraphs(keys []string, edges []kube.ResourceEdge, nodeMap map[string]kube.Resource) *kube.Graphs {
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
	var graphs []kube.Graph

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

		graphs = append(graphs, buildGraph(rootKey, visited, matchedKeySet, nodeMap, edges))
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

		graphs = append(graphs, buildGraph(key, visited, matchedKeySet, nodeMap, edges))
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

		graphs = append(graphs, buildGraph(rootKey, visited, matchedKeySet, nodeMap, edges))
	}

	sort.Slice(graphs, func(i, j int) bool {
		typeI := strings.Split(graphs[i].ID, "/")[0]
		typeJ := strings.Split(graphs[j].ID, "/")[0]

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

		return graphs[i].ID < graphs[j].ID
	})
	responseNodes := make(map[string]*kube.Resource, len(nodeMap))
	for _, n := range nodeMap {
		nCopy := n
		nCopy.Discovered = !matchedKeySet[n.Key]
		responseNodes[n.Key] = &nCopy
	}

	return &kube.Graphs{Graphs: graphs, Nodes: responseNodes}
}

func buildGraph(id string, visited map[string]bool, _ map[string]bool, _ map[string]kube.Resource, edges []kube.ResourceEdge) kube.Graph {
	graph := kube.Graph{ID: id}

	for _, e := range edges {
		if visited[e.Source] && visited[e.Target] {
			graph.Edges = append(graph.Edges, e)
		}
	}

	sort.Slice(graph.Edges, func(i, j int) bool {
		if graph.Edges[i].Source != graph.Edges[j].Source {
			return graph.Edges[i].Source < graph.Edges[j].Source
		}
		return graph.Edges[i].Target < graph.Edges[j].Target
	})

	return graph
}
