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
	result := &kube.Response{}
	out := captureStdout(t, func() {
		printGraphs(result, "ctx-a", "ns-a", "mock", []string{"*/ns-a/*"})
	})

	var parsed kube.Response
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("expected valid JSON output, got err: %v\noutput:\n%s", err, out)
	}
	if parsed.APIVersion != "v1" {
		t.Fatalf("expected apiVersion %q, got %q", "v1", parsed.APIVersion)
	}
	if parsed.Request.KeySelector != "*/ns-a/*" {
		t.Fatalf("unexpected request metadata in output: %+v", parsed.Request)
	}
	if parsed.Request.Context != "ctx-a" || parsed.Request.Namespace != "ns-a" || parsed.Request.ConfigPath != "mock" {
		t.Fatalf("unexpected normalized request metadata in output: %+v", parsed.Request)
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
