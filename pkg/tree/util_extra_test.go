package tree

import (
"strings"
"testing"

kube "github.com/karloie/kompass/pkg/kube"
)

// --- truncateMultiline ---

func TestTruncateMultiline_SingleLine(t *testing.T) {
s := "hello world"
if got := truncateMultiline(s); got != s {
t.Fatalf("expected %q unchanged, got %q", s, got)
}
}

func TestTruncateMultiline_MultilineAddsEllipsis(t *testing.T) {
s := "first line\nsecond line"
got := truncateMultiline(s)
if !strings.HasSuffix(got, "...") {
t.Fatalf("expected trailing '...', got %q", got)
}
if strings.Contains(got, "second") {
t.Fatalf("expected second line removed, got %q", got)
}
}

func TestTruncateMultiline_CarriageReturn(t *testing.T) {
s := "line one\rline two"
got := truncateMultiline(s)
if !strings.HasSuffix(got, "...") {
t.Fatalf("expected trailing '...', got %q", got)
}
}

func TestTruncateMultiline_LongFirstLineTruncated(t *testing.T) {
long := strings.Repeat("x", 100)
s := long + "\nsecond"
got := truncateMultiline(s)
// first line > 80 chars so truncated to 77 + "..." + "..."
if len(got) > 83 {
t.Fatalf("expected at most 83 chars for long first line, got len=%d %q", len(got), got)
}
if !strings.HasSuffix(got, "...") {
t.Fatalf("expected trailing '...', got %q", got)
}
}

func TestTruncateMultiline_ExactlyAtLimit(t *testing.T) {
// exactly 80 chars — should not trigger the >80 truncation
s := strings.Repeat("a", 80) + "\nnewline"
got := truncateMultiline(s)
if !strings.HasSuffix(got, "...") {
t.Fatalf("expected '...', got %q", got)
}
// first part should be the full 80 chars
if !strings.HasPrefix(got, strings.Repeat("a", 80)) {
t.Fatalf("expected first 80 chars preserved, got %q", got)
}
}

// --- sortChildrenByPriority ---

func newTreeNode(key, typ, name string) *kube.Tree {
meta := map[string]any{}
if name != "" {
meta["name"] = name
}
return &kube.Tree{Key: key, Type: typ, Meta: meta}
}

func TestSortChildrenByPriority_OrdersByPriority(t *testing.T) {
children := []*kube.Tree{
newTreeNode("k/mount/0", "mount", ""),
newTreeNode("k/env/0", "env", ""),
}
sortChildrenByPriority(children, map[string]int{"env": 0, "mount": 1})
if children[0].Type != "env" {
t.Fatalf("expected env first, got %q", children[0].Type)
}
}

func TestSortChildrenByPriority_UnknownTypeLast(t *testing.T) {
children := []*kube.Tree{
newTreeNode("k/unknown/x", "unknown", "x"),
newTreeNode("k/env/0", "env", ""),
}
sortChildrenByPriority(children, map[string]int{"env": 0})
if children[0].Type != "env" {
t.Fatalf("expected env first, got %q", children[0].Type)
}
if children[1].Type != "unknown" {
t.Fatalf("expected unknown last, got %q", children[1].Type)
}
}

func TestSortChildrenByPriority_SamePriorityByName(t *testing.T) {
children := []*kube.Tree{
newTreeNode("k/env/b", "env", "b"),
newTreeNode("k/env/a", "env", "a"),
}
sortChildrenByPriority(children, map[string]int{"env": 0})
if children[0].Meta["name"] != "a" {
t.Fatalf("expected 'a' first, got %q", children[0].Meta["name"])
}
}

func TestSortChildrenByPriority_SamePriorityAndNameByKey(t *testing.T) {
children := []*kube.Tree{
newTreeNode("k/env/2", "env", ""),
newTreeNode("k/env/1", "env", ""),
}
sortChildrenByPriority(children, map[string]int{"env": 0})
if children[0].Key != "k/env/1" {
t.Fatalf("expected key 'k/env/1' first, got %q", children[0].Key)
}
}

func TestSortChildrenByPriority_EmptySlice(t *testing.T) {
// must not panic
sortChildrenByPriority([]*kube.Tree{}, map[string]int{})
}

// --- podNameByIP ---

func makePodResource(namespace, name, podIP string) kube.Resource {
return kube.Resource{
Key:  "pod/" + namespace + "/" + name,
Type: "pod",
Resource: map[string]any{
"metadata": map[string]any{"namespace": namespace, "name": name},
"status":   map[string]any{"podIP": podIP},
},
}
}

func TestPodNameByIP_MatchFound(t *testing.T) {
nodeMap := map[string]kube.Resource{
"pod/ns/mypod": makePodResource("ns", "mypod", "10.0.0.1"),
}
got := podNameByIP("ns", []string{"10.0.0.1"}, nodeMap)
if got != "mypod" {
t.Fatalf("expected 'mypod', got %q", got)
}
}

func TestPodNameByIP_NoMatch(t *testing.T) {
nodeMap := map[string]kube.Resource{
"pod/ns/mypod": makePodResource("ns", "mypod", "10.0.0.1"),
}
got := podNameByIP("ns", []string{"10.0.0.2"}, nodeMap)
if got != "" {
t.Fatalf("expected empty, got %q", got)
}
}

func TestPodNameByIP_EmptyIPs(t *testing.T) {
nodeMap := map[string]kube.Resource{
"pod/ns/mypod": makePodResource("ns", "mypod", "10.0.0.1"),
}
got := podNameByIP("ns", nil, nodeMap)
if got != "" {
t.Fatalf("expected empty for nil ips, got %q", got)
}
}

func TestPodNameByIP_WrongNamespace(t *testing.T) {
nodeMap := map[string]kube.Resource{
"pod/ns-a/mypod": makePodResource("ns-a", "mypod", "10.0.0.1"),
}
got := podNameByIP("ns-b", []string{"10.0.0.1"}, nodeMap)
if got != "" {
t.Fatalf("expected empty for wrong namespace, got %q", got)
}
}

func TestPodNameByIP_EmptyIPsFiltered(t *testing.T) {
nodeMap := map[string]kube.Resource{
"pod/ns/mypod": makePodResource("ns", "mypod", "10.0.0.1"),
}
// all empty strings in list
got := podNameByIP("ns", []string{"", ""}, nodeMap)
if got != "" {
t.Fatalf("expected empty when all IPs are empty strings, got %q", got)
}
}

// --- policyAppliesToWorkload ---

func makePolicyResource(namespace, name string, matchLabels map[string]any) kube.Resource {
return kube.Resource{
Key:  "networkpolicy/" + namespace + "/" + name,
Type: "networkpolicy",
Resource: map[string]any{
"metadata": map[string]any{"namespace": namespace, "name": name},
"spec": map[string]any{
"matchLabels": matchLabels,
},
},
}
}

func TestPolicyAppliesToWorkload_MatchingLabels(t *testing.T) {
policy := makePolicyResource("ns", "pol", map[string]any{"app": "web"})
nodeMap := map[string]kube.Resource{policy.Key: policy}
podLabels := map[string]any{"app": "web"}
if !policyAppliesToWorkload(policy.Key, "ns", podLabels, nodeMap) {
t.Fatal("expected policy to match workload")
}
}

func TestPolicyAppliesToWorkload_NonMatchingLabels(t *testing.T) {
policy := makePolicyResource("ns", "pol", map[string]any{"app": "api"})
nodeMap := map[string]kube.Resource{policy.Key: policy}
podLabels := map[string]any{"app": "web"}
if policyAppliesToWorkload(policy.Key, "ns", podLabels, nodeMap) {
t.Fatal("expected policy not to match workload")
}
}

func TestPolicyAppliesToWorkload_MissingKey(t *testing.T) {
nodeMap := map[string]kube.Resource{}
if policyAppliesToWorkload("networkpolicy/ns/missing", "ns", map[string]any{"app": "x"}, nodeMap) {
t.Fatal("expected false for missing resource key")
}
}

func TestPolicyAppliesToWorkload_NilPodLabels(t *testing.T) {
policy := makePolicyResource("ns", "pol", map[string]any{"app": "x"})
nodeMap := map[string]kube.Resource{policy.Key: policy}
if policyAppliesToWorkload(policy.Key, "ns", nil, nodeMap) {
t.Fatal("expected false for nil podLabels")
}
}
