package tree

import (
	"strings"
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestRenderTextDoesNotExpandClusterwideCiliumPolicy(t *testing.T) {
	result := &kube.Response{
		Trees: []kube.Tree{{
			Key:  "deployment/ns/app",
			Type: "deployment",
			Meta: map[string]any{"name": "app", "namespace": "ns"},
			Children: []*kube.Tree{{
				Key:  "spec/ns/app",
				Type: "spec",
				Meta: map[string]any{},
				Children: []*kube.Tree{{
					Key:  "ciliumclusterwidenetworkpolicy/allow-ingress",
					Type: "ciliumclusterwidenetworkpolicy",
					Meta: map[string]any{"name": "allow-ingress"},
					Children: []*kube.Tree{{
						Key:  "pod/ns/example",
						Type: "pod",
						Meta: map[string]any{"name": "example", "namespace": "ns"},
					}},
				}},
			}},
		}},
	}

	out := RenderText(result, "header", true)
	if !strings.Contains(out, "ciliumclusterwidenetworkpolicy allow-ingress") {
		t.Fatalf("expected policy node in output, got:\n%s", out)
	}
	if strings.Contains(out, "pod example") {
		t.Fatalf("expected policy children to stay collapsed in text output, got:\n%s", out)
	}
}
