package tree

import (
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestNormalizePolicyPlacementMovesPoliciesToSpec(t *testing.T) {
	root := &kube.Tree{
		Key:  "deployment/ns/app",
		Type: "deployment",
		Meta: map[string]any{"name": "app"},
		Children: []*kube.Tree{
			{Key: "ciliumnetworkpolicy/ns/analyser", Type: "ciliumnetworkpolicy", Meta: map[string]any{"name": "analyser"}},
			{Key: "service/ns/app", Type: "service", Meta: map[string]any{"name": "app"}},
			{Key: "networkpolicy/ns/app-np", Type: "networkpolicy", Meta: map[string]any{"name": "app-np"}},
			{
				Key:  "replicaset/ns/app-rs",
				Type: "replicaset",
				Meta: map[string]any{"name": "app-rs"},
				Children: []*kube.Tree{
					{
						Key:  "pod/ns/app-pod",
						Type: "pod",
						Meta: map[string]any{"name": "app-pod"},
						Children: []*kube.Tree{
							{Key: "ciliumnetworkpolicy/ns/analyser", Type: "ciliumnetworkpolicy", Meta: map[string]any{"name": "analyser"}},
							{Key: "ciliumnetworkpolicy/ns/temp-egress", Type: "ciliumnetworkpolicy", Meta: map[string]any{"name": "temp-egress"}},
							{Key: "serviceaccount/ns/default", Type: "serviceaccount", Meta: map[string]any{"name": "default"}},
						},
					},
				},
			},
			{Key: "spec/ns/app", Type: "spec", Meta: map[string]any{}, Children: []*kube.Tree{{Key: "configmaps/ns/cm", Type: "configmaps", Meta: map[string]any{}}}},
		},
	}

	normalizePolicyPlacement(root)

	if hasPolicyNodeOutsideSpec(root.Children) {
		t.Fatalf("expected no policy nodes outside spec after normalization")
	}

	spec := findChildByType(root, "spec")
	if spec == nil {
		t.Fatalf("expected spec node to exist after normalization")
	}

	declaredKeys := map[string]bool{}
	for _, child := range spec.Children {
		if child == nil {
			continue
		}
		if isDeclaredSpecType(child.Type) {
			declaredKeys[child.Key] = true
		}
	}

	if !declaredKeys["ciliumnetworkpolicy/ns/analyser"] {
		t.Fatalf("expected analyser policy under spec")
	}
	if !declaredKeys["ciliumnetworkpolicy/ns/temp-egress"] {
		t.Fatalf("expected temp-egress policy under spec")
	}
	if !declaredKeys["service/ns/app"] {
		t.Fatalf("expected service under spec")
	}
	if !declaredKeys["serviceaccount/ns/default"] {
		t.Fatalf("expected serviceaccount under spec")
	}
	if !declaredKeys["networkpolicy/ns/app-np"] {
		t.Fatalf("expected networkpolicy under spec")
	}
}

func TestNormalizePolicyPlacementHoistsCronJobDeclaredNodesToSpec(t *testing.T) {
	root := &kube.Tree{
		Key:  "cronjob/ns/repo-update",
		Type: "cronjob",
		Meta: map[string]any{"name": "repo-update"},
		Children: []*kube.Tree{
			{
				Key:  "job/ns/repo-update-1",
				Type: "job",
				Meta: map[string]any{"name": "repo-update-1"},
				Children: []*kube.Tree{
					{
						Key:  "pod/ns/repo-update-1-abc",
						Type: "pod",
						Meta: map[string]any{"name": "repo-update-1-abc"},
						Children: []*kube.Tree{
							{
								Key:  "service/ns/fiskeoye",
								Type: "service",
								Meta: map[string]any{"name": "fiskeoye"},
								Children: []*kube.Tree{
									{Key: "endpoints/ns/fiskeoye", Type: "endpoints", Meta: map[string]any{"name": "fiskeoye"}},
									{Key: "endpointslice/ns/fiskeoye-rb9jr", Type: "endpointslice", Meta: map[string]any{"name": "fiskeoye-rb9jr"}},
								},
							},
							{Key: "serviceaccount/ns/fiskeoye-cronjob", Type: "serviceaccount", Meta: map[string]any{"name": "fiskeoye-cronjob"}},
						},
					},
				},
			},
		},
	}

	normalizePolicyPlacement(root)

	if hasPolicyNodeOutsideSpec(root.Children) {
		t.Fatalf("expected no declared spec nodes outside spec after cronjob normalization")
	}

	spec := findChildByType(root, "spec")
	if spec == nil {
		t.Fatalf("expected cronjob spec node to be created")
	}

	declaredKeys := map[string]bool{}
	for _, child := range spec.Children {
		if child == nil {
			continue
		}
		if isDeclaredSpecType(child.Type) {
			declaredKeys[child.Key] = true
		}
	}

	if !declaredKeys["service/ns/fiskeoye"] {
		t.Fatalf("expected service under cronjob spec")
	}
	if !declaredKeys["serviceaccount/ns/fiskeoye-cronjob"] {
		t.Fatalf("expected serviceaccount under cronjob spec")
	}

	// endpoints and endpointslices stay as children of their service, not top-level spec children
	svc := findChildByKey(spec, "service/ns/fiskeoye")
	if svc == nil {
		t.Fatalf("expected service/ns/fiskeoye under spec")
	}
	svcChildKeys := map[string]bool{}
	for _, c := range svc.Children {
		if c != nil {
			svcChildKeys[c.Key] = true
		}
	}
	if !svcChildKeys["endpoints/ns/fiskeoye"] {
		t.Fatalf("expected endpoints under service, not top-level spec")
	}
	if !svcChildKeys["endpointslice/ns/fiskeoye-rb9jr"] {
		t.Fatalf("expected endpointslice under service, not top-level spec")
	}
}

func TestNormalizePolicyPlacement_PreservesCertificateIssuerUnderService(t *testing.T) {
	root := &kube.Tree{
		Key:  "deployment/ns/web",
		Type: "deployment",
		Meta: map[string]any{"name": "web"},
		Children: []*kube.Tree{
			{
				Key:  "service/ns/web",
				Type: "service",
				Meta: map[string]any{"name": "web"},
				Children: []*kube.Tree{
					{
						Key:  "certificate/ns/web-cert",
						Type: "certificate",
						Meta: map[string]any{"name": "web-cert"},
						Children: []*kube.Tree{
							{Key: "issuer/ns/web-issuer", Type: "issuer", Meta: map[string]any{"name": "web-issuer"}},
						},
					},
				},
			},
		},
	}

	normalizePolicyPlacement(root)

	spec := findChildByType(root, "spec")
	if spec == nil {
		t.Fatalf("expected workload spec node after normalization")
	}

	svc := findChildByKey(spec, "service/ns/web")
	if svc == nil {
		t.Fatalf("expected service moved under spec")
	}

	cert := findChildByKey(svc, "certificate/ns/web-cert")
	if cert == nil {
		t.Fatalf("expected certificate to remain under service after normalization")
	}

	iss := findChildByKey(cert, "issuer/ns/web-issuer")
	if iss == nil {
		t.Fatalf("expected issuer to remain under certificate after normalization")
	}
}

func hasPolicyNodeOutsideSpec(children []*kube.Tree) bool {
	for _, child := range children {
		if child == nil {
			continue
		}
		if child.Type != "spec" {
			if containsDeclaredSpecNode(child) {
				return true
			}
			continue
		}
		for _, specChild := range child.Children {
			if specChild == nil {
				continue
			}
			if !isDeclaredSpecType(specChild.Type) && containsDeclaredSpecNode(specChild) {
				return true
			}
		}
	}
	return false
}

func containsDeclaredSpecNode(node *kube.Tree) bool {
	if node == nil {
		return false
	}
	if isDeclaredSpecType(node.Type) {
		return true
	}
	for _, child := range node.Children {
		if containsDeclaredSpecNode(child) {
			return true
		}
	}
	return false
}

func findChildByType(node *kube.Tree, childType string) *kube.Tree {
	if node == nil {
		return nil
	}
	for _, child := range node.Children {
		if child != nil && child.Type == childType {
			return child
		}
	}
	return nil
}

func findChildByKey(node *kube.Tree, key string) *kube.Tree {
	if node == nil {
		return nil
	}
	for _, child := range node.Children {
		if child != nil && child.Key == key {
			return child
		}
	}
	return nil
}
