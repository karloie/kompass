<script setup>
import { computed, onMounted, ref } from 'vue'
import Menu from './components/Menu.vue'
import Tree from './components/Tree.vue'

const theme = ref('light')
const contextTitle = ref('Context')
const namespaces = ref([])
const selectedNamespace = ref('')
const searchQuery = ref('')
const loading = ref(false)
const refreshKey = ref(0)

const themeIcon = computed(() => (theme.value === 'dark' ? '☀️' : '🌙'))
const themeLabel = computed(() => (theme.value === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'))

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
</script>

<template>
  <main class="app">
    <Menu
      :title="contextTitle"
      :theme-icon="themeIcon"
      :theme-label="themeLabel"
      :on-toggle-theme="toggleTheme"
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
      @update:context-title="contextTitle = $event"
      @update:namespaces="namespaces = $event"
      @suggest-namespace="applySuggestedNamespace"
      @update:loading="loading = $event"
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
