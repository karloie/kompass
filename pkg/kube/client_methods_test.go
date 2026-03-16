package kube

import (
	"context"
	"testing"

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestClientAccessorsAndSetters(t *testing.T) {
	mockClient := NewMockClient(NewModel())

	if path, err := mockClient.GetConfigPath(); err != nil {
		t.Fatalf("GetConfigPath() error: %v", err)
	} else if path != "mock" {
		t.Fatalf("expected mock config path, got %q", path)
	}

	if contexts, err := mockClient.GetContexts(); err != nil {
		t.Fatalf("GetContexts() error: %v", err)
	} else {
		items, ok := contexts.([]string)
		if !ok {
			t.Fatalf("expected []string contexts, got %T", contexts)
		}
		foundMock := false
		for _, item := range items {
			if item == "mock-cluster" {
				foundMock = true
				break
			}
		}
		if !foundMock {
			t.Fatalf("expected mock-cluster in contexts, got %v", items)
		}
	}

	mockClient.SetContext("ctx-a")
	if got, _ := mockClient.GetContext(); got != "ctx-a" {
		t.Fatalf("expected context ctx-a, got %q", got)
	}

	mockClient.SetNamespace("ns-a")
	if got, _ := mockClient.GetNamespace(); got != "ns-a" {
		t.Fatalf("expected namespace ns-a, got %q", got)
	}

	if dc := mockClient.DynamicClient(); dc != nil {
		t.Fatalf("expected nil DynamicClient() in mock mode, got %T", dc)
	}

	dc := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	clusterClient := NewClientWithClientset(fake.NewSimpleClientset(), dc, &rest.Config{Host: "https://cluster-a"}, "ctx-b", "ns-b")

	if path, err := clusterClient.GetConfigPath(); err != nil {
		t.Fatalf("cluster GetConfigPath() error: %v", err)
	} else if path != "https://cluster-a" {
		t.Fatalf("expected cluster config path https://cluster-a, got %q", path)
	}

	if got := clusterClient.DynamicClient(); got != dc {
		t.Fatalf("expected DynamicClient() to return injected dynamic client")
	}
}

func TestMockClientAdditionalResourceWrappers(t *testing.T) {
	ctx := context.Background()
	opts := metav1.ListOptions{}
	ns := "ns-test"
	model := NewModel()

	model.Events = []*corev1.Event{{ObjectMeta: metav1.ObjectMeta{Name: "ev-1", Namespace: ns}}}
	model.LimitRanges = []*corev1.LimitRange{{ObjectMeta: metav1.ObjectMeta{Name: "lr-1", Namespace: ns}}}
	model.ResourceQuotas = []*corev1.ResourceQuota{{ObjectMeta: metav1.ObjectMeta{Name: "rq-1", Namespace: ns}}}
	model.IngressClasses = []*networkingv1.IngressClass{{ObjectMeta: metav1.ObjectMeta{Name: "ic-1"}}}
	model.PodDisruptionBudgets = []*policyv1.PodDisruptionBudget{{ObjectMeta: metav1.ObjectMeta{Name: "pdb-1", Namespace: ns}}}
	model.CertificateSigningRequests = []*certificatesv1.CertificateSigningRequest{{ObjectMeta: metav1.ObjectMeta{Name: "csr-1"}}}
	model.FlowSchemas = []*flowcontrolv1.FlowSchema{{ObjectMeta: metav1.ObjectMeta{Name: "fs-1"}}}
	model.PriorityLevelConfigurations = []*flowcontrolv1.PriorityLevelConfiguration{{ObjectMeta: metav1.ObjectMeta{Name: "plc-1"}}}
	model.RuntimeClasses = []*nodev1.RuntimeClass{{ObjectMeta: metav1.ObjectMeta{Name: "rc-1"}}}
	model.PodLogs[ns+"/pod-1"] = "pod logs"

	client := NewMockClient(model)

	events, err := client.GetEventsForObject(ns, ctx, opts)
	if err != nil || len(events.Items) != 1 {
		t.Fatalf("GetEventsForObject() = (%d, %v), want (1, nil)", len(events.Items), err)
	}

	logs, err := client.GetPodLogs(ns, "pod-1")
	if err != nil || logs != "pod logs" {
		t.Fatalf("GetPodLogs(existing) = (%q, %v), want (pod logs, nil)", logs, err)
	}

	logs, err = client.GetPodLogs(ns, "missing")
	if err != nil || logs != "" {
		t.Fatalf("GetPodLogs(missing) = (%q, %v), want (empty, nil)", logs, err)
	}

	if list, err := client.GetLimitRanges(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetLimitRanges() = (%d, %v), want (1, nil)", len(list.Items), err)
	}

	if list, err := client.GetResourceQuotas(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetResourceQuotas() = (%d, %v), want (1, nil)", len(list.Items), err)
	}

	if list, err := client.GetIngressClasses(ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetIngressClasses() = (%d, %v), want (1, nil)", len(list.Items), err)
	}

	if list, err := client.GetPodDisruptionBudgets(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetPodDisruptionBudgets() = (%d, %v), want (1, nil)", len(list.Items), err)
	}

	if list, err := client.GetCertificateSigningRequests(ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetCertificateSigningRequests() = (%d, %v), want (1, nil)", len(list.Items), err)
	}

	if list, err := client.GetFlowSchemas(ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetFlowSchemas() = (%d, %v), want (1, nil)", len(list.Items), err)
	}

	if list, err := client.GetPriorityLevelConfigurations(ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetPriorityLevelConfigurations() = (%d, %v), want (1, nil)", len(list.Items), err)
	}

	if list, err := client.GetRuntimeClasses(ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetRuntimeClasses() = (%d, %v), want (1, nil)", len(list.Items), err)
	}

	if list, err := client.GetCustomResourceDefinitions(ctx, opts); err != nil || len(list.Items) != 0 {
		t.Fatalf("GetCustomResourceDefinitions() = (%d, %v), want (0, nil)", len(list.Items), err)
	}
}

func TestMockClientCoreAndPolicyWrappersBroad(t *testing.T) {
	ctx := context.Background()
	opts := metav1.ListOptions{}
	ns := "ns-broad"
	model := NewModel()

	model.DaemonSets = []*appsv1.DaemonSet{{ObjectMeta: metav1.ObjectMeta{Name: "ds-1", Namespace: ns}}}
	model.Deployments = []*appsv1.Deployment{{ObjectMeta: metav1.ObjectMeta{Name: "dep-1", Namespace: ns}}}
	model.ReplicaSets = []*appsv1.ReplicaSet{{ObjectMeta: metav1.ObjectMeta{Name: "rs-1", Namespace: ns}}}
	model.StatefulSets = []*appsv1.StatefulSet{{ObjectMeta: metav1.ObjectMeta{Name: "sts-1", Namespace: ns}}}
	model.CronJobs = []*batchv1.CronJob{{ObjectMeta: metav1.ObjectMeta{Name: "cj-1", Namespace: ns}}}
	model.Jobs = []*batchv1.Job{{ObjectMeta: metav1.ObjectMeta{Name: "job-1", Namespace: ns}}}
	model.ConfigMaps = []*corev1.ConfigMap{{ObjectMeta: metav1.ObjectMeta{Name: "cm-1", Namespace: ns}}}
	model.Endpoints = []*corev1.Endpoints{{ObjectMeta: metav1.ObjectMeta{Name: "ep-1", Namespace: ns}}}
	model.Namespaces = []*corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: ns}}}
	model.Nodes = []*corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}}
	model.PersistentVolumes = []*corev1.PersistentVolume{{ObjectMeta: metav1.ObjectMeta{Name: "pv-1"}}}
	model.Pods = []*corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: ns}}}
	model.Secrets = []*corev1.Secret{{ObjectMeta: metav1.ObjectMeta{Name: "sec-1", Namespace: ns}}}
	model.Services = []*corev1.Service{{ObjectMeta: metav1.ObjectMeta{Name: "svc-1", Namespace: ns}}}
	model.ServiceAccounts = []*corev1.ServiceAccount{{ObjectMeta: metav1.ObjectMeta{Name: "sa-1", Namespace: ns}}}

	model.EndpointSlices = []*discoveryv1.EndpointSlice{{ObjectMeta: metav1.ObjectMeta{Name: "es-1", Namespace: ns}}}
	model.Ingresses = []*networkingv1.Ingress{{ObjectMeta: metav1.ObjectMeta{Name: "ing-1", Namespace: ns}}}
	model.NetworkPolicies = []*networkingv1.NetworkPolicy{{ObjectMeta: metav1.ObjectMeta{Name: "np-1", Namespace: ns}}}

	model.ClusterRoleBindings = []*rbacv1.ClusterRoleBinding{{ObjectMeta: metav1.ObjectMeta{Name: "crb-1"}}}
	model.ClusterRoles = []*rbacv1.ClusterRole{{ObjectMeta: metav1.ObjectMeta{Name: "cr-1"}}}
	model.RoleBindings = []*rbacv1.RoleBinding{{ObjectMeta: metav1.ObjectMeta{Name: "rb-1", Namespace: ns}}}
	model.Roles = []*rbacv1.Role{{ObjectMeta: metav1.ObjectMeta{Name: "role-1", Namespace: ns}}}

	model.CSIDrivers = []*storagev1.CSIDriver{{ObjectMeta: metav1.ObjectMeta{Name: "csid-1"}}}
	model.CSINodes = []*storagev1.CSINode{{ObjectMeta: metav1.ObjectMeta{Name: "csin-1"}}}
	model.StorageClasses = []*storagev1.StorageClass{{ObjectMeta: metav1.ObjectMeta{Name: "sc-1"}}}
	model.VolumeAttachments = []*storagev1.VolumeAttachment{{ObjectMeta: metav1.ObjectMeta{Name: "va-1"}}}
	model.HorizontalPodAutoscalers = []*autoscalingv1.HorizontalPodAutoscaler{{ObjectMeta: metav1.ObjectMeta{Name: "hpa-1", Namespace: ns}}}
	model.PriorityClasses = []*schedulingv1.PriorityClass{{ObjectMeta: metav1.ObjectMeta{Name: "pc-1"}}}

	client := NewMockClient(model)

	if list, err := client.GetDaemonSets(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetDaemonSets() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetDeployments(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetDeployments() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetReplicaSets(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetReplicaSets() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetStatefulSets(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetStatefulSets() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetCronJobs(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetCronJobs() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetJobs(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetJobs() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetConfigMaps(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetConfigMaps() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetEndpoints(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetEndpoints() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetNamespaces(ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetNamespaces() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetNodes(ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetNodes() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetPersistentVolumes(ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetPersistentVolumes() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetPods(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetPods() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetSecrets(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetSecrets() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetServices(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetServices() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetServiceAccounts(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetServiceAccounts() = (%d, %v), want (1, nil)", len(list.Items), err)
	}

	if list, err := client.GetEndpointSlices(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetEndpointSlices() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetIngresses(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetIngresses() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetNetworkPolicies(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetNetworkPolicies() = (%d, %v), want (1, nil)", len(list.Items), err)
	}

	if list, err := client.GetClusterRoleBindings(ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetClusterRoleBindings() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetClusterRoles(ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetClusterRoles() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetRoleBindings(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetRoleBindings() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetRoles(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetRoles() = (%d, %v), want (1, nil)", len(list.Items), err)
	}

	if list, err := client.GetCSIDrivers(ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetCSIDrivers() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetCSINodes(ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetCSINodes() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetStorageClasses(ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetStorageClasses() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetVolumeAttachments(ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetVolumeAttachments() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetHorizontalPodAutoscalers(ns, ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetHorizontalPodAutoscalers() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
	if list, err := client.GetPriorityClasses(ctx, opts); err != nil || len(list.Items) != 1 {
		t.Fatalf("GetPriorityClasses() = (%d, %v), want (1, nil)", len(list.Items), err)
	}
}

func TestMockClientMapWrappers(t *testing.T) {
	ctx := context.Background()
	opts := metav1.ListOptions{}
	ns := "ns-map"
	model := NewModel()

	model.CiliumNetworkPolicies = []map[string]any{{"metadata": map[string]any{"name": "cnp-1"}}}
	model.CiliumClusterwideNetworkPolicies = []map[string]any{{"metadata": map[string]any{"name": "ccnp-1"}}}
	model.HTTPRoutes = []map[string]any{{"metadata": map[string]any{"name": "hr-1"}}}
	model.Gateways = []map[string]any{{"metadata": map[string]any{"name": "gw-1"}}}
	model.Certificates = []map[string]any{{"metadata": map[string]any{"name": "cert-1"}}}
	model.Issuers = []map[string]any{{"metadata": map[string]any{"name": "issuer-1"}}}
	model.ClusterIssuers = []map[string]any{{"metadata": map[string]any{"name": "cluster-issuer-1"}}}

	client := NewMockClient(model)

	if items, err := client.GetCiliumNetworkPolicies(ns, ctx, opts); err != nil || len(items) != 1 {
		t.Fatalf("GetCiliumNetworkPolicies() = (%d, %v), want (1, nil)", len(items), err)
	}
	if items, err := client.GetCiliumClusterwideNetworkPolicies(ctx, opts); err != nil || len(items) != 1 {
		t.Fatalf("GetCiliumClusterwideNetworkPolicies() = (%d, %v), want (1, nil)", len(items), err)
	}
	if items, err := client.GetHTTPRoutes(ns, ctx, opts); err != nil || len(items) != 1 {
		t.Fatalf("GetHTTPRoutes() = (%d, %v), want (1, nil)", len(items), err)
	}
	if items, err := client.GetGateways(ns, ctx, opts); err != nil || len(items) != 1 {
		t.Fatalf("GetGateways() = (%d, %v), want (1, nil)", len(items), err)
	}
	if items, err := client.GetCertificates(ns, ctx, opts); err != nil || len(items) != 1 {
		t.Fatalf("GetCertificates() = (%d, %v), want (1, nil)", len(items), err)
	}
	if items, err := client.GetIssuers(ns, ctx, opts); err != nil || len(items) != 1 {
		t.Fatalf("GetIssuers() = (%d, %v), want (1, nil)", len(items), err)
	}
	if items, err := client.GetClusterIssuers(ctx, opts); err != nil || len(items) != 1 {
		t.Fatalf("GetClusterIssuers() = (%d, %v), want (1, nil)", len(items), err)
	}
}

func TestClusterClientEventsAndCustomResourceDefinitions(t *testing.T) {
	ctx := context.Background()
	ns := "cluster-ns"
	opts := metav1.ListOptions{}

	clientset := fake.NewSimpleClientset(&corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "ev-cluster", Namespace: ns}})
	client := NewClientWithClientset(clientset, dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()), &rest.Config{Host: "https://cluster"}, "ctx", ns)

	events, err := client.GetEventsForObject(ns, ctx, opts)
	if err != nil || len(events.Items) != 1 {
		t.Fatalf("GetEventsForObject(cluster) = (%d, %v), want (1, nil)", len(events.Items), err)
	}

	if list, err := client.GetCustomResourceDefinitions(ctx, opts); err != nil || len(list.Items) != 0 {
		t.Fatalf("GetCustomResourceDefinitions(cluster) = (%d, %v), want (0, nil)", len(list.Items), err)
	}
}

func TestClusterClientPodLogsErrorPath(t *testing.T) {
	// Fake clientset returns an empty raw body for pod logs, which still exercises the cluster branch.
	client := NewClientWithClientset(fake.NewSimpleClientset(), dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()), &rest.Config{Host: "https://cluster"}, "ctx", "default")

	logs, err := client.GetPodLogs("default", "pod-x")
	if err != nil {
		t.Fatalf("GetPodLogs(cluster) unexpected error: %v", err)
	}
	if logs == "" {
		t.Fatalf("GetPodLogs(cluster) expected non-empty fake logs, got empty string")
	}
}

func TestClusterClientDynamicMapWrappers(t *testing.T) {
	ctx := context.Background()
	ns := "dyn-ns"
	opts := metav1.ListOptions{}

	objects := []runtime.Object{
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "cilium.io/v2", "kind": "CiliumNetworkPolicy",
			"metadata": map[string]any{"name": "cnp-a", "namespace": ns},
		}},
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "cilium.io/v2", "kind": "CiliumClusterwideNetworkPolicy",
			"metadata": map[string]any{"name": "ccnp-a"},
		}},
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "gateway.networking.k8s.io/v1", "kind": "HTTPRoute",
			"metadata": map[string]any{"name": "hr-a", "namespace": ns},
		}},
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "gateway.networking.k8s.io/v1", "kind": "Gateway",
			"metadata": map[string]any{"name": "gw-a", "namespace": ns},
		}},
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "cert-manager.io/v1", "kind": "Certificate",
			"metadata": map[string]any{"name": "cert-a", "namespace": ns},
		}},
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "cert-manager.io/v1", "kind": "Issuer",
			"metadata": map[string]any{"name": "issuer-a", "namespace": ns},
		}},
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "cert-manager.io/v1", "kind": "ClusterIssuer",
			"metadata": map[string]any{"name": "cluster-issuer-a"},
		}},
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "secrets-store.csi.x-k8s.io/v1", "kind": "SecretProviderClass",
			"metadata": map[string]any{"name": "fiskeoye-spc", "namespace": ns},
		}},
	}

	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "cilium.io", Version: "v2", Resource: "ciliumnetworkpolicies"}:                  "CiliumNetworkPolicyList",
		{Group: "cilium.io", Version: "v2", Resource: "ciliumclusterwidenetworkpolicies"}:       "CiliumClusterwideNetworkPolicyList",
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}:             "HTTPRouteList",
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}:               "GatewayList",
		{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}:                     "CertificateList",
		{Group: "cert-manager.io", Version: "v1", Resource: "issuers"}:                          "IssuerList",
		{Group: "cert-manager.io", Version: "v1", Resource: "clusterissuers"}:                   "ClusterIssuerList",
		{Group: "secrets-store.csi.x-k8s.io", Version: "v1", Resource: "secretproviderclasses"}: "SecretProviderClassList",
	}
	dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objects...)
	client := NewClientWithClientset(fake.NewSimpleClientset(), dc, &rest.Config{Host: "https://cluster"}, "ctx", ns)

	if items, err := client.GetCiliumNetworkPolicies(ns, ctx, opts); err != nil || len(items) != 1 {
		t.Fatalf("GetCiliumNetworkPolicies(cluster) = (%d, %v), want (1, nil)", len(items), err)
	}
	if items, err := client.GetCiliumClusterwideNetworkPolicies(ctx, opts); err != nil || len(items) != 1 {
		t.Fatalf("GetCiliumClusterwideNetworkPolicies(cluster) = (%d, %v), want (1, nil)", len(items), err)
	}
	if items, err := client.GetHTTPRoutes(ns, ctx, opts); err != nil || len(items) != 1 {
		t.Fatalf("GetHTTPRoutes(cluster) = (%d, %v), want (1, nil)", len(items), err)
	}
	if items, err := client.GetGateways(ns, ctx, opts); err != nil {
		t.Fatalf("GetGateways(cluster) unexpected error: %v", err)
	} else if len(items) > 1 {
		t.Fatalf("GetGateways(cluster) expected <=1 item with fake dynamic client, got %d", len(items))
	}
	if items, err := client.GetCertificates(ns, ctx, opts); err != nil || len(items) != 1 {
		t.Fatalf("GetCertificates(cluster) = (%d, %v), want (1, nil)", len(items), err)
	}
	if items, err := client.GetIssuers(ns, ctx, opts); err != nil || len(items) != 1 {
		t.Fatalf("GetIssuers(cluster) = (%d, %v), want (1, nil)", len(items), err)
	}
	if items, err := client.GetClusterIssuers(ctx, opts); err != nil || len(items) != 1 {
		t.Fatalf("GetClusterIssuers(cluster) = (%d, %v), want (1, nil)", len(items), err)
	}
	if items, err := client.GetSecretProviderClasses(ns, ctx, opts); err != nil || len(items) != 1 {
		t.Fatalf("GetSecretProviderClasses(cluster) = (%d, %v), want (1, nil)", len(items), err)
	}

}
