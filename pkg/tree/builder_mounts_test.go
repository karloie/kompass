package tree

import (
	"reflect"
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestBuildMountsNode_SortsByMountPath(t *testing.T) {
	containerKey := "pod/ns/app/container/0"
	namespace := "ns"

	volumeMounts := []any{
		map[string]any{"name": "tmp", "mountPath": "/tmp"},
		map[string]any{"name": "run", "mountPath": "/var/run"},
		map[string]any{"name": "secrets", "mountPath": "/run/secrets", "readOnly": true},
		map[string]any{"name": "data", "mountPath": "/var/lib/fiskeoye"},
	}
	volumes := []any{
		map[string]any{"name": "tmp", "emptyDir": map[string]any{}},
		map[string]any{"name": "run", "emptyDir": map[string]any{}},
		map[string]any{"name": "secrets", "csi": map[string]any{"driver": "secrets-store.csi.k8s.io", "nodePublishSecretRef": map[string]any{"name": "azure-kv-creds"}, "volumeAttributes": map[string]any{"secretProviderClass": "fiskeoye-spc"}}},
		map[string]any{"name": "data", "persistentVolumeClaim": map[string]any{"claimName": "fiskeoye-data"}},
	}

	node := buildMountsNode(containerKey, namespace, volumeMounts, volumes, nil, nil, nil)
	if node == nil {
		t.Fatalf("expected mounts node")
	}

	got := make([]string, 0, len(node.Children))
	refsByMount := make(map[string]int)
	for _, child := range node.Children {
		if mount, ok := child.Meta["mount"].(string); ok {
			got = append(got, mount)
			refsByMount[mount] = len(child.Children)
			if mount == "/run/secrets" {
				if len(child.Children) != 0 {
					t.Fatalf("expected /run/secrets to be mount metadata only")
				}
				if volume, _ := child.Meta["volume"].(string); volume != "secrets-store.csi.k8s.io" {
					t.Fatalf("expected /run/secrets volume metadata, got %#v", child.Meta)
				}
				if volumeType, _ := child.Meta["volumeType"].(string); volumeType != "csi" {
					t.Fatalf("expected CSI volumeType metadata, got %#v", child.Meta)
				}
				if spc, _ := child.Meta["secretProviderClass"].(string); spc != "fiskeoye-spc" {
					t.Fatalf("expected secretProviderClass metadata, got %#v", child.Meta)
				}
				if nps, _ := child.Meta["nodePublishSecretRef"].(string); nps != "azure-kv-creds" {
					t.Fatalf("expected nodePublishSecretRef metadata, got %#v", child.Meta)
				}
			}
		}
	}

	want := []string{"/run/secrets", "/tmp", "/var/lib/fiskeoye", "/var/run"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected mount ordering, got %v want %v", got, want)
	}

	if refsByMount["/run/secrets"] != 0 {
		t.Fatalf("expected /run/secrets mount to have no reference children")
	}
	if refsByMount["/tmp"] != 0 {
		t.Fatalf("expected /tmp emptyDir mount to have no reference leaf")
	}
}

func TestBuildMountsNode_CSIStorageIsMountMetadataOnly(t *testing.T) {
	containerKey := "pod/ns/app/container/0"
	namespace := "ns"

	volumeMounts := []any{
		map[string]any{"name": "secrets", "mountPath": "/run/secrets", "readOnly": true},
	}
	volumes := []any{
		map[string]any{
			"name": "secrets",
			"csi": map[string]any{
				"driver":               "secrets-store.csi.k8s.io",
				"nodePublishSecretRef": map[string]any{"name": "azure-kv-creds"},
				"volumeAttributes":     map[string]any{"secretProviderClass": "fiskeoye-spc"},
			},
		},
	}

	nodeMap := map[string]kube.Resource{
		"secret/ns/azure-kv-creds": {
			Key:      "secret/ns/azure-kv-creds",
			Type:     "secret",
			Resource: map[string]any{"metadata": map[string]any{"name": "azure-kv-creds", "namespace": "ns"}},
		},
		"secretproviderclass/ns/fiskeoye-spc": {
			Key:      "secretproviderclass/ns/fiskeoye-spc",
			Type:     "secretproviderclass",
			Resource: map[string]any{"metadata": map[string]any{"name": "fiskeoye-spc", "namespace": "ns"}},
		},
	}

	node := buildMountsNode(containerKey, namespace, volumeMounts, volumes, nil, nil, nodeMap)
	if node == nil {
		t.Fatalf("expected mounts node")
	}
	if len(node.Children) != 1 {
		t.Fatalf("expected one mount child, got %d", len(node.Children))
	}

	mountNode := node.Children[0]
	if len(mountNode.Children) != 0 {
		t.Fatalf("expected mount node to have no resource children, got %d", len(mountNode.Children))
	}
	if volume, _ := mountNode.Meta["volume"].(string); volume != "secrets-store.csi.k8s.io" {
		t.Fatalf("expected mount volume metadata, got %#v", mountNode.Meta)
	}
	if volumeType, _ := mountNode.Meta["volumeType"].(string); volumeType != "csi" {
		t.Fatalf("expected mount volumeType metadata, got %#v", mountNode.Meta)
	}
	if nps, _ := mountNode.Meta["nodePublishSecretRef"].(string); nps != "azure-kv-creds" {
		t.Fatalf("expected nodePublishSecretRef metadata, got %#v", mountNode.Meta)
	}
	if spc, _ := mountNode.Meta["secretProviderClass"].(string); spc != "fiskeoye-spc" {
		t.Fatalf("expected secretProviderClass metadata, got %#v", mountNode.Meta)
	}
}
