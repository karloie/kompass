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
  query: {
    type: String,
    default: '',
  },
  filtering: {
    type: Boolean,
    default: false,
  },
  disabled: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['refresh', 'update:namespace', 'update:context', 'update:query'])

function onNamespaceChange(value) {
  emit('update:namespace', String(value || ''))
}

function onContextChange(value) {
  emit('update:context', String(value || ''))
}

function onQueryInput(event) {
  emit('update:query', event.target.value)
}

function clearQuery() {
  emit('update:query', '')
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
      :loading="loading"
      :refresh-disabled="refreshDisabled"
      :theme-icon="themeIcon"
      :theme-label="themeLabel"
      :disabled="disabled"
      @refresh="emit('refresh')"
      @toggle-theme="toggleTheme"
      @update:namespace="onNamespaceChange"
      @update:context="onContextChange"
    />

    <div class="menu__filters">
      <label class="menu__field menu__field--grow">
        <span class="menu__label-wrap">
          <span class="menu__label">Filter</span>
          <span v-if="filtering" class="menu__filtering">Filtering...</span>
        </span>
        <input
          class="menu__input"
          type="text"
          placeholder="Examples: kafka* pod/?/api !crash"
          :value="query"
          :disabled="disabled"
          @input="onQueryInput"
        />
      </label>

      <button class="menu__clear" type="button" :disabled="disabled || !query" @click="clearQuery">
        Clear
      </button>
    </div>
  </section>
</template>

<style scoped>
.menu {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}

.menu__filters {
  display: flex;
  align-items: flex-end;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.menu__field {
  display: grid;
  gap: 0.25rem;
}

.menu__field--grow {
  flex: 1;
  min-width: 320px;
}

.menu__label {
  font-size: 0.75rem;
  color: var(--text-muted);
  font-weight: 600;
}

.menu__label-wrap {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
}

.menu__filtering {
  font-size: 0.72rem;
  color: var(--accent-color);
  font-weight: 600;
}

.menu__select,
.menu__input {
  border: 1px solid var(--button-border);
  border-radius: 6px;
  padding: 0.4rem 0.5rem;
  font-size: 0.9rem;
  background: var(--panel-bg);
  color: var(--text-main);
}

.menu__clear {
  border: 1px solid var(--button-border);
  background: var(--button-bg);
  color: var(--button-text);
  padding: 0.4rem 0.65rem;
  border-radius: 6px;
  cursor: pointer;
}

.menu__clear:disabled,
.menu__select:disabled,
.menu__input:disabled {
  opacity: 0.6;
  cursor: default;
}
</style>
