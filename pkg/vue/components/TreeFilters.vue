<script setup>
const props = defineProps({
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

const emit = defineEmits(['update:namespace', 'update:query'])

function onNamespaceChange(event) {
  emit('update:namespace', event.target.value)
}

function onQueryInput(event) {
  emit('update:query', event.target.value)
}

function clearQuery() {
  emit('update:query', '')
}
</script>

<template>
  <section class="tree-filters">
    <label class="tree-filters__field">
      <span class="tree-filters__label">Namespace</span>
      <select class="tree-filters__select" :value="namespace" :disabled="disabled" @change="onNamespaceChange">
        <option v-for="item in namespaces" :key="item" :value="item">{{ item }}</option>
      </select>
    </label>

    <label class="tree-filters__field tree-filters__field--grow">
      <span class="tree-filters__label">Filter</span>
      <input
        class="tree-filters__input"
        type="text"
        placeholder="Type to filter tree"
        :value="query"
        :disabled="disabled"
        @input="onQueryInput"
      />
    </label>

    <button class="tree-filters__clear" type="button" :disabled="disabled || !query" @click="clearQuery">
      Clear
    </button>
  </section>
</template>

<style scoped>
.tree-filters {
  margin-top: 0.75rem;
  display: flex;
  align-items: flex-end;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.tree-filters__field {
  display: grid;
  gap: 0.25rem;
}

.tree-filters__field--grow {
  flex: 1;
  min-width: 220px;
}

.tree-filters__label {
  font-size: 0.75rem;
  color: var(--text-muted);
  font-weight: 600;
}

.tree-filters__select,
.tree-filters__input {
  border: 1px solid var(--button-border);
  border-radius: 6px;
  padding: 0.4rem 0.5rem;
  font-size: 0.9rem;
  background: var(--panel-bg);
  color: var(--text-main);
}

.tree-filters__clear {
  border: 1px solid var(--button-border);
  background: var(--button-bg);
  color: var(--button-text);
  padding: 0.4rem 0.65rem;
  border-radius: 6px;
  cursor: pointer;
}

.tree-filters__clear:disabled,
.tree-filters__select:disabled,
.tree-filters__input:disabled {
  opacity: 0.6;
  cursor: default;
}
</style>
