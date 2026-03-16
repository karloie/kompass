package tui

import (
	"github.com/karloie/kompass/pkg/diagnostics"
)

func resolveNetpolProvider(p diagnostics.NetpolProvider) diagnostics.NetpolProvider {
	return diagnostics.ResolveNetpolProvider(p)
}

func resolveHubbleProvider(p diagnostics.HubbleProvider) diagnostics.HubbleProvider {
	return diagnostics.ResolveHubbleProvider(p)
}
