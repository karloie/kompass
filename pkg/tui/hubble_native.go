package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var dialHubbleRelay = func(addr string) (*grpc.ClientConn, error) {
	return grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

func observeHubbleNative(podRef string, last int, contextName string) (string, error) {
	_ = contextName
	if last <= 0 {
		last = 100
	}

	conn, err := dialHubbleRelay(hubbleRelayAddress())
	if err != nil {
		return "", err
	}
	defer conn.Close()

	client := observerpb.NewObserverClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), hubbleRelayTimeout())
	defer cancel()

	stream, err := client.GetFlows(ctx, &observerpb.GetFlowsRequest{Number: uint64(last)})
	if err != nil {
		return "", err
	}

	targetNS, targetPod := splitPodRef(podRef)
	lines := make([]string, 0, last)
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if len(lines) > 0 {
				return strings.Join(lines, "\n"), err
			}
			return "", err
		}
		if line, ok := formatNativeHubbleResponse(resp, targetNS, targetPod); ok {
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return fmt.Sprintf("(no hubble flows observed for %s)", podRef), nil
	}
	return strings.Join(lines, "\n"), nil
}

func hubbleRelayAddress() string {
	if addr := strings.TrimSpace(os.Getenv("KOMPASS_HUBBLE_ADDR")); addr != "" {
		return addr
	}
	return "127.0.0.1:4245"
}

func hubbleRelayTimeout() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("KOMPASS_HUBBLE_TIMEOUT")); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
	}
	return 2 * time.Second
}

func splitPodRef(podRef string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(podRef), "/", 2)
	if len(parts) != 2 {
		return "", strings.TrimSpace(podRef)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func formatNativeHubbleResponse(resp *observerpb.GetFlowsResponse, targetNS, targetPod string) (string, bool) {
	if resp == nil {
		return "", false
	}
	if lost := resp.GetLostEvents(); lost != nil {
		return fmt.Sprintf("lost-events count=%d", lost.GetNumEventsLost()), true
	}

	flow := resp.GetFlow()
	if flow == nil || !flowMatchesPod(flow, targetNS, targetPod) {
		return "", false
	}

	ts := "-"
	if t := flow.GetTime(); t != nil {
		ts = t.AsTime().Format("15:04:05")
	}
	verdict := strings.ToUpper(flow.GetVerdict().String())
	if verdict == "" {
		verdict = "UNKNOWN"
	}

	srcIP, dstIP := "", ""
	if ip := flow.GetIP(); ip != nil {
		srcIP = strings.TrimSpace(ip.GetSource())
		dstIP = strings.TrimSpace(ip.GetDestination())
	}
	src := endpointOrIP(flow.GetSource(), srcIP)
	dst := endpointOrIP(flow.GetDestination(), dstIP)

	arrow := "->"
	switch strings.ToUpper(flow.GetTrafficDirection().String()) {
	case "INGRESS":
		arrow = "<-"
	case "EGRESS":
		arrow = "->"
	}

	return fmt.Sprintf("%s %s %s %s %s", ts, verdict, src, arrow, dst), true
}

func flowMatchesPod(flow *flowpb.Flow, targetNS, targetPod string) bool {
	if flow == nil || strings.TrimSpace(targetPod) == "" {
		return false
	}
	match := func(ep *flowpb.Endpoint) bool {
		if ep == nil {
			return false
		}
		pod := strings.TrimSpace(ep.GetPodName())
		ns := strings.TrimSpace(ep.GetNamespace())
		if pod == "" {
			return false
		}
		if targetNS != "" {
			return pod == targetPod && ns == targetNS
		}
		return pod == targetPod
	}
	return match(flow.GetSource()) || match(flow.GetDestination())
}

func endpointOrIP(ep *flowpb.Endpoint, ip string) string {
	if ep != nil {
		ns := strings.TrimSpace(ep.GetNamespace())
		pod := strings.TrimSpace(ep.GetPodName())
		if ns != "" && pod != "" {
			return ns + "/" + pod
		}
		if pod != "" {
			return pod
		}
	}
	if strings.TrimSpace(ip) != "" {
		return strings.TrimSpace(ip)
	}
	return "unknown"
}
