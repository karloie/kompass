<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { availableViewsForNode, nodeRequestParams, viewLabel } from '../resourceViews'

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
  chromeTitle: {
    type: String,
    default: '🧭 Kompass - mock-cluster',
  },
  contextName: {
    type: String,
    default: 'mock-cluster',
  },
})

const emit = defineEmits(['close'])

const activeView = ref('')
const loading = ref(false)
const error = ref('')
const cache = ref({})
const copiedCommand = ref(false)

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
const content = computed(() => currentPayload.value?.content || '')
const fallbackCommand = computed(() => buildFallbackCommand(activeView.value, props.node, props.contextName))
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
  return highlightContent(content.value, mode)
})

watch(
  () => [props.node?.key, props.initialView, views.value.join(',')],
  () => {
    cache.value = {}
    error.value = ''
    activeView.value = pickInitialView()
  },
  { immediate: true },
)

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

  const kubectl = kubectlPrefix(contextName)
  const nsFlag = target.namespace ? ` -n ${shellQuote(target.namespace)}` : ''
  const kind = shellQuote(target.type)
  const name = shellQuote(target.name)

  switch (String(view || '').toLowerCase()) {
    case 'describe':
      return `${kubectl} describe ${kind} ${name}${nsFlag}`
    case 'logs':
      if (target.type !== 'pod') {
        return `${kubectl} describe ${kind} ${name}${nsFlag}`
      }
      return `${kubectl} logs ${name}${nsFlag} --tail=200`
    case 'events':
      return `${kubectl} get events${nsFlag} --field-selector involvedObject.name=${shellQuote(target.name)} --sort-by=.lastTimestamp`
    case 'yaml':
      return `${kubectl} get ${kind} ${name}${nsFlag} -o yaml`
    case 'hubble': {
      if (target.type !== 'pod' || !target.namespace) {
        return `${kubectl} get netpol${nsFlag}`
      }
      const ctx = String(contextName || '').trim()
      const hubbleCtx = ctx ? ` --context ${shellQuote(ctx)}` : ''
      const ciliumCtx = ctx ? ` --context ${shellQuote(ctx)}` : ''
      const podRef = `${target.namespace}/${target.name}`
      return `hubble observe${hubbleCtx} --namespace ${shellQuote(target.namespace)} --pod ${name} --last 100 || cilium monitor${ciliumCtx} --related-to ${shellQuote(podRef)}`
    }
    default:
      return `${kubectl} describe ${kind} ${name}${nsFlag}`
  }
}

function kubectlPrefix(contextName) {
  const ctx = String(contextName || '').trim()
  if (!ctx) {
    return 'kubectl'
  }
  return `kubectl --context ${shellQuote(ctx)}`
}

function parseNodeTarget(node) {
  const key = String(node?.key || '').trim()
  const parsed = key.split('/').filter(Boolean)

  const type = String(node?.type || parsed[0] || '').trim().toLowerCase()
  const namespace = String(node?.metadata?.namespace || (parsed.length >= 3 ? parsed[1] : '') || '').trim()
  const name = String(node?.metadata?.name || (parsed.length >= 3 ? parsed.slice(2).join('/') : parsed[1] || '') || '').trim()

  return { type, namespace, name }
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
  <section class="view" @click.self="closeView">
    <article class="view__panel">
      <header class="view__header">
        <div>
          <p class="view__eyebrow">{{ chromeTitle }}</p>
          <h2 class="view__title">{{ title }}</h2>
        </div>

        <button class="view__close" type="button" @click="closeView">Close</button>
      </header>

      <div v-if="fallbackCommand" class="view__command-row">
        <div class="view__command-wrap">
          <input
            id="view-fallback-command"
            class="view__command-input"
            type="text"
            :value="fallbackCommand"
            readonly
            @focus="$event.target.select()"
          />
          <button class="view__command-copy" type="button" @click="copyFallbackCommand">
            {{ copiedCommand ? 'Copied' : 'Copy' }}
          </button>
        </div>
      </div>

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
      <pre v-else class="view__content" v-html="highlightedContent"></pre>
    </article>
  </section>
</template>

<style scoped>
.view {
  position: fixed;
  inset: 0;
  z-index: 20;
  display: flex;
  justify-content: center;
  align-items: flex-start;
  overflow-y: auto;
  padding: 0.9rem 1.5rem 1.5rem;
  background: color-mix(in srgb, var(--page-bg) 40%, transparent);
  backdrop-filter: blur(8px);
}

.view__panel {
  width: min(1100px, 100%);
  min-height: min(18rem, calc(100dvh - 2.4rem));
  max-height: calc(100dvh - 2.4rem);
  display: grid;
  grid-template-rows: auto auto auto minmax(0, 1fr);
  gap: 0.9rem;
  padding: 1.1rem;
  margin: 0 auto;
  border: 1px solid var(--border-color);
  border-radius: 14px;
  background: var(--panel-bg);
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.22);
}

.view__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
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
  border-radius: 10px;
  cursor: pointer;
}

.view__close {
  padding: 0.42rem 0.78rem;
  font-size: 0.92rem;
  line-height: 1.2;
}

.view__tabs {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}

.view__tab {
  padding: 0.38rem 0.7rem;
  font-size: 0.92rem;
  line-height: 1.2;
}

.view__tab--active {
  background: var(--accent-color);
  color: var(--panel-bg);
  border-color: var(--accent-color);
}

.view__command-row {
  display: grid;
  gap: 0.35rem;
}

.view__command-label {
  font-size: 0.75rem;
  color: var(--text-muted);
  font-weight: 600;
}

.view__command-wrap {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 0.5rem;
}

.view__command-input {
  width: 100%;
  min-width: 0;
  border: 1px solid var(--button-border);
  background: var(--button-bg);
  color: var(--text-main);
  border-radius: 8px;
  padding: 0.45rem 0.6rem;
  font: 0.82rem/1.4 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
}

.view__command-copy {
  border: 1px solid var(--button-border);
  background: var(--button-bg);
  color: var(--button-text);
  border-radius: 8px;
  padding: 0.45rem 0.7rem;
  cursor: pointer;
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
  border-radius: 10px;
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
  color: #9a4d00;
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

.view__content :deep(.view__token--string) {
  color: #b23a7a;
}
.view__content :deep(.view__token--bool) {
  color: #1a7a1a;
}
.view__content :deep(.view__token--null) {
  color: #888;
  font-style: italic;
}
.view__content :deep(.view__token--log-prefix) {
  color: #888;
  font-weight: 400;
}
.view__content :deep(.view__token--stacktrace) {
  color: #888;
  font-style: italic;
}

.view__content :deep(.view__token--allow) {
  color: #8ed8ff;
}

.view__content :deep(.view__token--deny) {
  color: #ff9c91;
}
</style>