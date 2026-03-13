package pipeline

import "github.com/karloie/kompass/pkg/kube"

func GraphNodesForGraph(result *kube.Graphs, graph *kube.Graph) map[string]*kube.Resource {
	if graph == nil {
		return nil
	}
	if len(result.Nodes) > 0 {
		return result.Nodes
	}
	return nil
}
