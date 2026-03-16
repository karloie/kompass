<script setup>
import { computed } from 'vue'
import { availableViewsForNode, viewLabel, viewShortLabel } from '../resourceViews'

const props = defineProps({
  node: {
    type: Object,
    required: true,
  },
  viewActionsEnabled: {
    type: Boolean,
    default: true,
  },
})

const emit = defineEmits(['open-view'])

const title = computed(() => {
  const meta = props.node?.metadata || {}
  const name = meta.name || props.node?.key || 'unknown'
  const type = props.node?.type || 'resource'
  return `${type} ${name}`
})

const icon = computed(() => {
  if (typeof props.node?.icon === 'string' && props.node.icon.trim() !== '') {
    return props.node.icon
  }
  return ''
})

const childNodes = computed(() => props.node?.children || [])
const isLeaf = computed(() => childNodes.value.length === 0)
const availableViews = computed(() => availableViewsForNode(props.node, { appViewsEnabled: props.viewActionsEnabled }))

function openView(view) {
  emit('open-view', { node: props.node, view })
}
</script>

<template>
  <li class="tree-node">
    <details v-if="!isLeaf" open>
      <summary class="tree-node__summary">
        <span class="tree-node__summary-main">
          <span v-if="icon" class="tree-node__icon" aria-hidden="true">{{ icon }}</span>
          <span>{{ title }}</span>
        </span>
        <span v-if="availableViews.length" class="tree-node__actions">
          <button
            v-for="view in availableViews"
            :key="view"
            class="tree-node__action"
            type="button"
            :title="viewLabel(view)"
            @click.stop.prevent="openView(view)"
          >
            {{ viewShortLabel(view) }}
          </button>
        </span>
      </summary>
      <ul class="tree-node__children">
        <TreeNode
          v-for="(child, index) in childNodes"
          :key="child?.key || `${title}-${index}`"
          :node="child"
          :view-actions-enabled="viewActionsEnabled"
          @open-view="emit('open-view', $event)"
        />
      </ul>
    </details>
    <div v-else class="tree-node__summary tree-node__leaf">
      <span class="tree-node__summary-main">
        <span v-if="icon" class="tree-node__icon" aria-hidden="true">{{ icon }}</span>
        <span>{{ title }}</span>
      </span>
      <span v-if="availableViews.length" class="tree-node__actions tree-node__actions--static">
        <button
          v-for="view in availableViews"
          :key="view"
          class="tree-node__action"
          type="button"
          :title="viewLabel(view)"
          @click.stop.prevent="openView(view)"
        >
          {{ viewShortLabel(view) }}
        </button>
      </span>
    </div>
  </li>
</template>

<style scoped>
.tree-node {
  list-style: none;
}

.tree-node__summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  user-select: none;
}

.tree-node__summary-main {
  display: inline-flex;
  align-items: center;
  gap: 0.15rem;
  min-width: 0;
}

.tree-node__leaf {
  cursor: default;
}

.tree-node__icon {
  display: inline-block;
  width: 1.4em;
}

.tree-node__children {
  margin: 0.25rem 0 0.25rem 1rem;
  padding-left: 0.75rem;
  border-left: 1px dashed var(--button-border);
}

.tree-node__actions {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  opacity: 0;
  transition: opacity 120ms ease;
}

.tree-node__actions--static {
  opacity: 1;
}

.tree-node:hover .tree-node__actions,
.tree-node:focus-within .tree-node__actions {
  opacity: 1;
}

.tree-node__action {
  border: 1px solid var(--button-border);
  background: var(--button-bg);
  color: var(--button-text);
  border-radius: 999px;
  padding: 0.1rem 0.45rem;
  font-size: 0.72rem;
  font-weight: 700;
  line-height: 1.4;
  cursor: pointer;
}
</style>
