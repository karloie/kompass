package graph

import (
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestInferPersistentVolumeClaimAddsNode(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"namespace": "petshop", "name": "data"},
		"spec":     map[string]any{"volumeName": "pv-data"},
	}}

	if err := inferPersistentVolumeClaim(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferPersistentVolumeClaim error: %v", err)
	}
	if _, ok := nodes["persistentvolumeclaim/petshop/data"]; !ok {
		t.Fatalf("expected PVC node to be added")
	}
}

func TestInferPersistentVolumeAddsNode(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"name": "pv-data"},
		"spec":     map[string]any{"storageClassName": "fast"},
	}}

	if err := inferPersistentVolume(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferPersistentVolume error: %v", err)
	}
	if _, ok := nodes["persistentvolume/pv-data"]; !ok {
		t.Fatalf("expected PV node to be added")
	}
}

func TestInferStorageClassAddsUsesEdgesFromPV(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"persistentvolume/pv-data": {
			Key:  "persistentvolume/pv-data",
			Type: "persistentvolume",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "pv-data"},
				"spec":     map[string]any{"storageClassName": "fast"},
			},
		},
	}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"name": "fast"},
	}}

	if err := inferStorageClass(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferStorageClass error: %v", err)
	}
	if _, ok := nodes["storageclass/fast"]; !ok {
		t.Fatalf("expected storageclass node to be added")
	}

	found := false
	for _, e := range edges {
		if e.Source == "persistentvolume/pv-data" && e.Target == "storageclass/fast" && e.Label == "uses" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected PV uses storageclass edge, got %#v", edges)
	}
}

func TestInferVolumeAttachmentAttachedByEdge(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"name": "attach-1"},
		"spec": map[string]any{
			"source": map[string]any{"persistentVolumeName": "pv-data"},
		},
	}}

	if err := inferVolumeAttachment(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferVolumeAttachment error: %v", err)
	}
	if _, ok := nodes["volumeattachment/attach-1"]; !ok {
		t.Fatalf("expected volumeattachment node to be added")
	}

	found := false
	for _, e := range edges {
		if e.Source == "persistentvolume/pv-data" && e.Target == "volumeattachment/attach-1" && e.Label == "attached-by" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected attached-by edge, got %#v", edges)
	}
}

func TestInferCSIDriverLinksCSINode(t *testing.T) {
	edges := []kube.ResourceEdge{}
	nodes := map[string]kube.Resource{
		"csinode/node-1": {
			Key:  "csinode/node-1",
			Type: "csinode",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "node-1", "driverName": "driver.csi.test"},
			},
		},
	}
	item := &kube.Resource{Resource: map[string]any{
		"metadata": map[string]any{"name": "driver.csi.test"},
	}}

	if err := inferCSIDriver(&edges, item, &nodes, nil); err != nil {
		t.Fatalf("inferCSIDriver error: %v", err)
	}
	if _, ok := nodes["csidriver/driver.csi.test"]; !ok {
		t.Fatalf("expected csidriver node to be added")
	}

	found := false
	for _, e := range edges {
		if e.Source == "csinode/node-1" && e.Target == "csidriver/driver.csi.test" && e.Label == "uses" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected csinode uses csidriver edge, got %#v", edges)
	}
}

func TestInferCSINodeUsesDriverFromMetadataAndFallbacks(t *testing.T) {
	t.Run("metadata driverName", func(t *testing.T) {
		edges := []kube.ResourceEdge{}
		nodes := map[string]kube.Resource{
			"csidriver/driver.meta": {Key: "csidriver/driver.meta", Type: "csidriver"},
		}
		item := &kube.Resource{Resource: map[string]any{
			"metadata": map[string]any{"name": "node-1", "driverName": "driver.meta"},
		}}
		if err := inferCSINode(&edges, item, &nodes, nil); err != nil {
			t.Fatalf("inferCSINode error: %v", err)
		}
		if len(edges) != 1 || edges[0].Target != "csidriver/driver.meta" {
			t.Fatalf("expected uses edge to driver.meta, got %#v", edges)
		}
	})

	t.Run("labels fallback", func(t *testing.T) {
		edges := []kube.ResourceEdge{}
		nodes := map[string]kube.Resource{
			"csidriver/driver.label": {Key: "csidriver/driver.label", Type: "csidriver"},
		}
		item := &kube.Resource{Resource: map[string]any{
			"metadata": map[string]any{
				"name":   "node-2",
				"labels": map[string]any{"driverName": "driver.label"},
			},
		}}
		if err := inferCSINode(&edges, item, &nodes, nil); err != nil {
			t.Fatalf("inferCSINode error: %v", err)
		}
		if len(edges) != 1 || edges[0].Target != "csidriver/driver.label" {
			t.Fatalf("expected uses edge to driver.label, got %#v", edges)
		}
	})

	t.Run("spec drivers fallback", func(t *testing.T) {
		edges := []kube.ResourceEdge{}
		nodes := map[string]kube.Resource{
			"csidriver/driver.spec": {Key: "csidriver/driver.spec", Type: "csidriver"},
		}
		item := &kube.Resource{Resource: map[string]any{
			"metadata": map[string]any{"name": "node-3"},
			"spec": map[string]any{
				"drivers": []any{map[string]any{"name": "driver.spec"}},
			},
		}}
		if err := inferCSINode(&edges, item, &nodes, nil); err != nil {
			t.Fatalf("inferCSINode error: %v", err)
		}
		if len(edges) != 1 || edges[0].Target != "csidriver/driver.spec" {
			t.Fatalf("expected uses edge to driver.spec, got %#v", edges)
		}
	})
}
