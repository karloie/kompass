package kube

import (
	"context"
	"log/slog"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceLoader func(Provider, string, context.Context, metav1.ListOptions) ([]Resource, error)

type ResourceConfig struct {
	Group      string
	Version    string
	Resource   string
	Namespaced bool
}

type objectMeta interface {
	GetName() string
	GetNamespace() string
}

type objectMetaPtr[T any] interface {
	*T
	objectMeta
}

func initLoaders() map[string]ResourceLoader {
	return map[string]ResourceLoader{

		"configmap": namespacedLoad("configmap",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (*corev1.ConfigMapList, error) {
				return p.GetConfigMaps(ns, c, o)
			},
			func(l *corev1.ConfigMapList) []corev1.ConfigMap { return l.Items }),

		"endpoints": namespacedLoad("endpoints",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (*corev1.EndpointsList, error) {
				return p.GetEndpoints(ns, c, o)
			},
			func(l *corev1.EndpointsList) []corev1.Endpoints { return l.Items }),

		"role": namespacedLoad("role",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (*rbacv1.RoleList, error) {
				return p.GetRoles(ns, c, o)
			},
			func(l *rbacv1.RoleList) []rbacv1.Role { return l.Items }),

		"rolebinding": namespacedLoad("rolebinding",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (*rbacv1.RoleBindingList, error) {
				return p.GetRoleBindings(ns, c, o)
			},
			func(l *rbacv1.RoleBindingList) []rbacv1.RoleBinding { return l.Items }),

		"endpointslice": namespacedLoad("endpointslice",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (*discoveryv1.EndpointSliceList, error) {
				return p.GetEndpointSlices(ns, c, o)
			},
			func(l *discoveryv1.EndpointSliceList) []discoveryv1.EndpointSlice { return l.Items }),

		"serviceaccount": namespacedLoad("serviceaccount",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (*corev1.ServiceAccountList, error) {
				type saProvider interface {
					GetServiceAccounts(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.ServiceAccountList, error)
				}
				if sp, ok := p.(saProvider); ok {
					return sp.GetServiceAccounts(ns, c, o)
				}
				return &corev1.ServiceAccountList{}, nil
			},
			func(l *corev1.ServiceAccountList) []corev1.ServiceAccount { return l.Items }),

		"limitrange": namespacedLoad("limitrange",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (*corev1.LimitRangeList, error) {
				return p.GetLimitRanges(ns, c, o)
			},
			func(l *corev1.LimitRangeList) []corev1.LimitRange { return l.Items }),

		"resourcequota": namespacedLoad("resourcequota",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (*corev1.ResourceQuotaList, error) {
				return p.GetResourceQuotas(ns, c, o)
			},
			func(l *corev1.ResourceQuotaList) []corev1.ResourceQuota { return l.Items }),

		"persistentvolume": clusterLoad("persistentvolume",
			func(p Provider, c context.Context, o metav1.ListOptions) (*corev1.PersistentVolumeList, error) {
				return p.GetPersistentVolumes(c, o)
			},
			func(l *corev1.PersistentVolumeList) []corev1.PersistentVolume { return l.Items }),

		"storageclass": clusterLoad("storageclass",
			func(p Provider, c context.Context, o metav1.ListOptions) (*storagev1.StorageClassList, error) {
				return p.GetStorageClasses(c, o)
			},
			func(l *storagev1.StorageClassList) []storagev1.StorageClass { return l.Items }),

		"volumeattachment": clusterLoad("volumeattachment",
			func(p Provider, c context.Context, o metav1.ListOptions) (*storagev1.VolumeAttachmentList, error) {
				return p.GetVolumeAttachments(c, o)
			},
			func(l *storagev1.VolumeAttachmentList) []storagev1.VolumeAttachment { return l.Items }),

		"csidriver": clusterLoad("csidriver",
			func(p Provider, c context.Context, o metav1.ListOptions) (*storagev1.CSIDriverList, error) {
				return p.GetCSIDrivers(c, o)
			},
			func(l *storagev1.CSIDriverList) []storagev1.CSIDriver { return l.Items }),

		"csinode": clusterLoad("csinode",
			func(p Provider, c context.Context, o metav1.ListOptions) (*storagev1.CSINodeList, error) {
				return p.GetCSINodes(c, o)
			},
			func(l *storagev1.CSINodeList) []storagev1.CSINode { return l.Items }),

		"clusterrole": clusterLoad("clusterrole",
			func(p Provider, c context.Context, o metav1.ListOptions) (*rbacv1.ClusterRoleList, error) {
				return p.GetClusterRoles(c, o)
			},
			func(l *rbacv1.ClusterRoleList) []rbacv1.ClusterRole { return l.Items }),

		"clusterrolebinding": clusterLoad("clusterrolebinding",
			func(p Provider, c context.Context, o metav1.ListOptions) (*rbacv1.ClusterRoleBindingList, error) {
				return p.GetClusterRoleBindings(c, o)
			},
			func(l *rbacv1.ClusterRoleBindingList) []rbacv1.ClusterRoleBinding { return l.Items }),

		"ingressclass": clusterLoad("ingressclass",
			func(p Provider, c context.Context, o metav1.ListOptions) (*networkingv1.IngressClassList, error) {
				return p.GetIngressClasses(c, o)
			},
			func(l *networkingv1.IngressClassList) []networkingv1.IngressClass { return l.Items }),

		"deployment": workloadLoad("deployment",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (any, error) {
				return p.GetDeployments(ns, c, o)
			}),

		"replicaset": workloadLoad("replicaset",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (any, error) {
				return p.GetReplicaSets(ns, c, o)
			}),

		"statefulset": workloadLoad("statefulset",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (any, error) {
				return p.GetStatefulSets(ns, c, o)
			}),

		"daemonset": workloadLoad("daemonset",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (any, error) {
				return p.GetDaemonSets(ns, c, o)
			}),

		"persistentvolumeclaim": workloadLoad("persistentvolumeclaim",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (any, error) {
				return p.GetPersistentVolumeClaims(ns, c, o)
			}),

		"ingress": workloadLoad("ingress",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (any, error) {
				return p.GetIngresses(ns, c, o)
			}),

		"horizontalpodautoscaler": workloadLoad("horizontalpodautoscaler",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (any, error) {
				return p.GetHorizontalPodAutoscalers(ns, c, o)
			}),

		"cronjob": workloadLoad("cronjob",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (any, error) {
				return p.GetCronJobs(ns, c, o)
			}),

		"poddisruptionbudget": workloadLoad("poddisruptionbudget",
			func(p Provider, ns string, c context.Context, o metav1.ListOptions) (any, error) {
				return p.GetPodDisruptionBudgets(ns, c, o)
			}),
	}
}

var loaders = initLoaders()

func getLoader(resourceType string) ResourceLoader {
	return loaders[resourceType]
}

func skipForbiddenLoad(resourceType, namespace string, err error) bool {
	if err == nil || !apierrors.IsForbidden(err) {
		return false
	}
	slog.Warn("Skipping forbidden provider call", "resourceType", resourceType, "namespace", namespace, "error", err)
	return true
}

func LoadPod(provider Provider, namespace string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
	pods, err := provider.GetPods(namespace, ctx, opts)
	if err != nil {
		if skipForbiddenLoad("pod", namespace, err) {
			return []Resource{}, nil
		}
		return nil, err
	}
	out := make([]Resource, 0, len(pods.Items))
	for _, p := range pods.Items {
		r := Resource{
			Key:      "pod/" + p.Namespace + "/" + p.Name,
			Type:     "pod",
			Resource: toMap(p),
		}
		out = append(out, r)
	}
	return out, nil
}

func LoadService(provider Provider, namespace string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
	list, err := provider.GetServices(namespace, ctx, opts)
	if err != nil {
		if skipForbiddenLoad("service", namespace, err) {
			return []Resource{}, nil
		}
		return nil, err
	}
	out := make([]Resource, 0, len(list.Items))
	for _, service := range list.Items {
		serviceMap := toMap(service)
		r := Resource{
			Key:      "service/" + service.Namespace + "/" + service.Name,
			Type:     "service",
			Resource: serviceMap,
		}
		out = append(out, r)
	}
	return out, nil
}

func LoadSecret(provider Provider, namespace string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
	list, err := provider.GetSecrets(namespace, ctx, opts)
	if err != nil {
		if skipForbiddenLoad("secret", namespace, err) {
			return []Resource{}, nil
		}
		return nil, err
	}
	out := make([]Resource, 0, len(list.Items))
	for _, secret := range list.Items {
		secretMap := toMap(secret)
		redactSecretMap(secretMap)

		r := Resource{
			Key:      "secret/" + secret.Namespace + "/" + secret.Name,
			Type:     "secret",
			Resource: secretMap,
		}
		out = append(out, r)
	}
	return out, nil
}

func LoadNode(provider Provider, _ string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
	list, err := provider.GetNodes(ctx, opts)
	if err != nil {
		if skipForbiddenLoad("node", "", err) {
			return []Resource{}, nil
		}
		return nil, err
	}
	out := make([]Resource, 0, len(list.Items))
	for _, node := range list.Items {
		r := Resource{
			Key:      "node/" + node.Name,
			Type:     "node",
			Resource: toMap(node),
		}
		out = append(out, r)
	}
	return out, nil
}

func LoadJob(provider Provider, namespace string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
	jobs, err := provider.GetJobs(namespace, ctx, opts)
	if err != nil {
		if skipForbiddenLoad("job", namespace, err) {
			return []Resource{}, nil
		}
		return nil, err
	}
	out := make([]Resource, 0, len(jobs.Items))
	for _, job := range jobs.Items {
		r := Resource{
			Key:      "job/" + job.Namespace + "/" + job.Name,
			Type:     "job",
			Resource: toMap(job),
		}
		out = append(out, r)
	}
	return out, nil
}

func LoadNetworkPolicy(provider Provider, namespace string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
	nps, err := provider.GetNetworkPolicies(namespace, ctx, opts)
	if err != nil {
		if skipForbiddenLoad("networkpolicy", namespace, err) {
			return []Resource{}, nil
		}
		return nil, err
	}
	out := make([]Resource, 0, len(nps.Items))
	for _, np := range nps.Items {
		r := Resource{
			Key:      "networkpolicy/" + np.Namespace + "/" + np.Name,
			Type:     "networkpolicy",
			Resource: toMap(np),
		}
		out = append(out, r)
	}
	return out, nil
}

func LoadCiliumNetworkPolicy(provider Provider, namespace string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
	cp, ok := provider.(CiliumProvider)
	if !ok {
		return []Resource{}, nil
	}
	cnps, err := cp.GetCiliumNetworkPolicies(namespace, ctx, opts)
	if err != nil {
		if skipForbiddenLoad("ciliumnetworkpolicy", namespace, err) {
			return []Resource{}, nil
		}
		return nil, err
	}
	out := make([]Resource, 0, len(cnps))
	for _, cnp := range cnps {
		meta, _ := cnp["metadata"].(map[string]any)
		name, _ := meta["name"].(string)
		ns, _ := meta["namespace"].(string)
		if name == "" || ns == "" {
			continue
		}
		r := Resource{
			Key:      "ciliumnetworkpolicy/" + ns + "/" + name,
			Type:     "ciliumnetworkpolicy",
			Resource: cnp,
		}
		out = append(out, r)
	}
	return out, nil
}

func LoadCiliumClusterwideNetworkPolicy(provider Provider, _ string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
	cp, ok := provider.(CiliumProvider)
	if !ok {
		return []Resource{}, nil
	}
	ccnps, err := cp.GetCiliumClusterwideNetworkPolicies(ctx, opts)
	if err != nil {
		if skipForbiddenLoad("ciliumclusterwidenetworkpolicy", "", err) {
			return []Resource{}, nil
		}
		return nil, err
	}
	out := make([]Resource, 0, len(ccnps))
	for _, ccnp := range ccnps {
		meta, _ := ccnp["metadata"].(map[string]any)
		name, _ := meta["name"].(string)
		if name == "" {
			continue
		}
		r := Resource{
			Key:      "ciliumclusterwidenetworkpolicy/" + name,
			Type:     "ciliumclusterwidenetworkpolicy",
			Resource: ccnp,
		}
		out = append(out, r)
	}
	return out, nil
}

func LoadCertificate(provider Provider, namespace string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
	return conditionBasedLoad("certificate",
		func(p Provider) (any, bool) { cp, ok := p.(CertManagerProvider); return cp, ok },
		func(p any, ns string, c context.Context, o metav1.ListOptions) ([]map[string]any, error) {
			return p.(CertManagerProvider).GetCertificates(ns, c, o)
		}, true)(provider, namespace, ctx, opts)
}

func LoadHTTPRoute(provider Provider, namespace string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
	return conditionBasedLoad("httproute",
		func(p Provider) (any, bool) { gp, ok := p.(GatewayProvider); return gp, ok },
		func(p any, ns string, c context.Context, o metav1.ListOptions) ([]map[string]any, error) {
			return p.(GatewayProvider).GetHTTPRoutes(ns, c, o)
		}, true)(provider, namespace, ctx, opts)
}

func LoadGateway(provider Provider, namespace string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
	return conditionBasedLoad("gateway",
		func(p Provider) (any, bool) { gp, ok := p.(GatewayProvider); return gp, ok },
		func(p any, ns string, c context.Context, o metav1.ListOptions) ([]map[string]any, error) {
			return p.(GatewayProvider).GetGateways(ns, c, o)
		}, true)(provider, namespace, ctx, opts)
}

func LoadIssuer(provider Provider, namespace string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
	return conditionBasedLoad("issuer",
		func(p Provider) (any, bool) { cp, ok := p.(CertManagerProvider); return cp, ok },
		func(p any, ns string, c context.Context, o metav1.ListOptions) ([]map[string]any, error) {
			return p.(CertManagerProvider).GetIssuers(ns, c, o)
		}, true)(provider, namespace, ctx, opts)
}

func LoadClusterIssuer(provider Provider, _ string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
	return conditionBasedLoad("clusterissuer",
		func(p Provider) (any, bool) { cp, ok := p.(CertManagerProvider); return cp, ok },
		func(p any, ns string, c context.Context, o metav1.ListOptions) ([]map[string]any, error) {
			return p.(CertManagerProvider).GetClusterIssuers(c, o)
		}, false)(provider, "", ctx, opts)
}

func LoadSecretProviderClass(provider Provider, namespace string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
	return conditionBasedLoad("secretproviderclass",
		func(p Provider) (any, bool) { sp, ok := p.(SecretsStoreProvider); return sp, ok },
		func(p any, ns string, c context.Context, o metav1.ListOptions) ([]map[string]any, error) {
			return p.(SecretsStoreProvider).GetSecretProviderClasses(ns, c, o)
		}, true)(provider, namespace, ctx, opts)
}

func buildResourceKey(resourceType, namespace, name string) string {
	if name == "" {
		return ""
	}
	if namespace == "" {
		return resourceType + "/" + name
	}
	return resourceType + "/" + namespace + "/" + name
}

func newResource(resourceType, namespace, name string, resource map[string]any) (Resource, bool) {
	key := buildResourceKey(resourceType, namespace, name)
	if key == "" {
		return Resource{}, false
	}
	return Resource{Key: key, Type: resourceType, Resource: resource}, true
}

func namespacedLoad[T any, PT objectMetaPtr[T], L any](
	resourceType string,
	getter func(Provider, string, context.Context, metav1.ListOptions) (L, error),
	extractItems func(L) []T,
) func(Provider, string, context.Context, metav1.ListOptions) ([]Resource, error) {
	return func(provider Provider, namespace string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
		list, err := getter(provider, namespace, ctx, opts)
		if err != nil {
			if skipForbiddenLoad(resourceType, namespace, err) {
				return []Resource{}, nil
			}
			return nil, err
		}
		items := extractItems(list)
		out := make([]Resource, 0, len(items))
		for i := range items {
			item := PT(&items[i])
			if r, ok := newResource(resourceType, item.GetNamespace(), item.GetName(), toMap(item)); ok {
				out = append(out, r)
			}
		}
		return out, nil
	}
}

func clusterLoad[T any, PT objectMetaPtr[T], L any](
	resourceType string,
	getter func(Provider, context.Context, metav1.ListOptions) (L, error),
	extractItems func(L) []T,
) func(Provider, string, context.Context, metav1.ListOptions) ([]Resource, error) {
	return func(provider Provider, _ string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
		list, err := getter(provider, ctx, opts)
		if err != nil {
			if skipForbiddenLoad(resourceType, "", err) {
				return []Resource{}, nil
			}
			return nil, err
		}
		items := extractItems(list)
		out := make([]Resource, 0, len(items))
		for i := range items {
			item := PT(&items[i])
			if r, ok := newResource(resourceType, "", item.GetName(), toMap(item)); ok {
				out = append(out, r)
			}
		}
		return out, nil
	}
}

func conditionBasedLoad(resourceType string, providerCheck func(Provider) (any, bool), getter func(any, string, context.Context, metav1.ListOptions) ([]map[string]any, error), isNamespaced bool) ResourceLoader {
	return func(provider Provider, namespace string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
		p, ok := providerCheck(provider)
		if !ok {
			return []Resource{}, nil
		}
		items, err := getter(p, namespace, ctx, opts)
		if err != nil {
			if skipForbiddenLoad(resourceType, namespace, err) {
				return []Resource{}, nil
			}
			return nil, err
		}
		out := make([]Resource, 0, len(items))
		for _, item := range items {
			meta, _ := item["metadata"].(map[string]any)
			name, _ := meta["name"].(string)
			ns, _ := meta["namespace"].(string)
			if name == "" || (!isNamespaced && ns != "") || (isNamespaced && ns == "") {
				continue
			}
			namespacePart := ""
			if isNamespaced {
				namespacePart = ns
			}
			if r, ok := newResource(resourceType, namespacePart, name, item); ok {
				out = append(out, r)
			}
		}
		return out, nil
	}
}

func workloadLoad(resourceType string, getter func(Provider, string, context.Context, metav1.ListOptions) (any, error)) ResourceLoader {
	return func(provider Provider, namespace string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error) {
		list, err := getter(provider, namespace, ctx, opts)
		if err != nil {
			if skipForbiddenLoad(resourceType, namespace, err) {
				return []Resource{}, nil
			}
			return nil, err
		}
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		out := make([]Resource, 0, items.Len())
		for i := 0; i < items.Len(); i++ {
			item := items.Index(i).Interface()
			itemMap := toMap(item)
			meta, _ := itemMap["metadata"].(map[string]any)
			ns, _ := meta["namespace"].(string)
			name, _ := meta["name"].(string)
			if ns == "" || name == "" {
				continue
			}
			if r, ok := newResource(resourceType, ns, name, itemMap); ok {
				out = append(out, r)
			}
		}
		return out, nil
	}
}
