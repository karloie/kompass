<script setup>
import { computed } from 'vue'

const props = defineProps({
  node: {
    type: Object,
    required: true,
  },
})

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
</script>

<template>
  <li class="tree-node">
    <details v-if="!isLeaf" open>
      <summary class="tree-node__summary">
        <span v-if="icon" class="tree-node__icon" aria-hidden="true">{{ icon }}</span>
        <span>{{ title }}</span>
      </summary>
      <ul class="tree-node__children">
        <TreeNode
          v-for="(child, index) in childNodes"
          :key="child?.key || `${title}-${index}`"
          :node="child"
        />
      </ul>
    </details>
    <div v-else class="tree-node__summary tree-node__leaf">
      <span v-if="icon" class="tree-node__icon" aria-hidden="true">{{ icon }}</span>
      <span>{{ title }}</span>
    </div>
  </li>
</template>

<style scoped>
.tree-node {
  list-style: none;
}

.tree-node__summary {
  cursor: pointer;
  user-select: none;
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
</style>
