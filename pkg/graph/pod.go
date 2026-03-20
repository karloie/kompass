package graph

import kube "github.com/karloie/kompass/pkg/kube"

func inferPod(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
	podKey := addNode(edges, item, nodes, "pod")
	if podKey == "" {
		return nil
	}

	ruleEdges := ApplyEdgeRules(item, *nodes)
	*edges = append(*edges, ruleEdges...)

	return nil
}

func inferService(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
	key := addNode(edges, item, nodes, "service")
	if key == "" {
		return nil
	}

	_, spec := ExtractMetaSpec(item)
	if spec == nil {
		return nil
	}

	selector := extractSelector(spec.Raw())
	if len(selector) == 0 {
		return nil
	}

	for _, n := range *nodes {
		if n.Type != "pod" {
			continue
		}
		meta := M(n.AsMap()).Map("metadata")
		if meta != nil && matchesLabels(selector, meta.Raw()) {
			addEdge(edges, n.Key, key, "exposed-by")
		}
	}

	return nil
}

func inferSimpleNode(kind string) func(*[]kube.ResourceEdge, *kube.Resource, *map[string]kube.Resource, kube.Provider) error {
	return func(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, _ kube.Provider) error {
		addNode(edges, item, nodes, kind)
		return nil
	}
}
