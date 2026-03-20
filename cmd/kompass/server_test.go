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

func TestInferForRequest_ContextNamespace_NoMockBleedover(t *testing.T) {
	calledContext := ""
	calledNamespace := ""

	s := &server{clientFactory: func(contextArg, namespace string) (kube.Provider, error) {
		calledContext = contextArg
		calledNamespace = namespace
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetContext(contextArg)
		c.SetNamespace(namespace)
		return c, nil
	}}

	req := httptest.NewRequest(http.MethodGet, "/api/tree?selector=secretgen-web&context=tool-test-01&namespace=applikasjonsplattform", nil)
	selectors, namespace, provider, _, err := s.inferForRequest(req)
	if err != nil {
		t.Fatalf("inferForRequest returned error: %v", err)
	}

	if calledContext != "tool-test-01" {
		t.Fatalf("expected provider context tool-test-01, got %q", calledContext)
	}
	if calledNamespace != "applikasjonsplattform" {
		t.Fatalf("expected provider namespace applikasjonsplattform, got %q", calledNamespace)
	}
	if namespace != "applikasjonsplattform" {
		t.Fatalf("expected returned namespace applikasjonsplattform, got %q", namespace)
	}
	if len(selectors) != 1 || selectors[0] != "secretgen-web" {
		t.Fatalf("expected selectors to round-trip, got %#v", selectors)
	}
	if provider == nil {
		t.Fatalf("expected provider instance")
	}
}

func TestInferForRequest_ContextMock01_PassesThroughContext(t *testing.T) {
	calledContext := ""
	calledNamespace := ""

	s := &server{clientFactory: func(contextArg, namespace string) (kube.Provider, error) {
		calledContext = contextArg
		calledNamespace = namespace
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}

	req := httptest.NewRequest(http.MethodGet, "/api/tree?selector=*/petshop/*&context=mock-01&namespace=petshop", nil)
	_, _, _, _, err := s.inferForRequest(req)
	if err != nil {
		t.Fatalf("inferForRequest returned error: %v", err)
	}

	if calledContext != "mock-01" {
		t.Fatalf("expected context mock-01 passed to clientFactory, got %q", calledContext)
	}
	if calledNamespace != "petshop" {
		t.Fatalf("expected provider namespace petshop, got %q", calledNamespace)
	}
}

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
	if strings.Contains(rr.Body.String(), "currentContext") || strings.Contains(rr.Body.String(), "contexts") {
		t.Fatalf("expected metadata payload to exclude scope fields, got %q", rr.Body.String())
	}
}

func TestHandleScopeNoClient(t *testing.T) {
	s := &server{}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/scope", nil)

	s.handleScope(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rr.Code)
	}
}

func TestHandleScopeWithClient(t *testing.T) {
	s := &server{client: kube.NewMockClient(mock.GenerateMock()), contextArg: "mock-01"}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/scope", nil)

	s.handleScope(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "currentContext") || !strings.Contains(rr.Body.String(), "contexts") {
		t.Fatalf("expected scope JSON body, got %q", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "currentNamespace") || !strings.Contains(rr.Body.String(), "namespaces") {
		t.Fatalf("expected scope JSON body to include namespace data, got %q", rr.Body.String())
	}
}

func TestHandleScopeForExplicitContext(t *testing.T) {
	s := &server{client: kube.NewMockClient(mock.GenerateMock()), contextArg: "mock-01"}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/scope?context=mock-01", nil)

	s.handleScope(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "petshop") {
		t.Fatalf("expected scope JSON body to include mock namespaces, got %q", rr.Body.String())
	}
}

func TestGetProviderFromClientFactoryError(t *testing.T) {
	s := &server{clientFactory: func(contextArg, namespace string) (kube.Provider, error) {
		return nil, errors.New("factory error")
	}}
	_, err := s.getProvider("ctx-a", "petshop")
	if err == nil {
		t.Fatalf("expected error from client factory")
	}
}

func TestGetProviderUsesExistingClientAndUpdatesNamespace(t *testing.T) {
	c := kube.NewMockClient(mock.GenerateMock())
	c.SetNamespace("default")
	s := &server{client: c}

	provider, err := s.getProvider("mock-01", "petshop")
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
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Provider, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/graph?selector=*/petshop/*&context=mock-01&namespace=petshop", nil)

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
	if out.Request.Context != "mock-01" || out.Request.Namespace != "petshop" || out.Request.ConfigPath != "mock" {
		t.Fatalf("expected normalized request metadata, got %+v", out.Request)
	}
}

func TestHandleGraphAcceptsPluralSelectorsParam(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Provider, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/graph?selectors=*/petshop/*+OR+*/kafka-system/*&context=mock-01&namespace=petshop", nil)

	s.handleGraph(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	var out kube.Response
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("expected JSON output, got err: %v body=%q", err, rr.Body.String())
	}
	if len(out.Request.Selectors) != 2 {
		t.Fatalf("expected 2 selectors, got %+v", out.Request.Selectors)
	}
	if out.Request.Selectors[0] != "*/petshop/*" || out.Request.Selectors[1] != "*/kafka-system/*" {
		t.Fatalf("expected selectors to split by OR, got %+v", out.Request.Selectors)
	}
}

func TestHandleGraphProviderError(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Provider, error) {
		return nil, errors.New("provider failure")
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/graph?context=mock-01&namespace=petshop", nil)

	s.handleGraph(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func TestHandleGraphMissingScope(t *testing.T) {
	s := &server{}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/graph", nil)

	s.handleGraph(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%q", rr.Code, rr.Body.String())
	}
}

func TestHandleTreeSuccess(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Provider, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tree?selector=*/petshop/*&context=mock-01&namespace=petshop", nil)

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
	if out.Request.Context != "mock-01" || out.Request.Namespace != "petshop" || out.Request.ConfigPath != "mock" {
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
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Provider, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tree?selector=*/petshop/*&context=mock-01&namespace=petshop", nil)
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

func TestHandleTreeTextRichQuery(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Provider, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tree?selector=*/petshop/*&plain=false&context=mock-01&namespace=petshop", nil)
	req.Header.Set("Accept", "text/plain")

	s.handleTree(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "🫛") {
		t.Fatalf("expected rich /tree/text output to keep emojis, got %q", rr.Body.String())
	}
}

func TestHandleTreeHTML_DefaultShowsNamespaceSelector(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Provider, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tree?context=mock-01&namespace=petshop", nil)
	req.Header.Set("Accept", "text/html")

	s.handleTree(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "id=\"app\"") {
		t.Fatalf("expected default HTML tree to include Vue mount root")
	}
	if !strings.Contains(body, "id=\"kompass-config\"") {
		t.Fatalf("expected default HTML tree to include kompass-config bootstrap")
	}
	if !strings.Contains(body, "\"mode\":\"dynamic\"") {
		t.Fatalf("expected default HTML tree to be in dynamic mode bootstrap")
	}
	if !strings.Contains(body, "id=\"kompass-data\"") {
		t.Fatalf("expected default HTML tree to include kompass-data bootstrap")
	}
}

func TestHandleTreeHTML_StaticHidesNamespaceSelector(t *testing.T) {
	s := &server{namespaceArg: "petshop", clientFactory: func(contextArg, namespace string) (kube.Provider, error) {
		c := kube.NewMockClient(mock.GenerateMock())
		c.SetNamespace(namespace)
		return c, nil
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tree?context=mock-01&namespace=petshop&static=1", nil)
	req.Header.Set("Accept", "text/html")

	s.handleTree(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "id=\"kompass-config\"") {
		t.Fatalf("expected static HTML tree to include kompass-config bootstrap")
	}
	if !strings.Contains(body, "\"mode\":\"static\"") {
		t.Fatalf("expected static HTML tree to be in static mode bootstrap")
	}
	if !strings.Contains(body, "id=\"kompass-data\"") {
		t.Fatalf("expected static HTML tree to include kompass-data bootstrap")
	}
}

func TestHandleTreeProviderError(t *testing.T) {
	s := &server{clientFactory: func(contextArg, namespace string) (kube.Provider, error) {
		return nil, errors.New("provider failure")
	}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tree?context=mock-01&namespace=petshop", nil)

	s.handleTree(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 from /tree JSON, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "provider failure") {
		t.Fatalf("expected error text in body, got %q", rr.Body.String())
	}
}
