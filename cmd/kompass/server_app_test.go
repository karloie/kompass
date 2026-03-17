package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/karloie/kompass/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const appTestPodKey = "pod/petshop/petshop-tennant-5689f8488b-tr5ft"
const appTestPodName = "petshop-tennant-5689f8488b-tr5ft"
const appTestCertificateKey = "certificate/petshop/petshop-cert"

func TestHandleAppYAML(t *testing.T) {
	s := newAppTestServer()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/app/yaml?key="+appTestPodKey+"&context=mock-01&namespace=petshop", nil)

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
	req := httptest.NewRequest(http.MethodGet, "/api/app/logs?key="+appTestPodKey+"&context=mock-01&namespace=petshop", nil)

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
	req := httptest.NewRequest(http.MethodGet, "/api/app/events?key="+appTestPodKey+"&context=mock-01&namespace=petshop", nil)

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

func TestHandleAppEvents_LimitsToNewest100(t *testing.T) {
	model := kube.NewModel()
	model.Pods = append(model.Pods, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "petshop",
			Name:      appTestPodName,
			UID:       "pod-uid",
		},
	})
	base := time.Unix(1700000000, 0).UTC()
	for i := 1; i <= 130; i++ {
		model.Events = append(model.Events, &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{Namespace: "petshop", Name: fmt.Sprintf("petshop-tennant.%d", i)},
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Pod",
				Name:      appTestPodName,
				Namespace: "petshop",
				UID:       "pod-uid",
			},
			Reason:        "Started",
			Message:       fmt.Sprintf("event-%d", i),
			Type:          "Normal",
			LastTimestamp: metav1.NewTime(base.Add(time.Duration(i) * time.Second)),
		})
	}

	s := &server{clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		client := kube.NewMockClient(model)
		client.SetNamespace(namespace)
		return client, nil
	}}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/app/events?key="+appTestPodKey+"&context=mock-01&namespace=petshop", nil)

	s.handleAppEvents(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	var out appViewResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("expected JSON app response, got err: %v body=%q", err, rr.Body.String())
	}
	lines := strings.Split(strings.TrimSpace(out.Content), "\n")
	if len(lines) != 100 {
		t.Fatalf("expected 100 events, got %d", len(lines))
	}
	firstLine := lines[0]
	lastLine := lines[len(lines)-1]
	if !strings.Contains(firstLine, "event-31") || !strings.Contains(lastLine, "event-130") {
		t.Fatalf("expected newest window to include event-31..event-130, got first=%q last=%q", firstLine, lastLine)
	}
}

func TestHandleAppHubbleCombinesNetpolAndHubble(t *testing.T) {
	s := newAppTestServer()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/app/hubble?key="+appTestPodKey+"&context=mock-01&namespace=petshop", nil)

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
	if !strings.Contains(out.Content, "Captured mock flows for petshop/"+appTestPodName) {
		t.Fatalf("expected combined hubble view to include hubble output, got %q", out.Content)
	}
	if !strings.Contains(out.Content, "⛔") || !strings.Contains(out.Content, "⚠️") || !strings.Contains(out.Content, "✅") {
		t.Fatalf("expected hubble lines to include ⛔ ⚠️ ✅ emojis, got %q", out.Content)
	}
	persistentIdx := strings.Index(out.Content, "⛔")
	resolvedIdx := strings.Index(out.Content, "⚠️")
	allowIdx := strings.Index(out.Content, "✅")
	if persistentIdx > resolvedIdx || resolvedIdx > allowIdx {
		t.Fatalf("expected persistent-deny ⛔ before resolved-deny ⚠️ before allow ✅, got %q", out.Content)
	}
	if out.Title != "Cilium" {
		t.Fatalf("expected Cilium title, got %q", out.Title)
	}
}

func TestHandleAppCert(t *testing.T) {
	s := newAppTestServer()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/app/cert?key="+appTestCertificateKey+"&context=mock-01&namespace=petshop", nil)

	s.handleAppCert(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	var out appViewResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("expected JSON app response, got err: %v body=%q", err, rr.Body.String())
	}
	if out.Title != "Cert" {
		t.Fatalf("expected Cert title, got %q", out.Title)
	}
	if !strings.Contains(out.Content, "Certificate Analysis") {
		t.Fatalf("expected cert view to include analysis header, got %q", out.Content)
	}
	if !strings.Contains(out.Content, "Certificate:") {
		t.Fatalf("expected cert view to include certificate section, got %q", out.Content)
	}
	if !strings.Contains(out.Content, "Issuer Certificate:") {
		t.Fatalf("expected cert view to include issuer certificate section, got %q", out.Content)
	}
	if !strings.Contains(out.Content, "DNS Names: petshop.example.com") {
		t.Fatalf("expected cert view to include parsed SAN info, got %q", out.Content)
	}
}

func TestHandleAppCertRejectsNonCertificate(t *testing.T) {
	s := newAppTestServer()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/app/cert?key="+appTestPodKey+"&context=mock-01&namespace=petshop", nil)

	s.handleAppCert(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "cert view is only available for certificates") {
		t.Fatalf("expected cert-only validation error, got %q", rr.Body.String())
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
	model.Certificates = append(model.Certificates, map[string]any{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Certificate",
		"metadata": map[string]any{
			"namespace": "petshop",
			"name":      "petshop-cert",
		},
		"spec": map[string]any{
			"secretName": "petshop-cert",
			"issuerRef": map[string]any{
				"kind": "ClusterIssuer",
				"name": "letsencrypt-prod",
			},
		},
	})

	tlsCRT, err := generateTestCertificateChainPEM()
	if err != nil {
		panic(err)
	}
	model.Secrets = append(model.Secrets, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "petshop",
			Name:      "petshop-cert",
		},
		Data: map[string][]byte{
			"tls.crt": tlsCRT,
		},
	})

	return &server{clientFactory: func(contextArg, namespace string) (kube.Kube, error) {
		client := kube.NewMockClient(model)
		client.SetNamespace(namespace)
		return client, nil
	}}
}

func generateTestCertificateChainPEM() ([]byte, error) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1001),
		Subject:               pkix.Name{CommonName: "Test Root CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, err
	}

	leafTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2002),
		Subject:               pkix.Name{CommonName: "petshop.example.com"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(12 * time.Hour),
		DNSNames:              []string{"petshop.example.com"},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}

	leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	return append(leafPEM, caPEM...), nil
}
