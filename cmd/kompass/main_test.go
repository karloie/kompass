package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestServiceFlagDefaultsWithBareFlag(t *testing.T) {
	fs := flag.NewFlagSet("svc", flag.ContinueOnError)
	svc := &serviceFlag{}
	fs.Var(svc, "service", "")

	if err := fs.Parse([]string{"--service"}); err != nil {
		t.Fatalf("expected bare --service to parse, got err: %v", err)
	}
	if !svc.set {
		t.Fatalf("expected service flag to be set")
	}
	if svc.addr != "localhost:8080" {
		t.Fatalf("expected default addr localhost:8080, got %q", svc.addr)
	}
}

func TestServiceFlagAcceptsExplicitAddress(t *testing.T) {
	fs := flag.NewFlagSet("svc", flag.ContinueOnError)
	svc := &serviceFlag{}
	fs.Var(svc, "service", "")

	if err := fs.Parse([]string{"--service=:19090"}); err != nil {
		t.Fatalf("expected --service with address to parse, got err: %v", err)
	}
	if !svc.set {
		t.Fatalf("expected service flag to be set")
	}
	if svc.addr != ":19090" {
		t.Fatalf("expected addr :19090, got %q", svc.addr)
	}
}

func TestNormalizeServiceArgsSupportsSeparateAddressToken(t *testing.T) {
	args := normalizeServiceArgs([]string{"--service", ":19090", "--mock"})
	if len(args) < 2 {
		t.Fatalf("unexpected normalized args: %#v", args)
	}
	if args[0] != "--service=:19090" {
		t.Fatalf("expected first arg to be --service=:19090, got %q", args[0])
	}
	if args[1] != "--mock" {
		t.Fatalf("expected second arg to remain --mock, got %q", args[1])
	}
}

func TestNormalizeServiceArgsSupportsShortFlagWithSeparateAddressToken(t *testing.T) {
	args := normalizeServiceArgs([]string{"-s", ":19090", "--mock"})
	if len(args) < 2 {
		t.Fatalf("unexpected normalized args: %#v", args)
	}
	if args[0] != "--service=:19090" {
		t.Fatalf("expected first arg to be --service=:19090, got %q", args[0])
	}
	if args[1] != "--mock" {
		t.Fatalf("expected second arg to remain --mock, got %q", args[1])
	}
}

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := []string{}
	for i, a := range os.Args {
		if a == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	os.Args = append([]string{"kompass"}, args...)
	main()
	os.Exit(0)
}

func TestMainCLIPathRuns(t *testing.T) {
	res := runHelper(t, 20*time.Second, "--mock", "--output", "plain")
	if res.err != nil {
		t.Fatalf("expected CLI run to succeed, got err: %v\noutput:\n%s", res.err, res.output)
	}
	if !strings.Contains(res.output, "Context:") {
		t.Fatalf("expected CLI output to include context header, got:\n%s", res.output)
	}
}

func TestMainDebugFlagHonored(t *testing.T) {
	res := runHelper(t, 20*time.Second, "--mock", "--debug", "--output", "plain")
	if res.err != nil {
		t.Fatalf("expected debug CLI run to succeed, got err: %v\noutput:\n%s", res.err, res.output)
	}
	if !strings.Contains(res.output, `"level":"DEBUG"`) {
		t.Fatalf("expected debug logs in output when --debug is used, got:\n%s", res.output)
	}
}

func TestMainFormatJSONOverridesServiceMode(t *testing.T) {
	res := runHelper(t, 20*time.Second, "--mock", "--service", "--output", "json")
	if res.err != nil {
		t.Fatalf("expected --output json to run one-shot output mode, got err: %v\noutput:\n%s", res.err, res.output)
	}
	if !strings.Contains(res.output, `"apiVersion":"v1"`) {
		t.Fatalf("expected JSON output, got:\n%s", res.output)
	}
}

func TestMainFormatHTMLPrintsDocument(t *testing.T) {
	res := runHelper(t, 20*time.Second, "--mock", "--output", "html")
	if res.err != nil {
		t.Fatalf("expected --output html to succeed, got err: %v\noutput:\n%s", res.err, res.output)
	}
	if !strings.Contains(res.output, "<html") {
		t.Fatalf("expected HTML output, got:\n%s", res.output)
	}
}

func TestResolveExecutionMode(t *testing.T) {
	tests := []struct {
		name        string
		service     bool
		tui         bool
		format      outputFormat
		interactive bool
		want        executionMode
	}{
		{name: "cli", service: false, tui: false, format: outputFormatUnset, interactive: false, want: modeCLI},
		{name: "service", service: true, tui: false, format: outputFormatUnset, interactive: false, want: modeService},
		{name: "tui selector explicit", service: false, tui: true, format: outputFormatUnset, interactive: false, want: modeTUISelector},
		{name: "tui selector interactive default", service: false, tui: false, format: outputFormatUnset, interactive: true, want: modeTUISelector},
		{name: "service and tui", service: true, tui: true, format: outputFormatUnset, interactive: false, want: modeServiceAndTUI},
		{name: "output overrides service", service: true, tui: false, format: outputFormatJSON, interactive: true, want: modeCLI},
		{name: "output overrides service and tui", service: true, tui: true, format: outputFormatJSON, interactive: false, want: modeCLI},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveExecutionMode(tc.service, tc.tui, tc.format, tc.interactive)
			if got != tc.want {
				t.Fatalf("resolveExecutionMode(service=%t, tui=%t, format=%v, interactive=%t)=%v, want %v", tc.service, tc.tui, tc.format, tc.interactive, got, tc.want)
			}
		})
	}
}

func TestResolveOutputFormat(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    outputFormat
		wantErr bool
	}{
		{name: "empty", raw: "", want: outputFormatUnset},
		{name: "json", raw: "json", want: outputFormatJSON},
		{name: "text", raw: "text", want: outputFormatText},
		{name: "plain", raw: "plain", want: outputFormatPlain},
		{name: "html", raw: "html", want: outputFormatHTML},
		{name: "invalid", raw: "xml", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveOutputFormat(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if got != tc.want {
				t.Fatalf("resolveOutputFormat(%q)=%v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestMainServiceStartsServer(t *testing.T) {
	port := freePort(t)
	res, cmd, cancel := runHelperBackground(t, "--mock", "--debug", "--service", fmt.Sprintf(":%d", port))
	defer cancel()

	url := fmt.Sprintf("http://127.0.0.1:%d/api/healthz", port)
	var (
		httpRes *http.Response
		err     error
	)
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		httpRes, err = http.Get(url)
		if err == nil {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("expected server to be reachable at %s: %v\noutput:\n%s", url, err, readOutput(res))
	}
	defer httpRes.Body.Close()
	body, _ := io.ReadAll(httpRes.Body)
	if httpRes.StatusCode != http.StatusOK {
		t.Fatalf("expected /healthz status 200, got %d body=%q\noutput:\n%s", httpRes.StatusCode, string(body), readOutput(res))
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)
	_, _ = cmd.Process.Wait()
}

type helperResult struct {
	outputBuf *bytes.Buffer
	output    string
	err       error
}

func runHelper(t *testing.T, timeout time.Duration, appArgs ...string) helperResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := append([]string{"-test.run=TestMainHelperProcess", "--"}, appArgs...)
	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return helperResult{output: string(out), err: fmt.Errorf("timeout waiting for helper process")}
	}
	return helperResult{output: string(out), err: err}
}

func runHelperBackground(t *testing.T, appArgs ...string) (*helperResult, *exec.Cmd, func()) {
	t.Helper()
	args := append([]string{"-test.run=TestMainHelperProcess", "--"}, appArgs...)
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = buf

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}

	res := &helperResult{outputBuf: buf}
	cancel := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			_, _ = cmd.Process.Wait()
		}
	}
	return res, cmd, cancel
}

func readOutput(res *helperResult) string {
	if res == nil {
		return ""
	}
	if res.outputBuf != nil {
		res.output = res.outputBuf.String()
	}
	return res.output
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate free port: %v", err)
	}
	defer ln.Close()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected listener address type: %T", ln.Addr())
	}
	return addr.Port
}
