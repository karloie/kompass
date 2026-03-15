<script setup>
import { computed, onMounted, ref } from 'vue'
import Tree from './components/Tree.vue'

const theme = ref('light')

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
</script>

<template>
  <main class="app">
    <Tree :theme-icon="themeIcon" :theme-label="themeLabel" @toggle-theme="toggleTheme" />
  </main>
</template>

<style scoped>
.app {
  height: 100dvh;
  display: block;
  padding: 1.5rem;
  overflow: hidden;
}
</style>
