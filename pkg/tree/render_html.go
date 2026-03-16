package tree

import (
	"encoding/json"
	"io/fs"
	"os"
	"strings"

	kube "github.com/karloie/kompass/pkg/kube"
)

func ResolveAppWebRoot() fs.FS {
	if embeddedAppWebRoot != nil {
		return embeddedAppWebRoot
	}

	paths := []string{
		"pkg/tree/dist",
		"dist",
		"../dist",
		"../../pkg/tree/dist",
	}
	for _, p := range paths {
		local := os.DirFS(p)
		if _, err := fs.Stat(local, "index.html"); err == nil {
			return local
		}
	}

	return nil
}

type HTMLBootstrapConfig struct {
	Mode      string `json:"mode"`
	APIBase   string `json:"apiBase"`
	Static    bool   `json:"staticMode"`
	Context   string `json:"context,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

func RenderAppHTML(webRoot fs.FS, payload *kube.Response, cfg HTMLBootstrapConfig) string {
	htmlDoc, ok := loadWebIndexTemplate(webRoot)
	if !ok {
		return "<html><body><p>failed to render tree html</p></body></html>"
	}

	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return "<html><body><p>failed to render tree html</p></body></html>"
	}
	dataJSON, err := json.Marshal(payload)
	if err != nil {
		return "<html><body><p>failed to render tree html</p></body></html>"
	}

	injection := strings.Join([]string{
		"<script id=\"kompass-config\" type=\"application/json\">" + escapeJSONForScript(configJSON) + "</script>",
		"<script id=\"kompass-data\" type=\"application/json\">" + escapeJSONForScript(dataJSON) + "</script>",
	}, "\n")

	if strings.Contains(htmlDoc, "</body>") {
		return strings.Replace(htmlDoc, "</body>", injection+"\n</body>", 1)
	}
	return htmlDoc + "\n" + injection
}

func loadWebIndexTemplate(webRoot fs.FS) (string, bool) {
	if webRoot != nil {
		if content, err := fs.ReadFile(webRoot, "index.html"); err == nil {
			return string(content), true
		}
	}

	paths := []string{
		"pkg/tree/dist/index.html",
		"dist/index.html",
		"../dist/index.html",
		"../../pkg/tree/dist/index.html",
		"web/index.html",
		"../web/index.html",
		"../../web/index.html",
	}
	for _, p := range paths {
		if content, err := os.ReadFile(p); err == nil {
			return string(content), true
		}
	}
	return "", false
}

func escapeJSONForScript(raw []byte) string {
	replacer := strings.NewReplacer(
		"<", "\\u003c",
		">", "\\u003e",
		"&", "\\u0026",
		"\u2028", "\\u2028",
		"\u2029", "\\u2029",
	)
	return replacer.Replace(string(raw))
}
