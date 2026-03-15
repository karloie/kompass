package tui

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

var (
	fieldPrefixPattern  = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9 _-]*):(\s*)(.*)$`)
	anyFieldPattern     = regexp.MustCompile(`^([^:]+):(\s*)(.*)$`)
	yamlKeyPattern      = regexp.MustCompile(`^(\s*(?:-\s+)??)([^:#\n][^:\n]*):(\s*)(.*)$`)
	eventSeverityWords  = regexp.MustCompile(`(?i)\b(warning|normal|error|failed)\b`)
	logSeverityWords    = regexp.MustCompile(`(?i)\b(trace|debug|info|warn|warning|error|fatal|panic|failed)\b`)
	logLinePattern      = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:[\.,]\d+)?(?:Z|[+-]\d{2}:?\d{2})?)\s+([A-Za-z]+)\s+(.*)$`)
	logLevelPattern     = regexp.MustCompile(`^([A-Za-z]+)\s+(.*)$`)
	logMsgKeyPattern    = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9_-]*:`)
	hubblePodRefPattern = regexp.MustCompile(`\b[a-z0-9](?:[-a-z0-9]*[a-z0-9])?/[a-z0-9](?:[-a-z0-9.]*[a-z0-9])?\b`)
	hubbleIPPortPattern = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}:\d+\b`)
	hubblePolicyPattern = regexp.MustCompile(`\bpolicy-verdict:[^\s]+\b`)

	eventsKnownKeys = map[string]bool{
		"Type": true, "Reason": true, "Object": true, "InvolvedObject": true,
		"Source": true, "Message": true, "FirstSeen": true, "LastSeen": true,
		"Count": true, "Age": true,
	}
	logLevelKinds = map[string]string{
		"TRACE":   "debug",
		"DEBUG":   "debug",
		"INFO":    "info",
		"WARN":    "warn",
		"WARNING": "warn",
		"ERROR":   "error",
		"FATAL":   "error",
		"PANIC":   "error",
		"FAILED":  "error",
	}

	defaultHighlightTheme = highlightTheme{
		// A single global theme keeps YAML and non-YAML views visually aligned.
		YAMLStyleCandidates: []string{"nord", "github-dark", "native"},
		YAMLSimpleMode:      false,
		YAMLKeyColor:        "81",
		YAMLCommentColor:    "244",
		DescribeKeyColor:    "81",
		EventsKeyColor:      "81",
		EventWarnColor:      "110",
		EventErrColor:       "196",
		EventOKColor:        "81",
		LogTimestampColor:   "244",
		LogLevelInfoColor:   "81",
		LogLevelWarnColor:   "110",
		LogLevelErrColor:    "196",
		LogLevelDebugColor:  "117",
		LogKeyColor:         "81",
	}

	chromaFormatter = formatters.Get("terminal256")
	yamlLexer       = lexers.Get("yaml")

	currentHighlightTheme = defaultHighlightTheme

	describeKeyStyle  lipgloss.Style
	eventsKeyStyle    lipgloss.Style
	eventWarnStyle    lipgloss.Style
	eventErrStyle     lipgloss.Style
	eventOKStyle      lipgloss.Style
	logTimestampStyle lipgloss.Style
	logLevelInfoStyle lipgloss.Style
	logLevelWarnStyle lipgloss.Style
	logLevelErrStyle  lipgloss.Style
	logLevelDbgStyle  lipgloss.Style
	logKeyStyle       lipgloss.Style
	yamlKeyStyle      lipgloss.Style
	yamlCommentStyle  lipgloss.Style
	yamlChromaStyle   *chroma.Style
)

// Fixed styles for the netpol page — not theme-driven because the semantic
// meaning (green=open, orange=restricted, red=deny) should not vary by theme.
var (
	netpolHeaderStyle   = lipgloss.NewStyle().Bold(true)
	netpolDimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	netpolOpenStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("76"))
	netpolRestrictStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	netpolPolicyStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	netpolAllowStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("76"))
	netpolDenyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	hubbleHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	hubblePodStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	hubbleIPPortStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("110"))
	hubblePolicyStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	hubbleIngressStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("76"))
	hubbleEgressStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
)

type highlightTheme struct {
	YAMLStyleCandidates []string
	YAMLSimpleMode      bool
	YAMLKeyColor        string
	YAMLCommentColor    string
	DescribeKeyColor    string
	EventsKeyColor      string
	EventWarnColor      string
	EventErrColor       string
	EventOKColor        string
	LogTimestampColor   string
	LogLevelInfoColor   string
	LogLevelWarnColor   string
	LogLevelErrColor    string
	LogLevelDebugColor  string
	LogKeyColor         string
}

func applyHighlightTheme(theme highlightTheme) {
	currentHighlightTheme = theme

	describeKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.DescribeKeyColor))
	eventsKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.EventsKeyColor))
	eventWarnStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.EventWarnColor))
	eventErrStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.EventErrColor))
	eventOKStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.EventOKColor))
	logTimestampStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.LogTimestampColor))
	logLevelInfoStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.LogLevelInfoColor))
	logLevelWarnStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.LogLevelWarnColor))
	logLevelErrStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.LogLevelErrColor))
	logLevelDbgStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.LogLevelDebugColor))
	logKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.LogKeyColor))
	yamlKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.YAMLKeyColor))
	yamlCommentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.YAMLCommentColor))
	yamlChromaStyle = resolveChromaStyle(theme.YAMLStyleCandidates...)
}

func highlightResourceLine(pageName, line string) string {
	switch pageName {
	case "yaml":
		return highlightYAMLLine(line)
	case "describe":
		return highlightAnyFieldLine(line, describeKeyStyle)
	case "events":
		return highlightEventLine(line)
	case "logs":
		return highlightLogLine(line)
	case "hubble":
		return highlightHubbleLine(line)
	case "netpol":
		return highlightNetpolLine(line)
	default:
		return line
	}
}

func highlightHubbleLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return line
	}
	if strings.HasPrefix(trimmed, "hubble observe ") {
		return hubbleHeaderStyle.Render(line)
	}

	out := highlightLogSeverities(line)
	out = hubblePolicyPattern.ReplaceAllStringFunc(out, func(match string) string {
		return hubblePolicyStyle.Render(match)
	})
	out = hubbleIPPortPattern.ReplaceAllStringFunc(out, func(match string) string {
		return hubbleIPPortStyle.Render(match)
	})
	out = hubblePodRefPattern.ReplaceAllStringFunc(out, func(match string) string {
		return hubblePodStyle.Render(match)
	})
	out = strings.ReplaceAll(out, " -> ", " "+hubbleEgressStyle.Render("->")+" ")
	out = strings.ReplaceAll(out, " <- ", " "+hubbleIngressStyle.Render("<-")+" ")
	return out
}

func highlightNetpolLine(line string) string {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "NetworkPolicy analysis:"):
		return netpolHeaderStyle.Render(line)
	case strings.HasPrefix(trimmed, "Labels:"):
		return netpolDimStyle.Render(line)
	case (strings.HasPrefix(trimmed, "INGRESS:") || strings.HasPrefix(trimmed, "EGRESS:")) &&
		strings.Contains(trimmed, "OPEN"):
		return netpolOpenStyle.Render(line)
	case (strings.HasPrefix(trimmed, "INGRESS:") || strings.HasPrefix(trimmed, "EGRESS:")) &&
		strings.Contains(trimmed, "RESTRICTED"):
		return netpolRestrictStyle.Render(line)
	case strings.HasPrefix(trimmed, "No NetworkPolicy"):
		return netpolDimStyle.Render(line)
	case strings.HasPrefix(trimmed, "▸"):
		return netpolPolicyStyle.Render(line)
	case strings.HasPrefix(trimmed, "✅"):
		return netpolAllowStyle.Render(line)
	case strings.HasPrefix(trimmed, "🚫"), strings.HasPrefix(trimmed, "(no allow"):
		return netpolDenyStyle.Render(line)
	default:
		return line
	}
}

func resolveChromaStyle(candidates ...string) *chroma.Style {
	for _, name := range candidates {
		if strings.TrimSpace(name) == "" {
			continue
		}
		if style := styles.Get(name); style != nil {
			return style
		}
	}

	style := styles.Get("native")
	if style == nil {
		return styles.Fallback
	}
	return style
}

func highlightYAMLLine(line string) string {
	if strings.TrimSpace(line) == "" || yamlLexer == nil {
		return line
	}
	if currentHighlightTheme.YAMLSimpleMode {
		return highlightSimpleYAMLLine(line)
	}
	return highlightYAMLLineWithKeyOverride(line)
}

func highlightYAMLLineWithKeyOverride(line string) string {
	match := yamlKeyPattern.FindStringSubmatch(line)
	if len(match) != 5 {
		return highlightWithLexer(line, yamlLexer, yamlChromaStyle)
	}

	indentOrDash := match[1]
	key := strings.TrimRight(match[2], " ")
	spacing := match[3]
	rest := match[4]

	if strings.TrimSpace(rest) == "" {
		return indentOrDash + yamlKeyStyle.Render(key+":")
	}

	return indentOrDash + yamlKeyStyle.Render(key+":") + spacing + highlightWithLexer(rest, yamlLexer, yamlChromaStyle)
}

func highlightSimpleYAMLLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return line
	}
	if strings.HasPrefix(trimmed, "#") {
		return yamlCommentStyle.Render(line)
	}

	match := yamlKeyPattern.FindStringSubmatch(line)
	if len(match) != 5 {
		return line
	}
	indentOrDash := match[1]
	key := strings.TrimRight(match[2], " ")
	spacing := match[3]
	rest := match[4]
	return indentOrDash + yamlKeyStyle.Render(key+":") + spacing + rest
}

func highlightAnyFieldLine(line string, style lipgloss.Style) string {
	match := anyFieldPattern.FindStringSubmatch(line)
	if len(match) != 4 {
		return line
	}
	key := strings.TrimSpace(match[1])
	if key == "" {
		return line
	}
	return style.Render(match[1]+":") + match[2] + match[3]
}

func highlightEventLine(line string) string {
	out := highlightKnownFieldLine(line, eventsKnownKeys, eventsKeyStyle)
	out = eventSeverityWords.ReplaceAllStringFunc(out, func(match string) string {
		switch strings.ToLower(match) {
		case "warning":
			return eventWarnStyle.Render(match)
		case "normal":
			return eventOKStyle.Render(match)
		case "error", "failed":
			return eventErrStyle.Render(match)
		default:
			return match
		}
	})
	return out
}

func highlightLogLine(line string) string {
	if strings.TrimSpace(line) == "" {
		return line
	}

	if match := logLinePattern.FindStringSubmatch(line); len(match) == 4 {
		timestamp := logTimestampStyle.Render(match[1])
		level := renderLogLevel(match[2])
		message := highlightLogMessage(match[3])
		return timestamp + " " + level + " " + message
	}

	if match := logLevelPattern.FindStringSubmatch(line); len(match) == 3 {
		if isLogLevelToken(match[1]) {
			return renderLogLevel(match[1]) + " " + highlightLogMessage(match[2])
		}
	}

	// Conservative fallback: keep line plain except log level words.
	return highlightLogSeverities(line)
}

func highlightLogMessage(msg string) string {
	return logMsgKeyPattern.ReplaceAllStringFunc(msg, func(match string) string {
		return logKeyStyle.Render(match)
	})
}

func renderLogLevel(level string) string {
	upper := strings.ToUpper(level)
	switch logLevelKinds[upper] {
	case "debug":
		return logLevelDbgStyle.Render(level)
	case "info":
		return logLevelInfoStyle.Render(level)
	case "warn":
		return logLevelWarnStyle.Render(level)
	case "error":
		return logLevelErrStyle.Render(level)
	default:
		return level
	}
}

func isLogLevelToken(token string) bool {
	_, ok := logLevelKinds[strings.ToUpper(token)]
	return ok
}

func highlightLogSeverities(line string) string {
	return logSeverityWords.ReplaceAllStringFunc(line, renderLogLevel)
}

func highlightKnownFieldLine(line string, known map[string]bool, style lipgloss.Style) string {
	match := fieldPrefixPattern.FindStringSubmatch(line)
	if len(match) != 4 {
		return line
	}
	key := strings.TrimSpace(match[1])
	if !known[key] {
		return line
	}
	return style.Render(match[1]+":") + match[2] + match[3]
}

func highlightWithLexer(line string, lexer chroma.Lexer, style *chroma.Style) string {
	if strings.TrimSpace(line) == "" || lexer == nil || chromaFormatter == nil || style == nil {
		return line
	}

	lexer = chroma.Coalesce(lexer)
	iterator, err := lexer.Tokenise(nil, line)
	if err != nil {
		return line
	}

	var b bytes.Buffer
	if err := chromaFormatter.Format(&b, style, iterator); err != nil {
		return line
	}
	return strings.TrimRight(b.String(), "\n")
}
