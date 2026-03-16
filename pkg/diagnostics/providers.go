package diagnostics

import kube "github.com/karloie/kompass/pkg/kube"

// PodTarget identifies the pod being analyzed/observed.
type PodTarget struct {
	ResourceType string
	Name         string
	Namespace    string
}

// NetpolProvider abstracts netpol analysis so callers can inject mocks.
type NetpolProvider interface {
	AnalyzePod(target PodTarget, context string, resources map[string]*kube.Resource) (string, error)
}

// HubbleProvider abstracts flow observation so callers can inject mocks.
type HubbleProvider interface {
	ObservePod(podRef string, last int, context string) (string, error)
}