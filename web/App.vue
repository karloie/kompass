<script setup>
import { computed, onMounted, ref } from 'vue'
import Menu from './components/Menu.vue'
import Tree from './components/Tree.vue'
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
const namespaces = ref([])
const selectedNamespace = ref('')
const searchQuery = ref('')
const loading = ref(false)
const refreshKey = ref(0)
const activeResourceView = ref(null)

const themeIcon = computed(() => (theme.value === 'dark' ? '☀️' : '🌙'))
const themeLabel = computed(() => (theme.value === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'))
const mode = computed(() => String(props.bootstrapConfig?.mode || 'dynamic').trim().toLowerCase())
const isStaticMode = computed(() => mode.value === 'static')
const appApiBase = computed(() => '/api/app')
const brandedContextTitle = computed(() => formatKompassTitle(contextTitle.value))
const viewContextName = computed(() => String(contextTitle.value || '').trim() || 'mock-cluster')

onMounted(() => {
  const storedTheme = window.localStorage.getItem('kompass-theme')
  const preferredDark = window.matchMedia('(prefers-color-scheme: dark)').matches
  applyTheme(storedTheme === 'dark' || storedTheme === 'light' ? storedTheme : preferredDark ? 'dark' : 'light')
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

function applySuggestedNamespace(namespace) {
  if (!selectedNamespace.value && namespace) {
    selectedNamespace.value = namespace
  }
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
  const context = String(raw || '').trim() || 'mock-cluster'
  if (context.startsWith('🧭 Kompass - ')) {
    return context
  }
  return `🧭 Kompass - ${context}`
}
</script>

<template>
  <main class="app">
    <Menu
      :title="brandedContextTitle"
      :theme-icon="themeIcon"
      :theme-label="themeLabel"
      :on-toggle-theme="toggleTheme"
      :refresh-disabled="isStaticMode"
      :loading="loading"
      :namespaces="namespaces"
      :namespace="selectedNamespace"
      :query="searchQuery"
      :disabled="false"
      @refresh="refreshTree"
      @update:namespace="selectedNamespace = $event"
      @update:query="searchQuery = $event"
    />

    <Tree
      :namespace="selectedNamespace"
      :query="searchQuery"
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
      @close="closeResourceView"
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
