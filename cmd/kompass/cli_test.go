package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/karloie/kompass/pkg/kube"
)

func TestPrintHelpIncludesDebugFlag(t *testing.T) {
	out := captureStdout(t, func() {
		printHelp()
	})
	if !strings.Contains(out, "--debug") {
		t.Fatalf("expected help output to include --debug, got:\n%s", out)
	}
}

func TestPrintGraphsOutputsValidJSON(t *testing.T) {
	result := &kube.GraphResponse{}
	out := captureStdout(t, func() {
		printGraphs(result, "ctx-a", "ns-a", "mock", []string{"*/ns-a/*"})
	})

	var parsed JSONOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("expected valid JSON output, got err: %v\noutput:\n%s", err, out)
	}
	if parsed.Request.Context != "ctx-a" || parsed.Request.Namespace != "ns-a" || parsed.Request.ConfigPath != "mock" {
		t.Fatalf("unexpected request metadata in output: %+v", parsed.Request)
	}
}

func TestGetCacheStatsDisabledOrEmpty(t *testing.T) {
	if cs := getStats(nil); cs != nil {
		t.Fatalf("expected nil cache stats for nil input")
	}
	if cs := getStats(map[string]interface{}{"enabled": false}); cs != nil {
		t.Fatalf("expected nil cache stats when disabled")
	}
	if cs := getStats(map[string]interface{}{"enabled": true, "calls": int64(0)}); cs != nil {
		t.Fatalf("expected nil cache stats when calls is zero")
	}
}

func TestGetCacheStatsValid(t *testing.T) {
	stats := map[string]interface{}{
		"enabled": true,
		"calls":   int64(10),
		"hits":    int64(7),
		"misses":  int64(3),
		"hitRate": 70.0,
	}
	cs := getStats(stats)
	if cs == nil {
		t.Fatal("expected non-nil cache stats")
	}
	if cs.Calls != 10 || cs.Hits != 7 || cs.Misses != 3 || cs.HitRate != 70.0 {
		t.Fatalf("unexpected cache stats: %+v", cs)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = orig
	b, _ := io.ReadAll(r)
	_ = r.Close()
	return string(b)
}
