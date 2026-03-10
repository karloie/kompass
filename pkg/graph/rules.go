package graph

import (
	"strings"

	kube "github.com/karloie/kompass/pkg/kube"
	"github.com/ohler55/ojg/jp"
)

type EdgeRule struct {
	SourceType string
	TargetType string
	Label      string
	Paths      []*jp.Expr
	KeyFormat  string
}

type PathRule struct {
	SourceType string
	TargetType string
	Label      string
	Paths      []string
	KeyFormat  string
}

func CompileRule(pr PathRule) EdgeRule {
	exprs := make([]*jp.Expr, 0, len(pr.Paths))
	for _, path := range pr.Paths {
		expr := jp.MustParseString(path)
		exprs = append(exprs, &expr)
	}
	return EdgeRule{
		SourceType: pr.SourceType,
		TargetType: pr.TargetType,
		Label:      pr.Label,
		Paths:      exprs,
		KeyFormat:  pr.KeyFormat,
	}
}

var EdgeRules []EdgeRule

func init() {

	pathRules := []PathRule{

		{
			SourceType: "pod",
			TargetType: "secret",
			Label:      "uses",
			Paths: []string{
				"$.spec.containers[*].env[*].valueFrom.secretKeyRef.name",
				"$.spec.initContainers[*].env[*].valueFrom.secretKeyRef.name",
				"$.spec.ephemeralContainers[*].env[*].valueFrom.secretKeyRef.name",
				"$.spec.containers[*].envFrom[*].secretRef.name",
				"$.spec.initContainers[*].envFrom[*].secretRef.name",
				"$.spec.ephemeralContainers[*].envFrom[*].secretRef.name",
				"$.spec.volumes[*].secret.secretName",
			},
			KeyFormat: "secret/{namespace}/{name}",
		},
		{
			SourceType: "pod",
			TargetType: "secret",
			Label:      "pulls-using",
			Paths: []string{
				"$.spec.imagePullSecrets[*].name",
			},
			KeyFormat: "secret/{namespace}/{name}",
		},

		{
			SourceType: "pod",
			TargetType: "configmap",
			Label:      "uses",
			Paths: []string{
				"$.spec.containers[*].env[*].valueFrom.configMapKeyRef.name",
				"$.spec.initContainers[*].env[*].valueFrom.configMapKeyRef.name",
				"$.spec.ephemeralContainers[*].env[*].valueFrom.configMapKeyRef.name",
				"$.spec.containers[*].envFrom[*].configMapRef.name",
				"$.spec.initContainers[*].envFrom[*].configMapRef.name",
				"$.spec.ephemeralContainers[*].envFrom[*].configMapRef.name",
				"$.spec.volumes[*].configMap.name",
			},
			KeyFormat: "configmap/{namespace}/{name}",
		},

		{
			SourceType: "pod",
			TargetType: "serviceaccount",
			Label:      "uses",
			Paths: []string{
				"$.spec.serviceAccountName",
				"$.spec.serviceAccount",
			},
			KeyFormat: "serviceaccount/{namespace}/{name}",
		},

		{
			SourceType: "pod",
			TargetType: "persistentvolumeclaim",
			Label:      "mounts",
			Paths: []string{
				"$.spec.volumes[*].persistentVolumeClaim.claimName",
			},
			KeyFormat: "persistentvolumeclaim/{namespace}/{name}",
		},

		{
			SourceType: "persistentvolumeclaim",
			TargetType: "persistentvolume",
			Label:      "bound-to",
			Paths: []string{
				"$.spec.volumeName",
			},
			KeyFormat: "persistentvolume/{name}",
		},

		{
			SourceType: "persistentvolume",
			TargetType: "storageclass",
			Label:      "uses",
			Paths: []string{
				"$.spec.storageClassName",
			},
			KeyFormat: "storageclass/{name}",
		},

		{
			SourceType: "issuer",
			TargetType: "secret",
			Label:      "uses",
			Paths: []string{
				"$.spec.acme.privateKeySecretRef.name",
				"$.spec.vault.auth.kubernetes.secretRef.name",
				"$.spec.vault.auth.appRole.secretRef.name",
			},
			KeyFormat: "secret/{namespace}/{name}",
		},

		{
			SourceType: "issuer",
			TargetType: "serviceaccount",
			Label:      "authenticates-with",
			Paths: []string{
				"$.spec.vault.auth.kubernetes.serviceAccountRef.name",
			},
			KeyFormat: "serviceaccount/{namespace}/{name}",
		},
	}

	EdgeRules = make([]EdgeRule, 0, len(pathRules))
	for _, pr := range pathRules {
		EdgeRules = append(EdgeRules, CompileRule(pr))
	}
}

func ApplyEdgeRules(resource *kube.Resource, nodes map[string]kube.Resource) []kube.ResourceEdge {
	edges := []kube.ResourceEdge{}
	data := resource.AsMap()
	if data == nil {
		return edges
	}

	namespace := ""
	if meta, ok := data["metadata"].(map[string]any); ok {
		if ns, ok := meta["namespace"].(string); ok {
			namespace = ns
		}
	}

	for _, rule := range EdgeRules {
		if resource.Type != rule.SourceType {
			continue
		}

		targetNames := make(map[string]bool)
		for _, pathExpr := range rule.Paths {
			results := pathExpr.Get(data)
			for _, result := range results {
				if name, ok := result.(string); ok && name != "" {
					targetNames[name] = true
				}
			}
		}

		for name := range targetNames {
			targetKey := formatKey(rule.KeyFormat, namespace, name)
			if _, exists := nodes[targetKey]; exists {
				edges = append(edges, kube.ResourceEdge{
					Source: resource.Key,
					Target: targetKey,
					Label:  rule.Label,
				})
			}
		}
	}

	return edges
}

func formatKey(format, namespace, name string) string {
	result := strings.ReplaceAll(format, "{namespace}", namespace)
	result = strings.ReplaceAll(result, "{name}", name)
	return result
}
