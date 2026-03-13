package graph

import kube "github.com/karloie/kompass/pkg/kube"

func (m M) Exists(key string) bool {
	if m == nil {
		return false
	}
	_, exists := m[key]
	return exists
}

func (m M) BoolOr(key string, defaultVal bool) bool {
	if m == nil {
		return defaultVal
	}
	if b, ok := m[key].(bool); ok {
		return b
	}
	return defaultVal
}

func ResourceSpec(r *kube.Resource) M {
	return M(r.AsMap()).Map("spec")
}

func ResourceKey(r *kube.Resource, resType string) string {
	meta := ResourceMeta(r)
	return Key(resType, meta.String("namespace"), meta.String("name"))
}
