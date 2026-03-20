package graph

import kube "github.com/karloie/kompass/pkg/kube"

func inferPersistentVolumeClaim(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
	key := addNode(edges, item, nodes, "persistentvolumeclaim")
	if key == "" {
		return nil
	}

	ruleEdges := ApplyEdgeRules(item, *nodes)
	*edges = append(*edges, ruleEdges...)
	return nil
}

func inferPersistentVolume(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
	key := addNode(edges, item, nodes, "persistentvolume")
	if key == "" {
		return nil
	}

	ruleEdges := ApplyEdgeRules(item, *nodes)
	*edges = append(*edges, ruleEdges...)
	return nil
}

func inferStorageClass(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
	meta := M(item.AsMap()).Map("metadata")
	name := meta.String("name")
	scKey := Key("storageclass", "", name)
	if scKey == "" {
		return nil
	}
	(*nodes)[scKey] = *item
	for _, n := range *nodes {
		if n.Type != "persistentvolume" {
			continue
		}
		if spec := M(n.AsMap()).Map("spec"); spec != nil {
			if scName := spec.String("storageClassName"); scName == name {
				addEdge(edges, n.Key, scKey, "uses")
			}
		}
	}
	return nil
}

func inferVolumeAttachment(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
	key := addNode(edges, item, nodes, "volumeattachment")
	if key == "" {
		return nil
	}

	spec := M(item.AsMap()).Map("spec").Raw()
	if spec == nil {
		return nil
	}

	source := M(spec).Map("source").Raw()
	pvName := M(source).String("persistentVolumeName")
	if pvName != "" {
		pvKey := Key("persistentvolume", "", pvName)
		addEdge(edges, pvKey, key, "attached-by")
	}
	return nil
}

func inferCSIDriver(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
	key := addNode(edges, item, nodes, "csidriver")
	if key == "" {
		return nil
	}
	meta := M(item.AsMap()).Map("metadata")
	name := meta.String("name")
	for _, n := range *nodes {
		if n.Type == "csinode" {
			nodeMeta := M(n.AsMap()).Map("metadata")
			if driverName := nodeMeta.String("driverName"); driverName == name {
				addEdge(edges, n.Key, key, "uses")
			}
		}
	}
	return nil
}

func inferCSINode(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
	key := addNode(edges, item, nodes, "csinode")
	if key == "" {
		return nil
	}

	meta := M(item.AsMap()).Map("metadata")
	driverName := meta.String("driverName")
	if driverName == "" {
		if labels := meta.Map("labels"); labels != nil {
			driverName = labels.String("driverName")
		}
	}
	if driverName == "" {
		if spec := M(item.AsMap()).Map("spec"); spec != nil {
			if drivers := spec.Slice("drivers"); len(drivers) > 0 {
				if drvMap, ok := drivers[0].(map[string]interface{}); ok {
					driverName, _ = drvMap["name"].(string)
				}
			}
		}
	}

	if driverName != "" {
		driverKey := Key("csidriver", "", driverName)
		addEdgeIfNodeExists(edges, *nodes, key, driverKey, "uses")
	}
	return nil
}
