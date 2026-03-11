package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/mock"
)

func TestHandleHealthReadyNoClient(t *testing.T) {
	s := &server{}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	s.handleHealth("text", true)(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rr.Code)
	}
}

func TestHandleHealthReadyWithMockClient(t *testing.T) {
	s := &server{client: kube.NewMockClient(mock.GenerateMock()), namespaceArg: "petshop"}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	s.handleHealth("text", true)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
}

func TestHandleStatsNoClient(t *testing.T) {
	s := &server{}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)

	s.handleStats(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rr.Code)
	}
}

func TestHandleStatsWithClient(t *testing.T) {
	s := &server{client: kube.NewMockClient(mock.GenerateMock())}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)

	s.handleStats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "enabled") {
		t.Fatalf("expected stats JSON body, got %q", rr.Body.String())
	}
}

func TestGetProviderUnknownMock(t *testing.T) {
	s := &server{}
	_, err := s.getProvider("nope", "")
	if err == nil {
		t.Fatalf("expected error for unknown mock provider")
	}
}

func TestGetProviderFromClientFactoryError(t *testing.T) {
	s := &server{clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		return nil, errors.New("factory error")
	}}
	_, err := s.getProvider("", "petshop")
	if err == nil {
		t.Fatalf("expected error from client factory")
	}
}

func TestGetProviderUsesExistingClientAndSetsNamespace(t *testing.T) {
	c := kube.NewMockClient(mock.GenerateMock())
	c.SetNamespace("default")
	s := &server{client: c}

	provider, err := s.getProvider("", "petshop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatalf("expected provider")
	}
	ns, _ := c.GetNamespace()
	if ns != "petshop" {
		t.Fatalf("expected namespace to be updated to petshop, got %q", ns)
	}
}

func TestHandleGraphSuccess(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/graph?selector=*/petshop/*&mock=mock", nil)

	s.handleGraph(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	var out JSONOutput
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("expected JSON output, got err: %v body=%q", err, rr.Body.String())
	}
	if out.Response == nil {
		t.Fatalf("expected non-nil response")
	}
}

func TestHandleGraphProviderError(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		return nil, errors.New("provider failure")
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/graph", nil)

	s.handleGraph(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleTreeSuccess(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tree?selector=*/petshop/*", nil)

	s.handleTree(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Context:") {
		t.Fatalf("expected tree output to include context line, got %q", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "🫛") {
		t.Fatalf("expected default /tree output to keep emojis in plain mode, got %q", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "\x1b[") {
		t.Fatalf("expected default /tree output without ANSI color escapes, got %q", rr.Body.String())
	}
}

func TestHandleTreePlainQueryOverridesDefault(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tree?selector=*/petshop/*&plain=1", nil)

	s.handleTree(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "🫛") {
		t.Fatalf("expected plain tree output to keep emojis, got %q", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "\x1b[") {
		t.Fatalf("expected plain tree output without ANSI color escapes, got %q", rr.Body.String())
	}
}

func TestHandleTreeRichQueryOverridesDefault(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tree?selector=*/petshop/*&plain=false", nil)

	s.handleTree(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "🫛") {
		t.Fatalf("expected rich tree output to keep emojis, got %q", rr.Body.String())
	}
}

func TestHandleTreeProviderError(t *testing.T) {
	s := &server{clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		return nil, errors.New("provider failure")
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tree", nil)

	s.handleTree(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 with plain error message body, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Failed to connect to cluster") {
		t.Fatalf("expected error text in body, got %q", rr.Body.String())
	}
}
