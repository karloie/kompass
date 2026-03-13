package tree

import (
	"testing"
	"time"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestFormatCertExpiry_AlwaysIncludesExpiresIn(t *testing.T) {
	tests := []struct {
		name          string
		notAfter      string
		wantExpiresIn string
		wantStatus    string
	}{
		{
			name:          "long-lived certificate",
			notAfter:      time.Now().Add(90*24*time.Hour + time.Hour).Format(time.RFC3339),
			wantExpiresIn: "90d",
			wantStatus:    "Expires In 90d",
		},
		{
			name:          "near expiry certificate",
			notAfter:      time.Now().Add(10*24*time.Hour + time.Hour).Format(time.RFC3339),
			wantExpiresIn: "10d",
			wantStatus:    "Expires In 10d",
		},
		{
			name:          "expired certificate",
			notAfter:      time.Now().Add(-5 * 24 * time.Hour).Format(time.RFC3339),
			wantExpiresIn: "-5d",
			wantStatus:    "Expired 5d Ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCertExpiry(tt.notAfter, map[string]any{})
			expiry, ok := got.(map[string]any)
			if !ok {
				t.Fatalf("expected map result, got %#v", got)
			}

			expiresIn, ok := expiry["expiresIn"].(string)
			if !ok {
				t.Fatalf("expected expiresIn to always be set, got %#v", expiry)
			}
			if expiresIn != tt.wantExpiresIn {
				t.Fatalf("expected expiresIn=%q, got %q", tt.wantExpiresIn, expiresIn)
			}

			status, _ := expiry["status"].(string)
			if status != tt.wantStatus {
				t.Fatalf("expected status=%q, got map=%#v", tt.wantStatus, expiry)
			}
		})
	}
}

func TestFormatCertExpiry_NotReadyWithoutExpiryStatusStillGetsNotReady(t *testing.T) {
	notAfter := time.Now().Add(90*24*time.Hour + time.Hour).Format(time.RFC3339)
	fullResource := map[string]any{
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "False"},
			},
		},
	}

	got := formatCertExpiry(notAfter, fullResource)
	expiry, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %#v", got)
	}

	if expiresIn, _ := expiry["expiresIn"].(string); expiresIn != "90d" {
		t.Fatalf("expected expiresIn=90d, got %#v", expiry)
	}
	if status, _ := expiry["status"].(string); status != "NotReady, Expires In 90d" {
		t.Fatalf("expected status=NotReady, Expires In 90d, got %#v", expiry)
	}
}

func TestFormatIssuerStatus(t *testing.T) {
	tests := []struct {
		name       string
		conditions []any
		want       any
	}{
		{
			name:       "ready true",
			conditions: []any{map[string]any{"type": "Ready", "status": "True"}},
			want:       "Ready",
		},
		{
			name:       "ready false",
			conditions: []any{map[string]any{"type": "Ready", "status": "False"}},
			want:       "NotReady",
		},
		{
			name:       "ready unknown",
			conditions: []any{map[string]any{"type": "Ready", "status": "Unknown"}},
			want:       "Unknown",
		},
		{
			name:       "no ready condition",
			conditions: []any{map[string]any{"type": "Synced", "status": "True"}},
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatIssuerStatus(tt.conditions)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestFormatIssuerReadyReason(t *testing.T) {
	conditions := []any{
		map[string]any{"type": "Ready", "status": "False", "reason": "ErrInitIssuer"},
	}

	got := formatIssuerReadyReason(conditions)
	if got != "ErrInitIssuer" {
		t.Fatalf("expected ErrInitIssuer, got %v", got)
	}
}

func TestFormatIssuerType(t *testing.T) {
	tests := []struct {
		name string
		spec map[string]any
		want any
	}{
		{name: "acme", spec: map[string]any{"acme": map[string]any{}}, want: "acme"},
		{name: "ca", spec: map[string]any{"ca": map[string]any{}}, want: "ca"},
		{name: "vault", spec: map[string]any{"vault": map[string]any{}}, want: "vault"},
		{name: "self signed", spec: map[string]any{"selfSigned": map[string]any{}}, want: "self-signed"},
		{name: "venafi", spec: map[string]any{"venafi": map[string]any{}}, want: "venafi"},
		{name: "unknown", spec: map[string]any{"other": map[string]any{}}, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatIssuerType(tt.spec)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestApplyMetadataRules_SecretProviderClass(t *testing.T) {
	resource := kube.Resource{
		Type: "secretproviderclass",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "ad-explore-db-petshopvault", "namespace": "test"},
			"spec": map[string]any{
				"provider": "azure",
				"secretObjects": []any{
					map[string]any{"secretName": "ad-explore-db-secrets"},
					map[string]any{"secretName": "ad-explore-tls"},
				},
			},
		},
	}

	meta := ApplyMetadataRules(resource, nil)
	if got, _ := meta["name"].(string); got != "ad-explore-db-petshopvault" {
		t.Fatalf("expected name metadata, got %#v", meta)
	}
	if got, _ := meta["namespace"].(string); got != "test" {
		t.Fatalf("expected namespace metadata, got %#v", meta)
	}
	if got, _ := meta["provider"].(string); got != "azure" {
		t.Fatalf("expected provider metadata, got %#v", meta)
	}
	if got, _ := meta["secretObjects"].(int); got != 2 {
		t.Fatalf("expected secretObjects=2, got %#v", meta)
	}
}
