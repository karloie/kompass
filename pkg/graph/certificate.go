package graph

import (
	kube "github.com/karloie/kompass/pkg/kube"
)

func inferCertificate(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Kube) error {
	key := addNode(edges, item, nodes, "certificate")
	if key == "" {
		return nil
	}

	meta, spec := ExtractMetaSpec(item)
	namespace := ExtractNamespace(meta)
	if spec == nil {
		return nil
	}

	if issuerRef := spec.Map("issuerRef").Raw(); issuerRef != nil {
		issuerName := M(issuerRef).String("name")
		issuerKind := M(issuerRef).String("kind")
		if issuerName != "" && issuerKind != "" {
			var issuerKey string
			if issuerKind == "ClusterIssuer" {
				issuerKey = Key("clusterissuer", "", issuerName)
			} else {
				issuerKey = Key("issuer", namespace, issuerName)
			}
			addEdge(edges, key, issuerKey, "issued-by")
		}
	}

	return nil
}

func inferIssuer(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Kube) error {
	key := addNode(edges, item, nodes, "issuer")
	if key == "" {
		return nil
	}

	ruleEdges := ApplyEdgeRules(item, *nodes)
	*edges = append(*edges, ruleEdges...)
	return nil
}

func inferClusterIssuer(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Kube) error {
	key := addNode(edges, item, nodes, "clusterissuer")
	if key == "" {
		return nil
	}

	_, spec := ExtractMetaSpec(item)
	if spec == nil {
		return nil
	}

	if acme := M(spec).Map("acme").Raw(); acme != nil {
		if secretName := M(acme).Path("privateKeySecretRef").String("name"); secretName != "" {
			for _, ns := range []string{"cert-manager", "kube-system"} {
				if secretKey := Key("secret", ns, secretName); secretKey != "" {
					if _, exists := (*nodes)[secretKey]; exists {
						addEdge(edges, key, secretKey, "uses")
						break
					}
				}
			}
		}
	}
	return nil
}
