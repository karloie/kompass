<script setup>
import MenuHeader from './MenuHeader.vue'

const props = defineProps({
  contextName: {
    type: String,
    default: 'mock-01',
  },
  contexts: {
    type: Array,
    default: () => [],
  },
  themeIcon: {
    type: String,
    default: '🌙',
  },
  themeLabel: {
    type: String,
    default: 'Toggle theme',
  },
  onToggleTheme: {
    type: Function,
    default: null,
  },
  loading: {
    type: Boolean,
    default: false,
  },
  refreshDisabled: {
    type: Boolean,
    default: false,
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
  disabled: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['refresh', 'update:namespace', 'update:context', 'update:selectors'])

function onNamespaceChange(value) {
  emit('update:namespace', String(value || ''))
}

function onContextChange(value) {
  emit('update:context', String(value || ''))
}

function onSelectorsChange(value) {
  emit('update:selectors', String(value || ''))
}

function toggleTheme() {
  if (typeof props.onToggleTheme === 'function') {
    props.onToggleTheme()
  }
}
</script>

<template>
  <section class="menu">
    <MenuHeader
      :context-name="contextName"
      :contexts="contexts"
      :namespace="namespace"
      :namespaces="namespaces"
      :selectors="selectors"
      :loading="loading"
      :refresh-disabled="refreshDisabled"
      :theme-icon="themeIcon"
      :theme-label="themeLabel"
      :disabled="disabled"
      @refresh="emit('refresh')"
      @toggle-theme="toggleTheme"
      @update:namespace="onNamespaceChange"
      @update:context="onContextChange"
      @update:selectors="onSelectorsChange"
    />
  </section>
</template>

<style scoped>
.menu {
  display: flex;
  flex-direction: column;
  gap: 0;
}

</style>
