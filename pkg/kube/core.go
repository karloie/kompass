package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type warningHandler struct{}

const (
	defaultRequestTimeout = 15 * time.Second
	requestTimeoutEnvVar  = "KOMPASS_K8S_TIMEOUT"
)

func configureRequestTimeout(config *rest.Config) error {
	if config == nil || config.Timeout > 0 {
		return nil
	}

	config.Timeout = defaultRequestTimeout

	if raw := strings.TrimSpace(os.Getenv(requestTimeoutEnvVar)); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil {
			if seconds <= 0 {
				return fmt.Errorf("%s must be > 0 seconds when using integer format", requestTimeoutEnvVar)
			}
			config.Timeout = time.Duration(seconds) * time.Second
			return nil
		}

		timeout, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("invalid %s value %q: %w", requestTimeoutEnvVar, raw, err)
		}
		if timeout <= 0 {
			return fmt.Errorf("%s must be > 0", requestTimeoutEnvVar)
		}
		config.Timeout = timeout
	}

	return nil
}

func (warningHandler) HandleWarningHeader(code int, agent string, message string) {
	if strings.Contains(message, "Endpoints is deprecated") {
		return
	}
}

const jsonAPIVersion = "v1"

type CRDSelector struct {
	Kind      string
	Namespace string
}

type Request struct {
	Context     string        `json:"context,omitempty"`
	Namespace   string        `json:"namespace,omitempty"`
	ConfigPath  string        `json:"configPath,omitempty"`
	KeySelector string        `json:"keySelector"`
	CRDSelector []CRDSelector `json:"crdSelector"`
}

type Response struct {
	APIVersion string               `json:"apiVersion"`
	Request    Request              `json:"request"`
	Nodes      map[string]*Resource `json:"nodes,omitempty"`
	Graphs     []Graph              `json:"graphs"`
	Trees      []Tree               `json:"trees"`
	Metadata   *Metadata            `json:"metadata,omitempty"`
}

type Metadata struct {
	CacheEnabled      bool          `json:"cacheEnabled"`
	CacheSize         int           `json:"cacheSize"`
	CacheLastSync     time.Time     `json:"cacheLastSync"`
	CacheSyncInterval time.Duration `json:"cacheSyncInterval"`
	CacheTTL          time.Duration `json:"cacheTTL"`
	CacheCalls        int64         `json:"cacheCalls"`
	CacheHits         int64         `json:"cacheHits"`
	CacheMisses       int64         `json:"cacheMisses"`
	CacheHitRate      float64       `json:"cacheHitRate"`
}

type Graph struct {
	ID    string         `json:"id"`
	Edges []ResourceEdge `json:"edges,omitempty"`
}

type Tree struct {
	Key      string         `json:"key"`
	Type     string         `json:"type"`
	Icon     string         `json:"icon,omitempty"`
	Meta     map[string]any `json:"metadata"`
	Children []*Tree        `json:"children"`
}

type Resource struct {
	Key        string `json:"key"`
	Type       string `json:"type"`
	Resource   any    `json:"resource"`
	Discovered bool   `json:"discovered,omitempty"`
	Error      string `json:"error,omitempty"`
}

func (r *Resource) AsMap() map[string]any {
	if r.Resource == nil {
		return nil
	}

	if m, ok := r.Resource.(map[string]any); ok {
		return m
	}

	var m map[string]any
	b, err := json.Marshal(r.Resource)
	if err != nil {
		return map[string]any{"marshalError": err.Error()}
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{"unmarshalError": err.Error()}
	}
	return m
}

type ResourceEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
	Error  string `json:"error,omitempty"`
	Debug  string `json:"debug,omitempty"`
}

type Client struct {
	clientset      kubernetes.Interface
	dynamicClient  dynamic.Interface
	config         *rest.Config
	kubeconfig     string
	context        string
	namespace      string
	mockMode       bool
	mockConfig     MockConfig
	mockModel      *InMemoryModel
	cache          *resourceCache
	cacheEnabled   bool
	cacheTTL       time.Duration
	syncInterval   time.Duration
	syncCtx        context.Context
	syncCancel     context.CancelFunc
	syncNamespaces []string
	lastSyncTime   time.Time
	lastSyncMutex  sync.RWMutex

	rateLimiter    *rate.Limiter
	maxRetries     int
	initialBackoff time.Duration
	maxBackoff     time.Duration

	cacheCalls  int64
	cacheHits   int64
	cacheMisses int64
	cacheMutex  sync.RWMutex
}

type CiliumProvider interface {
	GetCiliumNetworkPolicies(namespace string, ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error)
	GetCiliumClusterwideNetworkPolicies(ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error)
}

type CertManagerProvider interface {
	GetCertificates(namespace string, ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error)
	GetIssuers(namespace string, ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error)
	GetClusterIssuers(ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error)
}

type GatewayProvider interface {
	GetHTTPRoutes(namespace string, ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error)
	GetGateways(namespace string, ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error)
}

type SecretsStoreProvider interface {
	GetSecretProviderClasses(namespace string, ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error)
}

type MockConfig struct {
	AllEmpty bool
	AllError bool
	Methods  map[string]MockMethodBehavior
}

type MockMethodBehavior struct {
	ReturnError  bool
	ReturnEmpty  bool
	ErrorMessage string
}

type InMemoryModel struct {
	ServiceAccounts                  []*corev1.ServiceAccount
	ConfigMaps                       []*corev1.ConfigMap
	Events                           []*corev1.Event
	Namespaces                       []*corev1.Namespace
	Nodes                            []*corev1.Node
	PersistentVolumeClaims           []*corev1.PersistentVolumeClaim
	PersistentVolumes                []*corev1.PersistentVolume
	Pods                             []*corev1.Pod
	Secrets                          []*corev1.Secret
	Services                         []*corev1.Service
	Endpoints                        []*corev1.Endpoints
	LimitRanges                      []*corev1.LimitRange
	ResourceQuotas                   []*corev1.ResourceQuota
	Deployments                      []*appsv1.Deployment
	DaemonSets                       []*appsv1.DaemonSet
	ReplicaSets                      []*appsv1.ReplicaSet
	StatefulSets                     []*appsv1.StatefulSet
	Jobs                             []*batchv1.Job
	CronJobs                         []*batchv1.CronJob
	ClusterRoleBindings              []*rbacv1.ClusterRoleBinding
	ClusterRoles                     []*rbacv1.ClusterRole
	RoleBindings                     []*rbacv1.RoleBinding
	Roles                            []*rbacv1.Role
	Ingresses                        []*networkingv1.Ingress
	NetworkPolicies                  []*networkingv1.NetworkPolicy
	IngressClasses                   []*networkingv1.IngressClass
	EndpointSlices                   []*discoveryv1.EndpointSlice
	PodDisruptionBudgets             []*policyv1.PodDisruptionBudget
	CSIDrivers                       []*storagev1.CSIDriver
	CSINodes                         []*storagev1.CSINode
	StorageClasses                   []*storagev1.StorageClass
	VolumeAttachments                []*storagev1.VolumeAttachment
	HorizontalPodAutoscalers         []*autoscalingv1.HorizontalPodAutoscaler
	CertificateSigningRequests       []*certificatesv1.CertificateSigningRequest
	FlowSchemas                      []*flowcontrolv1.FlowSchema
	PriorityLevelConfigurations      []*flowcontrolv1.PriorityLevelConfiguration
	RuntimeClasses                   []*nodev1.RuntimeClass
	PriorityClasses                  []*schedulingv1.PriorityClass
	CiliumNetworkPolicies            []map[string]any
	CiliumClusterwideNetworkPolicies []map[string]any
	Certificates                     []map[string]any
	Issuers                          []map[string]any
	ClusterIssuers                   []map[string]any
	HTTPRoutes                       []map[string]any
	Gateways                         []map[string]any
	SecretProviderClasses            []map[string]any
	CRDs                             []map[string]any
	Egresses                         []map[string]any
	PodLogs                          map[string]string
}

type Kube interface {
	GetEndpoints(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.EndpointsList, error)
	GetContext() (string, error)
	GetContexts() (any, error)
	SetContext(context string)
	GetConfigPath() (string, error)
	DynamicClient() any
	GetNamespace() (string, error)
	SetNamespace(namespace string)

	GetDaemonSets(namespace string, ctx context.Context, opts metav1.ListOptions) (*appsv1.DaemonSetList, error)
	GetDeployments(namespace string, ctx context.Context, opts metav1.ListOptions) (*appsv1.DeploymentList, error)
	GetReplicaSets(namespace string, ctx context.Context, opts metav1.ListOptions) (*appsv1.ReplicaSetList, error)
	GetStatefulSets(namespace string, ctx context.Context, opts metav1.ListOptions) (*appsv1.StatefulSetList, error)

	GetCronJobs(namespace string, ctx context.Context, opts metav1.ListOptions) (*batchv1.CronJobList, error)
	GetJobs(namespace string, ctx context.Context, opts metav1.ListOptions) (*batchv1.JobList, error)

	GetConfigMaps(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.ConfigMapList, error)
	GetEventsForObject(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.EventList, error)

	GetPodLogs(namespace, name string) (string, error)
	GetNamespaces(ctx context.Context, opts metav1.ListOptions) (*corev1.NamespaceList, error)
	GetNodes(ctx context.Context, opts metav1.ListOptions) (*corev1.NodeList, error)
	GetPersistentVolumeClaims(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.PersistentVolumeClaimList, error)
	GetPersistentVolumes(ctx context.Context, opts metav1.ListOptions) (*corev1.PersistentVolumeList, error)
	GetPods(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.PodList, error)
	GetSecrets(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.SecretList, error)
	GetServices(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.ServiceList, error)
	GetLimitRanges(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.LimitRangeList, error)
	GetResourceQuotas(namespace string, ctx context.Context, opts metav1.ListOptions) (*corev1.ResourceQuotaList, error)

	GetEndpointSlices(namespace string, ctx context.Context, opts metav1.ListOptions) (*discoveryv1.EndpointSliceList, error)

	GetIngresses(namespace string, ctx context.Context, opts metav1.ListOptions) (*networkingv1.IngressList, error)
	GetNetworkPolicies(namespace string, ctx context.Context, opts metav1.ListOptions) (*networkingv1.NetworkPolicyList, error)
	GetIngressClasses(ctx context.Context, opts metav1.ListOptions) (*networkingv1.IngressClassList, error)

	GetPodDisruptionBudgets(namespace string, ctx context.Context, opts metav1.ListOptions) (*policyv1.PodDisruptionBudgetList, error)

	GetClusterRoleBindings(ctx context.Context, opts metav1.ListOptions) (*rbacv1.ClusterRoleBindingList, error)
	GetClusterRoles(ctx context.Context, opts metav1.ListOptions) (*rbacv1.ClusterRoleList, error)
	GetRoleBindings(namespace string, ctx context.Context, opts metav1.ListOptions) (*rbacv1.RoleBindingList, error)
	GetRoles(namespace string, ctx context.Context, opts metav1.ListOptions) (*rbacv1.RoleList, error)

	GetCSIDrivers(ctx context.Context, opts metav1.ListOptions) (*storagev1.CSIDriverList, error)
	GetCSINodes(ctx context.Context, opts metav1.ListOptions) (*storagev1.CSINodeList, error)
	GetStorageClasses(ctx context.Context, opts metav1.ListOptions) (*storagev1.StorageClassList, error)
	GetVolumeAttachments(ctx context.Context, opts metav1.ListOptions) (*storagev1.VolumeAttachmentList, error)

	GetHorizontalPodAutoscalers(namespace string, ctx context.Context, opts metav1.ListOptions) (*autoscalingv1.HorizontalPodAutoscalerList, error)

	GetCertificateSigningRequests(ctx context.Context, opts metav1.ListOptions) (*certificatesv1.CertificateSigningRequestList, error)

	GetFlowSchemas(ctx context.Context, opts metav1.ListOptions) (*flowcontrolv1.FlowSchemaList, error)
	GetPriorityLevelConfigurations(ctx context.Context, opts metav1.ListOptions) (*flowcontrolv1.PriorityLevelConfigurationList, error)

	GetRuntimeClasses(ctx context.Context, opts metav1.ListOptions) (*nodev1.RuntimeClassList, error)

	GetPriorityClasses(ctx context.Context, opts metav1.ListOptions) (*schedulingv1.PriorityClassList, error)

	GetCustomResourceDefinitions(ctx context.Context, opts metav1.ListOptions) (*apiextensionsv1.CustomResourceDefinitionList, error)
}

func (c *Client) IsMockMode() bool {
	return c.mockMode
}

func (c *Client) GetMockModel() *InMemoryModel {
	return c.mockModel
}

func NewMockClient(model *InMemoryModel, configs ...MockConfig) *Client {
	config := MockConfig{}
	if len(configs) > 0 {
		config = configs[0]
	}
	if model == nil {
		model = &InMemoryModel{}
	}
	return &Client{
		mockMode:       true,
		mockConfig:     config,
		mockModel:      model,
		context:        "mock-cluster",
		namespace:      "default",
		kubeconfig:     "mock",
		cache:          newResourceCache(),
		cacheEnabled:   true,
		cacheTTL:       30 * time.Second,
		syncInterval:   30 * time.Second,
		rateLimiter:    rate.NewLimiter(rate.Limit(100), 50),
		maxRetries:     3,
		initialBackoff: 100 * time.Millisecond,
		maxBackoff:     5 * time.Second,
	}
}

func NewClient(contextName, namespace string) (*Client, error) {
	var config *rest.Config
	var err error
	var kubeconfig string

	config, err = rest.InClusterConfig()
	if err == nil {
		if err := configureRequestTimeout(config); err != nil {
			return nil, err
		}

		config.WarningHandler = warningHandler{}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}
		dynamicClient, err := dynamic.NewForConfig(config)
		if err != nil {
			return nil, err
		}
		if namespace == "" {
			namespace = "default"
		}
		return &Client{
			clientset:     clientset,
			dynamicClient: dynamicClient,
			config:        config,
			kubeconfig:    "in-cluster",
			context:       "in-cluster",
			namespace:     namespace,
			cache:         newResourceCache(),
			cacheEnabled:  true,
			cacheTTL:      30 * time.Second,
			syncInterval:  30 * time.Second,
		}, nil
	}
	kubeconfig = os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	configLoadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	configOverrides := &clientcmd.ConfigOverrides{}

	if contextName != "" {
		configOverrides.CurrentContext = contextName
	}

	if namespace != "" {
		configOverrides.Context.Namespace = namespace
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(configLoadingRules, configOverrides)

	config, err = kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	if err := configureRequestTimeout(config); err != nil {
		return nil, err
	}

	config.WarningHandler = warningHandler{}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	if contextName == "" {
		rawConfig, err := kubeConfig.RawConfig()
		if err == nil {
			contextName = rawConfig.CurrentContext
		}
	}

	if namespace == "" {
		ns, _, err := kubeConfig.Namespace()
		if err == nil && ns != "" {
			namespace = ns
		} else {
			namespace = "default"
		}
	}

	return &Client{
		clientset:      clientset,
		dynamicClient:  dynamicClient,
		config:         config,
		kubeconfig:     kubeconfig,
		context:        contextName,
		namespace:      namespace,
		cache:          newResourceCache(),
		cacheEnabled:   true,
		cacheTTL:       30 * time.Second,
		syncInterval:   30 * time.Second,
		rateLimiter:    rate.NewLimiter(rate.Limit(100), 50),
		maxRetries:     3,
		initialBackoff: 100 * time.Millisecond,
		maxBackoff:     5 * time.Second,
	}, nil
}

// NewClientWithClientset supports injected clients for integration and test usage.
func NewClientWithClientset(clientset kubernetes.Interface, dynamicClient dynamic.Interface, config *rest.Config, contextName, namespace string) *Client {
	if contextName == "" {
		contextName = "injected-cluster"
	}
	if namespace == "" {
		namespace = "default"
	}

	kubeconfig := "injected"
	if config != nil && config.Host != "" {
		kubeconfig = config.Host
	}

	return &Client{
		clientset:      clientset,
		dynamicClient:  dynamicClient,
		config:         config,
		kubeconfig:     kubeconfig,
		context:        contextName,
		namespace:      namespace,
		cache:          newResourceCache(),
		cacheEnabled:   true,
		cacheTTL:       30 * time.Second,
		syncInterval:   30 * time.Second,
		rateLimiter:    rate.NewLimiter(rate.Limit(100), 50),
		maxRetries:     3,
		initialBackoff: 100 * time.Millisecond,
		maxBackoff:     5 * time.Second,
	}
}
