package kube

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	flowcontrolv1 "k8s.io/api/flowcontrol/v1"
	networkingv1 "k8s.io/api/networking/v1"
	nodev1 "k8s.io/api/node/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func (c *Client) SetClientset(clientset kubernetes.Interface, dynamicClient dynamic.Interface, config *rest.Config) {
	if !c.mockMode {
		c.clientset = clientset
		c.dynamicClient = dynamicClient
		if config != nil {
			c.config = config
		}
	}
}

func (c *Client) GetClientset() kubernetes.Interface {
	if c.mockMode {
		return nil
	}
	return c.clientset
}

func (c *Client) GetDynamicClient() dynamic.Interface {
	if c.mockMode {
		return nil
	}
	return c.dynamicClient
}

func (c *Client) GetConfig() *rest.Config {
	if c.mockMode {
		return nil
	}
	return c.config
}

func (c *Client) GetContext() (string, error) { return c.context, nil }
func (c *Client) GetConfigPath() (string, error) {
	return map[bool]string{true: "mock", false: c.kubeconfig}[c.mockMode], nil
}
func (c *Client) GetContexts() (any, error) {
	values := make(map[string]struct{})

	if ctx := strings.TrimSpace(c.context); ctx != "" {
		values[ctx] = struct{}{}
	}

	kubeconfigEnv := strings.TrimSpace(os.Getenv("KUBECONFIG"))
	kubeconfig := kubeconfigEnv
	if kubeconfig == "" {
		if home, err := os.UserHomeDir(); err == nil {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	if kubeconfig != "" {
		loadingRules := &clientcmd.ClientConfigLoadingRules{}
		paths := filepath.SplitList(kubeconfig)
		if len(paths) == 1 && strings.TrimSpace(paths[0]) != "" {
			loadingRules.ExplicitPath = strings.TrimSpace(paths[0])
		} else if len(paths) > 1 {
			precedence := make([]string, 0, len(paths))
			for _, item := range paths {
				item = strings.TrimSpace(item)
				if item != "" {
					precedence = append(precedence, item)
				}
			}
			if len(precedence) > 0 {
				loadingRules.Precedence = precedence
			}
		}

		// If no valid explicit/preferred paths were derived (e.g., malformed env),
		// keep the active context fallback already collected from c.context.
		if loadingRules.ExplicitPath == "" && len(loadingRules.Precedence) == 0 && kubeconfigEnv != "" {
			loadingRules.ExplicitPath = kubeconfigEnv
		}

		if raw, err := loadingRules.Load(); err == nil && raw != nil {
			for name := range raw.Contexts {
				name = strings.TrimSpace(name)
				if name != "" {
					values[name] = struct{}{}
				}
			}
		}
	}

	if c.mockMode {
		values["mock-01"] = struct{}{}
		if isInClusterEnvironment() {
			values["in-cluster"] = struct{}{}
		}
	}

	if len(values) == 0 {
		return nil, nil
	}

	contexts := make([]string, 0, len(values))
	for item := range values {
		contexts = append(contexts, item)
	}
	sort.Strings(contexts)

	return contexts, nil
}

func isInClusterEnvironment() bool {
	return strings.TrimSpace(os.Getenv("KUBERNETES_SERVICE_HOST")) != "" &&
		strings.TrimSpace(os.Getenv("KUBERNETES_SERVICE_PORT")) != ""
}
func (c *Client) SetContext(context string)     { c.context = context }
func (c *Client) GetNamespace() (string, error) { return c.namespace, nil }
func (c *Client) SetNamespace(namespace string) { c.namespace = namespace }
func (c *Client) DynamicClient() any {
	return map[bool]any{true: nil, false: c.dynamicClient}[c.mockMode]
}

func (c *Client) GetDaemonSets(namespace string, ctx context.Context, opts metav1.ListOptions) (*appsv1.DaemonSetList, error) {
	return cachedGet(c, "daemonsets", namespace, opts, func() (*appsv1.DaemonSetList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetDaemonSets", &appsv1.DaemonSetList{}, derefSlice(c.mockModel.DaemonSets),
				func(items []appsv1.DaemonSet) *appsv1.DaemonSetList {
					return &appsv1.DaemonSetList{Items: items}
				})
		}
		return c.clientset.AppsV1().DaemonSets(namespace).List(ctx, opts)
	})
}

func (c *Client) GetDeployments(namespace string, ctx context.Context, opts metav1.ListOptions) (*appsv1.DeploymentList, error) {
	return cachedGet(c, "deployments", namespace, opts, func() (*appsv1.DeploymentList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetDeployments", &appsv1.DeploymentList{}, derefSlice(c.mockModel.Deployments),
				func(items []appsv1.Deployment) *appsv1.DeploymentList {
					return &appsv1.DeploymentList{Items: items}
				})
		}
		return c.clientset.AppsV1().Deployments(namespace).List(ctx, opts)
	})
}

func (c *Client) GetReplicaSets(namespace string, ctx context.Context, opts metav1.ListOptions) (*appsv1.ReplicaSetList, error) {
	return cachedGet(c, "replicasets", namespace, opts, func() (*appsv1.ReplicaSetList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetReplicaSets", &appsv1.ReplicaSetList{}, derefSlice(c.mockModel.ReplicaSets),
				func(items []appsv1.ReplicaSet) *appsv1.ReplicaSetList {
					return &appsv1.ReplicaSetList{Items: items}
				})
		}
		return c.clientset.AppsV1().ReplicaSets(namespace).List(ctx, opts)
	})
}

func (c *Client) GetStatefulSets(namespace string, ctx context.Context, opts metav1.ListOptions) (*appsv1.StatefulSetList, error) {
	return cachedGet(c, "statefulsets", namespace, opts, func() (*appsv1.StatefulSetList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetStatefulSets", &appsv1.StatefulSetList{}, derefSlice(c.mockModel.StatefulSets),
				func(items []appsv1.StatefulSet) *appsv1.StatefulSetList {
					return &appsv1.StatefulSetList{Items: items}
				})
		}
		return c.clientset.AppsV1().StatefulSets(namespace).List(ctx, opts)
	})
}

func (c *Client) GetCronJobs(namespace string, ctx context.Context, opts metav1.ListOptions) (*batchv1.CronJobList, error) {
	return cachedGet(c, "cronjobs", namespace, opts, func() (*batchv1.CronJobList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetCronJobs", &batchv1.CronJobList{}, derefSlice(c.mockModel.CronJobs),
				func(items []batchv1.CronJob) *batchv1.CronJobList {
					return &batchv1.CronJobList{Items: items}
				})
		}
		return c.clientset.BatchV1().CronJobs(namespace).List(ctx, opts)
	})
}

func (c *Client) GetJobs(namespace string, ctx context.Context, opts metav1.ListOptions) (*batchv1.JobList, error) {
	return cachedGet(c, "jobs", namespace, opts, func() (*batchv1.JobList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetJobs", &batchv1.JobList{}, derefSlice(c.mockModel.Jobs),
				func(items []batchv1.Job) *batchv1.JobList {
					return &batchv1.JobList{Items: items}
				})
		}
		return c.clientset.BatchV1().Jobs(namespace).List(ctx, opts)
	})
}

func (c *Client) GetConfigMaps(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.ConfigMapList, error) {
	return cachedGet(c, "configmaps", namespace, opts, func() (*corev1.ConfigMapList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetConfigMaps", &corev1.ConfigMapList{}, derefSlice(c.mockModel.ConfigMaps),
				func(items []corev1.ConfigMap) *corev1.ConfigMapList {
					return &corev1.ConfigMapList{Items: items}
				})
		}
		return c.clientset.CoreV1().ConfigMaps(namespace).List(ctx, opts)
	})
}

func (c *Client) GetEndpoints(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.EndpointsList, error) {
	return cachedGet(c, "endpoints", namespace, opts, func() (*corev1.EndpointsList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetEndpoints", &corev1.EndpointsList{}, derefSlice(c.mockModel.Endpoints),
				func(items []corev1.Endpoints) *corev1.EndpointsList {
					return &corev1.EndpointsList{Items: items}
				})
		}
		return c.clientset.CoreV1().Endpoints(namespace).List(ctx, opts)
	})
}

func (c *Client) GetEventsForObject(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.EventList, error) {
	if c.mockMode {
		return mockList(c.mockConfig, "GetEventsForObject", &corev1.EventList{}, derefSlice(c.mockModel.Events),
			func(items []corev1.Event) *corev1.EventList {
				return &corev1.EventList{Items: items}
			})
	}
	return cachedGet(c, "events", namespace, opts, func() (*corev1.EventList, error) {
		return c.clientset.CoreV1().Events(namespace).List(ctx, opts)
	})
}

func (c *Client) GetPodLogs(namespace, name string) (string, error) {
	slog.Debug("provider call", "provider", map[bool]string{true: "mock", false: "cluster"}[c.mockMode], "resource", "podlogs", "namespace", namespace, "name", name)
	if c.mockMode {
		key := namespace + "/" + name
		if logs, ok := c.mockModel.PodLogs[key]; ok {
			slog.Debug("provider call succeeded", "provider", "mock", "resource", "podlogs", "namespace", namespace, "name", name, "bytes", len(logs))
			return logs, nil
		}
		slog.Debug("provider call succeeded", "provider", "mock", "resource", "podlogs", "namespace", namespace, "name", name, "bytes", 0)
		return "", nil
	}
	tailLines := int64(100)
	req := c.clientset.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{TailLines: &tailLines})
	logs, err := req.DoRaw(context.Background())
	if err != nil {
		slog.Debug("provider call failed", "provider", "cluster", "resource", "podlogs", "namespace", namespace, "name", name, "error", err)
		return "", err
	}
	slog.Debug("provider call succeeded", "provider", "cluster", "resource", "podlogs", "namespace", namespace, "name", name, "bytes", len(logs))
	return string(logs), nil
}

func (c *Client) GetNamespaces(ctx context.Context, opts metav1.ListOptions) (*corev1.NamespaceList, error) {
	return cachedGet(c, "namespaces", "", opts, func() (*corev1.NamespaceList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetNamespaces", &corev1.NamespaceList{}, derefSlice(c.mockModel.Namespaces),
				func(items []corev1.Namespace) *corev1.NamespaceList {
					return &corev1.NamespaceList{Items: items}
				})
		}
		return c.clientset.CoreV1().Namespaces().List(ctx, opts)
	})
}

func (c *Client) GetNodes(ctx context.Context, opts metav1.ListOptions) (*corev1.NodeList, error) {
	return cachedGet(c, "nodes", "", opts, func() (*corev1.NodeList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetNodes", &corev1.NodeList{}, derefSlice(c.mockModel.Nodes),
				func(items []corev1.Node) *corev1.NodeList {
					return &corev1.NodeList{Items: items}
				})
		}
		return c.clientset.CoreV1().Nodes().List(ctx, opts)
	})
}

func (c *Client) GetPersistentVolumeClaims(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.PersistentVolumeClaimList, error) {
	return cachedGet(c, "persistentvolumeclaims", namespace, opts, func() (*corev1.PersistentVolumeClaimList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetPersistentVolumeClaims", &corev1.PersistentVolumeClaimList{}, derefSlice(c.mockModel.PersistentVolumeClaims),
				func(items []corev1.PersistentVolumeClaim) *corev1.PersistentVolumeClaimList {
					return &corev1.PersistentVolumeClaimList{Items: items}
				})
		}
		return c.clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, opts)
	})
}

func (c *Client) GetPersistentVolumes(ctx context.Context, opts metav1.ListOptions) (*corev1.PersistentVolumeList, error) {
	return cachedGet(c, "persistentvolumes", "", opts, func() (*corev1.PersistentVolumeList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetPersistentVolumes", &corev1.PersistentVolumeList{}, derefSlice(c.mockModel.PersistentVolumes),
				func(items []corev1.PersistentVolume) *corev1.PersistentVolumeList {
					return &corev1.PersistentVolumeList{Items: items}
				})
		}
		return c.clientset.CoreV1().PersistentVolumes().List(ctx, opts)
	})
}

func (c *Client) GetPods(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.PodList, error) {
	return cachedGet(c, "pods", namespace, opts, func() (*corev1.PodList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetPods", &corev1.PodList{}, derefSlice(c.mockModel.Pods),
				func(items []corev1.Pod) *corev1.PodList {
					return &corev1.PodList{Items: items}
				})
		}
		return c.clientset.CoreV1().Pods(namespace).List(ctx, opts)
	})
}

func (c *Client) GetSecrets(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.SecretList, error) {
	return cachedGet(c, "secrets", namespace, opts, func() (*corev1.SecretList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetSecrets", &corev1.SecretList{}, derefSlice(c.mockModel.Secrets),
				func(items []corev1.Secret) *corev1.SecretList {
					return &corev1.SecretList{Items: items}
				})
		}
		return c.clientset.CoreV1().Secrets(namespace).List(ctx, opts)
	})
}

func (c *Client) GetServices(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.ServiceList, error) {
	return cachedGet(c, "services", namespace, opts, func() (*corev1.ServiceList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetServices", &corev1.ServiceList{}, derefSlice(c.mockModel.Services),
				func(items []corev1.Service) *corev1.ServiceList {
					return &corev1.ServiceList{Items: items}
				})
		}
		return c.clientset.CoreV1().Services(namespace).List(ctx, opts)
	})
}

func (c *Client) GetServiceAccounts(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.ServiceAccountList, error) {
	return cachedGet(c, "serviceaccounts", namespace, opts, func() (*corev1.ServiceAccountList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetServiceAccounts", &corev1.ServiceAccountList{}, derefSlice(c.mockModel.ServiceAccounts),
				func(items []corev1.ServiceAccount) *corev1.ServiceAccountList {
					return &corev1.ServiceAccountList{Items: items}
				})
		}
		return c.clientset.CoreV1().ServiceAccounts(namespace).List(ctx, opts)
	})
}

func (c *Client) GetLimitRanges(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.LimitRangeList, error) {
	return cachedGet(c, "limitranges", namespace, opts, func() (*corev1.LimitRangeList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetLimitRanges", &corev1.LimitRangeList{}, derefSlice(c.mockModel.LimitRanges),
				func(items []corev1.LimitRange) *corev1.LimitRangeList {
					return &corev1.LimitRangeList{Items: items}
				})
		}
		return c.clientset.CoreV1().LimitRanges(namespace).List(ctx, opts)
	})
}

func (c *Client) GetResourceQuotas(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.ResourceQuotaList, error) {
	return cachedGet(c, "resourcequotas", namespace, opts, func() (*corev1.ResourceQuotaList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetResourceQuotas", &corev1.ResourceQuotaList{}, derefSlice(c.mockModel.ResourceQuotas),
				func(items []corev1.ResourceQuota) *corev1.ResourceQuotaList {
					return &corev1.ResourceQuotaList{Items: items}
				})
		}
		return c.clientset.CoreV1().ResourceQuotas(namespace).List(ctx, opts)
	})
}

func (c *Client) GetEndpointSlices(namespace string, ctx context.Context, opts metav1.ListOptions) (*discoveryv1.EndpointSliceList, error) {
	return cachedGet(c, "endpointslices", namespace, opts, func() (*discoveryv1.EndpointSliceList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetEndpointSlices", &discoveryv1.EndpointSliceList{}, derefSlice(c.mockModel.EndpointSlices),
				func(items []discoveryv1.EndpointSlice) *discoveryv1.EndpointSliceList {
					return &discoveryv1.EndpointSliceList{Items: items}
				})
		}
		return c.clientset.DiscoveryV1().EndpointSlices(namespace).List(ctx, opts)
	})
}

func (c *Client) GetIngresses(namespace string, ctx context.Context, opts metav1.ListOptions) (*networkingv1.IngressList, error) {
	return cachedGet(c, "ingresses", namespace, opts, func() (*networkingv1.IngressList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetIngresses", &networkingv1.IngressList{}, derefSlice(c.mockModel.Ingresses),
				func(items []networkingv1.Ingress) *networkingv1.IngressList {
					return &networkingv1.IngressList{Items: items}
				})
		}
		return c.clientset.NetworkingV1().Ingresses(namespace).List(ctx, opts)
	})
}

func (c *Client) GetNetworkPolicies(namespace string, ctx context.Context, opts metav1.ListOptions) (*networkingv1.NetworkPolicyList, error) {
	return cachedGet(c, "networkpolicies", namespace, opts, func() (*networkingv1.NetworkPolicyList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetNetworkPolicies", &networkingv1.NetworkPolicyList{}, derefSlice(c.mockModel.NetworkPolicies),
				func(items []networkingv1.NetworkPolicy) *networkingv1.NetworkPolicyList {
					return &networkingv1.NetworkPolicyList{Items: items}
				})
		}
		return c.clientset.NetworkingV1().NetworkPolicies(namespace).List(ctx, opts)
	})
}

func (c *Client) GetIngressClasses(ctx context.Context, opts metav1.ListOptions) (*networkingv1.IngressClassList, error) {
	return cachedGet(c, "ingressclasses", "", opts, func() (*networkingv1.IngressClassList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetIngressClasses", &networkingv1.IngressClassList{}, derefSlice(c.mockModel.IngressClasses),
				func(items []networkingv1.IngressClass) *networkingv1.IngressClassList {
					return &networkingv1.IngressClassList{Items: items}
				})
		}
		return c.clientset.NetworkingV1().IngressClasses().List(ctx, opts)
	})
}

func (c *Client) GetPodDisruptionBudgets(namespace string, ctx context.Context, opts metav1.ListOptions) (*policyv1.PodDisruptionBudgetList, error) {
	return cachedGet(c, "poddisruptionbudgets", namespace, opts, func() (*policyv1.PodDisruptionBudgetList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetPodDisruptionBudgets", &policyv1.PodDisruptionBudgetList{}, derefSlice(c.mockModel.PodDisruptionBudgets),
				func(items []policyv1.PodDisruptionBudget) *policyv1.PodDisruptionBudgetList {
					return &policyv1.PodDisruptionBudgetList{Items: items}
				})
		}
		return c.clientset.PolicyV1().PodDisruptionBudgets(namespace).List(ctx, opts)
	})
}

func (c *Client) GetClusterRoleBindings(ctx context.Context, opts metav1.ListOptions) (*rbacv1.ClusterRoleBindingList, error) {
	return cachedGet(c, "clusterrolebindings", "", opts, func() (*rbacv1.ClusterRoleBindingList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetClusterRoleBindings", &rbacv1.ClusterRoleBindingList{}, derefSlice(c.mockModel.ClusterRoleBindings),
				func(items []rbacv1.ClusterRoleBinding) *rbacv1.ClusterRoleBindingList {
					return &rbacv1.ClusterRoleBindingList{Items: items}
				})
		}
		return c.clientset.RbacV1().ClusterRoleBindings().List(ctx, opts)
	})
}

func (c *Client) GetClusterRoles(ctx context.Context, opts metav1.ListOptions) (*rbacv1.ClusterRoleList, error) {
	return cachedGet(c, "clusterroles", "", opts, func() (*rbacv1.ClusterRoleList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetClusterRoles", &rbacv1.ClusterRoleList{}, derefSlice(c.mockModel.ClusterRoles),
				func(items []rbacv1.ClusterRole) *rbacv1.ClusterRoleList {
					return &rbacv1.ClusterRoleList{Items: items}
				})
		}
		return c.clientset.RbacV1().ClusterRoles().List(ctx, opts)
	})
}

func (c *Client) GetRoleBindings(namespace string, ctx context.Context, opts metav1.ListOptions) (*rbacv1.RoleBindingList, error) {
	return cachedGet(c, "rolebindings", namespace, opts, func() (*rbacv1.RoleBindingList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetRoleBindings", &rbacv1.RoleBindingList{}, derefSlice(c.mockModel.RoleBindings),
				func(items []rbacv1.RoleBinding) *rbacv1.RoleBindingList {
					return &rbacv1.RoleBindingList{Items: items}
				})
		}
		return c.clientset.RbacV1().RoleBindings(namespace).List(ctx, opts)
	})
}

func (c *Client) GetRoles(namespace string, ctx context.Context, opts metav1.ListOptions) (*rbacv1.RoleList, error) {
	return cachedGet(c, "roles", namespace, opts, func() (*rbacv1.RoleList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetRoles", &rbacv1.RoleList{}, derefSlice(c.mockModel.Roles),
				func(items []rbacv1.Role) *rbacv1.RoleList {
					return &rbacv1.RoleList{Items: items}
				})
		}
		return c.clientset.RbacV1().Roles(namespace).List(ctx, opts)
	})
}

func (c *Client) GetCSIDrivers(ctx context.Context, opts metav1.ListOptions) (*storagev1.CSIDriverList, error) {
	return cachedGet(c, "csidrivers", "", opts, func() (*storagev1.CSIDriverList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetCSIDrivers", &storagev1.CSIDriverList{}, derefSlice(c.mockModel.CSIDrivers),
				func(items []storagev1.CSIDriver) *storagev1.CSIDriverList {
					return &storagev1.CSIDriverList{Items: items}
				})
		}
		return c.clientset.StorageV1().CSIDrivers().List(ctx, opts)
	})
}

func (c *Client) GetCSINodes(ctx context.Context, opts metav1.ListOptions) (*storagev1.CSINodeList, error) {
	return cachedGet(c, "csinodes", "", opts, func() (*storagev1.CSINodeList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetCSINodes", &storagev1.CSINodeList{}, derefSlice(c.mockModel.CSINodes),
				func(items []storagev1.CSINode) *storagev1.CSINodeList {
					return &storagev1.CSINodeList{Items: items}
				})
		}
		return c.clientset.StorageV1().CSINodes().List(ctx, opts)
	})
}

func (c *Client) GetStorageClasses(ctx context.Context, opts metav1.ListOptions) (*storagev1.StorageClassList, error) {
	return cachedGet(c, "storageclasses", "", opts, func() (*storagev1.StorageClassList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetStorageClasses", &storagev1.StorageClassList{}, derefSlice(c.mockModel.StorageClasses),
				func(items []storagev1.StorageClass) *storagev1.StorageClassList {
					return &storagev1.StorageClassList{Items: items}
				})
		}
		return c.clientset.StorageV1().StorageClasses().List(ctx, opts)
	})
}

func (c *Client) GetVolumeAttachments(ctx context.Context, opts metav1.ListOptions) (*storagev1.VolumeAttachmentList, error) {
	return cachedGet(c, "volumeattachments", "", opts, func() (*storagev1.VolumeAttachmentList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetVolumeAttachments", &storagev1.VolumeAttachmentList{}, derefSlice(c.mockModel.VolumeAttachments),
				func(items []storagev1.VolumeAttachment) *storagev1.VolumeAttachmentList {
					return &storagev1.VolumeAttachmentList{Items: items}
				})
		}
		return c.clientset.StorageV1().VolumeAttachments().List(ctx, opts)
	})
}

func (c *Client) GetHorizontalPodAutoscalers(namespace string, ctx context.Context, opts metav1.ListOptions) (*autoscalingv1.HorizontalPodAutoscalerList, error) {
	return cachedGet(c, "horizontalpodautoscalers", namespace, opts, func() (*autoscalingv1.HorizontalPodAutoscalerList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetHorizontalPodAutoscalers", &autoscalingv1.HorizontalPodAutoscalerList{}, derefSlice(c.mockModel.HorizontalPodAutoscalers),
				func(items []autoscalingv1.HorizontalPodAutoscaler) *autoscalingv1.HorizontalPodAutoscalerList {
					return &autoscalingv1.HorizontalPodAutoscalerList{Items: items}
				})
		}
		return c.clientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).List(ctx, opts)
	})
}

func (c *Client) GetCertificateSigningRequests(ctx context.Context, opts metav1.ListOptions) (*certificatesv1.CertificateSigningRequestList, error) {
	return cachedGet(c, "certificatesigningrequests", "", opts, func() (*certificatesv1.CertificateSigningRequestList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetCertificateSigningRequests", &certificatesv1.CertificateSigningRequestList{}, derefSlice(c.mockModel.CertificateSigningRequests),
				func(items []certificatesv1.CertificateSigningRequest) *certificatesv1.CertificateSigningRequestList {
					return &certificatesv1.CertificateSigningRequestList{Items: items}
				})
		}
		return c.clientset.CertificatesV1().CertificateSigningRequests().List(ctx, opts)
	})
}

func (c *Client) GetFlowSchemas(ctx context.Context, opts metav1.ListOptions) (*flowcontrolv1.FlowSchemaList, error) {
	return cachedGet(c, "flowschemas", "", opts, func() (*flowcontrolv1.FlowSchemaList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetFlowSchemas", &flowcontrolv1.FlowSchemaList{}, derefSlice(c.mockModel.FlowSchemas),
				func(items []flowcontrolv1.FlowSchema) *flowcontrolv1.FlowSchemaList {
					return &flowcontrolv1.FlowSchemaList{Items: items}
				})
		}
		return c.clientset.FlowcontrolV1().FlowSchemas().List(ctx, opts)
	})
}

func (c *Client) GetPriorityLevelConfigurations(ctx context.Context, opts metav1.ListOptions) (*flowcontrolv1.PriorityLevelConfigurationList, error) {
	return cachedGet(c, "prioritylevelconfigurations", "", opts, func() (*flowcontrolv1.PriorityLevelConfigurationList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetPriorityLevelConfigurations", &flowcontrolv1.PriorityLevelConfigurationList{}, derefSlice(c.mockModel.PriorityLevelConfigurations),
				func(items []flowcontrolv1.PriorityLevelConfiguration) *flowcontrolv1.PriorityLevelConfigurationList {
					return &flowcontrolv1.PriorityLevelConfigurationList{Items: items}
				})
		}
		return c.clientset.FlowcontrolV1().PriorityLevelConfigurations().List(ctx, opts)
	})
}

func (c *Client) GetRuntimeClasses(ctx context.Context, opts metav1.ListOptions) (*nodev1.RuntimeClassList, error) {
	return cachedGet(c, "runtimeclasses", "", opts, func() (*nodev1.RuntimeClassList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetRuntimeClasses", &nodev1.RuntimeClassList{}, derefSlice(c.mockModel.RuntimeClasses),
				func(items []nodev1.RuntimeClass) *nodev1.RuntimeClassList {
					return &nodev1.RuntimeClassList{Items: items}
				})
		}
		return c.clientset.NodeV1().RuntimeClasses().List(ctx, opts)
	})
}

func (c *Client) GetPriorityClasses(ctx context.Context, opts metav1.ListOptions) (*schedulingv1.PriorityClassList, error) {
	return cachedGet(c, "priorityclasses", "", opts, func() (*schedulingv1.PriorityClassList, error) {
		if c.mockMode {
			return mockList(c.mockConfig, "GetPriorityClasses", &schedulingv1.PriorityClassList{}, derefSlice(c.mockModel.PriorityClasses),
				func(items []schedulingv1.PriorityClass) *schedulingv1.PriorityClassList {
					return &schedulingv1.PriorityClassList{Items: items}
				})
		}
		return c.clientset.SchedulingV1().PriorityClasses().List(ctx, opts)
	})
}

func (c *Client) GetCustomResourceDefinitions(ctx context.Context, opts metav1.ListOptions) (*apiextensionsv1.CustomResourceDefinitionList, error) {
	if c.mockMode {
		return &apiextensionsv1.CustomResourceDefinitionList{}, nil
	}
	return &apiextensionsv1.CustomResourceDefinitionList{}, nil
}

func (c *Client) GetCiliumNetworkPolicies(namespace string, ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error) {
	if c.mockMode {
		return mockMapList(c.mockConfig, "GetCiliumNetworkPolicies", c.mockModel.CiliumNetworkPolicies)
	}
	return listDynamicResourceObjects(c.dynamicClient, schema.GroupVersionResource{
		Group:    "cilium.io",
		Version:  "v2",
		Resource: "ciliumnetworkpolicies",
	}, namespace, true, ctx, opts)
}

func (c *Client) GetCiliumClusterwideNetworkPolicies(ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error) {
	if c.mockMode {
		return mockMapList(c.mockConfig, "GetCiliumClusterwideNetworkPolicies", c.mockModel.CiliumClusterwideNetworkPolicies)
	}
	return listDynamicResourceObjects(c.dynamicClient, schema.GroupVersionResource{
		Group:    "cilium.io",
		Version:  "v2",
		Resource: "ciliumclusterwidenetworkpolicies",
	}, "", false, ctx, opts)
}

func (c *Client) GetHTTPRoutes(namespace string, ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error) {
	if c.mockMode {
		return mockMapList(c.mockConfig, "GetHTTPRoutes", c.mockModel.HTTPRoutes)
	}
	return listDynamicResourceObjects(c.dynamicClient, schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "httproutes",
	}, namespace, true, ctx, opts)
}

func (c *Client) GetGateways(namespace string, ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error) {
	if c.mockMode {
		return mockMapList(c.mockConfig, "GetGateways", c.mockModel.Gateways)
	}
	return listDynamicResourceObjects(c.dynamicClient, schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "gateways",
	}, namespace, true, ctx, opts)
}

func (c *Client) GetCertificates(namespace string, ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error) {
	if c.mockMode {
		return mockMapList(c.mockConfig, "GetCertificates", c.mockModel.Certificates)
	}
	return listDynamicResourceObjects(c.dynamicClient, schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1",
		Resource: "certificates",
	}, namespace, true, ctx, opts)
}

func (c *Client) GetIssuers(namespace string, ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error) {
	if c.mockMode {
		return mockMapList(c.mockConfig, "GetIssuers", c.mockModel.Issuers)
	}
	return listDynamicResourceObjects(c.dynamicClient, schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1",
		Resource: "issuers",
	}, namespace, true, ctx, opts)
}

func (c *Client) GetClusterIssuers(ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error) {
	if c.mockMode {
		return mockMapList(c.mockConfig, "GetClusterIssuers", c.mockModel.ClusterIssuers)
	}
	return listDynamicResourceObjects(c.dynamicClient, schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1",
		Resource: "clusterissuers",
	}, "", false, ctx, opts)
}

func (c *Client) GetSecretProviderClasses(namespace string, ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error) {
	if c.mockMode {
		return mockMapList(c.mockConfig, "GetSecretProviderClasses", c.mockModel.SecretProviderClasses)
	}
	return listDynamicResourceObjects(c.dynamicClient, schema.GroupVersionResource{
		Group:    "secrets-store.csi.x-k8s.io",
		Version:  "v1",
		Resource: "secretproviderclasses",
	}, namespace, true, ctx, opts)
}
