<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { availableViewsForNode, nodeDisplayTitle, nodeRequestParams, viewLabel } from '../resourceViews'

const props = defineProps({
  node: {
    type: Object,
    required: true,
  },
  initialView: {
    type: String,
    default: 'describe',
  },
  apiBase: {
    type: String,
    default: '/api/app',
  },
})

const emit = defineEmits(['close'])

const activeView = ref('')
const loading = ref(false)
const error = ref('')
const cache = ref({})

let currentController = null

const views = computed(() => availableViewsForNode(props.node))
const endpointMap = {
  describe: 'desc',
  logs: 'logs',
  events: 'events',
  hubble: 'hubble',
  yaml: 'yaml',
}

const currentPayload = computed(() => cache.value[activeView.value] || null)
const title = computed(() => currentPayload.value?.title || nodeDisplayTitle(props.node))
const content = computed(() => currentPayload.value?.content || '')

watch(
  () => [props.node?.key, props.initialView, views.value.join(',')],
  () => {
    cache.value = {}
    error.value = ''
    activeView.value = pickInitialView()
  },
  { immediate: true },
)

watch(
  activeView,
  (value) => {
    if (!value || cache.value[value]) {
      return
    }
    fetchView(value)
  },
  { immediate: true },
)

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  if (currentController) {
    currentController.abort()
  }
})

function pickInitialView() {
  if (!views.value.length) {
    return ''
  }
  if (views.value.includes(props.initialView)) {
    return props.initialView
  }
  return views.value[0]
}

async function fetchView(view) {
  if (currentController) {
    currentController.abort()
  }

  const controller = new AbortController()
  currentController = controller
  loading.value = true
  error.value = ''

  try {
    const params = nodeRequestParams(props.node)
    const endpoint = endpointMap[view] || view
    const response = await fetch(`${props.apiBase}/${endpoint}?${params.toString()}`, {
      headers: {
        Accept: 'application/json',
      },
      signal: controller.signal,
    })
    if (!response.ok) {
      throw new Error(await response.text() || `request failed: ${response.status}`)
    }
    const payload = await response.json()
    cache.value = {
      ...cache.value,
      [view]: payload,
    }
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      return
    }
    error.value = err instanceof Error ? err.message : 'failed to load view'
  } finally {
    if (currentController === controller) {
      currentController = null
    }
    loading.value = false
  }
}

function closeView() {
  emit('close')
}

function onKeydown(event) {
  if (event.key === 'Escape') {
    closeView()
  }
}
</script>

<template>
  <section class="view" @click.self="closeView">
    <article class="view__panel">
      <header class="view__header">
        <div>
          <p class="view__eyebrow">Resource View</p>
          <h2 class="view__title">{{ title }}</h2>
        </div>

        <button class="view__close" type="button" @click="closeView">Close</button>
      </header>

      <nav v-if="views.length" class="view__tabs" aria-label="Resource views">
        <button
          v-for="view in views"
          :key="view"
          class="view__tab"
          :class="{ 'view__tab--active': view === activeView }"
          type="button"
          @click="activeView = view"
        >
          {{ viewLabel(view) }}
        </button>
      </nav>

      <p v-if="error" class="view__error">{{ error }}</p>
      <p v-else-if="loading && !content" class="view__hint">Loading view...</p>
      <p v-else-if="!views.length" class="view__hint">No backend views available for this resource.</p>
      <pre v-else class="view__content">{{ content }}</pre>
    </article>
  </section>
</template>

<style scoped>
.view {
  position: fixed;
  inset: 0;
  z-index: 20;
  display: grid;
  place-items: center;
  padding: 1.5rem;
  background: color-mix(in srgb, var(--page-bg) 40%, transparent);
  backdrop-filter: blur(8px);
}

.view__panel {
  width: min(1100px, 100%);
  height: min(85dvh, 100%);
  display: grid;
  grid-template-rows: auto auto 1fr;
  gap: 0.9rem;
  padding: 1.1rem;
  border: 1px solid var(--border-color);
  border-radius: 14px;
  background: var(--panel-bg);
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.22);
}

.view__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.view__eyebrow {
  margin: 0 0 0.25rem;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.view__title {
  margin: 0;
  font-size: 1.05rem;
}

.view__close,
.view__tab {
  border: 1px solid var(--button-border);
  background: var(--button-bg);
  color: var(--button-text);
  border-radius: 999px;
  cursor: pointer;
}

.view__close {
  padding: 0.45rem 0.8rem;
}

.view__tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.view__tab {
  padding: 0.4rem 0.8rem;
}

.view__tab--active {
  background: var(--text-main);
  color: var(--panel-bg);
  border-color: var(--text-main);
}

.view__hint,
.view__error {
  margin: 0;
  color: var(--text-muted);
}

.view__error {
  color: var(--danger-text);
}

.view__content {
  margin: 0;
  min-height: 0;
  overflow: auto;
  padding: 1rem;
  border-radius: 10px;
  border: 1px solid var(--border-color);
  background: color-mix(in srgb, var(--button-bg) 65%, var(--panel-bg));
  color: var(--text-main);
  white-space: pre-wrap;
  word-break: break-word;
  font: 0.87rem/1.45 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
}
</style>