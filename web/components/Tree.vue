<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import TreeNode from './TreeNode.vue'

const props = defineProps({
  namespace: {
    type: String,
    default: '',
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

const emit = defineEmits(['update:context-title', 'update:namespaces', 'suggest-namespace', 'update:loading', 'open-view'])

const loading = ref(false)
const error = ref('')
const payload = ref(null)

let currentController = null
let fetchSeq = 0
let debounceTimer = null

const roots = computed(() => payload.value?.trees || [])
const contextTitle = computed(() => String(payload.value?.request?.context || 'Context').trim() || 'Context')
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
  const matcher = buildMatcher(props.query.trim())
  const namespace = props.namespace

  return roots.value
    .map((node) => filterTree(node, { matcher, namespace }))
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

    if (!props.namespace) {
      const responseNamespace = String(nextPayload?.request?.namespace || '').trim()
      const suggested = responseNamespace || firstNamespace(nextPayload?.trees || [])
      if (suggested) {
        emit('suggest-namespace', suggested)
      }
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
  if (props.bootstrapData && typeof props.bootstrapData === 'object') {
    payload.value = props.bootstrapData
    if (!props.namespace) {
      const responseNamespace = String(props.bootstrapData?.request?.namespace || '').trim()
      const suggested = responseNamespace || firstNamespace(props.bootstrapData?.trees || [])
      if (suggested) {
        emit('suggest-namespace', suggested)
      }
    }
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

watch(() => props.refreshKey, () => {
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

function treeURL() {
  const base = apiBase.value
  if (!base) {
    return '/api/tree'
  }
  const params = new URLSearchParams()
  const namespace = props.namespace.trim()

  if (namespace) {
    params.set('namespace', namespace)
  }

  const query = params.toString()
  return query ? `${base}?${query}` : base
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
  const queryMatches = !filters.matcher.hasTerms || filters.matcher.test(nodeText(node))
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
  padding: 1rem;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--panel-bg);
  color: var(--text-main);
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
  margin: 0.75rem 0 0;
  padding: 0;
  min-height: 0;
  overflow: auto;
}
</style>
