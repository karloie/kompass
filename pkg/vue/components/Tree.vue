<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import TreeNode from './TreeNode.vue'
import TreeFilters from './TreeFilters.vue'

const props = defineProps({
  themeIcon: {
    type: String,
    default: '🌙',
  },
  themeLabel: {
    type: String,
    default: 'Toggle theme',
  },
})

const emit = defineEmits(['toggle-theme'])

const loading = ref(false)
const error = ref('')
const payload = ref(null)
const selectedNamespace = ref('')
const searchQuery = ref('')

let currentController = null
let fetchSeq = 0
let debounceTimer = null

const roots = computed(() => payload.value?.response?.trees || [])
const contextTitle = computed(() => String(payload.value?.request?.context || 'Context').trim() || 'Context')

const namespaces = computed(() => {
  const values = new Set()
  collectNamespaces(roots.value, values)
  return [...values].sort((a, b) => a.localeCompare(b))
})

const filteredRoots = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  const namespace = selectedNamespace.value

  return roots.value
    .map((node) => filterTree(node, { query, namespace }))
    .filter(Boolean)
})

async function fetchTree() {
	if (debounceTimer) {
		clearTimeout(debounceTimer)
		debounceTimer = null
	}

	if (currentController) {
		currentController.abort()
	}

	const requestId = ++fetchSeq
	const controller = new AbortController()
	currentController = controller

  loading.value = true
  error.value = ''

  try {
    const response = await fetch(treeURL(), {
      headers: {
        Accept: 'application/json',
      },
      signal: controller.signal,
    })

    if (!response.ok) {
      throw new Error(`request failed: ${response.status}`)
    }

    const nextPayload = await response.json()
    payload.value = nextPayload

    if (!selectedNamespace.value) {
      const responseNamespace = String(nextPayload?.request?.namespace || '').trim()
      selectedNamespace.value = responseNamespace || firstNamespace(nextPayload?.response?.trees || [])
    }
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      return
    }
    error.value = err instanceof Error ? err.message : 'failed to load tree'
  } finally {
    if (requestId === fetchSeq) {
      loading.value = false
      currentController = null
    }
  }
}

onMounted(() => {
  fetchTree()
})

onBeforeUnmount(() => {
  if (debounceTimer) {
    clearTimeout(debounceTimer)
  }
  if (currentController) {
    currentController.abort()
  }
})

watch(selectedNamespace, () => {
  if (debounceTimer) {
    clearTimeout(debounceTimer)
  }
  debounceTimer = setTimeout(() => {
    fetchTree()
  }, 250)
})

function treeURL() {
  const params = new URLSearchParams()
  const namespace = selectedNamespace.value.trim()

  if (namespace) {
    params.set('namespace', namespace)
  }

  const query = params.toString()
  return query ? `/tree?${query}` : '/tree'
}

function collectNamespaces(nodes, out) {
  for (const node of nodes || []) {
    const ns = nodeNamespace(node)
    if (ns) {
      out.add(ns)
    }
    collectNamespaces(node?.children || [], out)
  }
}

function nodeNamespace(node) {
  const ns = node?.metadata?.namespace
  if (typeof ns === 'string' && ns.trim() !== '') {
    return ns.trim()
  }

  const key = node?.key || ''
  const parts = key.split('/')
  if (parts.length >= 3 && parts[1]) {
    return parts[1]
  }
  return ''
}

function nodeText(node) {
  const type = node?.type || ''
  const key = node?.key || ''
  const meta = node?.metadata || {}
  const name = meta.name || ''
  const ns = nodeNamespace(node)
  return `${type} ${name} ${key} ${ns}`.toLowerCase()
}

function filterTree(node, filters) {
  if (!node) {
    return null
  }

  const filteredChildren = (node.children || [])
    .map((child) => filterTree(child, filters))
    .filter(Boolean)

  const namespaceMatches = filters.namespace !== '' && nodeNamespace(node) === filters.namespace
  const queryMatches = !filters.query || nodeText(node).includes(filters.query)
  const matchesSelf = namespaceMatches && queryMatches

  if (matchesSelf || filteredChildren.length > 0) {
    return {
      ...node,
      children: filteredChildren,
    }
  }

  return null
}

function firstNamespace(nodes) {
  const values = new Set()
  collectNamespaces(nodes, values)
  for (const ns of values) {
    if (ns) {
      return ns
    }
  }
  return ''
}

</script>

<template>
  <section class="tree">
    <header class="tree__header">
      <h2 class="tree__title">{{ contextTitle }}</h2>
      <div class="tree__actions">
        <button class="tree__refresh" type="button" :disabled="loading" @click="fetchTree">
          {{ loading ? 'Loading...' : 'Refresh' }}
        </button>
        <button class="tree__theme" type="button" :aria-label="props.themeLabel" :title="props.themeLabel" @click="emit('toggle-theme')">
          {{ props.themeIcon }}
        </button>
      </div>
    </header>

    <TreeFilters
      :namespaces="namespaces"
      :namespace="selectedNamespace"
      :query="searchQuery"
      :disabled="false"
      @update:namespace="selectedNamespace = $event"
      @update:query="searchQuery = $event"
    />

    <p v-if="error" class="tree__error">Failed: {{ error }}</p>
    <p v-else-if="loading && !roots.length" class="tree__hint">Loading tree data...</p>
    <p v-else-if="!roots.length" class="tree__hint">No tree data returned.</p>
    <p v-else-if="!filteredRoots.length" class="tree__hint">No matches for current filters.</p>
    <p v-else-if="loading" class="tree__hint">Refreshing...</p>

    <ul v-else class="tree__list">
      <TreeNode
        v-for="(node, index) in filteredRoots"
        :key="node?.key || `root-${index}`"
        :node="node"
      />
    </ul>
  </section>
</template>

<style scoped>
.tree {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 1rem;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--panel-bg);
  color: var(--text-main);
}

.tree__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.tree__actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.tree__title {
  margin: 0;
  font-size: 1.1rem;
}

.tree__hint {
  margin: 0.75rem 0 0;
  color: var(--text-muted);
  font-size: 0.9rem;
}

.tree__refresh,
.tree__theme {
  border: 1px solid var(--button-border);
  background: var(--button-bg);
  color: var(--button-text);
  padding: 0.35rem 0.6rem;
  border-radius: 6px;
  cursor: pointer;
}

.tree__theme {
  min-width: 2.2rem;
  font-size: 1rem;
  line-height: 1;
  padding: 0.35rem;
}

.tree__refresh:disabled {
  opacity: 0.6;
  cursor: default;
}

.tree__error {
  margin: 0.75rem 0 0;
  color: var(--danger-text);
  font-size: 0.9rem;
}

.tree__list {
  margin: 0.75rem 0 0;
  padding: 0;
  min-height: 0;
  overflow: auto;
}
</style>
