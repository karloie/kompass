package kube

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// FetchResource retrieves a single Kubernetes resource by type, namespace, and name.
// This is much cheaper than running the full graph pipeline — exactly one API call.
// Namespace may be empty for cluster-scoped resources.
func (c *Client) FetchResource(resourceType, namespace, name string, ctx context.Context) (*Resource, error) {
	if c.mockMode {
		return c.fetchResourceMock(resourceType, namespace, name)
	}

	rt, ok := ResourceTypes[resourceType]
	if !ok {
		return nil, fmt.Errorf("unknown resource type %q", resourceType)
	}

	// Virtual types have no API representation
	if rt.Resource == "" {
		return nil, fmt.Errorf("resource type %q is virtual and cannot be fetched", resourceType)
	}

	gvr := schema.GroupVersionResource{
		Group:    rt.Group,
		Version:  rt.Version,
		Resource: rt.Resource,
	}

	var ri dynamic.ResourceInterface
	if rt.Namespaced && namespace != "" {
		ri = c.dynamicClient.Resource(gvr).Namespace(namespace)
	} else {
		ri = c.dynamicClient.Resource(gvr)
	}

	obj, err := ri.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	trimVerboseMetadata(obj.Object)
	if resourceType == "secret" {
		redactSecretMap(obj.Object)
	}

	key := buildResourceKey(resourceType, namespace, name)
	return &Resource{Key: key, Type: resourceType, Resource: obj.Object}, nil
}

// buildResourceKey builds a key in the same format used by the graph pipeline.
// fetchResourceMock searches the in-memory model for a resource matching the
// given type, namespace, and name.
func (c *Client) fetchResourceMock(resourceType, namespace, name string) (*Resource, error) {
	var items []map[string]any

	switch resourceType {
	// Core
	case "pod":
		for _, p := range c.mockModel.Pods {
			items = append(items, toMap(p))
		}
	case "service":
		for _, s := range c.mockModel.Services {
			items = append(items, toMap(s))
		}
	case "configmap":
		for _, cm := range c.mockModel.ConfigMaps {
			items = append(items, toMap(cm))
		}
	case "secret":
		for _, s := range c.mockModel.Secrets {
			items = append(items, toMap(s))
		}
	case "serviceaccount":
		for _, sa := range c.mockModel.ServiceAccounts {
			items = append(items, toMap(sa))
		}
	case "namespace":
		for _, ns := range c.mockModel.Namespaces {
			items = append(items, toMap(ns))
		}
	case "node":
		for _, n := range c.mockModel.Nodes {
			items = append(items, toMap(n))
		}
	case "endpoints":
		for _, e := range c.mockModel.Endpoints {
			items = append(items, toMap(e))
		}
	case "endpointslice":
		for _, e := range c.mockModel.EndpointSlices {
			items = append(items, toMap(e))
		}
	case "persistentvolumeclaim":
		for _, pvc := range c.mockModel.PersistentVolumeClaims {
			items = append(items, toMap(pvc))
		}
	case "persistentvolume":
		for _, pv := range c.mockModel.PersistentVolumes {
			items = append(items, toMap(pv))
		}
	case "limitrange":
		for _, lr := range c.mockModel.LimitRanges {
			items = append(items, toMap(lr))
		}
	case "resourcequota":
		for _, rq := range c.mockModel.ResourceQuotas {
			items = append(items, toMap(rq))
		}
	// Apps
	case "deployment":
		for _, d := range c.mockModel.Deployments {
			items = append(items, toMap(d))
		}
	case "daemonset":
		for _, ds := range c.mockModel.DaemonSets {
			items = append(items, toMap(ds))
		}
	case "replicaset":
		for _, rs := range c.mockModel.ReplicaSets {
			items = append(items, toMap(rs))
		}
	case "statefulset":
		for _, ss := range c.mockModel.StatefulSets {
			items = append(items, toMap(ss))
		}
	// Batch
	case "cronjob":
		for _, cj := range c.mockModel.CronJobs {
			items = append(items, toMap(cj))
		}
	case "job":
		for _, j := range c.mockModel.Jobs {
			items = append(items, toMap(j))
		}
	// RBAC
	case "clusterrole":
		for _, cr := range c.mockModel.ClusterRoles {
			items = append(items, toMap(cr))
		}
	case "clusterrolebinding":
		for _, crb := range c.mockModel.ClusterRoleBindings {
			items = append(items, toMap(crb))
		}
	case "role":
		for _, r := range c.mockModel.Roles {
			items = append(items, toMap(r))
		}
	case "rolebinding":
		for _, rb := range c.mockModel.RoleBindings {
			items = append(items, toMap(rb))
		}
	// Networking (k8s)
	case "ingress":
		for _, i := range c.mockModel.Ingresses {
			items = append(items, toMap(i))
		}
	case "networkpolicy":
		for _, np := range c.mockModel.NetworkPolicies {
			items = append(items, toMap(np))
		}
	case "ingressclass":
		for _, ic := range c.mockModel.IngressClasses {
			items = append(items, toMap(ic))
		}
	// Policy
	case "poddisruptionbudget":
		for _, pdb := range c.mockModel.PodDisruptionBudgets {
			items = append(items, toMap(pdb))
		}
	// Storage
	case "storageclass":
		for _, sc := range c.mockModel.StorageClasses {
			items = append(items, toMap(sc))
		}
	case "volumeattachment":
		for _, va := range c.mockModel.VolumeAttachments {
			items = append(items, toMap(va))
		}
	case "csidriver":
		for _, d := range c.mockModel.CSIDrivers {
			items = append(items, toMap(d))
		}
	case "csinode":
		for _, cn := range c.mockModel.CSINodes {
			items = append(items, toMap(cn))
		}
	// Autoscaling
	case "horizontalpodautoscaler":
		for _, hpa := range c.mockModel.HorizontalPodAutoscalers {
			items = append(items, toMap(hpa))
		}
	// CertificateSigningRequest
	case "certificatesigningrequest":
		for _, csr := range c.mockModel.CertificateSigningRequests {
			items = append(items, toMap(csr))
		}
	// Flow control
	case "flowschema":
		for _, fs := range c.mockModel.FlowSchemas {
			items = append(items, toMap(fs))
		}
	case "prioritylevelconfiguration":
		for _, plc := range c.mockModel.PriorityLevelConfigurations {
			items = append(items, toMap(plc))
		}
	// Node
	case "runtimeclass":
		for _, rc := range c.mockModel.RuntimeClasses {
			items = append(items, toMap(rc))
		}
	// Scheduling
	case "priorityclass":
		for _, pc := range c.mockModel.PriorityClasses {
			items = append(items, toMap(pc))
		}
	// Dynamic/CRD-based resources stored as map[string]any
	case "ciliumnetworkpolicy":
		items = c.mockModel.CiliumNetworkPolicies
	case "ciliumclusterwidenetworkpolicy":
		items = c.mockModel.CiliumClusterwideNetworkPolicies
	case "certificate":
		items = c.mockModel.Certificates
	case "issuer":
		items = c.mockModel.Issuers
	case "clusterissuer":
		items = c.mockModel.ClusterIssuers
	case "httproute":
		items = c.mockModel.HTTPRoutes
	case "gateway":
		items = c.mockModel.Gateways
	case "secretproviderclass":
		items = c.mockModel.SecretProviderClasses
	default:
		return nil, fmt.Errorf("resource %s/%s not found (unsupported type in mock)", resourceType, name)
	}

	m := findInMockItems(items, namespace, name)
	if m == nil {
		return nil, fmt.Errorf("resource %s/%s not found", resourceType, name)
	}
	if resourceType == "secret" {
		m = cloneMap(m)
		redactSecretMap(m)
	}

	key := buildResourceKey(resourceType, namespace, name)
	return &Resource{Key: key, Type: resourceType, Resource: m}, nil
}

// findInMockItems searches a slice of map[string]any objects (as produced by toMap)
// for a match on namespace and name in the standard metadata fields.
func findInMockItems(items []map[string]any, namespace, name string) map[string]any {
	for _, item := range items {
		meta, _ := item["metadata"].(map[string]any)
		if meta == nil {
			continue
		}
		itemName, _ := meta["name"].(string)
		if itemName != name {
			continue
		}
		if namespace == "" {
			return item
		}
		itemNS, _ := meta["namespace"].(string)
		if itemNS == namespace {
			return item
		}
	}
	return nil
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		child, ok := value.(map[string]any)
		if ok {
			out[key] = cloneMap(child)
			continue
		}
		out[key] = value
	}
	return out
}
