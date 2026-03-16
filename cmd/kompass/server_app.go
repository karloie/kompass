package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/karloie/kompass/pkg/diagnostics"
	"github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/pipeline"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type appResourceTarget struct {
	Key       string
	Type      string
	Namespace string
	Name      string
}

type appViewResponse struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

const (
	appEventsLimit = 100
	appHubbleLimit = 100
)

const (
	hubblePersistentDenyPriority = iota // deny with no matching allow in the window
	hubbleResolvedDenyPriority          // deny for which a corresponding allow exists
	hubbleAllowPriority
	hubbleOtherPriority
)

var (
	hubbleDenyPattern    = regexp.MustCompile(`\b(?:DENY|DENIED|DROP|DROPPED|BLOCKED)\b`)
	hubbleAllowPattern   = regexp.MustCompile(`\b(?:ALLOW|ALLOWED|OPEN|FORWARDED|PERMIT)\b`)
	hubbleTimePattern    = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?\b`)
	hubbleFlowKeyPattern = regexp.MustCompile(`(\S+)\s+->\s+(\S+)`)
)

func (s *server) handleAppDescribe(w http.ResponseWriter, r *http.Request) {
	target, provider, _, resource, err := s.inferAppResource(r)
	if err != nil {
		writeAppError(w, err)
		return
	}
	body, err := buildDescribeView(provider, target, resource)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeAppView(w, appViewResponse{Title: "Describe", Content: body})
}

func (s *server) handleAppLogs(w http.ResponseWriter, r *http.Request) {
	target, provider, _, _, err := s.inferAppResource(r)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if target.Type != "pod" {
		writeAppError(w, badRequest("logs are only available for pods"))
		return
	}
	body, err := provider.GetPodLogs(target.Namespace, target.Name)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if strings.TrimSpace(body) == "" {
		body = fmt.Sprintf("(no logs available for %s/%s)", target.Namespace, target.Name)
	}
	writeAppView(w, appViewResponse{Title: "Logs", Content: body})
}

func (s *server) handleAppEvents(w http.ResponseWriter, r *http.Request) {
	target, provider, _, resource, err := s.inferAppResource(r)
	if err != nil {
		writeAppError(w, err)
		return
	}
	body, err := buildEventsView(provider, target, resource)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeAppView(w, appViewResponse{Title: "Events", Content: body})
}

func (s *server) handleAppHubble(w http.ResponseWriter, r *http.Request) {
	target, provider, result, _, err := s.inferAppResource(r)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if target.Type != "pod" {
		writeAppError(w, badRequest("hubble is only available for pods"))
		return
	}
	body, err := buildHubbleView(provider, target, result)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeAppView(w, appViewResponse{Title: "Cilium", Content: body})
}

func (s *server) handleAppYAML(w http.ResponseWriter, r *http.Request) {
	_, _, _, resource, err := s.inferAppResource(r)
	if err != nil {
		writeAppError(w, err)
		return
	}
	body, err := buildYAMLView(resource)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeAppView(w, appViewResponse{Title: "YAML", Content: body})
}

func (s *server) inferAppResource(r *http.Request) (appResourceTarget, kube.Kube, *kube.Response, *kube.Resource, error) {
	target, err := parseAppResourceTarget(r)
	if err != nil {
		return appResourceTarget{}, nil, nil, nil, err
	}

	providerContext, err := requireContextArg(r)
	if err != nil {
		return appResourceTarget{}, nil, nil, nil, err
	}
	providerNamespace, err := requireExplicitNamespace(r)
	if err != nil {
		return appResourceTarget{}, nil, nil, nil, err
	}
	mockProvider := strings.TrimSpace(r.URL.Query().Get("mock"))
	if mockProvider == "" && providerContext == "mock-01" {
		mockProvider = "mock"
	}

	s.providerMu.Lock()
	defer s.providerMu.Unlock()

	provider, err := s.getProvider(mockProvider, providerContext, providerNamespace)
	if err != nil {
		return appResourceTarget{}, nil, nil, nil, err
	}

	result, err := pipeline.InferGraphs(provider, []string{target.Key})
	if err != nil {
		return appResourceTarget{}, nil, nil, nil, err
	}
	if result == nil {
		return appResourceTarget{}, nil, nil, nil, notFound("resource not found")
	}
	resource := result.Node(target.Key)
	if resource == nil {
		return appResourceTarget{}, nil, nil, nil, notFound("resource not found")
	}

	return target, provider, result, resource, nil
}

func parseAppResourceTarget(r *http.Request) (appResourceTarget, error) {
	target := appResourceTarget{
		Key:       strings.TrimSpace(r.URL.Query().Get("key")),
		Type:      strings.TrimSpace(r.URL.Query().Get("type")),
		Namespace: strings.TrimSpace(r.URL.Query().Get("namespace")),
		Name:      strings.TrimSpace(r.URL.Query().Get("name")),
	}
	if target.Key == "" {
		return appResourceTarget{}, badRequest("missing key")
	}

	parsed := parseResourceKey(target.Key)
	if target.Type == "" {
		target.Type = parsed.Type
	}
	if target.Namespace == "" {
		target.Namespace = parsed.Namespace
	}
	if target.Name == "" {
		target.Name = parsed.Name
	}
	if target.Type == "" || target.Name == "" {
		return appResourceTarget{}, badRequest("invalid resource key")
	}
	return target, nil
}

func parseResourceKey(key string) appResourceTarget {
	parts := strings.Split(strings.Trim(key, "/"), "/")
	switch {
	case len(parts) >= 3:
		return appResourceTarget{
			Key:       key,
			Type:      parts[0],
			Namespace: parts[1],
			Name:      strings.Join(parts[2:], "/"),
		}
	case len(parts) == 2:
		return appResourceTarget{
			Key:  key,
			Type: parts[0],
			Name: parts[1],
		}
	default:
		return appResourceTarget{Key: key}
	}
}

func buildDescribeView(provider kube.Kube, target appResourceTarget, resource *kube.Resource) (string, error) {
	contextName, _ := provider.GetContext()
	body, err := runKubectlDescribe(target, contextName)
	if err == nil && strings.TrimSpace(body) != "" {
		return body, nil
	}

	reason := "resource not found or empty"
	if err != nil {
		reason = err.Error()
	}
	slog.Warn("describe provider fallback", "from", "kubectl", "to", "in-memory", "resource", target.Key, "reason", reason)

	fallback, fallbackErr := buildDescribeFallback(resource)
	if fallbackErr != nil {
		if err != nil {
			return "", err
		}
		return "", fallbackErr
	}
	return fallback, nil
}

func runKubectlDescribe(target appResourceTarget, contextName string) (string, error) {
	args := []string{"describe", target.Type, target.Name}
	if strings.TrimSpace(target.Namespace) != "" {
		args = append(args, "-n", target.Namespace)
	}
	if strings.TrimSpace(contextName) != "" {
		args = append(args, "--context", contextName)
	}
	out, err := exec.Command("kubectl", args...).CombinedOutput()
	body := strings.TrimSpace(string(out))
	if err != nil {
		if body != "" {
			return body, err
		}
		return "", err
	}
	return body, nil
}

func buildDescribeFallback(resource *kube.Resource) (string, error) {
	if resource == nil {
		return "", notFound("resource not found")
	}
	obj := resource.AsMap()
	meta := nestedMap(obj, "metadata")
	kind := stringValue(obj["kind"])
	name := stringValue(meta["name"])
	namespace := stringValue(meta["namespace"])

	var body strings.Builder
	if kind != "" {
		body.WriteString("Kind: " + kind + "\n")
	}
	if name != "" {
		body.WriteString("Name: " + name + "\n")
	}
	if namespace != "" {
		body.WriteString("Namespace: " + namespace + "\n")
	}
	if labels := nestedMap(meta, "labels"); len(labels) > 0 {
		body.WriteString("Labels:\n")
		keys := sortedKeys(labels)
		for _, key := range keys {
			body.WriteString(fmt.Sprintf("  %s=%v\n", key, labels[key]))
		}
	}
	if spec := nestedMap(obj, "spec"); len(spec) > 0 {
		specYAML, err := yaml.Marshal(spec)
		if err != nil {
			return "", err
		}
		body.WriteString("\nSpec:\n")
		body.WriteString(strings.TrimSpace(string(specYAML)))
	}
	if status := nestedMap(obj, "status"); len(status) > 0 {
		statusYAML, err := yaml.Marshal(status)
		if err != nil {
			return "", err
		}
		body.WriteString("\n\nStatus:\n")
		body.WriteString(strings.TrimSpace(string(statusYAML)))
	}
	if strings.TrimSpace(body.String()) == "" {
		return buildYAMLView(resource)
	}
	return strings.TrimSpace(body.String()), nil
}

func buildYAMLView(resource *kube.Resource) (string, error) {
	if resource == nil {
		return "", notFound("resource not found")
	}
	body, err := yaml.Marshal(resource.AsMap())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func buildEventsView(provider kube.Kube, target appResourceTarget, resource *kube.Resource) (string, error) {
	if strings.TrimSpace(target.Namespace) == "" {
		return "(events unavailable for cluster-scoped resources)", nil
	}

	fieldSelector := "involvedObject.name=" + target.Name
	if kind := resourceKind(resource); kind != "" {
		fieldSelector += ",involvedObject.kind=" + kind
	}
	events, err := provider.GetEventsForObject(target.Namespace, context.Background(), metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		return "", err
	}
	filtered := filterEvents(events, target, resource)
	if len(filtered) == 0 {
		return fmt.Sprintf("(no events found for %s)", target.Key), nil
	}
	sort.Slice(filtered, func(i, j int) bool {
		return eventTimestamp(filtered[i]).Time.Before(eventTimestamp(filtered[j]).Time)
	})
	if len(filtered) > appEventsLimit {
		// Keep the newest events while preserving chronological rendering.
		filtered = filtered[len(filtered)-appEventsLimit:]
	}

	var body strings.Builder
	for _, item := range filtered {
		ts := eventTimestamp(item).Format(timeLayoutOrFallback(eventTimestamp(item)))
		if ts == "" {
			ts = "-"
		}
		line := fmt.Sprintf("%s  %s  %s", ts, strings.TrimSpace(item.Type), strings.TrimSpace(item.Reason))
		if msg := strings.TrimSpace(item.Message); msg != "" {
			line += "  " + msg
		}
		body.WriteString(strings.TrimSpace(line) + "\n")
	}
	return strings.TrimSpace(body.String()), nil
}

func buildHubbleView(provider kube.Kube, target appResourceTarget, result *kube.Response) (string, error) {
	resources := result.NodeMap()
	contextName, _ := provider.GetContext()
	podTarget := diagnostics.PodTarget{ResourceType: target.Type, Name: target.Name, Namespace: target.Namespace}

	netpolBody, netpolErr := diagnostics.ResolveNetpolProvider(nil).AnalyzePod(podTarget, contextName, resources)
	hubbleBody, hubbleErr := buildHubbleObserve(provider, target)
	formattedHubbleBody := formatHubbleLogs(hubbleBody)

	sections := make([]string, 0, 2)
	if strings.TrimSpace(netpolBody) != "" {
		sections = append(sections, ""+strings.TrimSpace(netpolBody))
	}
	if strings.TrimSpace(formattedHubbleBody) != "" {
		sections = append(sections, "Hubble logs:\n\n"+strings.TrimSpace(formattedHubbleBody))
	}
	if len(sections) == 0 {
		if netpolErr != nil {
			return "", netpolErr
		}
		if hubbleErr != nil {
			return "", hubbleErr
		}
		return "(no cilium data available)", nil
	}
	return strings.Join(sections, "\n\n"), nil
}

func buildHubbleObserve(provider kube.Kube, target appResourceTarget) (string, error) {
	if isMockProvider(provider) {
		return buildMockHubbleView(target), nil
	}
	contextName, _ := provider.GetContext()
	return diagnostics.ResolveHubbleProvider(nil).ObservePod(target.Namespace+"/"+target.Name, 100, contextName)
}

func isMockProvider(provider kube.Kube) bool {
	type mockAware interface {
		IsMockMode() bool
	}
	aware, ok := provider.(mockAware)
	return ok && aware.IsMockMode()
}

func buildMockHubbleView(target appResourceTarget) string {
	podRef := target.Namespace + "/" + target.Name
	// payment-gateway: only a deny, no allow → persistent deny (⛔)
	// petshop-backend-boys: deny + allow for same flow → resolved deny (⚠️)
	return strings.Join([]string{
		fmt.Sprintf("2026-03-16T09:58:05Z  DROPPED  pod/%s -> service/petshop-backend-boys:8080  policy denied", target.Name),
		fmt.Sprintf("2026-03-16T10:01:12Z  FORWARDED  pod/%s -> kube-dns/coredns-7b98449c4-xv9l2  DNS Query A api.petshop.internal", target.Name),
		fmt.Sprintf("2026-03-16T10:02:44Z  FORWARDED  pod/%s -> service/petshop-backend-boys:8080  HTTP GET /api/catalog", target.Name),
		fmt.Sprintf("2026-03-16T10:03:29Z  DROPPED  pod/%s -> service/payment-gateway:443  policy denied", target.Name),
		fmt.Sprintf("Captured mock flows for %s. Run against a live cluster for real-time Hubble output.", podRef),
	}, "\n")
}

type hubbleLogLine struct {
	raw      string
	priority int
	time     time.Time
	hasTime  bool
	index    int
}

func extractHubbleFlowKey(line string) string {
	m := hubbleFlowKeyPattern.FindStringSubmatch(line)
	if len(m) < 3 {
		return ""
	}
	return m[1] + " -> " + m[2]
}

func formatHubbleLogs(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")

	// First pass: collect flow keys that have at least one allow.
	allowFlowKeys := make(map[string]bool)
	for _, line := range lines {
		clean := strings.TrimSpace(line)
		if clean == "" {
			continue
		}
		if hubbleAllowPattern.MatchString(strings.ToUpper(clean)) {
			if key := extractHubbleFlowKey(clean); key != "" {
				allowFlowKeys[key] = true
			}
		}
	}

	// Second pass: classify and stamp each line.
	items := make([]hubbleLogLine, 0, len(lines))
	for i, line := range lines {
		clean := strings.TrimSpace(line)
		if clean == "" {
			continue
		}
		priority := hubbleOtherPriority
		switch {
		case hubbleDenyPattern.MatchString(strings.ToUpper(clean)):
			flowKey := extractHubbleFlowKey(clean)
			if flowKey != "" && allowFlowKeys[flowKey] {
				// A corresponding allow exists: resolved deny.
				priority = hubbleResolvedDenyPriority
			} else {
				// No allow found for this flow: persistent denial.
				priority = hubblePersistentDenyPriority
			}
		case hubbleAllowPattern.MatchString(strings.ToUpper(clean)):
			priority = hubbleAllowPriority
		}
		ts, hasTime := parseHubbleLineTime(clean)
		items = append(items, hubbleLogLine{
			raw:      clean,
			priority: priority,
			time:     ts,
			hasTime:  hasTime,
			index:    i,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left.priority != right.priority {
			return left.priority < right.priority
		}
		if left.hasTime != right.hasTime {
			return left.hasTime
		}
		if left.hasTime && right.hasTime && !left.time.Equal(right.time) {
			return left.time.After(right.time)
		}
		return left.index < right.index
	})

	if len(items) > appHubbleLimit {
		items = items[:appHubbleLimit]
	}

	var out strings.Builder
	for _, item := range items {
		line := item.raw
		switch item.priority {
		case hubblePersistentDenyPriority:
			if !strings.HasPrefix(line, "⛔") {
				line = "⛔ " + line
			}
		case hubbleResolvedDenyPriority:
			if !strings.HasPrefix(line, "⚠️") {
				line = "⚠️ " + line
			}
		case hubbleAllowPriority:
			if !strings.HasPrefix(line, "✅") {
				line = "✅ " + line
			}
		}
		out.WriteString(line + "\n")
	}
	return strings.TrimSpace(out.String())
}

func parseHubbleLineTime(line string) (time.Time, bool) {
	match := hubbleTimePattern.FindString(line)
	if match == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, match)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func filterEvents(events *corev1.EventList, target appResourceTarget, resource *kube.Resource) []corev1.Event {
	if events == nil {
		return nil
	}
	uid := resourceUID(resource)
	kind := resourceKind(resource)
	filtered := make([]corev1.Event, 0, len(events.Items))
	for _, item := range events.Items {
		if item.InvolvedObject.Name != target.Name {
			continue
		}
		if kind != "" && item.InvolvedObject.Kind != "" && item.InvolvedObject.Kind != kind {
			continue
		}
		if uid != "" && string(item.InvolvedObject.UID) != "" && string(item.InvolvedObject.UID) != uid {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func resourceKind(resource *kube.Resource) string {
	if resource == nil {
		return ""
	}
	return stringValue(resource.AsMap()["kind"])
}

func resourceUID(resource *kube.Resource) string {
	if resource == nil {
		return ""
	}
	return stringValue(nestedMap(resource.AsMap(), "metadata")["uid"])
}

func eventTimestamp(event corev1.Event) metav1.Time {
	if !event.EventTime.IsZero() {
		return metav1.NewTime(event.EventTime.Time)
	}
	if !event.LastTimestamp.IsZero() {
		return metav1.NewTime(event.LastTimestamp.Time)
	}
	if !event.FirstTimestamp.IsZero() {
		return metav1.NewTime(event.FirstTimestamp.Time)
	}
	return event.CreationTimestamp
}

func timeLayoutOrFallback(ts metav1.Time) string {
	if ts.IsZero() {
		return ""
	}
	return "2006-01-02 15:04:05"
}

func nestedMap(root map[string]any, key string) map[string]any {
	if root == nil {
		return nil
	}
	child, _ := root[key].(map[string]any)
	return child
}

func stringValue(value any) string {
	str, _ := value.(string)
	return strings.TrimSpace(str)
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func writeAppView(w http.ResponseWriter, response appViewResponse) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func writeAppError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	var appErr appHTTPError
	if errors.As(err, &appErr) {
		status = appErr.StatusCode
	}
	http.Error(w, err.Error(), status)
}

type appHTTPError struct {
	StatusCode int
	Message    string
}

func (e appHTTPError) Error() string {
	return e.Message
}

func badRequest(message string) error {
	return appHTTPError{StatusCode: http.StatusBadRequest, Message: message}
}

func notFound(message string) error {
	return appHTTPError{StatusCode: http.StatusNotFound, Message: message}
}
