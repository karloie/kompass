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
	fieldPrefixPattern = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9 _-]*):(\s*)(.*)$`)
	anyFieldPattern    = regexp.MustCompile(`^([^:]+):(\s*)(.*)$`)
	eventSeverityWords = regexp.MustCompile(`(?i)\b(warning|normal|error|failed)\b`)
	logSeverityWords   = regexp.MustCompile(`(?i)\b(trace|debug|info|warn|warning|error|fatal|panic|failed)\b`)
	logLinePattern     = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:[\.,]\d+)?(?:Z|[+-]\d{2}:?\d{2})?)\s+([A-Za-z]+)\s+(.*)$`)
	logLevelPattern    = regexp.MustCompile(`^([A-Za-z]+)\s+(.*)$`)
	logMsgKeyPattern   = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9_-]*:`)

	describeKeyStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	eventsKeyStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	eventWarnStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208"))
	eventErrStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	eventOKStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("70"))
	logTimestampStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	logLevelInfoStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	logLevelWarnStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	logLevelErrStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	logLevelDbgStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117"))
	logKeyStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("110"))

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

	chromaFormatter = formatters.Get("terminal256")
	chromaStyle     = resolveChromaStyle()
	yamlLexer       = lexers.Get("yaml")
)

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
	default:
		return line
	}
}

func resolveChromaStyle() *chroma.Style {
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
	return highlightWithLexer(line, yamlLexer)
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

func highlightWithLexer(line string, lexer chroma.Lexer) string {
	if strings.TrimSpace(line) == "" || lexer == nil || chromaFormatter == nil || chromaStyle == nil {
		return line
	}

	lexer = chroma.Coalesce(lexer)
	iterator, err := lexer.Tokenise(nil, line)
	if err != nil {
		return line
	}

	var b bytes.Buffer
	if err := chromaFormatter.Format(&b, chromaStyle, iterator); err != nil {
		return line
	}
	return strings.TrimRight(b.String(), "\n")
}
