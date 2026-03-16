<script setup>
import { computed, inject } from 'vue'
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
  depth: {
    type: Number,
    default: 0,
  },
})

const emit = defineEmits(['open-view'])

const treeExpand = inject('treeExpand', null)

const nodeKey = computed(() => String(props.node?.key || ''))
const nodeType = computed(() => String(props.node?.type || '').toLowerCase())

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

const isOpen = computed(() => {
  if (isLeaf.value || !treeExpand) return false
  return treeExpand.isNodeOpen(nodeKey.value, props.depth, nodeType.value)
})

function onToggle() {
  if (!treeExpand || isLeaf.value) return
  treeExpand.toggleNode(nodeKey.value, props.node, props.depth, nodeType.value)
}

function openView(view) {
  emit('open-view', { node: props.node, view })
}
</script>

<template>
  <li class="tree-node">
    <template v-if="!isLeaf">
      <div class="tree-node__summary tree-node__branch" @click="onToggle">
        <span class="tree-node__summary-main">
          <span class="tree-node__expand-icon" aria-hidden="true">{{ isOpen ? '▾' : '▸' }}</span>
          <span v-if="icon" class="tree-node__icon" aria-hidden="true">{{ icon }}</span>
          <span class="tree-node__label">{{ title }}</span>
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
      </div>
      <ul v-show="isOpen" class="tree-node__children">
        <TreeNode
          v-for="(child, index) in childNodes"
          :key="child?.key || `${title}-${index}`"
          :node="child"
          :depth="depth + 1"
          :view-actions-enabled="viewActionsEnabled"
          @open-view="emit('open-view', $event)"
        />
      </ul>
    </template>
    <div v-else class="tree-node__summary tree-node__leaf">
      <span class="tree-node__summary-main">
        <span class="tree-node__expand-icon" aria-hidden="true"> </span>
        <span v-if="icon" class="tree-node__icon" aria-hidden="true">{{ icon }}</span>
        <span class="tree-node__label">{{ title }}</span>
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
  white-space: nowrap;
  overflow: hidden;
  user-select: none;
}

.tree-node__branch {
  cursor: pointer;
}

.tree-node__summary:hover,
.tree-node__summary:focus-within {
  background: color-mix(in srgb, var(--accent-color) 18%, transparent);
  border-radius: 6px;
}

.tree-node__expand-icon {
  display: inline-block;
  width: 1em;
  flex-shrink: 0;
  font-size: 0.75rem;
  color: var(--text-muted);
  margin-right: 0.1rem;
}

.tree-node__summary-main {
  display: inline-flex;
  align-items: center;
  gap: 0.15rem;
  flex: 1;
  min-width: 0;
  overflow: hidden;
}

.tree-node__label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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
  flex-shrink: 0;
  margin-left: 0.5rem;
  opacity: 0;
  transition: opacity 120ms ease;
}

.tree-node__summary:hover > .tree-node__actions,
.tree-node__summary:focus-within > .tree-node__actions,
.tree-node__actions:hover,
.tree-node__actions:focus-within {
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
