<script setup>
import { computed } from 'vue'

const props = defineProps({
  contextName: {
    type: String,
    default: 'mock-01',
  },
  contexts: {
    type: Array,
    default: () => [],
  },
  namespace: {
    type: String,
    default: '',
  },
  namespaces: {
    type: Array,
    default: () => [],
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
  disabled: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['refresh', 'toggle-theme', 'update:namespace', 'update:context', 'update:selectors'])

const contextOptions = computed(() => {
  const out = new Set()
  for (const item of props.contexts || []) {
    const value = String(item || '').trim()
    if (value) out.add(value)
  }
  const current = String(props.contextName || '').trim()
  if (current) out.add(current)
  return [...out]
})

function onContextChange(event) {
  emit('update:context', event.target.value)
}

function onNamespaceChange(event) {
  emit('update:namespace', event.target.value)
}

function onSelectorsInput(event) {
  emit('update:selectors', event.target.value)
}
</script>

<template>
  <header class="shared-header">
    <div class="shared-header__left">
      <span class="shared-header__brand">🧭 Kompass</span>
      <select
        class="shared-header__select"
        :value="contextName"
        :disabled="disabled || contextOptions.length <= 1"
        @change="onContextChange"
      >
        <option v-for="item in contextOptions" :key="item" :value="item">{{ item }}</option>
      </select>
      <span class="shared-header__sep">:</span>
      <select class="shared-header__select" :value="namespace" :disabled="disabled" @change="onNamespaceChange">
        <option v-for="item in namespaces" :key="item" :value="item">{{ item }}</option>
      </select>
      <input
        class="shared-header__selectors"
        type="text"
        placeholder="selectors: */petshop/petshop* */kafka*/*"
        :value="selectors"
        :disabled="disabled"
        @input="onSelectorsInput"
      />
    </div>

    <div class="shared-header__actions">
      <button
        class="shared-header__btn shared-header__btn--icon"
        type="button"
        :aria-label="loading ? 'Loading' : 'Refresh'"
        :title="loading ? 'Loading' : 'Refresh'"
        :disabled="loading || disabled || refreshDisabled"
        @click="emit('refresh')"
      >
        {{ loading ? '⏳' : '🔄' }}
      </button>
      <button
        class="shared-header__btn shared-header__btn--theme"
        type="button"
        :aria-label="themeLabel"
        :title="themeLabel"
        :disabled="disabled"
        @click="emit('toggle-theme')"
      >
        {{ themeIcon }}
      </button>
    </div>
  </header>
</template>

<style scoped>
.shared-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.shared-header__left {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
  flex-wrap: wrap;
}

.shared-header__brand {
  font-size: 1.1rem;
  font-weight: 800;
  letter-spacing: 0.02em;
  color: var(--text-main);
}

.shared-header__sep {
  color: var(--text-muted);
  font-weight: 700;
}

.shared-header__select {
  border: 1px solid var(--button-border);
  border-radius: 6px;
  padding: 0.36rem 0.45rem;
  font-size: 0.95rem;
  background: var(--panel-bg);
  color: var(--text-main);
  min-width: 9ch;
  max-width: 34ch;
}

.shared-header__selectors {
  border: 1px solid var(--button-border);
  border-radius: 6px;
  padding: 0.36rem 0.45rem;
  font-size: 0.95rem;
  background: var(--panel-bg);
  color: var(--text-main);
  min-width: 33ch;
  max-width: 80ch;
}

.shared-header__actions {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}

.shared-header__btn {
  border: 1px solid var(--button-border);
  background: var(--button-bg);
  color: var(--button-text);
  padding: 0.4rem 0.5rem;
  border-radius: 6px;
  font-size: 0.9rem;
  line-height: 1;
  cursor: pointer;
}

.shared-header__btn--theme {
  min-width: 2.2rem;
}

.shared-header__btn--icon {
  min-width: 2.2rem;
}

.shared-header__btn:disabled,
.shared-header__select:disabled,
.shared-header__selectors:disabled {
  opacity: 0.6;
  cursor: default;
}
</style>
