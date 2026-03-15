package tui

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/karloie/kompass/pkg/diagnostics"
	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/tree"
)

var runViewCommand = func(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	body := strings.TrimRight(string(out), "\n")
	if body == "" {
		body = "(no output)"
	}
	if err != nil {
		body += "\n\nerror: " + err.Error()
	}
	return body, err
}

var runScopeListCommand = func(args ...string) (string, error) {
	out, err := exec.Command("kubectl", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// runNetpolAnalysis fetches the pod and all NetworkPolicies in its namespace,
// then returns a human-readable policy evaluation for that pod.
var runNetpolAnalysis = func(target resourceTarget, context string) (string, error) {
	if target.Name == "" || target.Namespace == "" {
		return "(no pod info available)", nil
	}
	args := []string{"get", "pod", target.Name, "-n", target.Namespace, "-o", "json"}
	args = appendContextArg(args, context)
	podOut, err := exec.Command("kubectl", args...).CombinedOutput()
	if err != nil {
		return "error fetching pod: " + strings.TrimSpace(string(podOut)), err
	}
	var podRaw map[string]any
	if err := json.Unmarshal(podOut, &podRaw); err != nil {
		return "error parsing pod JSON: " + err.Error(), err
	}

	npArgs := []string{"get", "networkpolicy", "-n", target.Namespace, "-o", "json"}
	npArgs = appendContextArg(npArgs, context)
	npOut, _ := exec.Command("kubectl", npArgs...).CombinedOutput()

	nodes := map[string]kube.Resource{}
	podKey := "pod/" + target.Namespace + "/" + target.Name
	nodes[podKey] = kube.Resource{Key: podKey, Type: "pod", Resource: podRaw}

	var npList map[string]any
	if err := json.Unmarshal(npOut, &npList); err == nil {
		if items, ok := npList["items"].([]any); ok {
			for _, item := range items {
				if m, ok := item.(map[string]any); ok {
					ns, _ := nestedString(m, "metadata", "namespace")
					name, _ := nestedString(m, "metadata", "name")
					if ns != "" && name != "" {
						k := "networkpolicy/" + ns + "/" + name
						nodes[k] = kube.Resource{Key: k, Type: "networkpolicy", Resource: m}
					}
				}
			}
		}
	}

	podResource := nodes[podKey]
	verdict := graph.AnalyzePodNetworkPolicies(nodes, podResource)
	return graph.FormatNetPolVerdict(verdict), nil
}

func buildNetpolPage(target resourceTarget, context string) ViewPage {
	return buildNetpolPageWithProvider(target, context, nil, nil)
}

func buildNetpolPageWithProvider(target resourceTarget, context string, provider NetpolProvider, resources map[string]*kube.Resource) ViewPage {
	body, err := resolveNetpolProvider(provider).AnalyzePod(diagnostics.PodTarget{
		ResourceType: target.ResourceType,
		Name:         target.Name,
		Namespace:    target.Namespace,
	}, context, resources)
	if err != nil && strings.TrimSpace(body) == "" {
		body = "(netpol analysis unavailable)"
	}
	title := "netpol: " + target.Namespace + "/" + target.Name
	rows := strings.Split(body, "\n")
	return ViewPage{Name: "netpol", Kind: FileOutput, Title: title, Rows: rows, Raw: body}
}

// runNetpolAnalysisFromResources avoids shelling out to kubectl when the
// current selector tree already has the target pod and networkpolicies loaded.
func runNetpolAnalysisFromResources(target resourceTarget, resources map[string]*kube.Resource) (string, bool) {
	if target.Name == "" || target.Namespace == "" {
		return "", false
	}
	loaded := resources
	if len(loaded) == 0 {
		return "", false
	}

	podKey := "pod/" + target.Namespace + "/" + target.Name
	podPtr, ok := loaded[podKey]
	if !ok || podPtr == nil {
		return "", false
	}
	podObj := podPtr.AsMap()
	meta, ok := podObj["metadata"].(map[string]any)
	if !ok {
		return "", false
	}
	podName, _ := meta["name"].(string)
	podNS, _ := meta["namespace"].(string)
	if strings.TrimSpace(podName) == "" || strings.TrimSpace(podNS) == "" {
		return "", false
	}

	nodes := make(map[string]kube.Resource, len(loaded))
	for key, res := range loaded {
		if res == nil {
			continue
		}
		if res.Type != "networkpolicy" && key != podKey {
			continue
		}
		nodes[key] = *res
	}
	if len(nodes) == 0 {
		return "", false
	}

	verdict := graph.AnalyzePodNetworkPolicies(nodes, *podPtr)
	return graph.FormatNetPolVerdict(verdict), true
}

// runHubbleCommand executes the hubble CLI tool.
var runHubbleCommand = func(args ...string) (string, error) {
	out, err := exec.Command("hubble", args...).CombinedOutput()
	body := strings.TrimRight(string(out), "\n")
	if err != nil && isHubbleRelayUnavailable(body) {
		// Relay not reachable — try to start it and retry once.
		if pfErr := startHubblePortForward(); pfErr == nil {
			out2, err2 := exec.Command("hubble", args...).CombinedOutput()
			body = strings.TrimRight(string(out2), "\n")
			err = err2
		}
	}
	if body == "" && err != nil {
		body = "hubble observe unavailable; ensure the hubble CLI is installed and relay is running"
	}
	return body, err
}

// startHubblePortForward runs "cilium hubble port-forward" in the background
// and waits briefly for the relay to become reachable.
var startHubblePortForward = func() error {
	cmd := exec.Command("cilium", "hubble", "port-forward")
	if err := cmd.Start(); err != nil {
		return err
	}
	// Give the port-forward up to 3 seconds to become ready.
	deadline := 30 // × 100ms
	for i := 0; i < deadline; i++ {
		time.Sleep(100 * time.Millisecond)
		probe, err := exec.Command("hubble", "observe", "--last", "1").CombinedOutput()
		if err == nil || !isHubbleRelayUnavailable(string(probe)) {
			return nil
		}
	}
	return nil // proceed anyway — the retry in runHubbleCommand will surface the error
}

func isHubbleRelayUnavailable(output string) bool {
	return strings.Contains(output, "rpc error") && strings.Contains(output, "Unavailable")
}

var hubbleProviderMode = func() string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("KOMPASS_HUBBLE_PROVIDER")))
	switch mode {
	case "native", "cli", "auto":
		return mode
	default:
		return "auto"
	}
}

// runHubbleObserve is the provider entrypoint. It supports native-ready mode
// selection while defaulting to current CLI behavior for compatibility.
var runHubbleObserve = func(podRef string, last int, context string) (string, error) {
	return observeHubbleByMode(podRef, last, context, hubbleProviderMode())
}

func observeHubbleByMode(podRef string, last int, context, mode string) (string, error) {
	switch mode {
	case "cli":
		return observeHubbleWithCLI(podRef, last, context)
	case "native":
		return observeHubbleNative(podRef, last, context)
	default: // auto
		body, err := observeHubbleNative(podRef, last, context)
		if err == nil && !isNativeHubbleNoData(body) {
			return body, nil
		}
		reason := "no native flow data"
		if err != nil {
			reason = err.Error()
		}
		slog.Warn("hubble provider fallback", "from", "native", "to", "cli", "pod", podRef, "reason", reason)
		return observeHubbleWithCLI(podRef, last, context)
	}
}

func isNativeHubbleNoData(body string) bool {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return true
	}
	return strings.HasPrefix(trimmed, "(no hubble flows observed")
}

func observeHubbleWithCLI(podRef string, last int, context string) (string, error) {
	_ = context // hubble CLI does not support a kubectl-style --context flag
	if last <= 0 {
		last = 100
	}
	args := []string{"observe", "--pod", podRef, "--last", fmt.Sprintf("%d", last)}
	return runHubbleCommand(args...)
}

func buildHubblePage(target resourceTarget, context string, provider HubbleProvider) ViewPage {
	podRef := target.Namespace + "/" + target.Name
	body, _ := resolveHubbleProvider(provider).ObservePod(podRef, 100, context)
	title := "hubble observe --pod " + podRef
	rows := decorateHubbleRows(body)
	return ViewPage{Name: "hubble", Kind: FileOutput, Title: title, Rows: rows, Raw: body}
}

func decorateHubbleRows(body string) []string {
	lines := strings.Split(body, "\n")
	rows := make([]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, decorateHubbleLine(line))
	}
	return rows
}

func decorateHubbleLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return line
	}
	lower := strings.ToLower(trimmed)

	var badges []string
	if strings.HasPrefix(lower, "hubble observe ") {
		badges = append(badges, "🔎")
	}
	if strings.Contains(lower, "warn") || strings.Contains(lower, "error") || strings.Contains(lower, "failed") {
		badges = append(badges, "⚠️")
	}
	if strings.Contains(lower, "denied") || strings.Contains(lower, "deny") || strings.Contains(lower, "dropped") || strings.Contains(lower, "drop") {
		badges = append(badges, "🚫")
	} else if strings.Contains(lower, "forwarded") || strings.Contains(lower, "allowed") || strings.Contains(lower, "allow") {
		badges = append(badges, "✅")
	}
	if strings.Contains(line, " <- ") {
		badges = append(badges, "⬅️")
	} else if strings.Contains(line, " -> ") {
		badges = append(badges, "➡️")
	}

	if len(badges) == 0 {
		return line
	}
	return strings.Join(badges, " ") + " " + line
}

func appendContextArg(args []string, context string) []string {
	if strings.TrimSpace(context) != "" {
		return append(args, "--context", strings.TrimSpace(context))
	}
	return args
}

func nestedString(m map[string]any, keys ...string) (string, bool) {
	cur := m
	for i, k := range keys {
		if i == len(keys)-1 {
			s, ok := cur[k].(string)
			return s, ok
		}
		next, ok := cur[k].(map[string]any)
		if !ok {
			return "", false
		}
		cur = next
	}
	return "", false
}

func listScopeOptions(mode, context string) ([]string, error) {
	args := []string{}
	switch mode {
	case "context":
		args = []string{"config", "get-contexts", "-o", "name"}
	case "namespace":
		args = []string{"get", "namespaces", "-o", "custom-columns=NAME:.metadata.name", "--no-headers"}
		if strings.TrimSpace(context) != "" {
			args = append(args, "--context", strings.TrimSpace(context))
		}
	default:
		return nil, nil
	}

	body, err := runScopeListCommand(args...)
	if err != nil && strings.TrimSpace(body) == "" {
		return nil, err
	}

	lines := strings.Split(body, "\n")
	options := make([]string, 0, len(lines))
	for _, line := range lines {
		item := strings.TrimSpace(line)
		if item == "" {
			continue
		}
		options = append(options, item)
	}
	return options, err
}

type resourceTarget struct {
	ResourceType string
	Name         string
	Namespace    string
}

func (m Model) rowAvailableActions(r *Row) []string {
	if r == nil {
		return nil
	}
	target := resourceViewTarget(*r, m.namespace)
	actions := make([]string, 0, 4)
	if args, _ := buildDescribeCommand(target, m.context); len(args) > 0 {
		actions = append(actions, "describe")
	}
	if args, _ := buildLogsCommand(target, m.context); len(args) > 0 {
		actions = append(actions, "logs")
	}
	if args, _ := buildEventsCommand(target, m.context); len(args) > 0 {
		actions = append(actions, "events")
	}
	if args, _ := buildYAMLCommand(target, m.context); len(args) > 0 {
		actions = append(actions, "yaml")
	}
	return actions
}

func (m Model) effectiveAction(r *Row) string {
	actions := m.rowAvailableActions(r)
	if len(actions) == 0 {
		return "describe"
	}
	for _, a := range actions {
		if a == m.selectedAction {
			return a
		}
	}
	return actions[0]
}

func (m Model) rowActionArgs(r *Row, action string) []string {
	if r == nil {
		return nil
	}
	target := resourceViewTarget(*r, m.namespace)
	switch action {
	case "logs":
		args, _ := buildLogsCommand(target, m.context)
		return args
	case "events":
		args, _ := buildEventsCommand(target, m.context)
		return args
	case "yaml":
		args, _ := buildYAMLCommand(target, m.context)
		return args
	default:
		args, _ := buildDescribeCommand(target, m.context)
		return args
	}
}

func viewResource(r Row, context, defaultNamespace string, resources map[string]*kube.Resource, netpolProvider NetpolProvider, hubbleProvider HubbleProvider) *View {
	target := resourceViewTarget(r, defaultNamespace)
	return viewResourceFromTarget(target, context, resources, netpolProvider, hubbleProvider)
}

func viewResourceFromTarget(target resourceTarget, context string, resources map[string]*kube.Resource, netpolProvider NetpolProvider, hubbleProvider HubbleProvider) *View {
	resourceName := strings.TrimSpace(target.Name)
	if resourceName == "" {
		resourceName = normalizeResourceType(target.ResourceType)
	}
	describeArgs, _ := buildDescribeCommand(target, context)
	logsArgs, _ := buildLogsCommand(target, context)
	eventsArgs, _ := buildEventsCommand(target, context)
	yamlArgs, _ := buildYAMLCommand(target, context)
	pages := make([]ViewPage, 0, 6)
	pages = appendViewPage(pages, "describe", describeArgs)
	pages = appendViewPage(pages, "logs", logsArgs)
	pages = appendViewPage(pages, "events", eventsArgs)
	pages = appendViewPage(pages, "yaml", yamlArgs)
	if normalizeResourceType(target.ResourceType) == "pod" {
		pages = append(pages, buildNetpolPageWithProvider(target, context, netpolProvider, resources))
		pages = append(pages, buildHubblePage(target, context, hubbleProvider))
	}
	if len(pages) == 0 {
		pages = append(pages, unavailableViewPage("resource", "resource unavailable"))
	}
	v := newPagedView(pages)
	v.ResourceName = resourceName
	v.Target = target
	return v
}

func appendViewPage(pages []ViewPage, name string, args []string) []ViewPage {
	if len(args) == 0 {
		return pages
	}
	return append(pages, commandViewPage(name, args))
}

func viewDescribe(r Row, context, defaultNamespace string) *View {
	target := resourceViewTarget(r, defaultNamespace)
	args, _ := buildDescribeCommand(target, context)
	page := commandViewPage("describe", args)
	resourceName := strings.TrimSpace(target.Name)
	if resourceName == "" {
		resourceName = normalizeResourceType(target.ResourceType)
	}
	return &View{Kind: page.Kind, ResourceName: resourceName, Target: target, Title: page.Title, Rows: page.Rows, Raw: page.Raw}
}

func buildDescribeArgs(r Row, context, defaultNamespace string) ([]string, string) {
	target := resourceViewTarget(r, defaultNamespace)
	args, title := buildDescribeCommand(target, context)
	return args, title
}

func resourceViewTarget(r Row, defaultNamespace string) resourceTarget {
	ref := tree.ParseResourceKeyRef(r.Key)
	resourceType := strings.TrimSpace(ref.Type)
	if resourceType == "" {
		resourceType = strings.TrimSpace(r.Type)
	}
	name := strings.TrimSpace(ref.Name)
	if name == "" {
		name = strings.TrimSpace(r.Name)
	}
	namespace := strings.TrimSpace(ref.Namespace)
	if namespace == "" {
		namespace = strings.TrimSpace(defaultNamespace)
	}
	return resourceTarget{ResourceType: resourceType, Name: name, Namespace: namespace}
}

func buildDescribeCommand(target resourceTarget, context string) ([]string, string) {
	args := []string{"describe", target.ResourceType}
	if target.Name != "" {
		args = append(args, target.Name)
	}
	args = appendScopeArgs(args, target.Namespace, context)

	title := target.ResourceType
	if target.Name != "" {
		title += "/" + target.Name
	}
	return args, title
}

func buildLogsCommand(target resourceTarget, context string) ([]string, string) {
	if target.ResourceType == "" || target.Name == "" {
		return nil, ""
	}
	if !supportsLogsView(target.ResourceType) {
		return nil, ""
	}
	args := []string{"logs", fmt.Sprintf("%s/%s", target.ResourceType, target.Name), "--tail=100"}
	args = appendScopeArgs(args, target.Namespace, context)
	return args, target.ResourceType + "/" + target.Name
}

func supportsLogsView(resourceType string) bool {
	kind := normalizeResourceType(resourceType)
	switch kind {
	case "pod", "po", "pods":
		return true
	case "deployment", "deploy", "deployments":
		return true
	case "daemonset", "ds", "daemonsets":
		return true
	case "statefulset", "sts", "statefulsets":
		return true
	case "job", "jobs":
		return true
	case "cronjob", "cj", "cronjobs":
		return true
	case "replicaset", "rs", "replicasets":
		return true
	case "replicationcontroller", "rc", "replicationcontrollers":
		return true
	default:
		return false
	}
}

func normalizeResourceType(resourceType string) string {
	kind := strings.ToLower(strings.TrimSpace(resourceType))
	if idx := strings.Index(kind, "."); idx > 0 {
		kind = kind[:idx]
	}
	return kind
}

func buildEventsCommand(target resourceTarget, context string) ([]string, string) {
	if target.ResourceType == "" || target.Name == "" {
		return nil, ""
	}
	args := []string{"events", "--for", fmt.Sprintf("%s/%s", target.ResourceType, target.Name)}
	args = appendScopeArgs(args, target.Namespace, context)
	return args, target.ResourceType + "/" + target.Name
}

func buildYAMLCommand(target resourceTarget, context string) ([]string, string) {
	if target.ResourceType == "" || target.Name == "" {
		return nil, ""
	}
	args := []string{"get", target.ResourceType, target.Name, "-o", "yaml"}
	args = appendScopeArgs(args, target.Namespace, context)
	return args, target.ResourceType + "/" + target.Name
}

func appendScopeArgs(args []string, namespace, context string) []string {
	if strings.TrimSpace(namespace) != "" {
		args = append(args, "-n", strings.TrimSpace(namespace))
	}
	if strings.TrimSpace(context) != "" {
		// kubectl has no short context flag; keep long form.
		args = append(args, "--context", strings.TrimSpace(context))
	}
	return args
}

func commandViewPage(name string, args []string) ViewPage {
	if len(args) == 0 {
		return unavailableViewPage(name, name+" unavailable")
	}
	body, _ := runViewCommand("kubectl", args...)
	title := "kubectl " + strings.Join(args, " ")
	raw := body
	rows := strings.Split(raw, "\n")
	return ViewPage{Name: name, Kind: FileOutput, Title: title, Rows: rows, Raw: raw}
}

func unavailableViewPage(name, title string) ViewPage {
	body := "(no output)"
	return ViewPage{Name: name, Kind: FileOutput, Title: title, Rows: []string{body}, Raw: body}
}

func viewHelp() *View {
	rows := []string{
		"Main View",
		"  Up/Down or j/k      move row",
		"  Left/Right or h/l   pan long lines",
		"  f or /              open live filter modal",
		"  c or n              open context/namespace list",
		"  Up/Down             move list selection",
		"  Tab/Shift+Tab       cycle context (roots)",
		"  1/2                 switch main context (tree/single)",
		"",
		"Actions",
		"  Space               toggle row selection",
		"  Enter               open resource view",
		"  Ctrl+T              cycle theme (blue/mint/amber)",
		"  r                   refresh main view data from cluster",
		"  o                   output selected/current keys and quit",
		"  + / -               increase/decrease footer panel height",
		"  Enter/Esc           apply/cancel filter modal",
		"",
		"Resource View",
		"  Tab/Shift+Tab       cycle context (applicable views)",
		"  Up/Down, PgUp/PgDn  scroll",
		"  Home/End            go to top/bottom",
		"  Left/Right or h/l   pan long lines",
		"  Esc or q            close resource view",
		"",
		"Exit",
		"  Esc / Ctrl+C     quit application",
	}
	raw := strings.Join(rows, "\n")
	return &View{Kind: FileHelp, Title: "Keybindings", Rows: rows, Raw: raw}
}

func fit(s string, width int) string {
	if width <= 0 {
		return s
	}
	return truncate(s, width)
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return string(r[:1])
	}
	if width <= 3 {
		return string(r[:width])
	}
	return string(r[:width-3]) + "..."
}

func pad(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) >= width {
		return string(r[:width])
	}
	return s + strings.Repeat(" ", width-len(r))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func containsInt(values []int, needle int) bool {
	for _, v := range values {
		if v == needle {
			return true
		}
	}
	return false
}
