<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { availableViewsForNode, nodeRequestParams, viewLabel } from '../resourceViews'
import MenuHeader from './MenuHeader.vue'

import { highlightContent } from '../highlighter'

const props = defineProps({
  node: {
    type: Object,
    required: true,
  },
  initialView: {
    type: String,
    default: 'describe',
  },
  apiBase: {
    type: String,
    default: '/api/app',
  },
  contextName: {
    type: String,
    default: 'mock-01',
  },
  contexts: {
    type: Array,
    default: () => [],
  },
  namespaces: {
    type: Array,
    default: () => [],
  },
  namespace: {
    type: String,
    default: '',
  },
  selectors: {
    type: String,
    default: '',
  },
  loading: {
    type: Boolean,
    default: false,
  },
  refreshDisabled: {
    type: Boolean,
    default: false,
  },
  themeIcon: {
    type: String,
    default: '🌙',
  },
  themeLabel: {
    type: String,
    default: 'Toggle theme',
  },
})

const emit = defineEmits(['close', 'refresh', 'update:namespace', 'update:context', 'update:selectors', 'update:view', 'toggle-theme'])

const activeView = ref('')
const loading = ref(false)
const error = ref('')
const cache = ref({})
const copiedCommand = ref(false)
const contentFilter = ref('')
const contentEl = ref(null)

const scrollToBottomViews = new Set(['logs', 'events'])

let currentController = null
let copiedCommandTimer = null

const views = computed(() => availableViewsForNode(props.node))
const endpointMap = {
  describe: 'desc',
  logs: 'logs',
  events: 'events',
  hubble: 'hubble',
  yaml: 'yaml',
}

const currentPayload = computed(() => cache.value[activeView.value] || null)
const title = computed(() => String(props.node?.key || '').trim() || currentPayload.value?.title || '(unknown resource)')
const titleKindEmoji = computed(() => {
  if (typeof props.node?.icon === 'string' && props.node.icon.trim() !== '') {
    return props.node.icon
  }
  return ''
})
const content = computed(() => currentPayload.value?.content || '')
const fallbackCommand = computed(() => buildFallbackCommand(activeView.value, props.node, props.contextName))
const supportsContentFilter = computed(() => {
  const view = String(activeView.value || '').toLowerCase()
  return view === 'logs' || view === 'events' || view === 'hubble' || view === 'yaml'
})
const viewRequestScope = computed(() => [
  String(props.contextName || '').trim(),
  String(props.namespace || '').trim(),
  String(props.selectors || '').trim(),
])
const normalizedContentFilter = computed(() => String(contentFilter.value || '').trim().toLowerCase())
const filteredContent = computed(() => {
  const source = String(content.value || '')
  if (!supportsContentFilter.value || !normalizedContentFilter.value) {
    return source
  }
  return source
    .split('\n')
    .filter((line) => line.toLowerCase().includes(normalizedContentFilter.value))
    .join('\n')
})
const contentFilterStats = computed(() => {
  if (!supportsContentFilter.value || !normalizedContentFilter.value) {
    return ''
  }
  const total = String(content.value || '').split('\n').filter((line) => line.length > 0).length
  const matched = String(filteredContent.value || '').split('\n').filter((line) => line.length > 0).length
  return `${matched}/${total}`
})
const highlightedContent = computed(() => {
  const view = (activeView.value || '').toLowerCase()
  let mode = 'default'
  if (view === 'yaml' || view === 'describe') {
    mode = 'yaml'
  } else if (view === 'logs' || view === 'events') {
    mode = 'logs'
  } else if (view === 'hubble') {
    mode = 'cilium'
  }
  return highlightContent(filteredContent.value, mode)
})

watch(
  () => [props.node?.key, props.initialView, views.value.join(',')],
  () => {
    cache.value = {}
    error.value = ''
    contentFilter.value = ''
    activeView.value = pickInitialView()
  },
  { immediate: true },
)

watch(activeView, (value) => {
  contentFilter.value = ''
  const next = String(value || '').trim()
  if (next) {
    emit('update:view', next)
  }
})

watch(highlightedContent, async () => {
  const view = String(activeView.value || '').toLowerCase()
  if (!scrollToBottomViews.has(view)) return
  await nextTick()
  if (contentEl.value) {
    contentEl.value.scrollTop = contentEl.value.scrollHeight
  }
})

watch(
  activeView,
  (value) => {
    if (!value || cache.value[value]) {
      return
    }
    fetchView(value)
  },
  { immediate: true },
)

watch(viewRequestScope, () => {
  const view = String(activeView.value || '').trim()
  if (!view) {
    return
  }
  cache.value = {
    ...cache.value,
    [view]: null,
  }
  fetchView(view)
})

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  if (currentController) {
    currentController.abort()
  }
  if (copiedCommandTimer) {
    clearTimeout(copiedCommandTimer)
  }
})

function pickInitialView() {
  if (!views.value.length) {
    return ''
  }
  if (views.value.includes(props.initialView)) {
    return props.initialView
  }
  return views.value[0]
}

async function fetchView(view) {
  if (currentController) {
    currentController.abort()
  }

  const controller = new AbortController()
  currentController = controller
  loading.value = true
  error.value = ''

  try {
    const params = nodeRequestParams(props.node)
    const selectedContext = String(props.contextName || '').trim()
    const selectors = String(props.selectors || '').trim()
    if (selectedContext) {
      params.set('context', selectedContext)
    }
    if (selectors) {
      params.set('selectors', selectors)
    }
    const endpoint = endpointMap[view] || view
    const response = await fetch(`${props.apiBase}/${endpoint}?${params.toString()}`, {
      headers: {
        Accept: 'application/json',
      },
      signal: controller.signal,
    })
    if (!response.ok) {
      throw new Error(await response.text() || `request failed: ${response.status}`)
    }
    const payload = await response.json()
    cache.value = {
      ...cache.value,
      [view]: payload,
    }
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      return
    }
    error.value = err instanceof Error ? err.message : 'failed to load view'
  } finally {
    if (currentController === controller) {
      currentController = null
    }
    loading.value = false
  }
}

function closeView() {
  emit('close')
}

function onKeydown(event) {
  if (event.key === 'Escape') {
    closeView()
  }
}

async function copyFallbackCommand() {
  const command = fallbackCommand.value
  if (!command) {
    return
  }
  try {
    await navigator.clipboard.writeText(command)
  } catch {
    return
  }
  copiedCommand.value = true
  if (copiedCommandTimer) {
    clearTimeout(copiedCommandTimer)
  }
  copiedCommandTimer = setTimeout(() => {
    copiedCommand.value = false
  }, 1200)
}

function buildFallbackCommand(view, node, contextName) {
  const target = parseNodeTarget(node)
  if (!target.type || !target.name) {
    return ''
  }

  const kubectl = 'kubectl'
  const context = String(contextName || '').trim()
  const namespace = String(target.namespace || '').trim()
  const kind = shellQuote(target.type)
  const name = shellQuote(target.name)

  switch (String(view || '').toLowerCase()) {
    case 'describe':
      return appendCommandScopeFlags(`${kubectl} describe ${kind} ${name}`, namespace, context)
    case 'logs':
      if (target.type !== 'pod') {
        return appendCommandScopeFlags(`${kubectl} describe ${kind} ${name}`, namespace, context)
      }
      return appendCommandScopeFlags(`${kubectl} logs ${name} --tail=100`, namespace, context)
    case 'events':
      return `${appendCommandScopeFlags(`${kubectl} get events --field-selector involvedObject.name=${shellQuote(target.name)} --sort-by=.lastTimestamp`, namespace, context)} | tail -n 100`
    case 'yaml':
      return appendCommandScopeFlags(`${kubectl} get ${kind} ${name} -o yaml`, namespace, context)
    case 'hubble': {
      if (target.type !== 'pod' || !target.namespace) {
        return appendCommandScopeFlags(`${kubectl} get netpol`, namespace, context)
      }
      const podRef = `${target.namespace}/${target.name}`
      const hubble = appendCommandScopeFlags(`hubble observe --pod ${name} --last 100`, namespace, context)
      const cilium = appendContextFlag(`cilium monitor --related-to ${shellQuote(podRef)}`, context)
      return `${hubble} || ${cilium}`
    }
    default:
      return appendCommandScopeFlags(`${kubectl} describe ${kind} ${name}`, namespace, context)
  }
}

function appendCommandScopeFlags(command, namespace, context) {
  let out = String(command || '').trim()
  const ns = String(namespace || '').trim()
  const ctx = String(context || '').trim()
  if (ns) {
    out += ` --namespace ${shellQuote(ns)}`
  }
  if (ctx) {
    out += ` --context ${shellQuote(ctx)}`
  }
  return out
}

function appendContextFlag(command, context) {
  let out = String(command || '').trim()
  const ctx = String(context || '').trim()
  if (ctx) {
    out += ` --context ${shellQuote(ctx)}`
  }
  return out
}

function parseNodeTarget(node) {
  const key = String(node?.key || '').trim()
  const parsed = key.split('/').filter(Boolean)

  const type = String(node?.type || parsed[0] || '').trim().toLowerCase()
  const namespace = String(node?.metadata?.namespace || (parsed.length >= 3 ? parsed[1] : '') || '').trim()
  const name = String(node?.metadata?.name || (parsed.length >= 3 ? parsed.slice(2).join('/') : parsed[1] || '') || '').trim()

  return { type, namespace, name }
}

function clearContentFilter() {
  contentFilter.value = ''
}

function shellQuote(value) {
  const text = String(value || '')
  if (!text) {
    return "''"
  }
  if (/^[A-Za-z0-9_./:-]+$/.test(text)) {
    return text
  }
  return `'${text.replace(/'/g, `'"'"'`)}'`
}
</script>

<template>
  <section class="view">
    <MenuHeader
      class="view__menu-header"
      :context-name="contextName"
      :contexts="contexts"
      :namespace="namespace"
      :namespaces="namespaces"
      :selectors="selectors"
      :loading="loading"
      :refresh-disabled="refreshDisabled"
      :theme-icon="themeIcon"
      :theme-label="themeLabel"
      @refresh="emit('refresh')"
      @toggle-theme="emit('toggle-theme')"
      @update:namespace="emit('update:namespace', $event)"
      @update:context="emit('update:context', $event)"
      @update:selectors="emit('update:selectors', $event)"
    />

    <article class="view__panel">

      <header class="view__header">
        <div class="view__title-wrap">
          <span v-if="titleKindEmoji" class="view__title-emoji" aria-hidden="true">{{ titleKindEmoji }}</span>
          <h2 class="view__title">{{ title }}</h2>
        </div>
        <button
          class="view__close"
          type="button"
          aria-label="Close view"
          title="Close view"
          @click="closeView"
        >
          ❌
        </button>
      </header>

      <nav v-if="views.length" class="view__tabs" aria-label="Resource views">
        <button
          v-for="view in views"
          :key="view"
          class="view__tab"
          :class="{ 'view__tab--active': view === activeView }"
          type="button"
          @click="activeView = view"
        >
          {{ viewLabel(view) }}
        </button>

      </nav>

      <p v-if="error" class="view__error">{{ error }}</p>
      <p v-else-if="loading && !content" class="view__hint">Loading view...</p>
      <p v-else-if="!views.length" class="view__hint">No backend views available for this resource.</p>
      <p v-else-if="supportsContentFilter && normalizedContentFilter && !filteredContent" class="view__hint">No lines match the filter.</p>
      <pre v-else ref="contentEl" class="view__content" v-html="highlightedContent"></pre>

      <div v-if="fallbackCommand || supportsContentFilter" class="view__command-row">
        <div v-if="fallbackCommand" class="view__command-wrap">
          <input
            id="view-fallback-command"
            class="view__command-input"
            type="text"
            :value="fallbackCommand"
            readonly
            @focus="$event.target.select()"
          />
          <button
            class="view__command-copy"
            type="button"
            :aria-label="copiedCommand ? 'Copied command' : 'Copy command'"
            :title="copiedCommand ? 'Copied command' : 'Copy command'"
            @click="copyFallbackCommand"
          >
            {{ copiedCommand ? '✅' : '📋' }}
          </button>
        </div>

        <div v-if="supportsContentFilter" class="view__filter-inline view__filter-inline--right">
          <input
            v-model="contentFilter"
            class="view__filter-input"
            type="text"
            placeholder="Filter lines"
            autocomplete="off"
            spellcheck="false"
          />
          <span v-if="contentFilterStats" class="view__filter-stats">{{ contentFilterStats }}</span>
          <button
            class="view__filter-clear"
            type="button"
            :disabled="!contentFilter"
            title="Clear filter"
            aria-label="Clear filter"
            @click="clearContentFilter"
          >
            🧹
          </button>
        </div>
      </div>
    </article>
  </section>
</template>

<style scoped>
.view {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  overflow: hidden;
}

.view__panel {
  width: 100%;
  min-height: 0;
  flex: 1;
  max-height: none;
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr) auto;
  gap: 0.15rem;
  padding: 0.8rem;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--panel-bg);
  overflow: hidden;
}

.view__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.view__tabs {
  grid-row: 2;
}

.view__hint,
.view__error,
.view__content {
  grid-row: 3;
}

.view__eyebrow {
  margin: 0 0 0.25rem;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.view__title {
  margin: 0;
  font-size: 1.05rem;
}

.view__title-wrap {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
}

.view__title-emoji {
  line-height: 1;
  font-size: 1.05rem;
}

.view__close,
.view__tab {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: auto;
  min-width: 0;
  min-height: 0;
  max-width: max-content;
  border: 1px solid var(--button-border);
  background: var(--button-bg);
  color: var(--button-text);
  border-radius: 6px;
  cursor: pointer;
}

.view__close {
  padding: 0.42rem 0.78rem;
  font-size: 0.8rem;
  line-height: 1.2;
}

.view__tabs {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  margin: 0;
  padding: 0 0.35rem;
  position: relative;
  z-index: 1;
}

.view__tab {
  padding: 0.38rem 0.7rem;
  font-size: 0.92rem;
  line-height: 1.2;
  border-radius: 8px 8px 0 0;
  margin-bottom: -1px;
  background: var(--panel-bg);
  color: var(--text-main);
  border-color: var(--border-color);
}

.view__tab--active {
  background: var(--button-bg);
  color: var(--button-text);
  border-color: var(--button-border);
  border-bottom-color: var(--panel-bg);
}

.view__command-row {
  grid-row: 4;
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

.view__command-label {
  font-size: 0.75rem;
  color: var(--text-muted);
  font-weight: 600;
}

.view__command-wrap {
  flex: 1;
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 0.6rem;
}

.view__command-input {
  width: 100%;
  min-width: 0;
  border: 1px solid var(--button-border);
  background: var(--button-bg);
  color: var(--text-main);
  border-radius: 6px;
  padding: 0.4rem 0.5rem;
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 0.9rem;
  line-height: 1.2;
}

.view__command-copy {
  border: 1px solid var(--button-border);
  background: var(--button-bg);
  color: var(--button-text);
  border-radius: 6px;
  padding: 0.4rem 0.65rem;
  font-size: 0.9rem;
  line-height: 1;
  cursor: pointer;
}

.view__filter-inline {
  display: grid;
  grid-template-columns: minmax(9rem, 14rem) auto auto;
  gap: 0.5rem;
  align-items: center;
}

.view__filter-inline--right {
  width: 25%;
  min-width: 14rem;
  margin-left: auto;
}

.view__filter-input {
  width: 100%;
  min-width: 0;
  border: 1px solid var(--button-border);
  background: var(--button-bg);
  color: var(--text-main);
  border-radius: 6px;
  padding: 0.38rem 0.5rem;
  font-size: 0.88rem;
  line-height: 1.2;
}

.view__filter-stats {
  color: var(--text-muted);
  font-size: 0.78rem;
  font-weight: 600;
}

.view__filter-clear {
  border: 1px solid var(--button-border);
  background: var(--button-bg);
  color: var(--button-text);
  border-radius: 6px;
  padding: 0.35rem 0.55rem;
  font-size: 0.85rem;
  line-height: 1;
  cursor: pointer;
}

.view__filter-clear:disabled {
  opacity: 0.5;
  cursor: default;
}

.view__hint,
.view__error {
  margin: 0;
  color: var(--text-muted);
}

.view__error {
  color: var(--danger-text);
}

.view__content {
  margin: 0;
  height: 100%;
  min-height: 0;
  overflow: auto;
  padding: 1rem;
  border-radius: 0 6px 6px 6px;
  border: 1px solid var(--border-color);
  background: color-mix(in srgb, var(--button-bg) 65%, var(--panel-bg));
  color: var(--text-main);
  white-space: pre-wrap;
  word-break: break-word;
  font: 0.87rem/1.45 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
}

.view__content :deep(.view__token) {
  font-weight: 600;
}

.view__content :deep(.view__token--yaml-key) {
  color: var(--accent-color);
}

.view__content :deep(.view__token--date) {
  color: var(--accent-color);
}

.view__content :deep(.view__token--date-main) {
  color: var(--accent-color);
}

.view__content :deep(.view__token--time-main) {
  color: var(--accent-strong);
}

.view__content :deep(.view__token--time-meta) {
  color: var(--accent-cyan);
}

.view__content :deep(.view__token--number) {
  color: #b26cff;
}

.view__content :deep(.view__token--level-trace) {
  color: #666;
}

.view__content :deep(.view__token--level-debug) {
  color: var(--accent-color);
}

.view__content :deep(.view__token--level-info) {
  color: var(--accent-strong);
}

.view__content :deep(.view__token--level-warn) {
  color: var(--accent-cyan);
}

.view__content :deep(.view__token--level-error) {
  color: #ff6b6b;
}

.view__content :deep(.view__token--level-fatal) {
  color: #ff3333;
  font-weight: 700;
}

.view__content :deep(.view__token--string) {
  color: #b23a7a;
}
.view__content :deep(.view__token--bool) {
  color: var(--accent-cyan);
}
.view__content :deep(.view__token--null) {
  color: #7f7f7f;
  font-style: italic;
}
.view__content :deep(.view__token--log-prefix) {
  color: #7f7f7f;
  font-weight: 400;
}
.view__content :deep(.view__token--stacktrace) {
  color: #7f7f7f;
  font-style: italic;
}

.view__content :deep(.view__token--allow) {
  color: #8ed8ff;
}

.view__content :deep(.view__token--deny) {
  color: #ff9c91;
}
</style>