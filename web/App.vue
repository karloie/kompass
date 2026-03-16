<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import Menu from './components/Menu.vue'
import Trees from './components/Trees.vue'
import Views from './components/Views.vue'

const props = defineProps({
  bootstrapConfig: {
    type: Object,
    default: null,
  },
  bootstrapData: {
    type: Object,
    default: null,
  },
})

const theme = ref('light')
const contextTitle = ref('')
const contexts = ref([])
const selectedContext = ref(resolveInitialContext())
const namespaces = ref([])
const selectedNamespace = ref(resolveInitialNamespace())
const searchDraftQuery = ref('')
const appliedSearchQuery = ref('')
const loading = ref(false)
const refreshKey = ref(0)
const activeResourceView = ref(null)
const scopeCache = new Map()

const themeIcon = computed(() => (theme.value === 'dark' ? '☀️' : '🌙'))
const themeLabel = computed(() => (theme.value === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'))
const mode = computed(() => String(props.bootstrapConfig?.mode || 'dynamic').trim().toLowerCase())
const isStaticMode = computed(() => mode.value === 'static')
const appApiBase = computed(() => '/api/app')
const brandedContextTitle = computed(() => formatKompassTitle(viewContextName.value))
const viewContextName = computed(() => String(selectedContext.value || contextTitle.value || '').trim())
const contextOptions = computed(() => {
  const values = new Set(contexts.value.map((item) => String(item || '').trim()).filter(Boolean))
  const current = String(viewContextName.value || '').trim()
  if (current) {
    values.add(current)
  }
  return [...values]
})
const filtering = computed(() => searchDraftQuery.value !== appliedSearchQuery.value)

onMounted(() => {
  const storedTheme = window.localStorage.getItem('kompass-theme')
  const preferredDark = window.matchMedia('(prefers-color-scheme: dark)').matches
  applyTheme(storedTheme === 'dark' || storedTheme === 'light' ? storedTheme : preferredDark ? 'dark' : 'light')
  initializeScope()
})

watch(
  () => [selectedContext.value, selectedNamespace.value],
  () => {
    syncScopeQueryParams()
  },
  { immediate: true },
)

function applyTheme(nextTheme) {
  theme.value = nextTheme
  document.documentElement.setAttribute('data-theme', nextTheme)
  window.localStorage.setItem('kompass-theme', nextTheme)
}

function toggleTheme() {
  applyTheme(theme.value === 'dark' ? 'light' : 'dark')
}

function refreshTree() {
  refreshKey.value += 1
}

function updateSearchQuery(value) {
  searchDraftQuery.value = String(value || '')
}

function applySearchQuery() {
  appliedSearchQuery.value = String(searchDraftQuery.value || '')
}

async function updateContext(next) {
  const value = String(next || '').trim()
  if (!value || value === selectedContext.value) {
    return
  }
  selectedContext.value = value
  await syncScopeForContext(value, { resetNamespace: true, preferCurrent: false })
  refreshTree()
}

async function initializeScope() {
  try {
    const payload = await fetchScopePayload()
    const values = Array.isArray(payload?.contexts) ? payload.contexts : []
    contexts.value = values.map((item) => String(item || '').trim()).filter(Boolean)
    const current = String(payload?.currentContext || '').trim()
    if (!selectedContext.value && current) {
      selectedContext.value = current
    }
    await syncScopeForContext(selectedContext.value || current, { resetNamespace: !selectedNamespace.value, scopePayload: payload })
  } catch {
    // Scope endpoint may be unavailable in static mode; keep bootstrap scope only.
  }
}

async function fetchScopePayload(context = '') {
  try {
    const params = new URLSearchParams()
    const contextName = String(context || '').trim()
    if (contextName) {
      params.set('context', contextName)
    }
    const suffix = params.toString()
    const response = await fetch(suffix ? `/api/scope?${suffix}` : '/api/scope', {
      headers: {
        Accept: 'application/json',
      },
    })
    if (!response.ok) {
      throw new Error(`request failed: ${response.status}`)
    }
    return await response.json()
  } catch (err) {
    throw err instanceof Error ? err : new Error('failed to load scope')
  }
}

async function syncScopeForContext(context, options = {}) {
  const contextName = String(context || '').trim()
  if (!contextName) {
    namespaces.value = []
    selectedNamespace.value = ''
    return
  }

  let entry = null
  if (options.scopePayload && String(options.scopePayload?.currentContext || '').trim() === contextName) {
    entry = normalizeScopeEntry(options.scopePayload)
  }
  if (!entry && scopeCache.has(contextName)) {
    entry = scopeCache.get(contextName)
  }
  if (!entry) {
    entry = normalizeScopeEntry(await fetchScopePayload(contextName))
  }

  scopeCache.set(contextName, entry)
  namespaces.value = entry.namespaces

  const currentSelection = String(selectedNamespace.value || '').trim()
  if (!options.resetNamespace && currentSelection && entry.namespaces.includes(currentSelection)) {
    return
  }

  if (options.preferCurrent !== false && entry.currentNamespace) {
    selectedNamespace.value = entry.currentNamespace
    return
  }
  selectedNamespace.value = entry.namespaces[0] || ''
}

function normalizeScopeEntry(payload) {
  return {
    currentNamespace: String(payload?.currentNamespace || '').trim(),
    namespaces: Array.isArray(payload?.namespaces)
      ? payload.namespaces.map((item) => String(item || '').trim()).filter(Boolean)
      : [],
  }
}

function syncScopeQueryParams() {
  if (typeof window === 'undefined') {
    return
  }

  const url = new URL(window.location.href)
  const currentContext = String(selectedContext.value || '').trim()
  const currentNamespace = String(selectedNamespace.value || '').trim()

  if (currentContext) {
    url.searchParams.set('context', currentContext)
  } else {
    url.searchParams.delete('context')
  }

  if (currentNamespace) {
    url.searchParams.set('namespace', currentNamespace)
  } else {
    url.searchParams.delete('namespace')
  }

  const nextURL = `${url.pathname}${url.search}${url.hash}`
  const currentURL = `${window.location.pathname}${window.location.search}${window.location.hash}`
  if (nextURL !== currentURL) {
    window.history.replaceState(null, '', nextURL)
  }
}

function resolveInitialScopeParam(name) {
  if (typeof window === 'undefined') {
    return ''
  }
  return String(new URLSearchParams(window.location.search).get(name) || '').trim()
}

function resolveInitialNamespace() {
  const fromURL = resolveInitialScopeParam('namespace')
  if (fromURL) {
    return fromURL
  }
  return String(props.bootstrapConfig?.namespace || props.bootstrapData?.request?.namespace || '').trim()
}

function resolveInitialContext() {
  const fromURL = resolveInitialScopeParam('context')
  if (fromURL) {
    return fromURL
  }
  return String(props.bootstrapConfig?.context || props.bootstrapData?.request?.context || '').trim()
}

function openResourceView(payload) {
  if (!payload?.node) {
    return
  }
  activeResourceView.value = {
    node: payload.node,
    view: payload.view || 'describe',
  }
}

function closeResourceView() {
  activeResourceView.value = null
}

function formatKompassTitle(raw) {
  const context = String(raw || '').trim() || 'Context'
  if (context.startsWith('🧭 Kompass ')) {
    return context
  }
  return `🧭 Kompass ${context}`
}
</script>

<template>
  <main class="app">
    <section v-show="!activeResourceView" class="app__tree-screen">
      <Menu
        :context-name="viewContextName"
        :contexts="contextOptions"
        :theme-icon="themeIcon"
        :theme-label="themeLabel"
        :on-toggle-theme="toggleTheme"
        :refresh-disabled="isStaticMode"
        :loading="loading"
        :namespaces="namespaces"
        :namespace="selectedNamespace"
        :query="searchDraftQuery"
        :filtering="filtering"
        :disabled="false"
        @refresh="refreshTree"
        @update:namespace="selectedNamespace = $event"
        @update:context="updateContext"
        @update:query="updateSearchQuery"
        @apply-query="applySearchQuery"
      />

      <Trees
        :context="viewContextName"
        :namespace="selectedNamespace"
        :query="appliedSearchQuery"
        :refresh-key="refreshKey"
        :bootstrap-config="bootstrapConfig"
        :bootstrap-data="bootstrapData"
        @update:context-title="contextTitle = $event"
        @update:loading="loading = $event"
        @open-view="openResourceView"
      />
    </section>

    <Views
      v-if="activeResourceView"
      :node="activeResourceView.node"
      :initial-view="activeResourceView.view"
      :api-base="appApiBase"
      :chrome-title="brandedContextTitle"
      :context-name="viewContextName"
      :contexts="contextOptions"
      :namespaces="namespaces"
      :namespace="selectedNamespace"
      :loading="loading"
      :refresh-disabled="isStaticMode"
      :theme-icon="themeIcon"
      :theme-label="themeLabel"
      @close="closeResourceView"
      @refresh="refreshTree"
      @update:namespace="selectedNamespace = $event"
      @update:context="updateContext"
      @toggle-theme="toggleTheme"
    />
  </main>
</template>

<style scoped>
.app {
  height: 100dvh;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1.5rem;
  overflow: hidden;
}

.app__tree-screen {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  overflow: hidden;
}
</style>
