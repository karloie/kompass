package diagnostics

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RunHubbleCommand executes hubble CLI with relay auto-port-forward retry.
var RunHubbleCommand = func(args ...string) (string, error) {
	out, err := exec.Command("hubble", args...).CombinedOutput()
	body := strings.TrimRight(string(out), "\n")
	if err != nil && IsHubbleRelayUnavailable(body) {
		// Relay not reachable - try to start it and retry once.
		if pfErr := StartHubblePortForward(); pfErr == nil {
			out2, err2 := exec.Command("hubble", args...).CombinedOutput()
			body = strings.TrimRight(string(out2), "\n")
			err = err2
		}
	}
	if body == "" && err != nil {
		body = "hubble observe unavailable; ensure the hubble CLI is installed and relay is running"
	}
	return body, err
}

// StartHubblePortForward runs cilium hubble port-forward and waits briefly.
var StartHubblePortForward = func() error {
	cmd := exec.Command("cilium", "hubble", "port-forward")
	if err := cmd.Start(); err != nil {
		return err
	}
	deadline := 30 // x100ms
	for i := 0; i < deadline; i++ {
		time.Sleep(100 * time.Millisecond)
		probe, err := exec.Command("hubble", "observe", "--last", "1").CombinedOutput()
		if err == nil || !IsHubbleRelayUnavailable(string(probe)) {
			return nil
		}
	}
	return nil
}

func IsHubbleRelayUnavailable(output string) bool {
	return strings.Contains(output, "rpc error") && strings.Contains(output, "Unavailable")
}

var HubbleProviderMode = func() string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("KOMPASS_HUBBLE_PROVIDER")))
	switch mode {
	case "native", "cli", "auto":
		return mode
	default:
		return "auto"
	}
}

var RunHubbleObserve = func(podRef string, last int, context string) (string, error) {
	return ObserveHubbleByMode(podRef, last, context, HubbleProviderMode())
}

func ObserveHubbleByMode(podRef string, last int, context, mode string) (string, error) {
	switch mode {
	case "cli":
		return observeHubbleWithCLI(podRef, last, context)
	case "native":
		return observeHubbleNative(podRef, last, context)
	default: // auto
		body, err := observeHubbleNative(podRef, last, context)
		if err == nil && !IsNativeHubbleNoData(body) {
			return body, nil
		}
		reason := "no native flow data"
		if err != nil {
			reason = err.Error()
		}
		slog.Warn("hubble provider fallback", "from", "native", "to", "cli", "pod", podRef, "reason", reason)
		return observeHubbleWithCLI(podRef, last, context)
	}
}

func IsNativeHubbleNoData(body string) bool {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return true
	}
	return strings.HasPrefix(trimmed, "(no hubble flows observed")
}

func observeHubbleWithCLI(podRef string, last int, context string) (string, error) {
	_ = context // hubble CLI does not support kubectl --context
	if last <= 0 {
		last = 100
	}
	args := []string{"observe", "--pod", podRef, "--last", fmt.Sprintf("%d", last)}
	return RunHubbleCommand(args...)
}

type defaultHubbleProvider struct{}

func (defaultHubbleProvider) ObservePod(podRef string, last int, context string) (string, error) {
	return RunHubbleObserve(podRef, last, context)
}

func ResolveHubbleProvider(p HubbleProvider) HubbleProvider {
	if p != nil {
		return p
	}
	return defaultHubbleProvider{}
}