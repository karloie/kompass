package main

import (
	"context"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
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
	"github.com/karloie/kompass/pkg/graph"
	"github.com/karloie/kompass/pkg/kube"
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
	// validKubeContextRE matches safe kubeconfig context/cluster names.
	validKubeContextRE = regexp.MustCompile(`^[a-zA-Z0-9._/:@-]+$`)
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
	target, provider, err := s.resolveAppTarget(r)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if target.Type != "pod" {
		writeAppError(w, badRequest("hubble is only available for pods"))
		return
	}

	// Hubble analysis needs the full graph for netpol resolution.
	// Check the fullGraphCache (populated by /api/tree or /api/graph calls) first.
	s.providerMu.Lock()
	contextArg, _ := provider.GetContext()
	providerNamespace, _ := provider.GetNamespace()
	cacheKey := contextArg + "|" + providerNamespace
	var result *kube.Response
	if entry, ok := s.fullGraphCache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		result = entry.result
	}
	s.providerMu.Unlock()

	if result == nil {
		req := kube.Request{}
		result, err = graph.BuildGraphs(provider, req)
		if err != nil {
			writeAppError(w, err)
			return
		}
		if client, ok := provider.(*kube.Client); ok {
			result.Metadata = client.GetResponseMeta()
		}
	}

	body, err := buildHubbleView(provider, target, result)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeAppView(w, appViewResponse{Title: "Cilium", Content: body})
}

func (s *server) handleAppHubbleWatch(w http.ResponseWriter, r *http.Request) {
	target, provider, _, _, err := s.inferAppResource(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if target.Type != "pod" {
		http.Error(w, "hubble watch is only available for pods", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	lines := make(chan string, 64)
	go func() {
		defer close(lines)
		if isMockProvider(provider) {
			streamMockHubble(ctx, target, lines)
			return
		}
		contextName, _ := provider.GetContext()
		podRef := target.Namespace + "/" + target.Name
		if err := diagnostics.WatchHubbleFlows(ctx, podRef, contextName, lines); err != nil {
			if !errors.Is(err, context.Canceled) {
				select {
				case lines <- "error: " + err.Error():
				case <-ctx.Done():
				}
			}
		}
	}()

	for {
		select {
		case line, ok := <-lines:
			if !ok {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", line); err != nil {
				return
			}
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

var mockHubbleFlowTemplates = []string{
	"FORWARDED  %s -> kube-dns:53  DNS Query A api.internal",
	"DROPPED    %s -> service/payment-gateway:443  policy denied",
	"FORWARDED  %s -> service/petshop-backend:8080  HTTP GET /api/catalog",
	"DROPPED    %s -> service/external-api:443  policy denied",
	"FORWARDED  %s -> service/petshop-backend:8080  HTTP POST /api/order",
}

func streamMockHubble(ctx context.Context, target appResourceTarget, lines chan<- string) {
	podRef := target.Namespace + "/" + target.Name
	now := time.Now()
	for i, tmpl := range mockHubbleFlowTemplates {
		ts := now.Add(time.Duration(-len(mockHubbleFlowTemplates)+i) * time.Second).Format("15:04:05")
		line := ts + "  " + fmt.Sprintf(tmpl, podRef)
		select {
		case lines <- line:
		case <-ctx.Done():
			return
		}
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	counter := 0
	liveTemplates := []string{
		"FORWARDED  %s -> service/petshop-backend:8080  HTTP GET /api/health",
		"FORWARDED  %s -> kube-dns:53  DNS Query A metrics.internal",
		"DROPPED    %s -> service/payment-gateway:443  policy denied",
	}
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			tmpl := liveTemplates[counter%len(liveTemplates)]
			line := t.Format("15:04:05") + "  " + fmt.Sprintf(tmpl, podRef)
			select {
			case lines <- line:
			case <-ctx.Done():
				return
			}
			counter++
		}
	}
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

func (s *server) handleAppCert(w http.ResponseWriter, r *http.Request) {
	target, provider, _, resource, err := s.inferAppResource(r)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if target.Type != "certificate" {
		writeAppError(w, badRequest("cert view is only available for certificates"))
		return
	}
	body, err := buildCertView(provider, resource)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeAppView(w, appViewResponse{Title: "Cert", Content: body})
}

func (s *server) inferAppResource(r *http.Request) (appResourceTarget, kube.Provider, *kube.Response, *kube.Resource, error) {
	target, provider, err := s.resolveAppTarget(r)
	if err != nil {
		return appResourceTarget{}, nil, nil, nil, err
	}

	resource, err := provider.FetchResource(target.Type, target.Namespace, target.Name, r.Context())
	if err != nil {
		return appResourceTarget{}, nil, nil, nil, notFound("resource not found: " + err.Error())
	}
	return target, provider, nil, resource, nil
}

// resolveAppTarget parses the target from the request and returns the provider.
// It does not make any Kubernetes API calls.
func (s *server) resolveAppTarget(r *http.Request) (appResourceTarget, kube.Provider, error) {
	target, err := parseAppResourceTarget(r)
	if err != nil {
		return appResourceTarget{}, nil, err
	}

	providerContext, err := requireContextArg(r)
	if err != nil {
		return appResourceTarget{}, nil, err
	}
	providerNamespace, err := requireExplicitNamespace(r)
	if err != nil {
		return appResourceTarget{}, nil, err
	}

	s.providerMu.Lock()
	defer s.providerMu.Unlock()

	provider, err := s.getProvider(providerContext, providerNamespace)
	if err != nil {
		return appResourceTarget{}, nil, err
	}
	return target, provider, nil
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

func buildDescribeView(provider kube.Provider, target appResourceTarget, resource *kube.Resource) (string, error) {
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
	if ctxName := strings.TrimSpace(contextName); ctxName != "" {
		if !validKubeContextRE.MatchString(ctxName) {
			return "", fmt.Errorf("invalid context name")
		}
		args = append(args, "--context", ctxName)
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

func buildCertView(provider kube.Provider, resource *kube.Resource) (string, error) {
	if resource == nil {
		return "", notFound("resource not found")
	}

	certObj := resource.AsMap()
	meta := nestedMap(certObj, "metadata")
	spec := nestedMap(certObj, "spec")
	certNamespace := stringValue(meta["namespace"])
	certName := stringValue(meta["name"])
	secretName := stringValue(spec["secretName"])

	var body strings.Builder
	body.WriteString("Certificate Analysis\n")
	body.WriteString("- Resource: ")
	body.WriteString(strings.TrimSpace(certName))
	body.WriteString("\n")
	body.WriteString("- Namespace: ")
	body.WriteString(strings.TrimSpace(certNamespace))
	body.WriteString("\n")
	body.WriteString("- Secret: ")
	if secretName == "" {
		body.WriteString("(missing spec.secretName)\n")
		return body.String(), nil
	}
	body.WriteString(secretName)
	body.WriteString("\n\n")

	secretList, err := provider.GetSecrets(certNamespace, context.Background(), metav1.ListOptions{})
	if err != nil {
		body.WriteString("Failed to load secrets: ")
		body.WriteString(err.Error())
		return body.String(), nil
	}

	var tlsCRT []byte
	for i := range secretList.Items {
		item := secretList.Items[i]
		if item.Name != secretName {
			continue
		}
		tlsCRT = item.Data["tls.crt"]
		break
	}

	if len(tlsCRT) == 0 {
		body.WriteString("No tls.crt found in secret ")
		body.WriteString(secretName)
		return body.String(), nil
	}

	certs, parseErr := parseCertificateChainPEM(tlsCRT)
	if parseErr != nil {
		body.WriteString("Failed to parse tls.crt: ")
		body.WriteString(parseErr.Error())
		return body.String(), nil
	}
	if len(certs) == 0 {
		body.WriteString("No X.509 certificates found in tls.crt")
		return body.String(), nil
	}

	writeX509CertificateSection(&body, "Certificate", certs[0])

	if len(certs) > 1 {
		body.WriteString("\n\n")
		writeX509CertificateSection(&body, "Issuer Certificate", certs[1])
	}

	return body.String(), nil
}

func parseCertificateChainPEM(pemBytes []byte) ([]*x509.Certificate, error) {
	remaining := pemBytes
	certs := make([]*x509.Certificate, 0, 2)
	for len(remaining) > 0 {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}
		remaining = rest
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

func writeX509CertificateSection(body *strings.Builder, title string, cert *x509.Certificate) {
	body.WriteString(title)
	body.WriteString(":\n")
	body.WriteString("- Subject: ")
	body.WriteString(cert.Subject.String())
	body.WriteString("\n")
	body.WriteString("- Issuer: ")
	body.WriteString(cert.Issuer.String())
	body.WriteString("\n")
	body.WriteString("- Serial Number: ")
	body.WriteString(cert.SerialNumber.String())
	body.WriteString("\n")
	body.WriteString("- Not Before: ")
	body.WriteString(cert.NotBefore.Format(time.RFC3339))
	body.WriteString("\n")
	body.WriteString("- Not After: ")
	body.WriteString(cert.NotAfter.Format(time.RFC3339))
	body.WriteString("\n")
	body.WriteString("- Signature Algorithm: ")
	body.WriteString(cert.SignatureAlgorithm.String())
	body.WriteString("\n")
	body.WriteString("- Public Key Algorithm: ")
	body.WriteString(cert.PublicKeyAlgorithm.String())
	body.WriteString("\n")
	body.WriteString("- Is CA: ")
	body.WriteString(fmt.Sprintf("%t", cert.IsCA))
	body.WriteString("\n")
	body.WriteString("- DNS Names: ")
	body.WriteString(strings.Join(cert.DNSNames, ", "))
	body.WriteString("\n")
	body.WriteString("- IP Addresses: ")
	if len(cert.IPAddresses) == 0 {
		body.WriteString("\n")
	} else {
		ips := make([]string, 0, len(cert.IPAddresses))
		for _, ip := range cert.IPAddresses {
			ips = append(ips, ip.String())
		}
		body.WriteString(strings.Join(ips, ", "))
		body.WriteString("\n")
	}
	body.WriteString("- URI SANs: ")
	if len(cert.URIs) == 0 {
		body.WriteString("\n")
	} else {
		uris := make([]string, 0, len(cert.URIs))
		for _, uri := range cert.URIs {
			uris = append(uris, uri.String())
		}
		body.WriteString(strings.Join(uris, ", "))
		body.WriteString("\n")
	}
	body.WriteString("- Key Usage: ")
	body.WriteString(strings.Join(x509KeyUsageStrings(cert.KeyUsage), ", "))
	body.WriteString("\n")
	body.WriteString("- Extended Key Usage: ")
	body.WriteString(strings.Join(x509ExtKeyUsageStrings(cert.ExtKeyUsage), ", "))
	body.WriteString("\n")
	body.WriteString("- Subject Key ID: ")
	body.WriteString(strings.ToUpper(hex.EncodeToString(cert.SubjectKeyId)))
	body.WriteString("\n")
	body.WriteString("- Authority Key ID: ")
	body.WriteString(strings.ToUpper(hex.EncodeToString(cert.AuthorityKeyId)))
}

func x509KeyUsageStrings(usage x509.KeyUsage) []string {
	out := make([]string, 0, 9)
	flags := []struct {
		mask x509.KeyUsage
		name string
	}{
		{x509.KeyUsageDigitalSignature, "DigitalSignature"},
		{x509.KeyUsageContentCommitment, "ContentCommitment"},
		{x509.KeyUsageKeyEncipherment, "KeyEncipherment"},
		{x509.KeyUsageDataEncipherment, "DataEncipherment"},
		{x509.KeyUsageKeyAgreement, "KeyAgreement"},
		{x509.KeyUsageCertSign, "CertSign"},
		{x509.KeyUsageCRLSign, "CRLSign"},
		{x509.KeyUsageEncipherOnly, "EncipherOnly"},
		{x509.KeyUsageDecipherOnly, "DecipherOnly"},
	}
	for _, item := range flags {
		if usage&item.mask != 0 {
			out = append(out, item.name)
		}
	}
	if len(out) == 0 {
		return []string{"(none)"}
	}
	return out
}

func x509ExtKeyUsageStrings(usages []x509.ExtKeyUsage) []string {
	if len(usages) == 0 {
		return []string{"(none)"}
	}
	out := make([]string, 0, len(usages))
	for _, usage := range usages {
		switch usage {
		case x509.ExtKeyUsageServerAuth:
			out = append(out, "ServerAuth")
		case x509.ExtKeyUsageClientAuth:
			out = append(out, "ClientAuth")
		case x509.ExtKeyUsageCodeSigning:
			out = append(out, "CodeSigning")
		case x509.ExtKeyUsageEmailProtection:
			out = append(out, "EmailProtection")
		case x509.ExtKeyUsageTimeStamping:
			out = append(out, "TimeStamping")
		case x509.ExtKeyUsageOCSPSigning:
			out = append(out, "OCSPSigning")
		case x509.ExtKeyUsageAny:
			out = append(out, "Any")
		default:
			out = append(out, fmt.Sprintf("%d", usage))
		}
	}
	return out
}

func buildEventsView(provider kube.Provider, target appResourceTarget, resource *kube.Resource) (string, error) {
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

func buildHubbleView(provider kube.Provider, target appResourceTarget, result *kube.Response) (string, error) {
	resources := result.NodeMap()
	contextName, _ := provider.GetContext()
	podTarget := diagnostics.PodTarget{ResourceType: target.Type, Name: target.Name, Namespace: target.Namespace}

	netpolBody, netpolErr := diagnostics.ResolveNetpolProvider(nil).AnalyzePod(podTarget, contextName, resources)
	hubbleBody, hubbleErr := buildHubbleObserve(provider, target)
	formattedHubbleBody := formatHubbleLogs(hubbleBody)

	sections := make([]string, 0, 2)
	if strings.TrimSpace(netpolBody) != "" {
		sections = append(sections, "Network policy:\n\n"+strings.TrimSpace(netpolBody))
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

func buildHubbleObserve(provider kube.Provider, target appResourceTarget) (string, error) {
	if isMockProvider(provider) {
		return buildMockHubbleView(target), nil
	}
	contextName, _ := provider.GetContext()
	return diagnostics.ResolveHubbleProvider(nil).ObservePod(target.Namespace+"/"+target.Name, 100, contextName)
}

func isMockProvider(provider kube.Provider) bool {
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
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Warn("writeAppView encode error", "error", err)
	}
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
