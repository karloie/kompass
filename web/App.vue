<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Menu from './components/Menu.vue'
import Trees from './components/Trees.vue'
import View from './components/View.vue'

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
const contextTitle = ref('Context')
const contexts = ref([])
const selectedContext = ref(resolveInitialContext())
const namespaces = ref([])
const selectedNamespace = ref(resolveInitialNamespace())
const searchQuery = ref('')
const debouncedSearchQuery = ref('')
const loading = ref(false)
const refreshKey = ref(0)
const activeResourceView = ref(null)
let queryDebounceTimer = null

const themeIcon = computed(() => (theme.value === 'dark' ? '☀️' : '🌙'))
const themeLabel = computed(() => (theme.value === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'))
const mode = computed(() => String(props.bootstrapConfig?.mode || 'dynamic').trim().toLowerCase())
const isStaticMode = computed(() => mode.value === 'static')
const appApiBase = computed(() => '/api/app')
const brandedContextTitle = computed(() => formatKompassTitle(viewContextName.value))
const viewContextName = computed(() => String(selectedContext.value || contextTitle.value || '').trim() || 'mock-01')
const contextOptions = computed(() => {
  const values = new Set(contexts.value.map((item) => String(item || '').trim()).filter(Boolean))
  const current = String(viewContextName.value || '').trim()
  if (current) {
    values.add(current)
  }
  return [...values]
})
const filtering = computed(() => searchQuery.value !== debouncedSearchQuery.value)

onMounted(() => {
  const storedTheme = window.localStorage.getItem('kompass-theme')
  const preferredDark = window.matchMedia('(prefers-color-scheme: dark)').matches
  applyTheme(storedTheme === 'dark' || storedTheme === 'light' ? storedTheme : preferredDark ? 'dark' : 'light')
  fetchMetadataContexts()
})

onBeforeUnmount(() => {
  if (queryDebounceTimer) {
    clearTimeout(queryDebounceTimer)
    queryDebounceTimer = null
  }
})

watch(searchQuery, (value) => {
  if (queryDebounceTimer) {
    clearTimeout(queryDebounceTimer)
  }
  if (!value) {
    debouncedSearchQuery.value = ''
    return
  }
  queryDebounceTimer = setTimeout(() => {
    debouncedSearchQuery.value = value
    queryDebounceTimer = null
  }, 140)
}, { immediate: true })

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
  searchQuery.value = String(value || '')
}

function applySuggestedNamespace(namespace) {
  if (!selectedNamespace.value && namespace) {
    selectedNamespace.value = namespace
  }
}

function updateContext(next) {
  const value = String(next || '').trim()
  if (!value || value === selectedContext.value) {
    return
  }
  selectedContext.value = value
  refreshTree()
}

async function fetchMetadataContexts() {
  try {
    const response = await fetch('/api/metadata', {
      headers: {
        Accept: 'application/json',
      },
    })
    if (!response.ok) {
      return
    }
    const payload = await response.json()
    const values = Array.isArray(payload?.contexts) ? payload.contexts : []
    contexts.value = values.map((item) => String(item || '').trim()).filter(Boolean)
    const current = String(payload?.currentContext || '').trim()
    if (!selectedContext.value && current) {
      selectedContext.value = current
    }
  } catch {
    // Metadata endpoint may be unavailable in static mode; keep fallback context only.
  }
}

function resolveInitialNamespace() {
  return String(props.bootstrapConfig?.namespace || props.bootstrapData?.request?.namespace || '').trim()
}

function resolveInitialContext() {
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
  const context = String(raw || '').trim() || 'mock-01'
  if (context.startsWith('🧭 Kompass - ')) {
    return context
  }
  return `🧭 Kompass - ${context}`
}
</script>

<template>
  <main class="app">
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
      :query="searchQuery"
      :filtering="filtering"
      :disabled="false"
      @refresh="refreshTree"
      @update:namespace="selectedNamespace = $event"
      @update:context="updateContext"
      @update:query="updateSearchQuery"
    />

    <Trees
      :context="viewContextName"
      :namespace="selectedNamespace"
      :query="debouncedSearchQuery"
      :refresh-key="refreshKey"
      :bootstrap-config="bootstrapConfig"
      :bootstrap-data="bootstrapData"
      @update:context-title="contextTitle = $event"
      @update:namespaces="namespaces = $event"
      @suggest-namespace="applySuggestedNamespace"
      @update:loading="loading = $event"
      @open-view="openResourceView"
    />

    <View
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
</style>
