package tree

import (
	"strings"
	"testing"
	"testing/fstest"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestRenderAppHTML_IncludesBootstrapScripts(t *testing.T) {
	root := fstest.MapFS{
		"index.html": {Data: []byte("<html><body><div id=\"app\"></div></body></html>")},
	}
	result := &kube.Response{APIVersion: "v1"}

	html := RenderAppHTML(root, result, HTMLBootstrapConfig{Mode: "dynamic", APIBase: "/api/tree"})

	if !strings.Contains(html, `id="kompass-config"`) {
		t.Fatalf("expected kompass-config script in output")
	}
	if !strings.Contains(html, `id="kompass-data"`) {
		t.Fatalf("expected kompass-data script in output")
	}
	if !strings.Contains(html, `"mode":"dynamic"`) {
		t.Fatalf("expected dynamic mode in config payload")
	}
	if !strings.Contains(html, `"apiVersion":"v1"`) {
		t.Fatalf("expected payload json in data bootstrap")
	}
}

func TestRenderAppHTML_EscapesScriptBreakingCharacters(t *testing.T) {
	root := fstest.MapFS{
		"index.html": {Data: []byte("<html><body><div id=\"app\"></div></body></html>")},
	}
	result := &kube.Response{Request: kube.Request{Context: `ctx-</script><script>alert(1)</script>`}}

	html := RenderAppHTML(root, result, HTMLBootstrapConfig{Mode: "static", Context: `ctx-</script>`})

	if strings.Contains(html, "</script><script>") {
		t.Fatalf("expected script breakers to be escaped in bootstrap json")
	}
	if !strings.Contains(html, `\u003c/script\u003e`) {
		t.Fatalf("expected html special chars to be escaped in bootstrap json")
	}
}
