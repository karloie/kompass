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

func TestGetProviderUsesExistingClientWithoutMutatingNamespace(t *testing.T) {
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
	if ns != "default" {
		t.Fatalf("expected namespace to remain default, got %q", ns)
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
	var out JSONOutputGraph
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("expected JSON output, got err: %v body=%q", err, rr.Body.String())
	}
	if out.Response == nil {
		t.Fatalf("expected non-nil response")
	}
	if out.APIVersion != jsonAPIVersion {
		t.Fatalf("expected apiVersion %q, got %q", jsonAPIVersion, out.APIVersion)
	}
	if out.Request.ConfigPath != "" {
		t.Fatalf("expected configPath to be omitted from server /graph response, got %q", out.Request.ConfigPath)
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
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json content-type, got %q", ct)
	}
	var out JSONOutputTree
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("expected JSON output from /tree, got err: %v body=%q", err, rr.Body.String())
	}
	if out.Response == nil || len(out.Response.Trees) == 0 {
		t.Fatalf("expected non-empty tree response")
	}
	for _, g := range out.Response.Trees {
		if g == nil {
			t.Fatalf("expected tree in /tree response graph")
		}
		assertTreeIcons(t, g)
	}
}

func assertTreeIcons(t *testing.T, node *kube.Tree) {
	t.Helper()
	if node == nil {
		return
	}
	if node.Icon == "" {
		t.Fatalf("expected icon for node %q (%s)", node.Key, node.Type)
	}
	for _, child := range node.Children {
		assertTreeIcons(t, child)
	}
}

func TestHandleTreeTextDefaultPlainRendering(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tree/text?selector=*/petshop/*", nil)

	s.handleTreeText(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Context:") {
		t.Fatalf("expected tree text output to include context line, got %q", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "🫛") {
		t.Fatalf("expected default /tree/text output to keep emojis, got %q", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "\x1b[") {
		t.Fatalf("expected default /tree/text output without ANSI color escapes, got %q", rr.Body.String())
	}
}

func TestHandleTreeTextRichQuery(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tree/text?selector=*/petshop/*&plain=false", nil)

	s.handleTreeText(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "🫛") {
		t.Fatalf("expected rich /tree/text output to keep emojis, got %q", rr.Body.String())
	}
}

func TestHandleTreeProviderError(t *testing.T) {
	s := &server{clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		return nil, errors.New("provider failure")
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tree", nil)

	s.handleTree(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 from /tree JSON, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "provider failure") {
		t.Fatalf("expected error text in body, got %q", rr.Body.String())
	}
}
