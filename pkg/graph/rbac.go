package graph

import kube "github.com/karloie/kompass/pkg/kube"

func inferSubject(resource, meta map[string]any) []any {

	for _, src := range []map[string]any{resource, meta} {
		if arr, ok := src["subjects"].([]any); ok {
			return arr
		}
	}

	for _, src := range []map[string]any{resource, meta} {
		if arrMap, ok := src["subjects"].([]map[string]any); ok {
			arr := make([]any, len(arrMap))
			for i, v := range arrMap {
				arr[i] = v
			}
			return arr
		}
	}

	return nil
}

func inferServiceAccount(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
	key := addNode(edges, item, nodes, "serviceaccount")
	if key == "" {
		return nil
	}

	meta := M(item.AsMap()).Map("metadata")
	name := meta.String("name")

	forEachNodeOfType(*nodes, "pod", func(n kube.Resource) {
		if spec := M(n.AsMap()).Map("spec"); spec != nil {
			if saName := spec.String("serviceAccountName"); saName == name {
				addEdge(edges, n.Key, key, "uses")
			}
		}
	})
	return nil
}

func inferServiceAccounts(edges *[]kube.ResourceEdge, subjects []any, bindingKey string) {
	for _, s := range subjects {
		subj, ok := s.(map[string]any)
		if !ok || subj["kind"] != "ServiceAccount" {
			continue
		}
		saName := M(subj).String("name")
		saNs := M(subj).String("namespace")
		if saName != "" && saNs != "" {
			saKey := Key("serviceaccount", saNs, saName)
			addEdge(edges, saKey, bindingKey, "bound-by")
		}
	}
}

func inferRole(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
	key := addNode(edges, item, nodes, "role")
	if key == "" {
		return nil
	}

	meta := M(item.AsMap()).Map("metadata")
	if subjects := inferSubject(item.AsMap(), meta.Raw()); subjects != nil {
		inferServiceAccounts(edges, subjects, key)
	}
	return nil
}

func inferClusterRole(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error {
	key := addNode(edges, item, nodes, "clusterrole")
	if key == "" {
		return nil
	}

	meta := M(item.AsMap()).Map("metadata")
	name := meta.String("name")

	forEachNodeOfType(*nodes, "clusterrolebinding", func(n kube.Resource) {
		if roleRef := M(n.AsMap()).Map("roleRef"); roleRef != nil && roleRef.String("name") == name {
			addEdge(edges, key, n.Key, "granted-by")
		}
	})
	return nil
}

func inferBinding(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider, bindingType, roleType string) error {
	key := addNode(edges, item, nodes, bindingType)
	if key == "" {
		return nil
	}

	meta := M(item.AsMap()).Map("metadata")
	namespace := meta.String("namespace")

	resource := M(item.AsMap())
	roleRef := resource.Map("roleRef")
	if roleRef == nil {
		roleRef = meta.Map("roleRef")
	}
	if roleRef != nil {
		if roleName := roleRef.String("name"); roleName != "" {
			var roleKey string
			if namespace != "" && roleType == "role" {
				roleKey = Key(roleType, namespace, roleName)
			} else {
				roleKey = Key(roleType, "", roleName)
			}
			addEdge(edges, key, roleKey, "grants")
		}
	}

	if subjects := inferSubject(item.AsMap(), meta.Raw()); subjects != nil {
		inferServiceAccounts(edges, subjects, key)
	}
	return nil
}
