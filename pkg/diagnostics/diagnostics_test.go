package diagnostics

import (
"strings"
"testing"
"time"

kube "github.com/karloie/kompass/pkg/kube"
)

// --- PodTarget ---

func TestPodTargetString(t *testing.T) {
pt := PodTarget{Namespace: "ns", Name: "mypod"}
if pt.String() != "ns/mypod" {
t.Fatalf("expected 'ns/mypod', got %q", pt.String())
}
}

// --- nestedString ---

func TestNestedString(t *testing.T) {
m := map[string]any{
"metadata": map[string]any{"name": "mypod"},
}
s, ok := nestedString(m, "metadata", "name")
if !ok || s != "mypod" {
t.Fatalf("expected 'mypod' ok=true, got %q ok=%v", s, ok)
}
_, ok = nestedString(m, "metadata", "missing")
if ok {
t.Fatal("expected ok=false for missing leaf key")
}
_, ok = nestedString(m, "missing", "name")
if ok {
t.Fatal("expected ok=false for missing intermediate key")
}
}

// --- IsHubbleRelayUnavailable ---

func TestIsHubbleRelayUnavailable(t *testing.T) {
for _, tc := range []struct {
input string
want  bool
}{
{"rpc error: code = Unavailable desc = ...", true},
{"rpc error: code = NotFound", false},
{"some other error", false},
{"", false},
} {
if got := IsHubbleRelayUnavailable(tc.input); got != tc.want {
t.Errorf("input=%q: expected %v, got %v", tc.input, tc.want, got)
}
}
}

// --- splitPodRef ---

func TestSplitPodRef(t *testing.T) {
for _, tc := range []struct{ input, wantNS, wantPod string }{
{"ns/pod", "ns", "pod"},
{"justname", "", "justname"},
{" myns / mypod ", "myns", "mypod"},
{"", "", ""},
} {
ns, pod := splitPodRef(tc.input)
if ns != tc.wantNS || pod != tc.wantPod {
t.Errorf("input=%q: expected ns=%q pod=%q, got ns=%q pod=%q",
tc.input, tc.wantNS, tc.wantPod, ns, pod)
}
}
}

// --- hubbleRelayAddress ---

func TestHubbleRelayAddress(t *testing.T) {
t.Setenv("KOMPASS_HUBBLE_ADDR", "")
if got := hubbleRelayAddress(); got != "127.0.0.1:4245" {
t.Fatalf("expected default '127.0.0.1:4245', got %q", got)
}
t.Setenv("KOMPASS_HUBBLE_ADDR", "custom.host:1234")
if got := hubbleRelayAddress(); got != "custom.host:1234" {
t.Fatalf("expected 'custom.host:1234', got %q", got)
}
}

// --- hubbleRelayTimeout ---

func TestHubbleRelayTimeout(t *testing.T) {
t.Setenv("KOMPASS_HUBBLE_TIMEOUT", "")
if got := hubbleRelayTimeout(); got != 2*time.Second {
t.Fatalf("expected default 2s, got %v", got)
}
t.Setenv("KOMPASS_HUBBLE_TIMEOUT", "5s")
if got := hubbleRelayTimeout(); got != 5*time.Second {
t.Fatalf("expected 5s, got %v", got)
}
t.Setenv("KOMPASS_HUBBLE_TIMEOUT", "invalid")
if got := hubbleRelayTimeout(); got != 2*time.Second {
t.Fatalf("expected fallback to 2s on invalid value, got %v", got)
}
}

// --- HubbleProviderMode ---

func TestHubbleProviderMode(t *testing.T) {
for _, tc := range []struct{ env, want string }{
{"native", "native"},
{"cli", "cli"},
{"auto", "auto"},
{"NATIVE", "native"},
{"other", "auto"},
{"", "auto"},
} {
t.Setenv("KOMPASS_HUBBLE_PROVIDER", tc.env)
if got := HubbleProviderMode(); got != tc.want {
t.Errorf("env=%q: expected %q, got %q", tc.env, tc.want, got)
}
}
}

// --- ResolveNetpolProvider ---

func TestResolveNetpolProvider(t *testing.T) {
p := ResolveNetpolProvider(nil)
if p == nil {
t.Fatal("expected non-nil default provider")
}
custom := defaultNetpolProvider{}
if got := ResolveNetpolProvider(custom); got != custom {
t.Fatal("expected provided provider returned unchanged")
}
}

// --- ResolveHubbleProvider ---

func TestResolveHubbleProvider(t *testing.T) {
p := ResolveHubbleProvider(nil)
if p == nil {
t.Fatal("expected non-nil default provider")
}
custom := defaultHubbleProvider{}
if got := ResolveHubbleProvider(custom); got != custom {
t.Fatal("expected provided provider returned unchanged")
}
}

// --- RunNetpolAnalysis (var func) ---

func TestRunNetpolAnalysis_EmptyTarget(t *testing.T) {
// Both Name and Namespace empty: returns early without calling kubectl.
result, err := RunNetpolAnalysis(PodTarget{}, "test-ctx")
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if result != "(no pod info available)" {
t.Fatalf("expected early-return message, got %q", result)
}
}

// --- AnalyzePodNetworkPoliciesFromResources helpers ---

func podPtr(namespace, name string, labels map[string]string) *kube.Resource {
labelMap := make(map[string]any, len(labels))
for k, v := range labels {
labelMap[k] = v
}
r := kube.Resource{
Key:  "pod/" + namespace + "/" + name,
Type: "pod",
Resource: map[string]any{
"metadata": map[string]any{
"namespace": namespace,
"name":      name,
"labels":    labelMap,
},
},
}
return &r
}

func netpolPtr(namespace, name string, podSel map[string]any, policyTypes []string) *kube.Resource {
ptAny := make([]any, len(policyTypes))
for i, pt := range policyTypes {
ptAny[i] = pt
}
r := kube.Resource{
Key:  "networkpolicy/" + namespace + "/" + name,
Type: "networkpolicy",
Resource: map[string]any{
"metadata": map[string]any{"namespace": namespace, "name": name},
"spec": map[string]any{
"podSelector": podSel,
"policyTypes": ptAny,
},
},
}
return &r
}

// --- AnalyzePodNetworkPoliciesFromResources ---

func TestAnalyzePodNetworkPoliciesFromResources_EmptyTarget(t *testing.T) {
_, ok := AnalyzePodNetworkPoliciesFromResources(PodTarget{}, nil)
if ok {
t.Fatal("expected false for empty target")
}
}

func TestAnalyzePodNetworkPoliciesFromResources_NilResources(t *testing.T) {
_, ok := AnalyzePodNetworkPoliciesFromResources(PodTarget{Namespace: "ns", Name: "pod"}, nil)
if ok {
t.Fatal("expected false for nil resources")
}
}

func TestAnalyzePodNetworkPoliciesFromResources_PodNotFound(t *testing.T) {
resources := map[string]*kube.Resource{
"pod/ns/other": podPtr("ns", "other", nil),
}
_, ok := AnalyzePodNetworkPoliciesFromResources(PodTarget{Namespace: "ns", Name: "mypod"}, resources)
if ok {
t.Fatal("expected false when pod key not in resources")
}
}

func TestAnalyzePodNetworkPoliciesFromResources_NilPodEntry(t *testing.T) {
resources := map[string]*kube.Resource{
"pod/ns/mypod": nil,
}
_, ok := AnalyzePodNetworkPoliciesFromResources(PodTarget{Namespace: "ns", Name: "mypod"}, resources)
if ok {
t.Fatal("expected false for nil pod entry")
}
}

func TestAnalyzePodNetworkPoliciesFromResources_OpenNoNetpols(t *testing.T) {
pod := podPtr("petshop", "api", map[string]string{"app": "api"})
resources := map[string]*kube.Resource{pod.Key: pod}
result, ok := AnalyzePodNetworkPoliciesFromResources(PodTarget{Namespace: "petshop", Name: "api"}, resources)
if !ok {
t.Fatal("expected ok=true for valid pod with no netpols")
}
if !strings.Contains(result, "INGRESS: OPEN") || !strings.Contains(result, "EGRESS: OPEN") {
t.Fatalf("expected fully open verdict, got:\n%s", result)
}
}

func TestAnalyzePodNetworkPoliciesFromResources_RestrictedIngress(t *testing.T) {
pod := podPtr("petshop", "api", map[string]string{"app": "api"})
netpol := netpolPtr("petshop", "deny-ingress",
map[string]any{"matchLabels": map[string]any{"app": "api"}},
[]string{"Ingress"},
)
resources := map[string]*kube.Resource{pod.Key: pod, netpol.Key: netpol}
result, ok := AnalyzePodNetworkPoliciesFromResources(PodTarget{Namespace: "petshop", Name: "api"}, resources)
if !ok {
t.Fatal("expected ok=true")
}
if !strings.Contains(result, "INGRESS: RESTRICTED") {
t.Fatalf("expected INGRESS: RESTRICTED, got:\n%s", result)
}
if !strings.Contains(result, "EGRESS: OPEN") {
t.Fatalf("expected EGRESS: OPEN, got:\n%s", result)
}
}

func TestAnalyzePodNetworkPoliciesFromResources_CrossNamespacePolicyIgnored(t *testing.T) {
pod := podPtr("ns-a", "api", map[string]string{"app": "api"})
netpol := netpolPtr("ns-b", "deny-ingress",
map[string]any{"matchLabels": map[string]any{"app": "api"}},
[]string{"Ingress"},
)
resources := map[string]*kube.Resource{pod.Key: pod, netpol.Key: netpol}
result, ok := AnalyzePodNetworkPoliciesFromResources(PodTarget{Namespace: "ns-a", Name: "api"}, resources)
if !ok {
t.Fatal("expected ok=true")
}
if !strings.Contains(result, "INGRESS: OPEN") {
t.Fatalf("cross-namespace policy must not apply, got:\n%s", result)
}
}

func TestAnalyzePodNetworkPoliciesFromResources_UnmatchedSelector(t *testing.T) {
pod := podPtr("ns", "api", map[string]string{"app": "api"})
netpol := netpolPtr("ns", "deny-web",
map[string]any{"matchLabels": map[string]any{"app": "web"}},
[]string{"Ingress"},
)
resources := map[string]*kube.Resource{pod.Key: pod, netpol.Key: netpol}
result, ok := AnalyzePodNetworkPoliciesFromResources(PodTarget{Namespace: "ns", Name: "api"}, resources)
if !ok {
t.Fatal("expected ok=true")
}
if !strings.Contains(result, "INGRESS: OPEN") {
t.Fatalf("unmatched selector must not restrict pod, got:\n%s", result)
}
}

// --- defaultNetpolProvider.AnalyzePod ---

func TestDefaultNetpolProvider_UsesInMemoryPath(t *testing.T) {
pod := podPtr("myns", "mypod", map[string]string{"role": "backend"})
resources := map[string]*kube.Resource{pod.Key: pod}
p := defaultNetpolProvider{}
result, err := p.AnalyzePod(PodTarget{Namespace: "myns", Name: "mypod"}, "", resources)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if !strings.Contains(result, "INGRESS: OPEN") {
t.Fatalf("expected open verdict via in-memory path, got:\n%s", result)
}
}
