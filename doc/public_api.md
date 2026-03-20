# Kompass Public API (v0.1.0)

This document defines the clean public API surface for Kompass as a library.

## Philosophy

- **Minimal surface**: Only expose what's necessary for downstream apps
- **Two entry points**: `graph.BuildGraphs` and `tree.BuildTrees`
- **Data-centric**: Public types are data structures; implementation details are private

## Public API Surface

### Package: `github.com/karloie/kompass/pkg/graph`

#### Functions
```go
// BuildGraphs constructs a graph of Kubernetes resources and their relationships.
// It loads resources matching the request selectors, analyzes their dependencies,
// and returns a Response containing nodes, edges, and connected components.
func BuildGraphs(provider kube.Provider, request kube.Request) (*kube.Response, error)
```

#### Variables
```go
// ResourceTypes maps resource type names to their metadata (emoji, loaders, handlers)
// Exported for introspection and custom resource type registration
var ResourceTypes map[string]ResourceType
```

#### Types (needed for ResourceTypes)
```go
type ResourceType struct {
	Emoji        string
	Loader       kube.ResourceLoader
	Handler      func(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error
	LeafChildren []string
}
```

---

### Package: `github.com/karloie/kompass/pkg/tree`

#### Functions
```go
// BuildTrees transforms a graph response into a hierarchical tree representation.
// Each connected component becomes a tree with parent-child relationships,
// enriched with metadata for display purposes.
func BuildTrees(response *kube.Response) *kube.Response

```

---

### Package: `github.com/karloie/kompass/pkg/kube`

#### Core Types

```go
// Request contains parameters for building a Kubernetes resource graph
type Request struct {
	Context      string        // Kubernetes context name
	Namespace    string        // Target namespace (empty = current)
	ConfigPath   string        // Path to kubeconfig
	Selectors    []string      // Resource selectors (e.g., "deployment/default/api")
	CRDSelectors []CRDSelector // Custom resource selectors
}

// CRDSelector identifies a custom resource to include
type CRDSelector struct {
	Kind      string
	Namespace string
}

// Response contains the resource graph and optional hierarchical trees
type Response struct {
	APIVersion string         // "v1"
	Request    Request        // Original request
	Nodes      []Resource     // All resources in graph
	Edges      []ResourceEdge // Relationships between resources
	Components []Component    // Connected components (subgraphs)
	Trees      []Tree         // Hierarchical representation (if BuildTrees called)
	Metadata   *Metadata      // Cache stats and timing info
}

// Resource represents a single Kubernetes object
type Resource struct {
	Key      string         // Unique identifier: "type/namespace/name"
	Type     string         // Resource type (lowercase, e.g., "deployment")
	Resource map[string]any // Full Kubernetes object as map
}

// ResourceEdge represents a dependency between two resources
type ResourceEdge struct {
	Source     string // Source resource key
	Target     string // Target resource key
	Type       string // Relationship type (e.g., "owns", "mounts", "routes-to")
	Attributes string // Additional edge metadata
}

// Component represents a connected subgraph
type Component struct {
	ID    string   // Component identifier
	Root  string   // Root resource key
	Nodes []string // All resource keys in component
}

// Tree represents a hierarchical view of a component
type Tree struct {
	Key      string         // Resource key
	Type     string         // Resource type
	Icon     string         // Display emoji
	Meta     map[string]any // Enriched metadata for display
	Children []*Tree        // Child nodes
}

// Metadata contains operational info about the graph build
type Metadata struct {
	CacheHitRate float64       // Cache effectiveness (0.0-1.0)
	CacheStats   string        // Human-readable cache stats
	Timing       string        // Build timing info
	Context      string        // Kubernetes context used
	Namespace    string        // Default namespace
	ClusterName  string        // Cluster identifier
}
```

#### Interface

```go
// Provider is the interface for loading Kubernetes resources
type Provider interface {
	// Resource loaders
	GetPods(namespace string, ctx context.Context, opts metav1.ListOptions) ([]corev1.Pod, error)
	GetServices(namespace string, ctx context.Context, opts metav1.ListOptions) ([]corev1.Service, error)
	GetDeployments(namespace string, ctx context.Context, opts metav1.ListOptions) ([]appsv1.Deployment, error)
	GetReplicaSets(namespace string, ctx context.Context, opts metav1.ListOptions) ([]appsv1.ReplicaSet, error)
	GetStatefulSets(namespace string, ctx context.Context, opts metav1.ListOptions) ([]appsv1.StatefulSet, error)
	GetDaemonSets(namespace string, ctx context.Context, opts metav1.ListOptions) ([]appsv1.DaemonSet, error)
	GetJobs(namespace string, ctx context.Context, opts metav1.ListOptions) ([]batchv1.Job, error)
	GetCronJobs(namespace string, ctx context.Context, opts metav1.ListOptions) ([]batchv1.CronJob, error)
	GetConfigMaps(namespace string, ctx context.Context, opts metav1.ListOptions) ([]corev1.ConfigMap, error)
	GetSecrets(namespace string, ctx context.Context, opts metav1.ListOptions) ([]corev1.Secret, error)
	GetServiceAccounts(namespace string, ctx context.Context, opts metav1.ListOptions) ([]corev1.ServiceAccount, error)
	GetPersistentVolumes(ctx context.Context, opts metav1.ListOptions) ([]corev1.PersistentVolume, error)
	GetPersistentVolumeClaims(namespace string, ctx context.Context, opts metav1.ListOptions) ([]corev1.PersistentVolumeClaim, error)
	GetStorageClasses(ctx context.Context, opts metav1.ListOptions) ([]storagev1.StorageClass, error)
	GetNetworkPolicies(namespace string, ctx context.Context, opts metav1.ListOptions) ([]networkingv1.NetworkPolicy, error)
	GetIngresses(namespace string, ctx context.Context, opts metav1.ListOptions) ([]networkingv1.Ingress, error)
	GetNodes(ctx context.Context, opts metav1.ListOptions) ([]corev1.Node, error)
	GetNamespaces(ctx context.Context, opts metav1.ListOptions) ([]corev1.Namespace, error)
	GetEndpoints(namespace string, ctx context.Context, opts metav1.ListOptions) ([]corev1.Endpoints, error)
	GetEndpointSlices(namespace string, ctx context.Context, opts metav1.ListOptions) ([]discoveryv1.EndpointSlice, error)
	GetRoles(namespace string, ctx context.Context, opts metav1.ListOptions) ([]rbacv1.Role, error)
	GetClusterRoles(ctx context.Context, opts metav1.ListOptions) ([]rbacv1.ClusterRole, error)
	GetRoleBindings(namespace string, ctx context.Context, opts metav1.ListOptions) ([]rbacv1.RoleBinding, error)
	GetClusterRoleBindings(ctx context.Context, opts metav1.ListOptions) ([]rbacv1.ClusterRoleBinding, error)
	
	// Dynamic resource loader for CRDs
	GetCRD(crd CRDSelector, ctx context.Context, opts metav1.ListOptions) ([]map[string]any, error)
	
	// Context and namespace info
	GetNamespace() (string, error)
	GetContext() string
}

// ResourceLoader is a function type for loading k8s resources
type ResourceLoader func(provider Provider, namespace string, ctx context.Context, opts metav1.ListOptions) ([]Resource, error)
```

#### Provider Constructors

```go
// NewClient creates a kube.Client from kubeconfig
func NewClient(contextName, configPath, namespace string) (*Client, error)

// NewMockClient creates a test client with mock data
func NewMockClient(resources []Resource, configs ...MockConfig) *Client

// Client is the production Kubernetes client implementation
type Client struct {
	// Exported fields if needed for inspection
	// Internal fields unexported
}

// MockConfig configures mock client behavior
type MockConfig struct {
	AllError       bool
	ForbiddenError map[string]bool
	// ... other config options
}
```

#### Helper Methods on Response

```go
func (r *Response) NodeMap() map[string]*Resource
func (r *Request) NormalizedSelectors() []string
func (r *Request) DefaultNamespace() string
func (res *Resource) AsMap() map[string]any
```

---

## Private (Unexported) API

Everything else should be unexported:

### pkg/graph
- `buildGraphs()` (internal)
- `inferPod()`, `inferService()`, etc. (resource handlers)
- `findWorkloadRoot()`, `isWorkloadType()` (helpers)
- All helper functions and types

### pkg/tree
- `BuildTreeInternal()` → should be `buildTreeInternal()`
- `NewTree()` → should be `newTree()`
- `ChildBuilder`, `ChildrenBuilder` → unexported
- `FilterOwnedJobRoots()`, `FilterOwnedSecretRoots()` → should be `filterOwnedJobRoots()`
- All builder functions (`buildPodChildren()` etc.)
- All rendering/metadata helpers

### pkg/kube
- `GetLoader()` → unexported or removed (use ResourceTypes)
- All specific loader functions → use via ResourceTypes
- Cache implementation details
- Internal client methods

---

## Usage Examples

### Basic Usage
```go
import (
	"github.com/karloie/kompass/pkg/graph"
	"github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/tree"
)

// Create provider
provider, err := kube.NewClient("", "", "default")
if err != nil {
	return err
}

// Build graph
req := kube.Request{
	Selectors: []string{"deployment/default/api"},
}
result, err := graph.BuildGraphs(provider, req)
if err != nil {
	return err
}

// Optionally build trees
result = tree.BuildTrees(result)

// Use result
for _, node := range result.Nodes {
	fmt.Printf("%s: %s\n", node.Type, node.Key)
}
```

### CLI/Server Usage
```go
// For kompass CLI/server, call graph.BuildGraphs directly and add metadata
req := kube.Request{Selectors: []string{"deployment/default/api"}}
result, err := graph.BuildGraphs(provider, req)
if err != nil {
	return err
}
if client, ok := provider.(*kube.Client); ok {
	result.Metadata = client.GetResponseMeta()
}
```

---

## Migration Plan

1. Make functions in tree/graph packages unexported (rename to lowercase)
2. Ensure kube.Client constructor and types remain exported
3. Add godoc to all public exports
4. Run tests to ensure cmd package still builds (it's in same module)
5. Document breaking changes for any external users

---

## Version Compatibility

This API definition is for v0.1.0. Breaking changes require a major version bump.

Stability guarantees:
- ✅ Stable: `graph.BuildGraphs`, `tree.BuildTrees`, `kube.Request`, `kube.Response`, `kube.Provider`
- ⚠️  Experimental: `graph.ResourceTypes` mutation
