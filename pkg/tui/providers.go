package tui

import (
	"log/slog"

	"github.com/karloie/kompass/pkg/diagnostics"
	kube "github.com/karloie/kompass/pkg/kube"
)

type NetpolProvider = diagnostics.NetpolProvider

type HubbleProvider = diagnostics.HubbleProvider

type defaultNetpolProvider struct{}

func (defaultNetpolProvider) AnalyzePod(target diagnostics.PodTarget, context string, resources map[string]*kube.Resource) (string, error) {
	t := resourceTarget{ResourceType: target.ResourceType, Name: target.Name, Namespace: target.Namespace}
	if analysis, ok := runNetpolAnalysisFromResources(t, resources); ok {
		return analysis, nil
	}
	slog.Warn("netpol provider fallback", "provider", "kubectl", "namespace", t.Namespace, "name", t.Name, "reason", "in-memory analysis unavailable")
	return runNetpolAnalysis(t, context)
}

type defaultHubbleProvider struct{}

func (defaultHubbleProvider) ObservePod(podRef string, last int, context string) (string, error) {
	return runHubbleObserve(podRef, last, context)
}

func resolveNetpolProvider(p NetpolProvider) NetpolProvider {
	if p != nil {
		return p
	}
	return defaultNetpolProvider{}
}

func resolveHubbleProvider(p HubbleProvider) HubbleProvider {
	if p != nil {
		return p
	}
	return defaultHubbleProvider{}
}
