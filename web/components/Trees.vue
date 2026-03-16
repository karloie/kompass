<script setup>
import { computed, onBeforeUnmount, onMounted, provide, reactive, ref, watch } from 'vue'
import TreeNode from './TreeNode.vue'

const props = defineProps({
  context: {
    type: String,
    default: '',
  },
  namespace: {
    type: String,
    default: '',
  },
  selectors: {
    type: String,
    default: '',
  },
  draftQuery: {
    type: String,
    default: '',
  },
  filtering: {
    type: Boolean,
    default: false,
  },
  query: {
    type: String,
    default: '',
  },
  refreshKey: {
    type: Number,
    default: 0,
  },
  bootstrapConfig: {
    type: Object,
    default: null,
  },
  bootstrapData: {
    type: Object,
    default: null,
  },
})

const emit = defineEmits(['update:context-title', 'update:namespaces', 'suggest-namespace', 'update:loading', 'update:query', 'apply-query', 'open-view'])

const loading = ref(false)
const error = ref('')
const payload = ref(null)

let currentController = null
let fetchSeq = 0
let debounceTimer = null

const roots = computed(() => payload.value?.trees || [])
const contextTitle = computed(() => String(payload.value?.request?.context || '').trim())
const apiBase = computed(() => String(props.bootstrapConfig?.apiBase || '/api/tree').trim() || '/api/tree')
const dynamicEnabled = computed(() => {
  const mode = String(props.bootstrapConfig?.mode || 'dynamic').trim().toLowerCase()
  if (mode === 'static') {
    return false
  }
  if (mode === 'dynamic') {
    return true
  }
  return apiBase.value !== ''
})
const appViewsEnabled = computed(() => {
  const mode = String(props.bootstrapConfig?.mode || 'dynamic').trim().toLowerCase()
  return mode !== 'static'
})
const matcher = computed(() => buildMatcher(props.query.trim()))
const searchIndex = ref(new Map())

const namespaces = computed(() => {
  const values = new Set()

  const requestNamespace = String(payload.value?.request?.namespace || '').trim()
  if (requestNamespace) {
    values.add(requestNamespace)
  }
  if (props.namespace) {
    values.add(props.namespace)
  }

  collectNamespaces(roots.value, values)

  for (const resource of payload.value?.nodes || []) {
    const ns = resourceNamespace(resource)
    if (ns) {
      values.add(ns)
    }
  }

  return [...values].sort((a, b) => a.localeCompare(b))
})

const filteredRoots = computed(() => {
  const activeMatcher = matcher.value
  const namespace = props.namespace

  return roots.value
    .map((node) => filterTree(node, { matcher: activeMatcher, namespace }))
    .filter(Boolean)
})

// ── Expand/collapse state ──────────────────────────────────────────────────────
// Map<key, boolean>: explicit user overrides (true=open, false=closed)
const expandOverride = reactive(new Map())
const queryFilterActive = computed(() => matcher.value.hasTerms)

function defaultOpenByDepth(depth, _nodeType) {
  if (depth === 0) return false
  return false
}

function isNodeOpen(key, depth, nodeType) {
  if (expandOverride.has(key)) return expandOverride.get(key)
  return defaultOpenByDepth(depth, nodeType)
}

function toggleNode(key, node, depth, nodeType) {
  const open = isNodeOpen(key, depth, nodeType)
  if (open) {
    expandOverride.set(key, false)
  } else {
    expandOverride.set(key, true)
  }
}

provide('treeExpand', { isNodeOpen, toggleNode, queryFilterActive })
provide('treeNamespace', computed(() => String(props.namespace || '').trim()))

async function fetchTree() {
  if (!props.context.trim() || !props.namespace.trim()) {
    error.value = ''
    loading.value = false
    return
  }

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
  if (props.bootstrapData && typeof props.bootstrapData === 'object') {
    payload.value = props.bootstrapData
  }
  if (dynamicEnabled.value) {
    fetchTree()
  }
})

onBeforeUnmount(() => {
  if (debounceTimer) {
    clearTimeout(debounceTimer)
  }
  if (currentController) {
    currentController.abort()
  }
})

watch(() => props.namespace, () => {
  if (!dynamicEnabled.value) {
    return
  }
  if (debounceTimer) {
    clearTimeout(debounceTimer)
  }
  debounceTimer = setTimeout(() => {
    fetchTree()
  }, 250)
})

watch(() => props.context, () => {
  if (!dynamicEnabled.value) {
    return
  }
  fetchTree()
})

watch(() => props.refreshKey, () => {
  if (!dynamicEnabled.value) {
    return
  }
  fetchTree()
})

watch(() => props.selectors, () => {
  if (!dynamicEnabled.value) {
    return
  }
  fetchTree()
})

watch(contextTitle, (value) => {
  emit('update:context-title', value)
}, { immediate: true })

watch(namespaces, (value) => {
  emit('update:namespaces', value)
}, { immediate: true })

watch(loading, (value) => {
  emit('update:loading', value)
}, { immediate: true })

// Reset user overrides and rebuild search index whenever tree data changes.
watch(payload, (nextPayload) => {
  expandOverride.clear()
  searchIndex.value = buildSearchIndex(nextPayload?.trees || [])
  openPodPaths(nextPayload?.trees || [])
})

function openPodPaths(nodes) {
  markPodPathsOpen(nodes, 0)
}

function markPodPathsOpen(nodes, depth) {
  let hasPodInBranch = false

  for (const node of nodes || []) {
    const type = String(node?.type || '').trim().toLowerCase()
    const childHasPod = markPodPathsOpen(workloadPathChildren(node), depth + 1)
    const includesPod = type === 'pod' || childHasPod

    if (includesPod) {
      hasPodInBranch = true
      const key = String(node?.key || '').trim()
      if (type === 'pod') {
        // Pod nodes stay collapsed by default — use the existing override map to pin them closed.
        if (key) {
          expandOverride.set(key, false)
        }
      } else if (depth >= 1 && key && Array.isArray(node?.children) && node.children.length > 0) {
        // Keep rendered roots collapsed; allow one-level-below roots (e.g. ReplicaSets) to auto-open.
        expandOverride.set(key, true)
      }
    }
  }

  return hasPodInBranch
}

function workloadPathChildren(node) {
  const children = Array.isArray(node?.children) ? node.children : []
  if (children.length === 0) {
    return []
  }

  const parentType = String(node?.type || '').trim().toLowerCase()
  const allowed = workloadPathTransitions[parentType]
  if (!allowed) {
    return []
  }

  return children.filter((child) => {
    const childType = String(child?.type || '').trim().toLowerCase()
    return allowed.has(childType)
  })
}

const workloadPathTransitions = {
  deployment: new Set(['replicaset']),
  replicaset: new Set(['pod']),
  cronjob: new Set(['job']),
  job: new Set(['pod']),
  statefulset: new Set(['pod']),
  daemonset: new Set(['pod']),
}

function treeURL() {
  const base = apiBase.value
  if (!base) {
    return '/api/tree'
  }
  const params = new URLSearchParams()
  const namespace = props.namespace.trim()
  const context = props.context.trim()
  const selectors = String(props.selectors || '').trim()
  params.set('context', context)
  params.set('namespace', namespace)
  if (selectors) {
    params.set('selectors', selectors)
  }

  const query = params.toString()
  return query ? `${base}?${query}` : base
}

function onQueryInput(event) {
  emit('update:query', event.target.value)
}

function applyQuery() {
  emit('apply-query')
}

function clearQuery() {
  emit('update:query', '')
  emit('apply-query')
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

function resourceNamespace(resource) {
  const ns = resource?.namespace
  if (typeof ns === 'string' && ns.trim() !== '') {
    return ns.trim()
  }

  const metaNS = resource?.resource?.metadata?.namespace
  if (typeof metaNS === 'string' && metaNS.trim() !== '') {
    return metaNS.trim()
  }

  const key = resource?.key || ''
  const parts = key.split('/')
  if (parts.length >= 3 && parts[1]) {
    return parts[1]
  }

  return ''
}

function nodeText(node) {
  const label = nodeLabel(node)
  const searchText = buildNodeSearchText(node?.type || '', label, node?.metadata || {})
  return searchText.toLowerCase()
}

function nodeSearchText(node) {
  const key = String(node?.key || '')
  if (key && searchIndex.value.has(key)) {
    return searchIndex.value.get(key)
  }
  return nodeText(node)
}

function buildSearchIndex(nodes) {
  const index = new Map()

  function walk(list) {
    for (const node of list || []) {
      const key = String(node?.key || '')
      if (key) {
        index.set(key, nodeText(node))
      }
      walk(node?.children || [])
    }
  }

  walk(nodes)
  return index
}

function nodeLabel(node) {
  const type = node?.type || ''
  const key = node?.key || ''
  const meta = node?.metadata || {}
  const name = meta.name || ''
  const ns = nodeNamespace(node)
  return `${type} ${name} ${key} ${ns}`.trim()
}

function buildNodeSearchText(nodeType, label, meta) {
  const tokens = [nodeType, label]
  appendSearchTokens(tokens, meta)
  return tokens.join(' ')
}

function appendSearchTokens(tokens, value) {
  if (value == null) {
    return
  }

  if (typeof value === 'string') {
    if (shouldIndexToken(value)) {
      tokens.push(value)
    }
    return
  }

  if (typeof value === 'number' || typeof value === 'boolean') {
    tokens.push(String(value))
    return
  }

  if (Array.isArray(value)) {
    for (const item of value) {
      appendSearchTokens(tokens, item)
    }
    return
  }

  if (typeof value === 'object') {
    const keys = Object.keys(value).sort((a, b) => a.localeCompare(b))
    for (const key of keys) {
      if (isNoisyMetadataKey(key)) {
        continue
      }
      if (shouldIndexToken(key)) {
        tokens.push(key)
      }
      appendSearchTokens(tokens, value[key])
    }
    return
  }

  const raw = String(value)
  if (shouldIndexToken(raw)) {
    tokens.push(raw)
  }
}

function isNoisyMetadataKey(key) {
  return noisyMetadataKeys.has(String(key).trim().toLowerCase())
}

function shouldIndexToken(value) {
  const token = String(value).trim()
  if (!token) {
    return false
  }
  if (token.length > 140) {
    return false
  }

  const lower = token.toLowerCase()
  if (lower.includes('sha256:')) {
    return false
  }
  if (hashLikeToken.test(lower)) {
    return false
  }

  return true
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function globToRegExp(glob) {
  let out = '^'
  for (const ch of glob) {
    if (ch === '*') {
      out += '.*'
    } else if (ch === '?') {
      out += '.'
    } else {
      out += escapeRegExp(ch)
    }
  }
  out += '$'
  return new RegExp(out, 'i')
}

function buildMatcher(rawQuery) {
  const terms = rawQuery
    .split(/\s+/)
    .filter(Boolean)
    .map((item) => {
      let token = item
      let negated = false
      if (token.startsWith('!')) {
        negated = true
        token = token.slice(1)
      }
      const wildcard = token.includes('*') || token.includes('?')
      return {
        negated,
        token,
        wildcard,
        re: wildcard && token ? globToRegExp(token) : null,
        lower: token.toLowerCase(),
      }
    })
    .filter((item) => item.token.length > 0)

  const positives = terms.filter((item) => !item.negated)
  const negatives = terms.filter((item) => item.negated)

  return {
    hasTerms: terms.length > 0,
    test(value) {
      const lower = value.toLowerCase()
      for (const term of positives) {
        if (term.wildcard) {
          if (!term.re.test(value)) {
            return false
          }
        } else if (!lower.includes(term.lower)) {
          return false
        }
      }

      for (const term of negatives) {
        if (term.wildcard) {
          if (term.re.test(value)) {
            return false
          }
        } else if (lower.includes(term.lower)) {
          return false
        }
      }

      return positives.length > 0 || negatives.length > 0
    },
  }
}

function filterTree(node, filters) {
  if (!node) {
    return null
  }

  const filteredChildren = (node.children || [])
    .map((child) => filterTree(child, filters))
    .filter(Boolean)

  const namespaceMatches = filters.namespace === '' || nodeNamespace(node) === filters.namespace
  const queryMatches = !filters.matcher.hasTerms || filters.matcher.test(nodeSearchText(node))
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

const noisyMetadataKeys = new Set([
  '__nodetype',
  'annotations',
  'creationtimestamp',
  'managedfields',
  'ownerreferences',
  'resourceversion',
  'uid',
  'lasttransitiontime',
  'containerid',
])

const hashLikeToken = /^[a-f0-9]{24,}$/

</script>

<template>
  <section class="tree">
    <div class="tree__filters">
      <label class="tree__field tree__field--grow">
        <span class="tree__label-wrap">
          <span v-if="filtering" class="tree__filtering">Filtering...</span>
        </span>
        <input
          class="tree__input"
          type="text"
          placeholder="Filter: kafka* pod/?/api !crash"
          :value="draftQuery"
          @input="onQueryInput"
          @keydown.enter.prevent="applyQuery"
        />
      </label>

      <button
        class="tree__apply"
        type="button"
        :aria-label="'Apply filter'"
        :title="'Apply filter (same as Enter)'"
        @click="applyQuery"
      >
        🔎
      </button>

      <button
        class="tree__clear"
        type="button"
        :aria-label="'Clear filter'"
        :title="'Clear filter'"
        :disabled="!draftQuery"
        @click="clearQuery"
      >
        🧹
      </button>
    </div>

    <p v-if="error" class="tree__error">Failed: {{ error }}</p>
    <p v-else-if="!roots.length && !loading" class="tree__hint">No tree data returned.</p>
    <p v-else-if="!filteredRoots.length && !loading" class="tree__hint">No matches for current filters.</p>

    <ul v-if="!error && (roots.length || loading)" class="tree__list">
      <li v-if="loading" class="tree__loading-row">Loading tree rows...</li>
      <TreeNode
        v-for="(node, index) in filteredRoots"
        :key="node?.key || `root-${index}`"
        :node="node"
        :depth="0"
        :view-actions-enabled="appViewsEnabled"
        @open-view="emit('open-view', $event)"
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
  gap: 0.5rem;
  padding: 0.4rem 0.5rem;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--panel-bg);
  color: var(--text-main);
}

.tree__filters {
  display: flex;
  align-items: flex-end;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.tree__field {
  display: grid;
  gap: 0.25rem;
}

.tree__field--grow {
  flex: 1;
  min-width: 320px;
}

.tree__label-wrap {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
}

.tree__filtering {
  font-size: 0.72rem;
  color: var(--accent-color);
  font-weight: 600;
}

.tree__input {
  border: 1px solid var(--button-border);
  border-radius: 6px;
  padding: 0.4rem 0.5rem;
  font-size: 0.9rem;
  background: var(--panel-bg);
  color: var(--text-main);
}

.tree__clear,
.tree__apply {
  border: 1px solid var(--button-border);
  background: var(--button-bg);
  color: var(--button-text);
  padding: 0.4rem 0.65rem;
  border-radius: 6px;
  cursor: pointer;
}

.tree__clear:disabled {
  opacity: 0.6;
  cursor: default;
}

.tree__hint {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.9rem;
}

.tree__error {
  margin: 0;
  color: var(--danger-text);
  font-size: 0.9rem;
}

.tree__list {
  margin: 0;
  padding: 0;
  min-height: 0;
  overflow: auto;
}

.tree__loading-row {
  list-style: none;
  margin: 0 0 0.5rem;
  padding: 0.35rem 0.45rem;
  border: 1px dashed var(--button-border);
  border-radius: 6px;
  color: var(--text-muted);
  font-size: 0.86rem;
  background: color-mix(in srgb, var(--button-bg) 55%, transparent);
}
</style>
