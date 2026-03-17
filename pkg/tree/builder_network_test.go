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

func TestBuildEndpointsChildren_AddsPodRefWithFQDN(t *testing.T) {
	endpointsKey := "endpoints/shop/api"
	podKey := "pod/shop/api-0"

	endpoints := kube.Resource{
		Key:  endpointsKey,
		Type: "endpoints",
		Resource: map[string]any{
			"subsets": []any{
				map[string]any{
					"addresses": []any{
						map[string]any{
							"ip":       "10.2.0.5",
							"hostname": "api-0",
							"targetRef": map[string]any{
								"kind": "Pod",
								"name": "api-0",
							},
						},
					},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		endpointsKey: endpoints,
		podKey: {
			Key:        podKey,
			Type:       "pod",
			Discovered: true,
			Resource:   map[string]any{"status": map[string]any{"podIP": "10.2.0.5"}},
		},
	}

	children := buildEndpointsChildren(endpointsKey, endpoints, map[string][]string{}, newTreeBuildState(), nodeMap)
	if len(children) != 1 {
		t.Fatalf("expected one subset child, got %d", len(children))
	}
	if len(children[0].Children) != 1 {
		t.Fatalf("expected one address child, got %d", len(children[0].Children))
	}
	addressNode := children[0].Children[0]
	if len(addressNode.Children) != 1 {
		t.Fatalf("expected one pod-ref child, got %d", len(addressNode.Children))
	}
	podRef := addressNode.Children[0]
	if podRef.Type != "pod-ref" {
		t.Fatalf("expected pod-ref child type, got %q", podRef.Type)
	}
	if got, _ := podRef.Meta["name"].(string); got != "api-0.api.shop.svc.cluster.local" {
		t.Fatalf("expected fqdn name, got %q", got)
	}
}

func TestBuildEndpointsChildren_HidesPodRefForInTreePod(t *testing.T) {
	endpointsKey := "endpoints/shop/api"
	podKey := "pod/shop/api-0"

	endpoints := kube.Resource{
		Key:  endpointsKey,
		Type: "endpoints",
		Resource: map[string]any{
			"subsets": []any{
				map[string]any{
					"addresses": []any{
						map[string]any{
							"ip": "10.2.0.5",
							"targetRef": map[string]any{
								"kind": "Pod",
								"name": "api-0",
							},
						},
					},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		endpointsKey: endpoints,
		podKey: {
			Key:      podKey,
			Type:     "pod",
			Resource: map[string]any{"status": map[string]any{"podIP": "10.2.0.5"}},
		},
	}

	children := buildEndpointsChildren(endpointsKey, endpoints, map[string][]string{}, newTreeBuildState(), nodeMap)
	if len(children) != 1 {
		t.Fatalf("expected one subset child, got %d", len(children))
	}
	if len(children[0].Children) != 1 {
		t.Fatalf("expected one address child, got %d", len(children[0].Children))
	}
	if len(children[0].Children[0].Children) != 0 {
		t.Fatalf("expected no pod-ref child for in-tree pod, got %d", len(children[0].Children[0].Children))
	}
}

func TestBuildEndpointSliceChildren_GuessesPodRefByHostname(t *testing.T) {
	endpointSliceKey := "endpointslice/shop/api-slice"
	podKey := "pod/shop/api-1"

	endpointSlice := kube.Resource{
		Key:  endpointSliceKey,
		Type: "endpointslice",
		Resource: map[string]any{
			"metadata": map[string]any{
				"namespace": "shop",
				"labels": map[string]any{
					"kubernetes.io/service-name": "api",
				},
			},
			"endpoints": []any{
				map[string]any{
					"hostname":  "api-1",
					"addresses": []any{"10.2.0.6"},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		endpointSliceKey: endpointSlice,
		podKey:           {Key: podKey, Type: "pod", Discovered: true, Resource: map[string]any{}},
	}

	children := buildEndpointSliceChildren(endpointSliceKey, endpointSlice, map[string][]string{}, newTreeBuildState(), nodeMap)
	if len(children) != 1 {
		t.Fatalf("expected one endpoint child, got %d", len(children))
	}
	if len(children[0].Children) != 1 {
		t.Fatalf("expected one pod-ref child, got %d", len(children[0].Children))
	}
	podRef := children[0].Children[0]
	if podRef.Type != "pod-ref" {
		t.Fatalf("expected pod-ref child type, got %q", podRef.Type)
	}
	if got, _ := podRef.Meta["name"].(string); got != "api-1.api.shop.svc.cluster.local" {
		t.Fatalf("expected fqdn name, got %q", got)
	}
}

func TestBuildEndpointSliceChildren_HidesPodRefForInTreePod(t *testing.T) {
	endpointSliceKey := "endpointslice/shop/api-slice"
	podKey := "pod/shop/api-1"

	endpointSlice := kube.Resource{
		Key:  endpointSliceKey,
		Type: "endpointslice",
		Resource: map[string]any{
			"metadata": map[string]any{
				"namespace": "shop",
				"labels": map[string]any{
					"kubernetes.io/service-name": "api",
				},
			},
			"endpoints": []any{
				map[string]any{
					"addresses": []any{"10.2.0.6"},
					"targetRef": map[string]any{
						"kind": "Pod",
						"name": "api-1",
					},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		endpointSliceKey: endpointSlice,
		podKey:           {Key: podKey, Type: "pod", Resource: map[string]any{}},
	}

	children := buildEndpointSliceChildren(endpointSliceKey, endpointSlice, map[string][]string{}, newTreeBuildState(), nodeMap)
	if len(children) != 1 {
		t.Fatalf("expected one endpoint child, got %d", len(children))
	}
	if len(children[0].Children) != 0 {
		t.Fatalf("expected no pod-ref child for in-tree pod, got %d", len(children[0].Children))
	}
}

func TestBuildHTTPRouteChildren_ShowsGatewayAsLeaf(t *testing.T) {
	routeKey := "httproute/applikasjonsplattform/ad-explore-web"
	gatewayKey := "gateway/los-platform/internal-gateway"
	serviceKey := "service/applikasjonsplattform/ad-explore-web"
	certKey := "certificate/los-platform/default-gateway-certificate"
	issuerKey := "clusterissuer/letsencrypt-prod"

	route := kube.Resource{
		Key:  routeKey,
		Type: "httproute",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "ad-explore-web", "namespace": "applikasjonsplattform"},
			"spec": map[string]any{
				"parentRefs": []any{
					map[string]any{"kind": "Gateway", "name": "internal-gateway", "namespace": "los-platform"},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		routeKey:   route,
		gatewayKey: {Key: gatewayKey, Type: "gateway", Resource: map[string]any{"metadata": map[string]any{"name": "internal-gateway", "namespace": "los-platform"}}},
		serviceKey: {Key: serviceKey, Type: "service", Resource: map[string]any{"metadata": map[string]any{"name": "ad-explore-web", "namespace": "applikasjonsplattform"}}},
		certKey: {
			Key:  certKey,
			Type: "certificate",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "default-gateway-certificate", "namespace": "los-platform"},
				"spec":     map[string]any{"issuerRef": map[string]any{"kind": "ClusterIssuer", "name": "letsencrypt-prod"}},
			},
		},
		issuerKey: {Key: issuerKey, Type: "clusterissuer", Resource: map[string]any{"metadata": map[string]any{"name": "letsencrypt-prod"}}},
	}

	graphChildren := map[string][]string{
		routeKey:   {serviceKey, gatewayKey},
		gatewayKey: {routeKey, certKey},
		certKey:    {issuerKey},
	}

	children := buildHTTPRouteChildren(routeKey, route, graphChildren, newTreeBuildState(), nodeMap)

	hasGateway := false
	hasService := false
	gatewayChildrenCount := -1
	hasCert := false
	for _, child := range children {
		switch child.Key {
		case gatewayKey:
			hasGateway = true
			gatewayChildrenCount = len(child.Children)
		case serviceKey:
			hasService = true
		case certKey:
			hasCert = true
		}
	}

	if !hasGateway {
		t.Fatalf("expected gateway child under httproute")
	}
	if gatewayChildrenCount != 0 {
		t.Fatalf("expected gateway to be a leaf under httproute, got %d children", gatewayChildrenCount)
	}
	if hasService {
		t.Fatalf("expected service child to be hidden under httproute")
	}
	if hasCert {
		t.Fatalf("expected certificate to not be projected directly under httproute")
	}
	_ = issuerKey
}

func TestBuildCertificateChildren_ShowsBackingSecretAsLeaf(t *testing.T) {
	certKey := "certificate/petshop/api-cert"
	secretKey := "secret/petshop/api-cert"
	issuerKey := "issuer/petshop/letsencrypt"
	secretChildKey := "configmap/petshop/unexpected"

	cert := kube.Resource{
		Key:  certKey,
		Type: "certificate",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "api-cert", "namespace": "petshop"},
			"spec": map[string]any{"secretName": "api-cert"},
		},
	}

	nodeMap := map[string]kube.Resource{
		certKey:        cert,
		secretKey:      {Key: secretKey, Type: "secret", Resource: map[string]any{"metadata": map[string]any{"name": "api-cert", "namespace": "petshop"}}},
		issuerKey:      {Key: issuerKey, Type: "issuer", Resource: map[string]any{"metadata": map[string]any{"name": "letsencrypt", "namespace": "petshop"}}},
		secretChildKey: {Key: secretChildKey, Type: "configmap", Resource: map[string]any{"metadata": map[string]any{"name": "unexpected", "namespace": "petshop"}}},
	}

	graphChildren := map[string][]string{
		certKey:   {issuerKey, secretKey},
		secretKey: {secretChildKey},
	}

	children := buildCertificateChildren(certKey, cert, graphChildren, newTreeBuildState(), nodeMap)

	foundSecretLeaf := false
	foundIssuer := false
	for _, child := range children {
		switch child.Key {
		case secretKey:
			foundSecretLeaf = true
			if len(child.Children) != 0 {
				t.Fatalf("expected backing secret to render as leaf, got %d children", len(child.Children))
			}
		case issuerKey:
			foundIssuer = true
		}
	}

	if !foundSecretLeaf {
		t.Fatalf("expected backing secret child under certificate")
	}
	if !foundIssuer {
		t.Fatalf("expected issuer child under certificate")
	}
}

func TestBuildHTTPRouteChildren_AddsParentRefGatewayLeafWithoutGraphEdge(t *testing.T) {
	routeKey := "httproute/applikasjonsplattform/ad-explore-web"
	gatewayKey := "gateway/los-platform/internal-gateway"

	route := kube.Resource{
		Key:  routeKey,
		Type: "httproute",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "ad-explore-web", "namespace": "applikasjonsplattform"},
			"spec": map[string]any{
				"parentRefs": []any{
					map[string]any{"kind": "Gateway", "name": "internal-gateway", "namespace": "los-platform"},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		routeKey:   route,
		gatewayKey: {Key: gatewayKey, Type: "gateway", Resource: map[string]any{"metadata": map[string]any{"name": "internal-gateway", "namespace": "los-platform"}}},
	}

	graphChildren := map[string][]string{
		routeKey: {},
	}

	children := buildHTTPRouteChildren(routeKey, route, graphChildren, newTreeBuildState(), nodeMap)

	if len(children) != 1 {
		t.Fatalf("expected one gateway child under httproute, got %d", len(children))
	}
	if children[0].Key != gatewayKey {
		t.Fatalf("expected gateway child %q, got %q", gatewayKey, children[0].Key)
	}
	if len(children[0].Children) != 0 {
		t.Fatalf("expected gateway leaf under httproute, got %d children", len(children[0].Children))
	}
}

func TestBuildHTTPRouteChildren_DoesNotAddGatewayLeafWhenGatewayAlreadySeen(t *testing.T) {
	routeKey := "httproute/applikasjonsplattform/ad-explore-web"
	gatewayKey := "gateway/los-platform/internal-gateway"

	route := kube.Resource{
		Key:  routeKey,
		Type: "httproute",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "ad-explore-web", "namespace": "applikasjonsplattform"},
			"spec": map[string]any{
				"parentRefs": []any{
					map[string]any{"kind": "Gateway", "name": "internal-gateway", "namespace": "los-platform"},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		routeKey:   route,
		gatewayKey: {Key: gatewayKey, Type: "gateway", Resource: map[string]any{"metadata": map[string]any{"name": "internal-gateway", "namespace": "los-platform"}}},
	}

	graphChildren := map[string][]string{
		routeKey: {},
	}

	state := newTreeBuildState()
	state.MarkSeen(gatewayKey)
	children := buildHTTPRouteChildren(routeKey, route, graphChildren, state, nodeMap)

	if len(children) != 0 {
		t.Fatalf("expected no gateway leaf under httproute when gateway already seen, got %d children", len(children))
	}
}
