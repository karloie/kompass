package tree

import (
	"reflect"
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestBuildPodWithSimplifiedContainers_RuntimeChildrenSorted(t *testing.T) {
	podKey := "pod/petshop/secretgen-web-7b548b454f-78bns"
	pod := kube.Resource{
		Key:  podKey,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "secretgen-web-7b548b454f-78bns"},
			"spec": map[string]any{
				"containers": []any{
					map[string]any{
						"name":           "app",
						"livenessProbe":  map[string]any{"httpGet": map[string]any{"path": "/livez"}},
						"readinessProbe": map[string]any{"httpGet": map[string]any{"path": "/readyz"}},
						"startupProbe":   map[string]any{"httpGet": map[string]any{"path": "/startupz"}},
					},
				},
			},
			"status": map[string]any{
				"containerStatuses": []any{
					map[string]any{
						"name":    "app",
						"ready":   true,
						"started": true,
						"image":   "cr.petshop.com/secretgen-web:main_test",
						"imageID": "cr.petshop.com/secretgen-web@sha256:abc",
						"state": map[string]any{
							"running": map[string]any{"startedAt": "2026-03-12T10:00:00Z"},
						},
						"resources": map[string]any{
							"requests": map[string]any{"cpu": "200m", "memory": "128Mi"},
						},
						"allocatedResources": map[string]any{
							"cpu": "200m", "memory": "128Mi",
						},
					},
				},
			},
		},
	}

	node := buildPodWithSimplifiedContainers(podKey, pod)
	if node == nil || len(node.Children) != 1 {
		t.Fatalf("expected one container node, got %#v", node)
	}

	container := node.Children[0]
	gotTypes := make([]string, 0, len(container.Children))
	var runtimeImage *kube.Tree
	var runtimeResources *kube.Tree
	for _, child := range container.Children {
		gotTypes = append(gotTypes, child.Type)
		if child.Type == "image" {
			runtimeImage = child
		}
		if child.Type == "resources" {
			runtimeResources = child
		}
	}

	wantTypes := []string{"image", "resources"}
	if !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("unexpected child order, got %v want %v", gotTypes, wantTypes)
	}

	if got, _ := container.Meta["livenessStatus"].(string); got != "passing" {
		t.Fatalf("expected livenessStatus=passing, got %#v", container.Meta)
	}
	if got, _ := container.Meta["readinessStatus"].(string); got != "ready" {
		t.Fatalf("expected readinessStatus=ready, got %#v", container.Meta)
	}
	if got, _ := container.Meta["startupStatus"].(string); got != "started" {
		t.Fatalf("expected startupStatus=started, got %#v", container.Meta)
	}

	if runtimeImage == nil {
		t.Fatalf("expected runtime image child")
	}
	if got, _ := runtimeImage.Meta["name"].(string); got != "cr.petshop.com/secretgen-web@sha256:abc" {
		t.Fatalf("expected runtime image name to be imageID digest, got %#v", runtimeImage.Meta)
	}
	if _, hasID := runtimeImage.Meta["id"]; hasID {
		t.Fatalf("expected runtime image metadata to omit id field, got %#v", runtimeImage.Meta)
	}
	if runtimeResources == nil {
		t.Fatalf("expected runtime resources child")
	}
	if got, ok := runtimeResources.Meta["allocated"].(map[string]any); !ok || !reflect.DeepEqual(got, map[string]any{"cpu": "200m", "memory": "128Mi"}) {
		t.Fatalf("expected runtime resources to only expose allocated metadata, got %#v", runtimeResources.Meta)
	}
	if _, hasRequests := runtimeResources.Meta["requests"]; hasRequests {
		t.Fatalf("expected runtime resources metadata to omit requests, got %#v", runtimeResources.Meta)
	}
	if _, hasLimits := runtimeResources.Meta["limits"]; hasLimits {
		t.Fatalf("expected runtime resources metadata to omit limits, got %#v", runtimeResources.Meta)
	}
}

func TestBuildPodChildren_RuntimeContainerFlattensProbeStatus(t *testing.T) {
	podKey := "pod/petshop/petshop-db-xyz"
	pod := kube.Resource{
		Key:  podKey,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "petshop-db-xyz", "namespace": "petshop"},
			"spec": map[string]any{
				"containers": []any{
					map[string]any{
						"name":           "app",
						"livenessProbe":  map[string]any{"tcpSocket": map[string]any{"port": 7687}},
						"readinessProbe": map[string]any{"tcpSocket": map[string]any{"port": 7687}},
						"startupProbe":   map[string]any{"tcpSocket": map[string]any{"port": 7687}},
					},
				},
			},
			"status": map[string]any{
				"containerStatuses": []any{
					map[string]any{
						"name":    "app",
						"ready":   true,
						"started": true,
						"state": map[string]any{
							"running": map[string]any{"startedAt": "2026-03-12T10:00:00Z"},
						},
					},
				},
			},
		},
	}

	children := buildPodChildren(podKey, pod, map[string][]string{}, newTreeBuildState(), map[string]kube.Resource{podKey: pod})

	var runtimeContainer *kube.Tree
	for _, child := range children {
		if child.Type == "container" {
			runtimeContainer = child
			break
		}
	}
	if runtimeContainer == nil {
		t.Fatalf("expected runtime container child under pod")
	}

	types := map[string]bool{}
	for _, child := range runtimeContainer.Children {
		types[child.Type] = true
	}

	if types["livenessprobe"] || types["readinessprobe"] || types["startupprobe"] {
		t.Fatalf("expected no runtime probe leaf nodes under pod/container, got child types %#v", types)
	}

	if got, _ := runtimeContainer.Meta["livenessStatus"].(string); got != "passing" {
		t.Fatalf("expected flattened livenessStatus=passing, got %#v", runtimeContainer.Meta)
	}
	if got, _ := runtimeContainer.Meta["readinessStatus"].(string); got != "ready" {
		t.Fatalf("expected flattened readinessStatus=ready, got %#v", runtimeContainer.Meta)
	}
	if got, _ := runtimeContainer.Meta["startupStatus"].(string); got != "started" {
		t.Fatalf("expected flattened startupStatus=started, got %#v", runtimeContainer.Meta)
	}
}

func TestBuildPodWithSimplifiedContainers_RuntimeFallsBackToSpecImageOnly(t *testing.T) {
	podKey := "pod/ns/app-0"
	pod := kube.Resource{
		Key:  podKey,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "app-0", "namespace": "ns"},
			"spec": map[string]any{
				"containers": []any{
					map[string]any{
						"name":  "app",
						"image": "repo/app:v1",
						"resources": map[string]any{
							"requests": map[string]any{"cpu": "100m", "memory": "128Mi"},
							"limits":   map[string]any{"cpu": "500m", "memory": "512Mi"},
						},
					},
				},
			},
			"status": map[string]any{
				"phase": "Running",
				"containerStatuses": []any{
					map[string]any{
						"name": "app",
						"state": map[string]any{
							"running": map[string]any{"startedAt": "2026-03-12T10:00:00Z"},
						},
					},
				},
			},
		},
	}

	node := buildPodWithSimplifiedContainers(podKey, pod)
	if node == nil || len(node.Children) != 1 {
		t.Fatalf("expected one container node, got %#v", node)
	}

	container := node.Children[0]
	hasImage := false
	hasResources := false
	for _, child := range container.Children {
		if child.Type == "image" {
			hasImage = true
		}
		if child.Type == "resources" {
			hasResources = true
		}
	}

	if !hasImage {
		t.Fatalf("expected runtime image node from spec fallback")
	}
	if hasResources {
		t.Fatalf("expected runtime resources node to be hidden when there is no allocated runtime data")
	}
}

func TestBuildPodWithSimplifiedContainers_RuntimeHidesEmptyResources(t *testing.T) {
	podKey := "pod/monitoring/alertmanager-kube-prometheus-stack-alertmanager-0"
	pod := kube.Resource{
		Key:  podKey,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "alertmanager-kube-prometheus-stack-alertmanager-0", "namespace": "monitoring"},
			"spec": map[string]any{
				"containers": []any{
					map[string]any{
						"name":      "config-reloader",
						"image":     "quay.io/prometheus-operator/prometheus-config-reloader:v0.83.0",
						"resources": map[string]any{},
					},
				},
			},
			"status": map[string]any{
				"phase": "Running",
				"containerStatuses": []any{
					map[string]any{
						"name":      "config-reloader",
						"image":     "quay.io/prometheus-operator/prometheus-config-reloader:v0.83.0",
						"imageID":   "quay.io/prometheus-operator/prometheus-config-reloader@sha256:78aec597",
						"ready":     true,
						"started":   true,
						"resources": map[string]any{},
						"state": map[string]any{
							"running": map[string]any{"startedAt": "2026-03-17T10:00:00Z"},
						},
					},
				},
			},
		},
	}

	node := buildPodWithSimplifiedContainers(podKey, pod)
	if node == nil || len(node.Children) != 1 {
		t.Fatalf("expected one container node, got %#v", node)
	}

	container := node.Children[0]
	hasResources := false
	for _, child := range container.Children {
		if child.Type == "resources" {
			hasResources = true
			break
		}
	}

	if hasResources {
		t.Fatalf("expected empty runtime resources node to be hidden")
	}
}

func TestExpandVolumesAsResources_IncludesCSIStorage(t *testing.T) {
	parentKey := "pod/ns/app/spec"
	namespace := "ns"
	volumes := []any{
		map[string]any{
			"name": "secrets",
			"csi": map[string]any{
				"driver":               "secrets-store.csi.k8s.io",
				"nodePublishSecretRef": map[string]any{"name": "azure-kv-creds"},
				"volumeAttributes":     map[string]any{"secretProviderClass": "ad-explore-db-petshopvault"},
			},
		},
		map[string]any{"name": "tmp", "emptyDir": map[string]any{}},
	}

	nodeMap := map[string]kube.Resource{
		"secret/ns/azure-kv-creds": {
			Key:      "secret/ns/azure-kv-creds",
			Type:     "secret",
			Resource: map[string]any{"metadata": map[string]any{"name": "azure-kv-creds", "namespace": "ns"}},
		},
		"secretproviderclass/ns/ad-explore-db-petshopvault": {
			Key:      "secretproviderclass/ns/ad-explore-db-petshopvault",
			Type:     "secretproviderclass",
			Resource: map[string]any{"metadata": map[string]any{"name": "ad-explore-db-petshopvault", "namespace": "ns"}},
		},
	}

	nodes := expandVolumesAsResources(parentKey, namespace, volumes, nil, newTreeBuildState(), nodeMap)
	if len(nodes) != 1 {
		t.Fatalf("expected one expanded volume node, got %d", len(nodes))
	}

	storageNode := nodes[0]
	if storageNode.Type != "secretstore" {
		t.Fatalf("expected secretstore node, got %q", storageNode.Type)
	}
	if providerClass, _ := storageNode.Meta["secretProviderClass"].(string); providerClass != "ad-explore-db-petshopvault" {
		t.Fatalf("expected secretProviderClass metadata, got %#v", storageNode.Meta)
	}

	seenSecret := false
	seenSPC := false
	for _, child := range storageNode.Children {
		switch child.Type {
		case "secret":
			seenSecret = true
		case "secretproviderclass":
			seenSPC = true
		}
	}

	if !seenSecret {
		t.Fatalf("expected secret child under CSI storage node")
	}
	if !seenSPC {
		t.Fatalf("expected secretproviderclass child under CSI storage node")
	}
}

func TestBuildPodChildren_HoistsEnvSecretToSpecLevel(t *testing.T) {
	podKey := "pod/ns/app"
	secretKey := "secret/ns/ad-explore-motor-secrets"

	pod := kube.Resource{
		Key:  podKey,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "app", "namespace": "ns"},
			"spec": map[string]any{
				"containers": []any{
					map[string]any{
						"name": "app",
						"env": []any{
							map[string]any{"name": "LDAP_SECRET", "valueFrom": map[string]any{"secretKeyRef": map[string]any{"name": "ad-explore-motor-secrets", "key": "SA-AD-EXPLORE-MOTOR-PASSWORD"}}},
							map[string]any{"name": "NEO4J_PASSWORD", "valueFrom": map[string]any{"secretKeyRef": map[string]any{"name": "ad-explore-motor-secrets", "key": "AD-EXPLORE-DATABASE-PASSWORD"}}},
						},
					},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		podKey: pod,
		secretKey: {
			Key:  secretKey,
			Type: "secret",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "ad-explore-motor-secrets", "namespace": "ns"},
				"type":     "Opaque",
				"data": map[string]any{
					"SA-AD-EXPLORE-MOTOR-PASSWORD": "<SECRET>",
					"AD-EXPLORE-DATABASE-PASSWORD": "<SECRET>",
				},
			},
		},
	}

	graphChildren := map[string][]string{
		podKey:    {secretKey},
		secretKey: {podKey},
	}

	children := buildPodChildren(podKey, pod, graphChildren, newTreeBuildState(), nodeMap)

	var specNode *kube.Tree
	for _, child := range children {
		if child.Type == "spec" {
			specNode = child
			break
		}
	}
	if specNode == nil {
		t.Fatalf("expected spec node")
	}

	secretsNodeCountUnderSpec := 0
	secretNodeCountUnderSecrets := 0
	var envNode *kube.Tree
	for _, child := range specNode.Children {
		if child.Type == "secrets" {
			secretsNodeCountUnderSpec++
			for _, secretsChild := range child.Children {
				if secretsChild.Type == "secret" && secretsChild.Key == secretKey {
					secretNodeCountUnderSecrets++
				}
			}
		}
		if child.Type == "environment" {
			envNode = child
		}
	}
	if secretsNodeCountUnderSpec != 1 {
		t.Fatalf("expected exactly one secrets group under spec, got %d", secretsNodeCountUnderSpec)
	}
	if secretNodeCountUnderSecrets != 1 {
		t.Fatalf("expected exactly one hoisted secret under spec/secrets, got %d", secretNodeCountUnderSecrets)
	}
	if envNode == nil {
		t.Fatalf("expected environment node under spec")
	}

	secretRefsUnderEnv := 0
	for _, envChild := range envNode.Children {
		if len(envChild.Children) == 1 && envChild.Children[0].Type == "secret" && envChild.Children[0].Key == secretKey {
			secretRefsUnderEnv++
		}
	}
	if secretRefsUnderEnv != 0 {
		t.Fatalf("expected no env-level secret child refs when grouped under spec/secrets, got %d", secretRefsUnderEnv)
	}

	secretNodeCountAtPodRoot := 0
	for _, child := range children {
		if child.Type == "secret" && child.Key == secretKey {
			secretNodeCountAtPodRoot++
		}
	}
	if secretNodeCountAtPodRoot != 0 {
		t.Fatalf("expected no duplicate pod-root secret nodes after hoisting, got %d", secretNodeCountAtPodRoot)
	}
}

func TestBuildPodChildren_HasSyntheticStorageNode(t *testing.T) {
	podKey := "pod/ns/app"
	pvcKey := "persistentvolumeclaim/ns/ad-explore-db-data"

	pod := kube.Resource{
		Key:  podKey,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "app", "namespace": "ns"},
			"spec": map[string]any{
				"containers": []any{
					map[string]any{"name": "app"},
				},
				"volumes": []any{
					map[string]any{"name": "data", "persistentVolumeClaim": map[string]any{"claimName": "ad-explore-db-data"}},
					map[string]any{"name": "secrets", "csi": map[string]any{"driver": "secrets-store.csi.k8s.io", "volumeAttributes": map[string]any{"secretProviderClass": "ad-explore-web-petshopvault"}}},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		podKey: pod,
		pvcKey: {
			Key:      pvcKey,
			Type:     "persistentvolumeclaim",
			Resource: map[string]any{"metadata": map[string]any{"name": "ad-explore-db-data", "namespace": "ns"}},
		},
		"secretproviderclass/ns/ad-explore-web-petshopvault": {
			Key:      "secretproviderclass/ns/ad-explore-web-petshopvault",
			Type:     "secretproviderclass",
			Resource: map[string]any{"metadata": map[string]any{"name": "ad-explore-web-petshopvault", "namespace": "ns"}},
		},
	}

	children := buildPodChildren(podKey, pod, map[string][]string{}, newTreeBuildState(), nodeMap)

	var specNode *kube.Tree
	for _, child := range children {
		if child.Type == "spec" {
			specNode = child
			break
		}
	}
	if specNode == nil {
		t.Fatalf("expected spec node")
	}

	var storageNode *kube.Tree
	var secretsNode *kube.Tree
	for _, child := range specNode.Children {
		if child.Type == "storage" {
			storageNode = child
		}
		if child.Type == "secrets" {
			secretsNode = child
		}
	}
	if storageNode == nil {
		t.Fatalf("expected synthetic storage node under spec")
	}
	if secretsNode == nil {
		t.Fatalf("expected synthetic secrets node under spec")
	}

	seenPVC := false
	for _, child := range storageNode.Children {
		switch child.Type {
		case "persistentvolumeclaim":
			seenPVC = true
		}
	}

	if !seenPVC {
		t.Fatalf("expected pvc branch under synthetic storage node")
	}

	seenSecretStore := false
	for _, child := range secretsNode.Children {
		if child.Type == "secretstore" {
			seenSecretStore = true
			break
		}
	}
	if !seenSecretStore {
		t.Fatalf("expected secretstore branch under synthetic secrets node")
	}
}

func TestBuildPodChildren_DedupesSecretStoreUnderSecrets(t *testing.T) {
	podKey := "pod/ns/app"

	pod := kube.Resource{
		Key:  podKey,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "app", "namespace": "ns"},
			"spec": map[string]any{
				"containers": []any{map[string]any{"name": "app"}},
				"volumes": []any{
					map[string]any{"name": "secrets-a", "csi": map[string]any{"driver": "secrets-store.csi.k8s.io", "volumeAttributes": map[string]any{"secretProviderClass": "ad-explore-db-petshopvault"}}},
					map[string]any{"name": "secrets-b", "csi": map[string]any{"driver": "secrets-store.csi.k8s.io", "volumeAttributes": map[string]any{"secretProviderClass": "ad-explore-db-petshopvault"}}},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		podKey: pod,
		"secretproviderclass/ns/ad-explore-db-petshopvault": {
			Key:      "secretproviderclass/ns/ad-explore-db-petshopvault",
			Type:     "secretproviderclass",
			Resource: map[string]any{"metadata": map[string]any{"name": "ad-explore-db-petshopvault", "namespace": "ns"}},
		},
	}

	children := buildPodChildren(podKey, pod, map[string][]string{}, newTreeBuildState(), nodeMap)

	var specNode *kube.Tree
	for _, child := range children {
		if child.Type == "spec" {
			specNode = child
			break
		}
	}
	if specNode == nil {
		t.Fatalf("expected spec node")
	}

	var secretsNode *kube.Tree
	for _, child := range specNode.Children {
		if child.Type == "secrets" {
			secretsNode = child
			break
		}
	}
	if secretsNode == nil {
		t.Fatalf("expected secrets node")
	}

	secretStoreCount := 0
	for _, child := range secretsNode.Children {
		if child.Type == "secretstore" {
			secretStoreCount++
		}
	}
	if secretStoreCount != 1 {
		t.Fatalf("expected one deduped secretstore under spec/secrets, got %d", secretStoreCount)
	}
}

func TestBuildPodChildren_EnvSecretStoreMetadataAndSecretStoreLeaves(t *testing.T) {
	podKey := "pod/ns/app"
	secretKey := "secret/ns/petshop-db-secrets"

	pod := kube.Resource{
		Key:  podKey,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "app", "namespace": "ns"},
			"spec": map[string]any{
				"containers": []any{
					map[string]any{
						"name": "app",
						"env": []any{
							map[string]any{"name": "NEO_DB_PASSWORD", "valueFrom": map[string]any{"secretKeyRef": map[string]any{"name": "petshop-db-secrets", "key": "PSB-DATABASE-PASSWORD"}}},
						},
					},
				},
				"volumes": []any{
					map[string]any{"name": "store", "csi": map[string]any{"driver": "secrets-store.csi.k8s.io", "volumeAttributes": map[string]any{"secretProviderClass": "petshop-db-vault"}}},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		podKey: pod,
		secretKey: {
			Key:      secretKey,
			Type:     "secret",
			Resource: map[string]any{"metadata": map[string]any{"name": "petshop-db-secrets", "namespace": "ns"}},
		},
		"secretproviderclass/ns/petshop-db-vault": {
			Key:  "secretproviderclass/ns/petshop-db-vault",
			Type: "secretproviderclass",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "petshop-db-vault", "namespace": "ns"},
				"spec":     map[string]any{"secretObjects": []any{map[string]any{"secretName": "petshop-db-secrets"}}},
			},
		},
	}

	children := buildPodChildren(podKey, pod, map[string][]string{}, newTreeBuildState(), nodeMap)

	var specNode *kube.Tree
	for _, child := range children {
		if child.Type == "spec" {
			specNode = child
			break
		}
	}
	if specNode == nil {
		t.Fatalf("expected spec node")
	}

	var envNode *kube.Tree
	var secretsNode *kube.Tree
	for _, child := range specNode.Children {
		switch child.Type {
		case "environment":
			envNode = child
		case "secrets":
			secretsNode = child
		}
	}
	if envNode == nil {
		t.Fatalf("expected environment node under spec")
	}
	if secretsNode == nil {
		t.Fatalf("expected secrets node under spec")
	}

	if len(envNode.Children) != 1 {
		t.Fatalf("expected one env entry, got %d", len(envNode.Children))
	}
	if store, _ := envNode.Children[0].Meta["secretStore"].(string); store != "petshop-db-vault" {
		t.Fatalf("expected env secretStore metadata, got %#v", envNode.Children[0].Meta)
	}

	secretStoreCount := 0
	foundUsedSecretLeaf := false
	foundEnvUsageUnderSyncedSecret := false
	foundTopLevelSecret := false
	for _, child := range secretsNode.Children {
		if child.Type == "secret" && child.Key == secretKey {
			foundTopLevelSecret = true
		}
		if child.Type != "secretstore" {
			continue
		}
		secretStoreCount++
		for _, storeChild := range child.Children {
			if storeChild.Type == "secret" && storeChild.Key == secretKey {
				foundUsedSecretLeaf = true
				for _, secretChild := range storeChild.Children {
					if secretChild.Type != "env" {
						continue
					}
					if name, _ := secretChild.Meta["name"].(string); name != "NEO_DB_PASSWORD" {
						continue
					}
					if value, _ := secretChild.Meta["value"].(string); value != "<SECRET>" {
						continue
					}
					if key, _ := secretChild.Meta["key"].(string); key != "PSB-DATABASE-PASSWORD" {
						continue
					}
					foundEnvUsageUnderSyncedSecret = true
				}
			}
		}
	}
	if secretStoreCount != 1 {
		t.Fatalf("expected one secretstore under spec/secrets, got %d", secretStoreCount)
	}
	if !foundUsedSecretLeaf {
		t.Fatalf("expected used secret leaf under secretstore")
	}
	if !foundEnvUsageUnderSyncedSecret {
		t.Fatalf("expected env usage child under synced secret leaf")
	}
	if foundTopLevelSecret {
		t.Fatalf("expected no redundant top-level secret when it is shown under secretstore")
	}
}

func TestBuildPodChildren_SecretStoreShowsMountUsageUnderSecrets(t *testing.T) {
	podKey := "pod/ns/app"

	pod := kube.Resource{
		Key:  podKey,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "app", "namespace": "ns"},
			"spec": map[string]any{
				"containers": []any{
					map[string]any{
						"name": "app",
						"volumeMounts": []any{
							map[string]any{"name": "store", "mountPath": "/mnt/azure-keyvault", "readOnly": true},
						},
					},
				},
				"volumes": []any{
					map[string]any{"name": "store", "csi": map[string]any{"driver": "secrets-store.csi.k8s.io", "volumeAttributes": map[string]any{"secretProviderClass": "petshop-db-vault"}}},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{podKey: pod}
	children := buildPodChildren(podKey, pod, map[string][]string{}, newTreeBuildState(), nodeMap)

	var specNode *kube.Tree
	for _, child := range children {
		if child.Type == "spec" {
			specNode = child
			break
		}
	}
	if specNode == nil {
		t.Fatalf("expected spec node")
	}

	var secretsNode *kube.Tree
	for _, child := range specNode.Children {
		if child.Type == "secrets" {
			secretsNode = child
			break
		}
	}
	if secretsNode == nil {
		t.Fatalf("expected secrets node")
	}

	var secretStoreNode *kube.Tree
	for _, child := range secretsNode.Children {
		if child.Type == "secretstore" {
			secretStoreNode = child
			break
		}
	}
	if secretStoreNode == nil {
		t.Fatalf("expected secretstore under spec/secrets")
	}

	foundUsage := false
	for _, child := range secretStoreNode.Children {
		if child.Type != "mount" {
			continue
		}
		if mode, _ := child.Meta["mode"].(string); mode != "csi" {
			continue
		}
		if mount, _ := child.Meta["mount"].(string); mount != "/mnt/azure-keyvault" {
			continue
		}
		if spc, _ := child.Meta["secretProviderClass"].(string); spc != "petshop-db-vault" {
			t.Fatalf("expected secretProviderClass metadata on usage child, got %#v", child.Meta)
		}
		foundUsage = true
	}

	if !foundUsage {
		t.Fatalf("expected mount usage child under secretstore")
	}
}

func TestBuildPodChildren_SecretStoreShowsSPCSyncedSecretWithoutEnvUsage(t *testing.T) {
	podKey := "pod/ns/app"
	secretKey := "secret/ns/fiskeoye-secrets"

	pod := kube.Resource{
		Key:  podKey,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "app", "namespace": "ns"},
			"spec": map[string]any{
				"containers": []any{map[string]any{"name": "app"}},
				"volumes": []any{
					map[string]any{"name": "store", "csi": map[string]any{"driver": "secrets-store.csi.k8s.io", "volumeAttributes": map[string]any{"secretProviderClass": "fiskeoye-tlosappplatt"}}},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		podKey: pod,
		secretKey: {
			Key:      secretKey,
			Type:     "secret",
			Resource: map[string]any{"metadata": map[string]any{"name": "fiskeoye-secrets", "namespace": "ns"}},
		},
		"secretproviderclass/ns/fiskeoye-tlosappplatt": {
			Key:  "secretproviderclass/ns/fiskeoye-tlosappplatt",
			Type: "secretproviderclass",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "fiskeoye-tlosappplatt", "namespace": "ns"},
				"spec":     map[string]any{"secretObjects": []any{map[string]any{"secretName": "fiskeoye-secrets"}}},
			},
		},
	}

	children := buildPodChildren(podKey, pod, map[string][]string{}, newTreeBuildState(), nodeMap)

	var specNode *kube.Tree
	for _, child := range children {
		if child.Type == "spec" {
			specNode = child
			break
		}
	}
	if specNode == nil {
		t.Fatalf("expected spec node")
	}

	var secretsNode *kube.Tree
	for _, child := range specNode.Children {
		if child.Type == "secrets" {
			secretsNode = child
			break
		}
	}
	if secretsNode == nil {
		t.Fatalf("expected secrets node")
	}

	foundUnderStore := false
	for _, child := range secretsNode.Children {
		if child.Type != "secretstore" {
			continue
		}
		for _, storeChild := range child.Children {
			if storeChild.Type == "secret" && storeChild.Key == secretKey {
				foundUnderStore = true
			}
		}
	}
	if !foundUnderStore {
		t.Fatalf("expected SPC-synced secret to be shown under secretstore")
	}
}

func TestBuildPodChildren_SecretVolumeGroupedUnderSecrets(t *testing.T) {
	podKey := "pod/ns/app"
	secretKey := "secret/ns/kafka-tls-certs"

	pod := kube.Resource{
		Key:  podKey,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "app", "namespace": "ns"},
			"spec": map[string]any{
				"containers": []any{map[string]any{"name": "app"}},
				"volumes": []any{
					map[string]any{"name": "kafka-tls-vol", "secret": map[string]any{"secretName": "kafka-tls-certs"}},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		podKey: pod,
		secretKey: {
			Key:      secretKey,
			Type:     "secret",
			Resource: map[string]any{"metadata": map[string]any{"name": "kafka-tls-certs", "namespace": "ns"}},
		},
	}

	children := buildPodChildren(podKey, pod, map[string][]string{}, newTreeBuildState(), nodeMap)

	var specNode *kube.Tree
	for _, child := range children {
		if child.Type == "spec" {
			specNode = child
			break
		}
	}
	if specNode == nil {
		t.Fatalf("expected spec node")
	}

	var secretsNode *kube.Tree
	var storageNode *kube.Tree
	for _, child := range specNode.Children {
		switch child.Type {
		case "secrets":
			secretsNode = child
		case "storage":
			storageNode = child
		}
	}

	if secretsNode == nil {
		t.Fatalf("expected secrets node")
	}

	foundSecretUnderSecrets := false
	for _, child := range secretsNode.Children {
		if child.Type == "secret" && child.Key == secretKey {
			foundSecretUnderSecrets = true
			break
		}
	}
	if !foundSecretUnderSecrets {
		t.Fatalf("expected secret volume resource under spec/secrets")
	}

	if storageNode != nil {
		for _, child := range storageNode.Children {
			if child.Type == "secret" && child.Key == secretKey {
				t.Fatalf("expected secret not to be grouped under spec/storage")
			}
		}
	}
}

func TestBuildPodChildren_LiteralEnvPathIncludesSecretNameMetadata(t *testing.T) {
	podKey := "pod/ns/app"

	pod := kube.Resource{
		Key:  podKey,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "app", "namespace": "ns"},
			"spec": map[string]any{
				"containers": []any{
					map[string]any{
						"name": "app",
						"env": []any{
							map[string]any{"name": "GITHUB_PRIVATE_KEY_PATH", "value": "/run/secrets/FISKEOYE-GITHUB-PRIVATE-KEY"},
						},
						"volumeMounts": []any{
							map[string]any{"name": "secrets", "mountPath": "/run/secrets", "readOnly": true},
						},
					},
				},
				"volumes": []any{
					map[string]any{"name": "secrets", "csi": map[string]any{"driver": "secrets-store.csi.k8s.io", "volumeAttributes": map[string]any{"secretProviderClass": "fiskeoye-petshopvault"}}},
				},
			},
		},
	}

	children := buildPodChildren(podKey, pod, map[string][]string{}, newTreeBuildState(), map[string]kube.Resource{podKey: pod})

	var specNode *kube.Tree
	for _, child := range children {
		if child.Type == "spec" {
			specNode = child
			break
		}
	}
	if specNode == nil {
		t.Fatalf("expected spec node")
	}

	var envNode *kube.Tree
	for _, child := range specNode.Children {
		if child.Type == "environment" {
			envNode = child
			break
		}
	}
	if envNode == nil || len(envNode.Children) != 1 {
		t.Fatalf("expected one environment entry")
	}

	entry := envNode.Children[0]
	if store, _ := entry.Meta["secretStore"].(string); store != "fiskeoye-petshopvault" {
		t.Fatalf("expected secretStore metadata, got %#v", entry.Meta)
	}
	if name, _ := entry.Meta["secretName"].(string); name != "FISKEOYE-GITHUB-PRIVATE-KEY" {
		t.Fatalf("expected secretName metadata, got %#v", entry.Meta)
	}
}

func TestBuildPodChildren_ConfigMapBackedEnvsGroupedUnderSpecConfigMaps(t *testing.T) {
	podKey := "pod/ns/app"
	cmKey := "configmap/ns/kafka-server-config"

	pod := kube.Resource{
		Key:  podKey,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "app", "namespace": "ns"},
			"spec": map[string]any{
				"containers": []any{
					map[string]any{
						"name": "app",
						"env": []any{
							map[string]any{"name": "KAFKA_LOG_LEVEL", "valueFrom": map[string]any{"configMapKeyRef": map[string]any{"name": "kafka-server-config", "key": "log-level"}}},
						},
					},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		podKey: pod,
		cmKey: {
			Key:  cmKey,
			Type: "configmap",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "kafka-server-config", "namespace": "ns"},
				"data": map[string]any{
					"log-level": "INFO",
				},
			},
		},
	}

	graphChildren := map[string][]string{
		podKey: {cmKey},
		cmKey:  {podKey},
	}

	children := buildPodChildren(podKey, pod, graphChildren, newTreeBuildState(), nodeMap)

	var specNode *kube.Tree
	for _, child := range children {
		if child.Type == "spec" {
			specNode = child
			break
		}
	}
	if specNode == nil {
		t.Fatalf("expected spec node")
	}

	var envNode *kube.Tree
	var configMapsNode *kube.Tree
	for _, child := range specNode.Children {
		switch child.Type {
		case "environment":
			envNode = child
		case "configmaps":
			configMapsNode = child
		}
	}
	if envNode == nil {
		t.Fatalf("expected environment node")
	}
	if configMapsNode == nil {
		t.Fatalf("expected configmaps node under spec")
	}
	if len(configMapsNode.Children) != 1 || configMapsNode.Children[0].Type != "configmap" || configMapsNode.Children[0].Key != cmKey {
		t.Fatalf("expected grouped configmap under spec/configmaps, got %#v", configMapsNode.Children)
	}

	if len(envNode.Children) != 1 {
		t.Fatalf("expected one env entry")
	}
	envEntry := envNode.Children[0]
	if cfg, _ := envEntry.Meta["configMap"].(string); cfg != "kafka-server-config" {
		t.Fatalf("expected configMap metadata reference, got %#v", envEntry.Meta)
	}
	if len(envEntry.Children) != 0 {
		t.Fatalf("expected no env-level configmap leaf children, got %d", len(envEntry.Children))
	}
}

func TestBuildPodChildren_ConfigMapsGroupingShowsMultipleConfigMaps(t *testing.T) {
	podKey := "pod/ns/kafka"
	cmAKey := "configmap/ns/kafka-server-config"
	cmBKey := "configmap/ns/kafka-runtime-config"

	pod := kube.Resource{
		Key:  podKey,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "kafka", "namespace": "ns"},
			"spec": map[string]any{
				"containers": []any{map[string]any{
					"name": "app",
					"env": []any{
						map[string]any{"name": "KAFKA_LOG_LEVEL", "valueFrom": map[string]any{"configMapKeyRef": map[string]any{"name": "kafka-server-config", "key": "log-level"}}},
						map[string]any{"name": "KAFKA_RETENTION_HOURS", "valueFrom": map[string]any{"configMapKeyRef": map[string]any{"name": "kafka-runtime-config", "key": "retention-hours"}}},
					},
				}},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		podKey: pod,
		cmAKey: {Key: cmAKey, Type: "configmap", Resource: map[string]any{"metadata": map[string]any{"name": "kafka-server-config", "namespace": "ns"}}},
		cmBKey: {Key: cmBKey, Type: "configmap", Resource: map[string]any{"metadata": map[string]any{"name": "kafka-runtime-config", "namespace": "ns"}}},
	}

	graphChildren := map[string][]string{podKey: {cmAKey, cmBKey}, cmAKey: {podKey}, cmBKey: {podKey}}
	children := buildPodChildren(podKey, pod, graphChildren, newTreeBuildState(), nodeMap)

	var specNode *kube.Tree
	for _, child := range children {
		if child.Type == "spec" {
			specNode = child
			break
		}
	}
	if specNode == nil {
		t.Fatalf("expected spec node")
	}

	var configMapsNode *kube.Tree
	for _, child := range specNode.Children {
		if child.Type == "configmaps" {
			configMapsNode = child
			break
		}
	}
	if configMapsNode == nil {
		t.Fatalf("expected configmaps node")
	}
	if len(configMapsNode.Children) != 2 {
		t.Fatalf("expected two configmaps in grouped node, got %d", len(configMapsNode.Children))
	}

	seen := map[string]bool{}
	for _, child := range configMapsNode.Children {
		seen[child.Key] = true
	}
	if !seen[cmAKey] || !seen[cmBKey] {
		t.Fatalf("expected grouped configmaps to include %q and %q, got %#v", cmAKey, cmBKey, seen)
	}
}

func TestBuildPodChildren_ConfigMapProjectedUsageShownUnderConfigMaps(t *testing.T) {
	podKey := "pod/ns/kafka"
	cmKey := "configmap/ns/kafka-server-config"

	pod := kube.Resource{
		Key:  podKey,
		Type: "pod",
		Resource: map[string]any{
			"metadata": map[string]any{"name": "kafka", "namespace": "ns"},
			"spec": map[string]any{
				"containers": []any{map[string]any{
					"name": "app",
					"volumeMounts": []any{
						map[string]any{"name": "projected-combined", "mountPath": "/etc/kafka/projected"},
					},
				}},
				"volumes": []any{
					map[string]any{
						"name": "projected-combined",
						"projected": map[string]any{
							"sources": []any{
								map[string]any{"configMap": map[string]any{"name": "kafka-server-config", "items": []any{map[string]any{"key": "server.properties", "path": "server.properties"}}}},
							},
						},
					},
				},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		podKey: pod,
		cmKey:  {Key: cmKey, Type: "configmap", Resource: map[string]any{"metadata": map[string]any{"name": "kafka-server-config", "namespace": "ns"}}},
	}

	children := buildPodChildren(podKey, pod, map[string][]string{}, newTreeBuildState(), nodeMap)

	var specNode *kube.Tree
	for _, child := range children {
		if child.Type == "spec" {
			specNode = child
			break
		}
	}
	if specNode == nil {
		t.Fatalf("expected spec node")
	}

	var configMapsNode *kube.Tree
	for _, child := range specNode.Children {
		if child.Type == "configmaps" {
			configMapsNode = child
			break
		}
	}
	if configMapsNode == nil || len(configMapsNode.Children) != 1 {
		t.Fatalf("expected one grouped configmap node")
	}

	cmNode := configMapsNode.Children[0]
	if cmNode.Key != cmKey {
		t.Fatalf("unexpected configmap key %q", cmNode.Key)
	}
	if len(cmNode.Children) == 0 {
		t.Fatalf("expected projected usage child under configmap")
	}

	usage := cmNode.Children[0]
	if usage.Type != "mount" {
		t.Fatalf("expected usage child type mount, got %q", usage.Type)
	}
	if mode, _ := usage.Meta["mode"].(string); mode != "projected" {
		t.Fatalf("expected projected mode metadata, got %#v", usage.Meta)
	}
	if mount, _ := usage.Meta["mount"].(string); mount != "/etc/kafka/projected" {
		t.Fatalf("expected projected mount path metadata, got %#v", usage.Meta)
	}
	items, _ := usage.Meta["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected projected key/path items metadata, got %#v", usage.Meta)
	}
}
