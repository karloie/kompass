package tui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/karloie/kompass/pkg/diagnostics"
	kube "github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/tree"
)

var runViewCommand = func(name string, args ...string) (string, error) {
	cmdCtx, cancel := commandTimeoutContext()
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, name, args...).CombinedOutput()
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
	cmdCtx, cancel := commandTimeoutContext()
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, "kubectl", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func buildNetpolPageWithProvider(target resourceTarget, context string, provider diagnostics.NetpolProvider, resources map[string]*kube.Resource) ViewPage {
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
func buildHubblePage(target resourceTarget, context string, provider diagnostics.HubbleProvider) ViewPage {
	podRef := target.Namespace + "/" + target.Name
	body, err := resolveHubbleProvider(provider).ObservePod(podRef, 100, context)
	if err != nil && strings.TrimSpace(body) == "" {
		body = "(hubble unavailable)\n\nerror: " + err.Error()
	}
	title := "hubble observe --pod " + podRef
	rows := decorateHubbleRows(body)
	return ViewPage{Name: "hubble", Kind: FileOutput, Title: title, Rows: rows, Raw: body}
}

func resourceTargetsEqual(a, b resourceTarget) bool {
	return strings.TrimSpace(a.ResourceType) == strings.TrimSpace(b.ResourceType) &&
		strings.TrimSpace(a.Name) == strings.TrimSpace(b.Name) &&
		strings.TrimSpace(a.Namespace) == strings.TrimSpace(b.Namespace)
}

func loadViewPageNow(target resourceTarget, context string, resources map[string]*kube.Resource, netpolProvider diagnostics.NetpolProvider, hubbleProvider diagnostics.HubbleProvider, pageName string) ViewPage {
	switch pageName {
	case "describe":
		args, _ := buildDescribeCommand(target, context)
		return commandViewPage("describe", args)
	case "logs":
		args, _ := buildLogsCommand(target, context)
		return commandViewPage("logs", args)
	case "events":
		args, _ := buildEventsCommand(target, context)
		return commandViewPage("events", args)
	case "yaml":
		args, _ := buildYAMLCommand(target, context)
		return commandViewPage("yaml", args)
	case "netpol":
		return buildNetpolPageWithProvider(target, context, netpolProvider, resources)
	case "hubble":
		return buildHubblePage(target, context, hubbleProvider)
	default:
		return unavailableViewPage(pageName, pageName+" unavailable")
	}
}

func loadViewPageCmd(target resourceTarget, context string, resources map[string]*kube.Resource, netpolProvider diagnostics.NetpolProvider, hubbleProvider diagnostics.HubbleProvider, pageName string) tea.Cmd {
	return func() tea.Msg {
		page := loadViewPageNow(target, context, resources, netpolProvider, hubbleProvider, pageName)
		return viewPageLoadResultMsg{target: target, page: page}
	}
}

func loadingPageForName(target resourceTarget, context, pageName string) ViewPage {
	loading := "loading..."
	switch pageName {
	case "describe":
		args, _ := buildDescribeCommand(target, context)
		return ViewPage{Name: "describe", Kind: FileOutput, Title: "kubectl " + strings.Join(args, " "), Rows: []string{loading}, Raw: loading}
	case "logs":
		args, _ := buildLogsCommand(target, context)
		return ViewPage{Name: "logs", Kind: FileOutput, Title: "kubectl " + strings.Join(args, " "), Rows: []string{loading}, Raw: loading}
	case "events":
		args, _ := buildEventsCommand(target, context)
		return ViewPage{Name: "events", Kind: FileOutput, Title: "kubectl " + strings.Join(args, " "), Rows: []string{loading}, Raw: loading}
	case "yaml":
		args, _ := buildYAMLCommand(target, context)
		return ViewPage{Name: "yaml", Kind: FileOutput, Title: "kubectl " + strings.Join(args, " "), Rows: []string{loading}, Raw: loading}
	case "netpol":
		return ViewPage{Name: "netpol", Kind: FileOutput, Title: "netpol: " + target.Namespace + "/" + target.Name, Rows: []string{loading}, Raw: loading}
	case "hubble":
		podRef := target.Namespace + "/" + target.Name
		return ViewPage{Name: "hubble", Kind: FileOutput, Title: "hubble observe --pod " + podRef, Rows: []string{loading}, Raw: loading}
	default:
		return unavailableViewPage(pageName, pageName+" unavailable")
	}
}

func openResourceViewAsync(target resourceTarget, context string, resources map[string]*kube.Resource, netpolProvider diagnostics.NetpolProvider, hubbleProvider diagnostics.HubbleProvider, preferredPage string) (*View, tea.Cmd) {
	resourceName := strings.TrimSpace(target.Name)
	if resourceName == "" {
		resourceName = normalizeResourceType(target.ResourceType)
	}

	pageNames := make([]string, 0, 6)
	if args, _ := buildDescribeCommand(target, context); len(args) > 0 {
		pageNames = append(pageNames, "describe")
	}
	if args, _ := buildLogsCommand(target, context); len(args) > 0 {
		pageNames = append(pageNames, "logs")
	}
	if args, _ := buildEventsCommand(target, context); len(args) > 0 {
		pageNames = append(pageNames, "events")
	}
	if args, _ := buildYAMLCommand(target, context); len(args) > 0 {
		pageNames = append(pageNames, "yaml")
	}
	if normalizeResourceType(target.ResourceType) == "pod" {
		pageNames = append(pageNames, "netpol", "hubble")
	}

	if len(pageNames) == 0 {
		v := newPagedView([]ViewPage{unavailableViewPage("resource", "resource unavailable")})
		v.ResourceName = resourceName
		v.Target = target
		return v, nil
	}

	pages := make([]ViewPage, 0, len(pageNames))
	for _, name := range pageNames {
		pages = append(pages, loadingPageForName(target, context, name))
	}
	v := newPagedView(pages)
	v.ResourceName = resourceName
	v.Target = target

	active := 0
	if strings.TrimSpace(preferredPage) != "" {
		for i, name := range pageNames {
			if name == preferredPage {
				active = i
				break
			}
		}
	}
	v.ActivePage = active

	cmds := make([]tea.Cmd, 0, len(pageNames))
	for _, name := range pageNames {
		cmds = append(cmds, loadViewPageCmd(target, context, resources, netpolProvider, hubbleProvider, name))
	}
	v.syncFromActivePage()
	if len(cmds) == 0 {
		return v, nil
	}
	return v, tea.Batch(cmds...)
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

func viewResource(r Row, context, defaultNamespace string, resources map[string]*kube.Resource, netpolProvider diagnostics.NetpolProvider, hubbleProvider diagnostics.HubbleProvider) *View {
	target := resourceViewTarget(r, defaultNamespace)
	return viewResourceFromTarget(target, context, resources, netpolProvider, hubbleProvider)
}

func viewResourceFromTarget(target resourceTarget, context string, resources map[string]*kube.Resource, netpolProvider diagnostics.NetpolProvider, hubbleProvider diagnostics.HubbleProvider) *View {
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
