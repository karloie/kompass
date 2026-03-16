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
	"github.com/karloie/kompass/pkg/pipeline"
	"github.com/karloie/kompass/pkg/tree"
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

func TestHandleMetadataNoClient(t *testing.T) {
	s := &server{}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/metadata", nil)

	s.handleMetadata(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rr.Code)
	}
}

func TestHandleMetadataWithClient(t *testing.T) {
	s := &server{client: kube.NewMockClient(mock.GenerateMock())}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/metadata", nil)

	s.handleMetadata(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "cacheEnabled") {
		t.Fatalf("expected metadata JSON body, got %q", rr.Body.String())
	}
}

func TestGetProviderMockNotAllowed(t *testing.T) {
	s := &server{}
	_, err := s.getProvider("mock", "")
	if err == nil {
		t.Fatalf("expected error when mock access is not allowed")
	}
}

func TestGetProviderUnknownMock(t *testing.T) {
	s := &server{allowMock: true}
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

func TestGetProviderUsesExistingClientAndUpdatesNamespace(t *testing.T) {
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
		t.Fatalf("expected namespace to update to petshop, got %q", ns)
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
	var out kube.Response
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("expected JSON output, got err: %v body=%q", err, rr.Body.String())
	}
	if len(out.Components) == 0 {
		t.Fatalf("expected non-empty graph response")
	}
	if out.APIVersion != "v1" {
		t.Fatalf("expected apiVersion %q, got %q", "v1", out.APIVersion)
	}
	if len(out.Request.Selectors) != 1 || out.Request.Selectors[0] != "*/petshop/*" {
		t.Fatalf("expected request selectors to round-trip, got %+v", out.Request)
	}
	if out.Request.Context != "mock-cluster" || out.Request.Namespace != "petshop" || out.Request.ConfigPath != "mock" {
		t.Fatalf("expected normalized request metadata, got %+v", out.Request)
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
	var out kube.Response
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("expected JSON output from /tree, got err: %v body=%q", err, rr.Body.String())
	}
	if len(out.Trees) == 0 {
		t.Fatalf("expected non-empty tree response")
	}
	if out.Request.Context != "mock-cluster" || out.Request.Namespace != "petshop" || out.Request.ConfigPath != "mock" {
		t.Fatalf("expected normalized request metadata in tree response, got %+v", out.Request)
	}
	for i := range out.Trees {
		assertTreeIcons(t, &out.Trees[i])
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
	req := httptest.NewRequest(http.MethodGet, "/api/tree?selector=*/petshop/*", nil)
	req.Header.Set("Accept", "text/plain")

	s.handleTree(rr, req)
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

func TestHandleTreeTextIgnoresPlainFalseQuery(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tree?selector=*/petshop/*&plain=false", nil)
	req.Header.Set("Accept", "text/plain")

	s.handleTree(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "🫛") {
		t.Fatalf("expected /tree/text output to keep emojis, got %q", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "\x1b[") {
		t.Fatalf("expected plain /tree/text output even with plain=false query, got %q", rr.Body.String())
	}
}

func TestHandleTreeTextHeaderMatchesCLIPrintTrees(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tree?selector=*/petshop/*&plain=1", nil)
	req.Header.Set("Accept", "text/plain")

	s.handleTree(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}

	serverHeader := strings.SplitN(rr.Body.String(), "\n", 2)[0]

	cliProvider := kube.NewMockClient(mock.GenerateMock())
	cliProvider.SetNamespace("petshop")
	cliGraph, err := pipeline.InferGraphs(cliProvider, []string{"*/petshop/*"})
	if err != nil {
		t.Fatalf("expected cli infer graph success, got err: %v", err)
	}
	cliTree := tree.BuildResponseTree(cliGraph)
	cliOutput := captureStdout(t, func() {
		printTreesText(cliTree, "mock-cluster", "petshop", "mock", []string{"*/petshop/*"}, true)
	})
	cliHeader := strings.SplitN(cliOutput, "\n", 2)[0]

	if serverHeader != cliHeader {
		t.Fatalf("expected server and CLI headers to be byte-identical\nserver: %q\ncli:    %q", serverHeader, cliHeader)
	}

	sharedPrefix := "🌍 " + tree.FormatTreeHeader("mock-cluster", "petshop", "mock", []string{"*/petshop/*"})
	if !strings.HasPrefix(serverHeader, sharedPrefix) {
		t.Fatalf("expected shared formatted header prefix\nwant prefix: %q\nactual:      %q", sharedPrefix, serverHeader)
	}
}

func TestHandleTreeHTML_DefaultShowsNamespaceSelector(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tree?namespace=petshop", nil)
	req.Header.Set("Accept", "text/html")

	s.handleTree(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "id=\"namespace-select\"") {
		t.Fatalf("expected default HTML tree to include namespace selector")
	}
}

func TestHandleTreeHTML_StaticHidesNamespaceSelector(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tree?namespace=petshop&static=1", nil)
	req.Header.Set("Accept", "text/html")

	s.handleTree(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, "id=\"namespace-select\"") {
		t.Fatalf("expected static HTML tree to hide namespace selector")
	}
	if !strings.Contains(body, "id=\"tree-filter\"") {
		t.Fatalf("expected static HTML tree to keep filter input")
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
