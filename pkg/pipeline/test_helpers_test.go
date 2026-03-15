package pipeline

import "github.com/karloie/kompass/pkg/kube"

func GraphNodesForComponent(result *kube.Response, component *kube.Component) map[string]*kube.Resource {
	if component == nil {
		return nil
	}
	return result.NodeMap()
}
