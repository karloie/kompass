package tree

import "strings"

type ResourceKeyRef struct {
	Type      string
	Namespace string
	Name      string
}

func ParseResourceKeyRef(key string) ResourceKeyRef {
	parts := strings.Split(key, "/")
	switch len(parts) {
	case 3:
		return ResourceKeyRef{Type: parts[0], Namespace: parts[1], Name: parts[2]}
	case 2:
		return ResourceKeyRef{Type: parts[0], Name: parts[1]}
	case 1:
		return ResourceKeyRef{Type: parts[0]}
	default:
		return ResourceKeyRef{}
	}
}

func BuildResourceKeyRef(resourceType, namespace, name string) string {
	if name == "" {
		return ""
	}
	if namespace == "" {
		return resourceType + "/" + name
	}
	return resourceType + "/" + namespace + "/" + name
}
