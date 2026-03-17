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
const treeNamespace = inject('treeNamespace', null)

const nodeKey = computed(() => String(props.node?.key || ''))
const nodeType = computed(() => String(props.node?.type || '').toLowerCase())

const title = computed(() => {
  const meta = props.node?.metadata || {}
  const key = String(props.node?.key || '').trim()
  const type = String(props.node?.type || extractTypeFromKey(key) || 'resource').trim()
  const typeLower = type.toLowerCase()
  if (typeLower === 'env') {
    const envName = String(meta.name || extractNameFromKey(key) || '').trim()
    const hasValue = Object.prototype.hasOwnProperty.call(meta, 'value')
    const envValue = hasValue ? String(meta.value ?? '') : ''
    if (envName && hasValue) {
      return `${envName}=${envValue}`
    }
    if (envName) {
      return envName
    }
  }
  if (typeLower === 'mount') {
    const mountPath = String(meta.mount || '').trim()
    if (mountPath) {
      return mountPath
    }
  }
  const name = String(meta.name || extractNameFromKey(key) || 'unknown').trim()
  if (name && type && name.toLowerCase() === type.toLowerCase()) {
    return type
  }
  return `${type} ${name}`
})

const icon = computed(() => {
  if (typeof props.node?.icon === 'string' && props.node.icon.trim() !== '') {
    return props.node.icon
  }
  return ''
})

const childNodes = computed(() => props.node?.children || [])
const hasChildren = computed(() => childNodes.value.length > 0)
const availableViews = computed(() => availableViewsForNode(props.node, { appViewsEnabled: props.viewActionsEnabled }))

const isOpen = computed(() => {
  if (!hasChildren.value || !treeExpand) return false
  return treeExpand.isNodeOpen(nodeKey.value, props.depth, nodeType.value)
})

const isPolicyNode = computed(() => nodeType.value.includes('policy'))

const policyRuleBadge = computed(() => {
  const key = nodeKey.value.toLowerCase()
  const type = nodeType.value
  const ruleType = String(props.node?.metadata?.ruleType || props.node?.metadata?.ruletype || '').toLowerCase()

  let direction = ''
  if (key.includes('/ingress/rule/') || key.includes('/ingressdeny/rule/') || type.includes('ingress')) {
    direction = 'IN'
  } else if (key.includes('/egress/rule/') || key.includes('/egressdeny/rule/') || type.includes('egress')) {
    direction = 'OUT'
  }

  if (!direction) {
    return null
  }

  const isDeny =
    key.includes('/ingressdeny/') ||
    key.includes('/egressdeny/') ||
    type.includes('deny') ||
    ruleType.includes('deny')

  return isDeny
    ? { text: `${direction} DENY`, title: `${direction === 'IN' ? 'Ingress' : 'Egress'} rule: deny`, tone: 'deny' }
    : { text: `${direction} ALLOW`, title: `${direction === 'IN' ? 'Ingress' : 'Egress'} rule: allow`, tone: 'allow' }
})

const statusBadge = computed(() => {
  const meta = props.node?.metadata || {}
  const raw = pickStatusValue(meta)
  if (!raw) {
    return null
  }
  return {
    text: compactStatusText(raw),
    tone: statusTone(raw),
  }
})

const stateBadge = computed(() => {
  const meta = props.node?.metadata || {}
  const raw = compactStateValue(meta.state)
  if (!raw) {
    return null
  }
  return {
    text: compactStatusText(raw),
    tone: statusTone(raw),
  }
})

const trafficBadges = computed(() => {
  if (policyRuleBadge.value) {
    return [policyRuleBadge.value]
  }

  if (!isPolicyNode.value) {
    return []
  }

  const meta = props.node?.metadata || {}
  const badges = []
  const keyHints = collectPolicyDirectionHints(childNodes.value)

  const ingress = asBool(meta.ingress)
  const ingressState = ingress !== null ? ingress : keyHints.hasIngressRules
  const ingressTitle =
    ingress !== null
      ? ingressState
        ? 'Inbound direction: default deny enabled'
        : 'Inbound direction: default deny not enabled'
      : ingressState
        ? 'Inbound direction: default deny inferred from ingress rules'
        : 'Inbound direction: default deny inferred as disabled (no ingress rules)'
  badges.push(
    ingressState
      ? { text: 'IN DENY', title: ingressTitle, tone: 'deny' }
      : { text: 'IN ALLOW', title: ingressTitle, tone: 'allow' },
  )

  const egress = asBool(meta.egress)
  const egressState = egress !== null ? egress : keyHints.hasEgressRules
  const egressTitle =
    egress !== null
      ? egressState
        ? 'Outbound direction: default deny enabled'
        : 'Outbound direction: default deny not enabled'
      : egressState
        ? 'Outbound direction: default deny inferred from egress rules'
        : 'Outbound direction: default deny inferred as disabled (no egress rules)'
  badges.push(
    egressState
      ? { text: 'OUT DENY', title: egressTitle, tone: 'deny' }
      : { text: 'OUT ALLOW', title: egressTitle, tone: 'allow' },
  )

  return badges
})

function collectPolicyDirectionHints(nodes) {
  const hints = {
    hasIngressRules: false,
    hasEgressRules: false,
  }
  for (const child of nodes || []) {
    const key = String(child?.key || '').toLowerCase()
    const type = String(child?.type || '').toLowerCase()
    if (
      key.includes('/ingress/rule/') ||
      key.includes('/ingressdeny/rule/') ||
      type.includes('ingress')
    ) {
      hints.hasIngressRules = true
    }
    if (
      key.includes('/egress/rule/') ||
      key.includes('/egressdeny/rule/') ||
      type.includes('egress')
    ) {
      hints.hasEgressRules = true
    }
  }
  return hints
}

const metadataInline = computed(() => {
  const pairs = visibleMetadataEntries.value
    .filter((entry) => !statusMetadataKeys.has(entry.key))
    .map((entry) => `${entry.key}: ${entry.value}`)
  if (!pairs.length) {
    return ''
  }
  const joined = pairs.join(' | ')
  if (joined.length <= 90) {
    return joined
  }
  return `${joined.slice(0, 87)}...`
})

function pickStatusValue(meta) {
  const keys = [
    'status',
    'phase',
    'conditions',
    'replicaCounts',
    'daemonCounts',
    'jobCounts',
    'hpaStatus',
    'issuerStatus',
  ]
  for (const key of keys) {
    const value = String(meta?.[key] || '').trim()
    if (value) {
      return value
    }
  }
  return ''
}

function compactStatusText(raw) {
  const text = String(raw || '').trim()
  if (text.length <= 22) {
    return text
  }
  return `${text.slice(0, 19)}...`
}

function compactStateValue(value) {
  if (value == null) {
    return ''
  }
  if (typeof value === 'string') {
    return value.trim()
  }
  if (typeof value === 'object') {
    const keys = Object.keys(value)
    if (!keys.length) {
      return ''
    }
    return keys[0]
  }
  return String(value).trim()
}

function asBool(value) {
  if (typeof value === 'boolean') {
    return value
  }
  if (typeof value === 'string') {
    const v = value.trim().toLowerCase()
    if (v === 'true') return true
    if (v === 'false') return false
  }
  return null
}

function statusTone(raw) {
  const value = String(raw || '').toLowerCase()
  if (
    value.includes('crash') ||
    value.includes('error') ||
    value.includes('fail') ||
    value.includes('degrad') ||
    value.includes('unhealthy') ||
    value.includes('not ready') ||
    value.includes('oom') ||
    value.includes('backoff')
  ) {
    return 'bad'
  }
  if (
    value.includes('pending') ||
    value.includes('progress') ||
    value.includes('starting') ||
    value.includes('terminating') ||
    value.includes('unknown')
  ) {
    return 'warn'
  }
  if (
    value.includes('running') ||
    value.includes('ready') ||
    value.includes('active') ||
    value.includes('bound') ||
    value.includes('healthy') ||
    value.includes('succeed') ||
    value.includes('pass') ||
    value.includes('ok')
  ) {
    return 'good'
  }
  return 'neutral'
}

function onToggle() {
  if (!treeExpand || !hasChildren.value) return
  treeExpand.toggleNode(nodeKey.value, props.node, props.depth, nodeType.value)
}

function openView(view) {
  emit('open-view', { node: props.node, view })
}

function extractNameFromKey(key) {
  const parts = String(key || '').split('/').filter(Boolean)
  if (!parts.length) {
    return ''
  }
  return parts[parts.length - 1]
}

function extractTypeFromKey(key) {
  const parts = String(key || '').split('/').filter(Boolean)
  if (!parts.length) {
    return ''
  }
  return parts[0]
}

const hiddenMetadataKeys = new Set([
  'annotations',
  'available',
  'current',
  '__nodetype',
  'count',
  'creationtimestamp',
  'displayprefix',
  'expiresin',
  'index',
  'kind',
  'labels',
  'managedfields',
  'mount',
  'name',
  'orphaned',
  'ownerreferences',
  'policytype',
  'livenessstatus',
  'readyreason',
  'readinessstatus',
  'startupstatus',
  'resourceversion',
  'ruletype',
  'ready',
  'source',
  'sourcetype',
  'targetkind',
  'updated',
  'uid',
  'value',
  'volumetype',
])

const statusMetadataKeys = new Set([
  'status',
  'state',
  'ingress',
  'egress',
  'phase',
  'conditions',
  'replicaCounts',
  'daemonCounts',
  'jobCounts',
  'hpaStatus',
  'issuerStatus',
])

const visibleMetadataEntries = computed(() => {
  const meta = props.node?.metadata || {}
  const selectedNamespace = String(treeNamespace?.value || '').trim()
  const keys = Object.keys(meta).sort((a, b) => a.localeCompare(b))
  const entries = []
  for (const key of keys) {
    const value = meta[key]
    if (value == null) {
      continue
    }
    if (hiddenMetadataKeys.has(String(key).trim().toLowerCase())) {
      continue
    }
    if (String(key).trim().toLowerCase() === 'namespace' && selectedNamespace) {
      const nsValue = String(value || '').trim()
      if (nsValue === selectedNamespace) {
        continue
      }
    }
    let displayValue = value
    if (typeof value === 'object') {
      displayValue = JSON.stringify(value, null, 0).replace(/,/g, ', ')
    }
    const text = String(displayValue).trim()
    if (!text) {
      continue
    }
    entries.push({ key, value: text })
  }
  return entries
})
</script>

<template>
  <li class="tree-node">
    <div class="tree-node__summary" :class="hasChildren ? 'tree-node__branch' : 'tree-node__leaf'" @click="onToggle">
      <span class="tree-node__expand-icon" aria-hidden="true">{{ hasChildren ? (isOpen ? '▾' : '▸') : '' }}</span>
      <span v-if="icon" class="tree-node__icon" aria-hidden="true">{{ icon }}</span>
      <span class="tree-node__label">{{ title }}</span>
      <span v-if="statusBadge" class="tree-node__status" :class="`tree-node__status--${statusBadge.tone}`">{{ statusBadge.text }}</span>
      <span v-if="stateBadge" class="tree-node__status" :class="`tree-node__status--${stateBadge.tone}`">{{ stateBadge.text }}</span>
      <span
        v-for="badge in trafficBadges"
        :key="badge.text"
        class="tree-node__policy-badge"
        :class="`tree-node__policy-badge--${badge.tone || 'deny'}`"
        :title="badge.title"
      >
        {{ badge.text }}
      </span>
      <span v-if="metadataInline" class="tree-node__meta-inline">{{ metadataInline }}</span>
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

    <ul v-if="hasChildren && isOpen" class="tree-node__children">
      <TreeNode
        v-for="(child, index) in childNodes"
        :key="child?.key || `${title}-${index}`"
        :node="child"
        :depth="depth + 1"
        :view-actions-enabled="viewActionsEnabled"
        @open-view="emit('open-view', $event)"
      />
    </ul>
  </li>
</template>

<style scoped>
.tree-node {
  list-style: none;
  width: max-content;
  min-width: 100%;
}

.tree-node__summary {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  justify-content: flex-start;
  white-space: nowrap;
  width: max-content;
  min-width: 100%;
  user-select: none;
  position: relative;
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

.tree-node__label {
  white-space: nowrap;
}

.tree-node__leaf {
  cursor: default;
}

.tree-node__status {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  border-radius: 999px;
  border: 1px solid var(--button-border);
  padding: 0.05rem 0.45rem;
  font-size: 0.7rem;
  line-height: 1.2;
  font-weight: 700;
  white-space: nowrap;
}

.tree-node__status--good {
  color: #2fb36a;
  background: color-mix(in srgb, #2fb36a 14%, transparent);
}

.tree-node__status--warn {
  color: #d0a53a;
  background: color-mix(in srgb, #d0a53a 14%, transparent);
}

.tree-node__status--bad {
  color: #de5b5b;
  background: color-mix(in srgb, #de5b5b 14%, transparent);
}

.tree-node__status--neutral {
  color: var(--text-muted);
  background: color-mix(in srgb, var(--text-muted) 12%, transparent);
}

.tree-node__policy-badge {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  border-radius: 999px;
  border: 1px solid var(--button-border);
  padding: 0.05rem 0.45rem;
  font-size: 0.68rem;
  line-height: 1.2;
  font-weight: 700;
  white-space: nowrap;
}

.tree-node__policy-badge--deny {
  color: #de5b5b;
  background: color-mix(in srgb, #de5b5b 14%, transparent);
}

.tree-node__policy-badge--allow {
  color: #2fb36a;
  background: color-mix(in srgb, #2fb36a 14%, transparent);
}

.tree-node__meta-inline {
  color: var(--text-muted);
  font-size: 0.74rem;
  line-height: 1.2;
  white-space: nowrap;
}

.tree-node__icon {
  display: inline-block;
  width: 1.4em;
}

.tree-node__children {
  margin: 0.25rem 0 0.25rem 1rem;
  padding-left: 0.5rem;
  border-left: 1px dashed var(--button-border);
}

.tree-node__actions {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  position: sticky;
  right: 0.35rem;
  margin-left: auto;
  padding-left: 0.5rem;
  opacity: 0;
  transition: opacity 120ms ease;
  z-index: 1;
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
