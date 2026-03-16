// Basic syntax highlighter for YAML, logs, and generic text.
// Uses placeholders so later regex passes never mutate generated HTML tags.

const HTML_ESCAPE_MAP = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
}

function formatISODate(match, yyyy, mm, dd, hh, mi, ss, ms, zone, ...rest) {
  const mark = rest[rest.length - 1]
  let out = mark(`${mm}.${dd}.${yyyy}`, 'view__token--date-main')
  if (hh && mi) {
    out += ' ' + mark(`${hh}:${mi}`, 'view__token--time-main')
  }
  if (ss) {
    out += mark(`:${ss}`, 'view__token--time-meta')
  }
  if (ms) {
    out += mark(`.${ms}`, 'view__token--time-meta')
  }
  if (zone) {
    out += ' ' + mark(zone, 'view__token--time-meta')
  }
  return out
}

function formatPrettyDate(match, mm, dd, yyyy, hh, mi, ss, ms, zone, ...rest) {
  const mark = rest[rest.length - 1]
  let out = mark(`${mm}.${dd}.${yyyy}`, 'view__token--date-main')
  if (hh && mi) {
    out += ' ' + mark(`${hh}:${mi}`, 'view__token--time-main')
  }
  if (ss) {
    out += mark(`:${ss}`, 'view__token--time-meta')
  }
  if (ms) {
    out += mark(`.${ms}`, 'view__token--time-meta')
  }
  if (zone) {
    out += ' ' + mark(zone, 'view__token--time-meta')
  }
  return out
}

function formatTimeOnly(match, hh, mi, ss, ms, zone, ...rest) {
  const mark = rest[rest.length - 1]
  let out = mark(`${hh}:${mi}`, 'view__token--time-main')
  if (ss) {
    out += mark(`:${ss}`, 'view__token--time-meta')
  }
  if (ms) {
    out += mark(`.${ms}`, 'view__token--time-meta')
  }
  if (zone) {
    out += ' ' + mark(zone, 'view__token--time-meta')
  }
  return out
}

function formatRFCDateTime(match, weekday, datePart, hh, mi, ss, zone, ...rest) {
  const mark = rest[rest.length - 1]
  let out = ''
  if (weekday) {
    out += mark(weekday, 'view__token--time-meta') + ' '
  }
  out += mark(datePart, 'view__token--date-main')
  out += ' ' + mark(`${hh}:${mi}`, 'view__token--time-main')
  if (ss) {
    out += mark(`:${ss}`, 'view__token--time-meta')
  }
  if (zone) {
    out += ' ' + mark(zone, 'view__token--time-meta')
  }
  return out
}

function formatISOBasic(match, yyyy, mm, dd, hh, mi, ss, ms, zone, ...rest) {
  const mark = rest[rest.length - 1]
  let out = mark(`${mm}.${dd}.${yyyy}`, 'view__token--date-main')
  out += ' ' + mark(`${hh}:${mi}`, 'view__token--time-main')
  out += mark(`:${ss}`, 'view__token--time-meta')
  if (ms) {
    out += mark(`.${ms}`, 'view__token--time-meta')
  }
  if (zone) {
    out += ' ' + mark(zone, 'view__token--time-meta')
  }
  return out
}

const TOKEN_PATTERNS = {
  yaml: [
    {
      className: 'view__token--yaml-key',
      // Match YAML keys at line start only.
      regex: /^([ ]*)([A-Za-z_][\w.-]*):/gm,
      replacer: (match, indent, key, ...rest) => {
        const mark = rest[rest.length - 1]
        return `${indent}${mark(key, 'view__token--yaml-key')}:`
      },
    },
    { className: 'view__token--string', regex: /("[^"]*"|'[^']*')/g },
    { className: 'view__token--bool', regex: /\b(?:true|false)\b/gi },
    { className: 'view__token--null', regex: /\bnull\b/gi },
    { className: 'view__token--number', regex: /\b(?:0x[0-9A-Fa-f]+|\d+(?:\.\d+)?)\b/g },
    {
      className: 'view__token--datetime',
      regex: /\b(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2})(?::(\d{2}))?(?:\.(\d+))?([zZ]|[+-]\d{2}:?\d{2})?)?\b/g,
      replacer: formatISODate,
    },
    {
      className: 'view__token--datetime-pretty',
      regex: /\b(\d{2})\.(\d{2})\.(\d{4})(?:\s+(\d{2}):(\d{2})(?::(\d{2}))?(?:\.(\d+))?)?(?:\s*([zZ]|[+-]\d{2}:?\d{2}))?\b/g,
      replacer: formatPrettyDate,
    },
    {
      className: 'view__token--time-only',
      regex: /\b(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(?:\s*([zZ]|[+-]\d{2}:?\d{2}))?\b/g,
      replacer: formatTimeOnly,
    },
  ],
  logs: [
    {
      className: 'view__token--datetime-rfc1123',
      regex: /\b((?:Mon|Tue|Wed|Thu|Fri|Sat|Sun),)\s+(\d{2}\s+(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{4})\s+(\d{2}):(\d{2}):(\d{2})(?:\s+([A-Za-z]{2,5}|[+-]\d{4}))?\b/g,
      replacer: formatRFCDateTime,
    },
    {
      className: 'view__token--datetime-rfc822',
      regex: /\b(\d{2}\s+(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{2})\s+(\d{2}):(\d{2})(?::(\d{2}))?(?:\s+([A-Za-z]{2,5}|[+-]\d{4}))\b/g,
      replacer: (match, datePart, hh, mi, ss, zone, ...rest) => formatRFCDateTime(match, '', datePart, hh, mi, ss, zone, ...rest),
    },
    {
      className: 'view__token--datetime-iso-basic',
      regex: /\b(\d{4})(\d{2})(\d{2})[T ](\d{2})(\d{2})(\d{2})(?:\.(\d+))?([zZ]|[+-]\d{2}:?\d{2})\b/g,
      replacer: formatISOBasic,
    },
    {
      className: 'view__token--datetime',
      regex: /\b(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2})(?::(\d{2}))?(?:\.(\d+))?([zZ]|[+-]\d{2}:?\d{2})?)?\b/g,
      replacer: formatISODate,
    },
    {
      className: 'view__token--datetime-pretty',
      regex: /\b(\d{2})\.(\d{2})\.(\d{4})(?:\s+(\d{2}):(\d{2})(?::(\d{2}))?(?:\.(\d+))?)?(?:\s*([zZ]|[+-]\d{2}:?\d{2}))?\b/g,
      replacer: formatPrettyDate,
    },
    {
      className: 'view__token--time-only',
      regex: /\b(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(?:\s*([zZ]|[+-]\d{2}:?\d{2}))?\b/g,
      replacer: formatTimeOnly,
    },
    { className: 'view__token--level-debug', regex: /\bDEBUG\b/g },
    { className: 'view__token--level-info', regex: /\bINFO\b/g },
    { className: 'view__token--level-warn', regex: /\bWARN\b/g },
    {
      className: 'view__token--time-meta',
      regex: /\b\d{10}(?:\d{3})?\b/g,
      replacer: (match, ...rest) => {
        const mark = rest[rest.length - 1]
        return mark(match, 'view__token--time-meta')
      },
    },
    { className: 'view__token--number', regex: /\b(?:0x[0-9A-Fa-f]+|\d+(?:\.\d+)?)\b/g },
    { className: 'view__token--log-prefix', regex: /^(\s*\[?\w+\]?[:\-])\s+/gm },
    { className: 'view__token--stacktrace', regex: /^\s+at\s+.*$/gm },
  ],
  cilium: [
    {
      className: 'view__token--datetime-rfc1123',
      regex: /\b((?:Mon|Tue|Wed|Thu|Fri|Sat|Sun),)\s+(\d{2}\s+(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{4})\s+(\d{2}):(\d{2}):(\d{2})(?:\s+([A-Za-z]{2,5}|[+-]\d{4}))?\b/g,
      replacer: formatRFCDateTime,
    },
    {
      className: 'view__token--datetime-rfc822',
      regex: /\b(\d{2}\s+(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{2})\s+(\d{2}):(\d{2})(?::(\d{2}))?(?:\s+([A-Za-z]{2,5}|[+-]\d{4}))\b/g,
      replacer: (match, datePart, hh, mi, ss, zone, ...rest) => formatRFCDateTime(match, '', datePart, hh, mi, ss, zone, ...rest),
    },
    {
      className: 'view__token--datetime-iso-basic',
      regex: /\b(\d{4})(\d{2})(\d{2})[T ](\d{2})(\d{2})(\d{2})(?:\.(\d+))?([zZ]|[+-]\d{2}:?\d{2})\b/g,
      replacer: formatISOBasic,
    },
    {
      className: 'view__token--datetime',
      regex: /\b(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2})(?::(\d{2}))?(?:\.(\d+))?([zZ]|[+-]\d{2}:?\d{2})?)?\b/g,
      replacer: formatISODate,
    },
    {
      className: 'view__token--datetime-pretty',
      regex: /\b(\d{2})\.(\d{2})\.(\d{4})(?:\s+(\d{2}):(\d{2})(?::(\d{2}))?(?:\.(\d+))?)?(?:\s*([zZ]|[+-]\d{2}:?\d{2}))?\b/g,
      replacer: formatPrettyDate,
    },
    {
      className: 'view__token--time-only',
      regex: /\b(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(?:\s*([zZ]|[+-]\d{2}:?\d{2}))?\b/g,
      replacer: formatTimeOnly,
    },
    {
      className: 'view__token--allow',
      regex: /\b(?:ALLOW|ALLOWED|OPEN|FORWARDED|PERMIT)\b/g,
      replacer: (match, ...rest) => {
        const mark = rest[rest.length - 1]
        return mark(`✅ ${match}`, 'view__token--allow')
      },
    },
    {
      className: 'view__token--deny',
      regex: /\b(?:DENY|DENIED|DROP|DROPPED|BLOCKED)\b/g,
      replacer: (match, ...rest) => {
        const mark = rest[rest.length - 1]
        return mark(`⛔ ${match}`, 'view__token--deny')
      },
    },
    {
      className: 'view__token--time-meta',
      regex: /\b\d{10}(?:\d{3})?\b/g,
      replacer: (match, ...rest) => {
        const mark = rest[rest.length - 1]
        return mark(match, 'view__token--time-meta')
      },
    },
    { className: 'view__token--number', regex: /\b(?:0x[0-9A-Fa-f]+|\d+(?:\.\d+)?)\b/g },
    { className: 'view__token--log-prefix', regex: /^([A-Z]+:)\s+/gm },
  ],
  default: [
    {
      className: 'view__token--datetime',
      regex: /\b(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2})(?::(\d{2}))?(?:\.(\d+))?([zZ]|[+-]\d{2}:?\d{2})?)?\b/g,
      replacer: formatISODate,
    },
    {
      className: 'view__token--datetime-pretty',
      regex: /\b(\d{2})\.(\d{2})\.(\d{4})(?:\s+(\d{2}):(\d{2})(?::(\d{2}))?(?:\.(\d+))?)?(?:\s*([zZ]|[+-]\d{2}:?\d{2}))?\b/g,
      replacer: formatPrettyDate,
    },
    {
      className: 'view__token--time-only',
      regex: /\b(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(?:\s*([zZ]|[+-]\d{2}:?\d{2}))?\b/g,
      replacer: formatTimeOnly,
    },
    { className: 'view__token--number', regex: /\b(?:0x[0-9A-Fa-f]+|\d+(?:\.\d+)?)\b/g },
  ],
}

function escapeHtml(value) {
  return String(value).replace(/[&<>"']/g, (char) => HTML_ESCAPE_MAP[char])
}

function indexToAlpha(index) {
  let n = index
  let out = ''
  do {
    out = String.fromCharCode(65 + (n % 26)) + out
    n = Math.floor(n / 26) - 1
  } while (n >= 0)
  return out
}

export function highlightContent(source, mode = 'default') {
  const raw = String(source || '')
  if (!raw) {
    return ''
  }

  const patterns = TOKEN_PATTERNS[mode] || TOKEN_PATTERNS.default
  const htmlStore = []

  const mark = (value, className) => {
    const idx = htmlStore.length
    const token = `@@KMP_${indexToAlpha(idx)}@@`
    htmlStore.push({ token, html: `<span class="view__token ${className}">${value}</span>` })
    return token
  }

  // Escape first so user content can never inject HTML.
  let text = escapeHtml(raw)

  // Apply highlighting by inserting placeholders (not raw HTML).
  for (const pattern of patterns) {
    text = text.replace(pattern.regex, (...args) => {
      if (pattern.replacer) {
        return pattern.replacer(...args, mark)
      }
      const match = args[0]
      return mark(match, pattern.className)
    })
  }

  // Restore placeholders as HTML spans.
  for (const item of htmlStore) {
    text = text.split(item.token).join(item.html)
  }

  return text
}
