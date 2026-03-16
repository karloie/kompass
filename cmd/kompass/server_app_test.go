package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/karloie/kompass/pkg/diagnostics"
	"github.com/karloie/kompass/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const appTestPodKey = "pod/petshop/petshop-tennant-5689f8488b-tr5ft"
const appTestPodName = "petshop-tennant-5689f8488b-tr5ft"

func TestHandleAppYAML(t *testing.T) {
	s := newAppTestServer()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/app/yaml?key="+appTestPodKey, nil)

	s.handleAppYAML(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	var out appViewResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("expected JSON app response, got err: %v body=%q", err, rr.Body.String())
	}
	if !strings.Contains(out.Content, "name: "+appTestPodName) || !strings.Contains(out.Content, "namespace: petshop") {
		t.Fatalf("expected YAML body, got %q", out.Content)
	}
	if out.Title != "YAML" {
		t.Fatalf("expected YAML title, got %q", out.Title)
	}
}

func TestHandleAppLogs(t *testing.T) {
	s := newAppTestServer()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/app/logs?key="+appTestPodKey, nil)

	s.handleAppLogs(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	var out appViewResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("expected JSON app response, got err: %v body=%q", err, rr.Body.String())
	}
	if out.Content != "log line" {
		t.Fatalf("expected pod logs, got %q", out.Content)
	}
}

func TestHandleAppEvents(t *testing.T) {
	s := newAppTestServer()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/app/events?key="+appTestPodKey, nil)

	s.handleAppEvents(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	var out appViewResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("expected JSON app response, got err: %v body=%q", err, rr.Body.String())
	}
	if !strings.Contains(out.Content, "Created container "+appTestPodName) {
		t.Fatalf("expected filtered event content, got %q", out.Content)
	}
}

func TestHandleAppHubbleCombinesNetpolAndHubble(t *testing.T) {
	prev := diagnostics.RunHubbleObserve
	diagnostics.RunHubbleObserve = func(podRef string, last int, context string) (string, error) {
		return "flow line", nil
	}
	defer func() {
		diagnostics.RunHubbleObserve = prev
	}()

	s := newAppTestServer()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/app/hubble?key="+appTestPodKey, nil)

	s.handleAppHubble(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	var out appViewResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("expected JSON app response, got err: %v body=%q", err, rr.Body.String())
	}
	if !strings.Contains(out.Content, "NetworkPolicy") {
		t.Fatalf("expected combined hubble view to include NetworkPolicy section, got %q", out.Content)
	}
	if !strings.Contains(out.Content, "flow line") {
		t.Fatalf("expected combined hubble view to include hubble output, got %q", out.Content)
	}
	if out.Title != "Hubble" {
		t.Fatalf("expected Hubble title, got %q", out.Title)
	}
}

func newAppTestServer() *server {
	model := kube.NewModel()
	model.Pods = append(model.Pods, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "petshop",
			Name:      appTestPodName,
			Labels:    map[string]string{"app.kubernetes.io/name": "petshop-tennant", "pod-template-hash": "5689f8488b"},
			UID:       "pod-uid",
		},
	})
	model.Events = append(model.Events, &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Namespace: "petshop", Name: "petshop-tennant.123"},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      appTestPodName,
			Namespace: "petshop",
			UID:       "pod-uid",
		},
		Reason:  "Started",
		Message: "Created container " + appTestPodName,
		Type:    "Normal",
	})
	model.PodLogs["petshop/"+appTestPodName] = "log line"

	return &server{clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		client := kube.NewMockClient(model)
		client.SetNamespace(namespace)
		return client, nil
	}}
}