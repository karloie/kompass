package tree

import (
	"strings"
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestBuildNodeSearchText_IncludesTypeLabelAndMetadata(t *testing.T) {
	meta := map[string]any{
		"name":   "DB_PASSWORD",
		"source": "secretKeyRef",
		"key":    "PSB-DATABASE-PASSWORD",
		"nested": map[string]any{
			"service": "petshop-db",
		},
		"keys": []any{"alpha", "beta"},
	}

	searchText := buildNodeSearchText("env", "ENV DB_PASSWORD", meta)

	mustContain := []string{
		"env",
		"ENV DB_PASSWORD",
		"name",
		"DB_PASSWORD",
		"source",
		"secretKeyRef",
		"service",
		"petshop-db",
		"alpha",
		"beta",
	}

	for _, want := range mustContain {
		if !strings.Contains(searchText, want) {
			t.Fatalf("expected search text to contain %q, got %q", want, searchText)
		}
	}
}

func TestBuildNodeSearchText_HandlesNilMetadata(t *testing.T) {
	searchText := buildNodeSearchText("mount", "mount /tmp", nil)
	if !strings.Contains(searchText, "mount") {
		t.Fatalf("expected search text to contain node type, got %q", searchText)
	}
	if !strings.Contains(searchText, "mount /tmp") {
		t.Fatalf("expected search text to contain label, got %q", searchText)
	}
}

func TestBuildNodeSearchText_ExcludesNoisyMetadataAndHashes(t *testing.T) {
	meta := map[string]any{
		"uid":                 "123e4567-e89b-12d3-a456-426614174000",
		"resourceVersion":     "987654321",
		"name":                "kafka-runtime-config",
		"image":               "docker-hub/confluentinc/cp-kafka:7.5.0@sha256:abcdef",
		"containerID":         "containerd://d34db33fd34db33fd34db33fd34db33f",
		"secretProviderClass": "petshop-kafka-petshopvault",
	}

	searchText := strings.ToLower(buildNodeSearchText("configmap", "configmap kafka-runtime-config", meta))

	if strings.Contains(searchText, "uid") {
		t.Fatalf("expected noisy key uid to be excluded, got %q", searchText)
	}
	if strings.Contains(searchText, "resourceversion") {
		t.Fatalf("expected noisy key resourceVersion to be excluded, got %q", searchText)
	}
	if strings.Contains(searchText, "sha256:") {
		t.Fatalf("expected sha256 digest token to be excluded, got %q", searchText)
	}
	if !strings.Contains(searchText, "secretproviderclass") {
		t.Fatalf("expected relevant metadata key to remain searchable, got %q", searchText)
	}
	if !strings.Contains(searchText, "petshop-kafka-petshopvault") {
		t.Fatalf("expected relevant metadata value to remain searchable, got %q", searchText)
	}
}

func TestRenderHTML_DoesNotIncludeLiveReloadMetadata(t *testing.T) {
	html := RenderHTML(&kube.Response{}, "ctx", "petshop", "mock", nil, false)
	if strings.Contains(html, `data-live-reload-poll-ms=`) {
		t.Fatalf("expected html to omit live reload poll metadata, got %q", html)
	}
	if strings.Contains(html, `data-process-stamp=`) {
		t.Fatalf("expected html to omit process stamp metadata, got %q", html)
	}
}

func TestRenderHTML_StaticModeDoesNotIncludeLiveReloadMetadata(t *testing.T) {
	html := RenderHTML(&kube.Response{}, "ctx", "petshop", "mock", nil, true)
	if strings.Contains(html, `data-live-reload-poll-ms=`) {
		t.Fatalf("expected static html to omit live reload poll metadata, got %q", html)
	}
	if strings.Contains(html, `data-process-stamp=`) {
		t.Fatalf("expected static html to omit process stamp metadata, got %q", html)
	}
}

func TestShouldUseRuntimeTemplateFiles(t *testing.T) {
	original := BuildMode
	t.Cleanup(func() { BuildMode = original })

	BuildMode = "release"
	if shouldUseRuntimeTemplateFiles() {
		t.Fatalf("expected release mode to disable runtime template files")
	}

	BuildMode = "dev"
	if !shouldUseRuntimeTemplateFiles() {
		t.Fatalf("expected non-release mode to enable runtime template files")
	}
}
