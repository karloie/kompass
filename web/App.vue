<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import MenuHeader from './components/MenuHeader.vue'
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
const contextNamespacePrefs = ref(loadContextNamespacePrefs())
const selectors = ref(resolveInitialSelectors())
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
const scopeQueryParams = computed(() => ({
  context: String(selectedContext.value || '').trim(),
  namespace: String(selectedNamespace.value || '').trim(),
  selectors: String(selectors.value || '').trim(),
  resource: String(activeResourceView.value?.node?.key || '').trim(),
  view: String(activeResourceView.value?.view || '').trim(),
}))

onMounted(() => {
  const storedTheme = window.localStorage.getItem('kompass-theme')
  const preferredDark = window.matchMedia('(prefers-color-scheme: dark)').matches
  applyTheme(storedTheme === 'dark' || storedTheme === 'light' ? storedTheme : preferredDark ? 'dark' : 'light')
  initializeScope()
})

watch(scopeQueryParams, syncScopeQueryParams, { immediate: true })

watch(selectedContext, (value) => {
  const contextName = String(value || '').trim()
  if (!contextName) {
    return
  }
  setCookie('kompass-last-context', contextName)
})

watch([selectedContext, selectedNamespace], ([contextValue, namespaceValue]) => {
  const contextName = String(contextValue || '').trim()
  const namespaceName = String(namespaceValue || '').trim()
  if (!contextName || !namespaceName) {
    return
  }

  const next = {
    ...(contextNamespacePrefs.value || {}),
    [contextName]: namespaceName,
  }
  contextNamespacePrefs.value = next
  persistContextNamespacePrefs(next)
})

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
  await syncScopeForContext(value, {
    resetNamespace: true,
    preferCurrent: false,
    preferredNamespace: String(contextNamespacePrefs.value?.[value] || '').trim(),
  })
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
    const activeContext = selectedContext.value || current
    await syncScopeForContext(activeContext, {
      resetNamespace: !selectedNamespace.value,
      scopePayload: payload,
      preferredNamespace: String(contextNamespacePrefs.value?.[activeContext] || '').trim(),
    })
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
      cache: 'no-store',
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

  const preferredNamespace = String(options.preferredNamespace || '').trim()
  if (preferredNamespace && entry.namespaces.includes(preferredNamespace)) {
    selectedNamespace.value = preferredNamespace
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
  const currentSelectors = String(selectors.value || '').trim()
  const currentResource = String(activeResourceView.value?.node?.key || '').trim()
  const currentView = String(activeResourceView.value?.view || '').trim()

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

  if (currentSelectors) {
    url.searchParams.set('selectors', currentSelectors)
  } else {
    url.searchParams.delete('selectors')
  }

  if (currentResource) {
    url.searchParams.set('resource', currentResource)
  } else {
    url.searchParams.delete('resource')
  }

  if (currentView) {
    url.searchParams.set('view', currentView)
  } else {
    url.searchParams.delete('view')
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
  const context = resolveInitialContext()
  const fromCookieMap = String(loadContextNamespacePrefs()?.[context] || '').trim()
  if (fromCookieMap) {
    return fromCookieMap
  }
  return String(props.bootstrapConfig?.namespace || props.bootstrapData?.request?.namespace || '').trim()
}

function resolveInitialContext() {
  const fromURL = resolveInitialScopeParam('context')
  if (fromURL) {
    return fromURL
  }
  const fromCookie = String(getCookie('kompass-last-context') || '').trim()
  if (fromCookie) {
    return fromCookie
  }
  return String(props.bootstrapConfig?.context || props.bootstrapData?.request?.context || '').trim()
}

function resolveInitialSelectors() {
  const fromURL = resolveInitialScopeParam('selectors')
  if (fromURL) {
    return fromURL
  }
  return String(props.bootstrapConfig?.selectors || props.bootstrapData?.request?.selectors || '').trim()
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

function updateResourceViewTab(nextView) {
  if (!activeResourceView.value) {
    return
  }
  const view = String(nextView || '').trim()
  if (!view) {
    return
  }
  activeResourceView.value = {
    ...activeResourceView.value,
    view,
  }
}

function loadContextNamespacePrefs() {
  const raw = String(getCookie('kompass-context-namespace-map') || '').trim()
  if (!raw) {
    return {}
  }
  try {
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return {}
    }
    const out = {}
    for (const [key, value] of Object.entries(parsed)) {
      const contextName = String(key || '').trim()
      const namespaceName = String(value || '').trim()
      if (!contextName || !namespaceName) {
        continue
      }
      out[contextName] = namespaceName
    }
    return out
  } catch {
    return {}
  }
}

function persistContextNamespacePrefs(map) {
  const safe = {}
  for (const [key, value] of Object.entries(map || {})) {
    const contextName = String(key || '').trim()
    const namespaceName = String(value || '').trim()
    if (!contextName || !namespaceName) {
      continue
    }
    safe[contextName] = namespaceName
  }
  setCookie('kompass-context-namespace-map', JSON.stringify(safe))
}

function getCookie(name) {
  if (typeof document === 'undefined') {
    return ''
  }
  const needle = `${encodeURIComponent(String(name || '').trim())}=`
  const parts = String(document.cookie || '').split(';')
  for (const part of parts) {
    const item = part.trim()
    if (!item || !item.startsWith(needle)) {
      continue
    }
    return decodeURIComponent(item.slice(needle.length))
  }
  return ''
}

function setCookie(name, value) {
  if (typeof document === 'undefined') {
    return
  }
  const encodedName = encodeURIComponent(String(name || '').trim())
  const encodedValue = encodeURIComponent(String(value || ''))
  const oneYear = 60 * 60 * 24 * 365
  document.cookie = `${encodedName}=${encodedValue}; Max-Age=${oneYear}; Path=/; SameSite=Lax`
}
</script>

<template>
  <main class="app">
    <section v-show="!activeResourceView" class="app__tree-screen">
      <MenuHeader
        class="app__menu-header"
        :context-name="viewContextName"
        :contexts="contextOptions"
        :theme-icon="themeIcon"
        :theme-label="themeLabel"
        :refresh-disabled="isStaticMode"
        :loading="loading"
        :namespaces="namespaces"
        :namespace="selectedNamespace"
        :selectors="selectors"
        :disabled="loading"
        @refresh="refreshTree"
        @toggle-theme="toggleTheme"
        @update:namespace="selectedNamespace = $event"
        @update:context="updateContext"
        @update:selectors="selectors = $event"
      />

      <Trees
        :context="viewContextName"
        :namespace="selectedNamespace"
        :selectors="selectors"
        :draft-query="searchDraftQuery"
        :filtering="filtering"
        :query="appliedSearchQuery"
        :refresh-key="refreshKey"
        :bootstrap-config="bootstrapConfig"
        :bootstrap-data="bootstrapData"
        @update:context-title="contextTitle = $event"
        @update:loading="loading = $event"
        @update:query="updateSearchQuery"
        @apply-query="applySearchQuery"
        @open-view="openResourceView"
      />
    </section>

    <Views
      v-if="activeResourceView"
      :node="activeResourceView.node"
      :initial-view="activeResourceView.view"
      :api-base="appApiBase"
      :context-name="viewContextName"
      :contexts="contextOptions"
      :namespaces="namespaces"
      :namespace="selectedNamespace"
      :selectors="selectors"
      :loading="loading"
      :refresh-disabled="isStaticMode"
      :theme-icon="themeIcon"
      :theme-label="themeLabel"
      @close="closeResourceView"
      @refresh="refreshTree"
      @update:namespace="selectedNamespace = $event"
      @update:context="updateContext"
      @update:selectors="selectors = $event"
      @update:view="updateResourceViewTab"
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
  padding: 0.75rem;
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

.app__menu-header {
  flex-shrink: 0;
}
</style>
