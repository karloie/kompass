package tree

import (
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestBuildCiliumNetworkPolicyChildren_HidesInTreePodReferences(t *testing.T) {
	policyKey := "ciliumnetworkpolicy/applikasjonsplattform/fiskeoye"
	serviceKey := "service/applikasjonsplattform/fiskeoye"
	inTreePodKey := "pod/applikasjonsplattform/fiskeoye-8699f8f467-fphr6"
	outOfTreePodKey := "pod/applikasjonsplattform/fiskeoye-repo-update-29555460-fzd8m"

	policy := kube.Resource{
		Key:  policyKey,
		Type: "ciliumnetworkpolicy",
		Resource: map[string]any{
			"spec": map[string]any{},
		},
	}

	nodeMap := map[string]kube.Resource{
		policyKey:    policy,
		serviceKey:   {Key: serviceKey, Type: "service", Resource: map[string]any{"metadata": map[string]any{"name": "fiskeoye"}}},
		inTreePodKey: {Key: inTreePodKey, Type: "pod", Resource: map[string]any{"metadata": map[string]any{"name": "fiskeoye-8699f8f467-fphr6"}}},
		outOfTreePodKey: {
			Key:        outOfTreePodKey,
			Type:       "pod",
			Discovered: true,
			Resource:   map[string]any{"metadata": map[string]any{"name": "fiskeoye-repo-update-29555460-fzd8m"}},
		},
	}

	graphChildren := map[string][]string{
		policyKey: {serviceKey, inTreePodKey, outOfTreePodKey},
	}

	children := buildCiliumNetworkPolicyChildren(policyKey, policy, graphChildren, newTreeBuildState(), nodeMap)

	hasService := false
	hasInTreePod := false
	hasOutOfTreePod := false
	for _, child := range children {
		switch child.Key {
		case serviceKey:
			hasService = true
		case inTreePodKey:
			hasInTreePod = true
		case outOfTreePodKey:
			hasOutOfTreePod = true
		}
	}

	if !hasService {
		t.Fatalf("expected service child to be included")
	}
	if hasInTreePod {
		t.Fatalf("expected in-tree pod child to be excluded")
	}
	if !hasOutOfTreePod {
		t.Fatalf("expected out-of-tree pod child to be included")
	}
}

func TestBuildServiceChildren_HidesInTreePodReferences(t *testing.T) {
	serviceKey := "service/applikasjonsplattform/fiskeoye"
	endpointsKey := "endpoints/applikasjonsplattform/fiskeoye"
	inTreePodKey := "pod/applikasjonsplattform/fiskeoye-8699f8f467-fphr6"
	outOfTreePodKey := "pod/applikasjonsplattform/fiskeoye-repo-update-29555460-fzd8m"

	service := kube.Resource{
		Key:      serviceKey,
		Type:     "service",
		Resource: map[string]any{"metadata": map[string]any{"name": "fiskeoye"}},
	}

	nodeMap := map[string]kube.Resource{
		serviceKey:      service,
		endpointsKey:    {Key: endpointsKey, Type: "endpoints", Resource: map[string]any{}},
		inTreePodKey:    {Key: inTreePodKey, Type: "pod", Resource: map[string]any{"metadata": map[string]any{"name": "fiskeoye-8699f8f467-fphr6"}}},
		outOfTreePodKey: {Key: outOfTreePodKey, Type: "pod", Discovered: true, Resource: map[string]any{"metadata": map[string]any{"name": "fiskeoye-repo-update-29555460-fzd8m"}}},
	}

	graphChildren := map[string][]string{
		serviceKey: {endpointsKey, inTreePodKey, outOfTreePodKey},
	}

	children := buildServiceChildren(serviceKey, service, graphChildren, newTreeBuildState(), nodeMap)

	hasEndpoints := false
	hasInTreePod := false
	hasOutOfTreePod := false
	for _, child := range children {
		switch child.Key {
		case endpointsKey:
			hasEndpoints = true
		case inTreePodKey:
			hasInTreePod = true
		case outOfTreePodKey:
			hasOutOfTreePod = true
		}
	}

	if !hasEndpoints {
		t.Fatalf("expected endpoints child to be included")
	}
	if hasInTreePod {
		t.Fatalf("expected in-tree pod child to be excluded")
	}
	if !hasOutOfTreePod {
		t.Fatalf("expected out-of-tree pod child to be included")
	}
}

func TestBuildServiceAccountChildren_HidesInTreePodReferences(t *testing.T) {
	serviceAccountKey := "serviceaccount/applikasjonsplattform/fiskeoye-cronjob"
	roleBindingKey := "rolebinding/applikasjonsplattform/fiskeoye-cronjob"
	roleKey := "role/applikasjonsplattform/fiskeoye-cronjob"
	inTreePodKey := "pod/applikasjonsplattform/fiskeoye-repo-update-29555460-fzd8m"
	outOfTreePodKey := "pod/applikasjonsplattform/fiskeoye-repo-update-manual-1773320866-vwr78"

	serviceAccount := kube.Resource{
		Key:      serviceAccountKey,
		Type:     "serviceaccount",
		Resource: map[string]any{"metadata": map[string]any{"name": "fiskeoye-cronjob"}},
	}

	nodeMap := map[string]kube.Resource{
		serviceAccountKey: serviceAccount,
		roleBindingKey:    {Key: roleBindingKey, Type: "rolebinding", Resource: map[string]any{"metadata": map[string]any{"name": "fiskeoye-cronjob"}}},
		roleKey:           {Key: roleKey, Type: "role", Resource: map[string]any{"metadata": map[string]any{"name": "fiskeoye-cronjob"}}},
		inTreePodKey:      {Key: inTreePodKey, Type: "pod", Resource: map[string]any{"metadata": map[string]any{"name": "fiskeoye-repo-update-29555460-fzd8m"}}},
		outOfTreePodKey:   {Key: outOfTreePodKey, Type: "pod", Discovered: true, Resource: map[string]any{"metadata": map[string]any{"name": "fiskeoye-repo-update-manual-1773320866-vwr78"}}},
	}

	graphChildren := map[string][]string{
		serviceAccountKey: {inTreePodKey, outOfTreePodKey, roleBindingKey},
		roleBindingKey:    {serviceAccountKey, roleKey},
		roleKey:           {roleBindingKey},
	}

	children := buildServiceAccountChildren(serviceAccountKey, serviceAccount, graphChildren, newTreeBuildState(), nodeMap)

	hasRoleBinding := false
	hasInTreePod := false
	hasOutOfTreePod := false
	hasRoleUnderRoleBinding := false

	for _, child := range children {
		switch child.Key {
		case roleBindingKey:
			hasRoleBinding = true
			for _, rbChild := range child.Children {
				if rbChild.Key == roleKey {
					hasRoleUnderRoleBinding = true
				}
			}
		case inTreePodKey:
			hasInTreePod = true
		case outOfTreePodKey:
			hasOutOfTreePod = true
		}
	}

	if !hasRoleBinding {
		t.Fatalf("expected rolebinding child to be included")
	}
	if !hasRoleUnderRoleBinding {
		t.Fatalf("expected role to remain visible under rolebinding")
	}
	if hasInTreePod {
		t.Fatalf("expected in-tree pod child to be excluded")
	}
	if !hasOutOfTreePod {
		t.Fatalf("expected out-of-tree pod child to be included")
	}
}
