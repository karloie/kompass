<script setup>
const props = defineProps({
  title: {
    type: String,
    default: 'Context',
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
  disabled: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['refresh', 'update:namespace', 'update:query'])

function onNamespaceChange(event) {
  emit('update:namespace', event.target.value)
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
    <div class="menu__top">
      <h2 class="menu__title">{{ title }}</h2>

      <div class="menu__actions">
        <button class="menu__refresh" type="button" :disabled="loading || disabled || refreshDisabled" @click="emit('refresh')">
          {{ loading ? 'Loading...' : 'Refresh' }}
        </button>
        <button
          class="menu__theme"
          type="button"
          :aria-label="themeLabel"
          :title="themeLabel"
          :disabled="disabled"
          @click="toggleTheme"
        >
          {{ themeIcon }}
        </button>
      </div>
    </div>

    <div class="menu__filters">
      <label class="menu__field">
        <span class="menu__label">Namespace</span>
        <select class="menu__select" :value="namespace" :disabled="disabled" @change="onNamespaceChange">
          <option v-for="item in namespaces" :key="item" :value="item">{{ item }}</option>
        </select>
      </label>

      <label class="menu__field menu__field--grow">
        <span class="menu__label">Filter</span>
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

.menu__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.menu__title {
  margin: 0;
  font-size: 1.1rem;
}

.menu__actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
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
  min-width: 220px;
}

.menu__label {
  font-size: 0.75rem;
  color: var(--text-muted);
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

.menu__refresh,
.menu__theme,
.menu__clear {
  border: 1px solid var(--button-border);
  background: var(--button-bg);
  color: var(--button-text);
  padding: 0.4rem 0.65rem;
  border-radius: 6px;
  cursor: pointer;
}

.menu__theme {
  min-width: 2.2rem;
  font-size: 1rem;
  line-height: 1;
  padding: 0.35rem;
}

.menu__refresh:disabled,
.menu__theme:disabled,
.menu__clear:disabled,
.menu__select:disabled,
.menu__input:disabled {
  opacity: 0.6;
  cursor: default;
}
</style>
