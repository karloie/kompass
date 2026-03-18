// Kubernetes API resource types that support kubectl operations
const kubernetesApiTypes = new Set([
  'pod', 'pod', 'deployment', 'service', 'replicaset', 'statefulset', 'daemonset',
  'job', 'cronjob', 'certificate', 'issuer', 'clusterissuer', 'configmap', 'secret',
  'persistentvolume', 'persistentvolumeclaim', 'namespace', 'networkpolicy',
  'ciliumnetworkpolicy', 'ciliumclusterwidenetworkpolicy', 'ingress', 'ingressclass',
  'node', 'endpoints', 'endpointslice', 'service', 'serviceaccount', 'storageclass',
  'csidriver', 'csinode', 'horizontalpodautoscaler', 'nats', 'jetstream', 'streamtemplate'
])

export function availableViewsForNode(node, options = {}) {
  const appViewsEnabled = options.appViewsEnabled !== false
  const type = nodeType(node)
  if (!type) {
    return []
  }
  if (!kubernetesApiTypes.has(type)) {
    return []
  }
  if (!appViewsEnabled) {
    return []
  }
  if (type === 'pod') {
    return ['describe', 'logs', 'events', 'cilium', 'yaml']
  }
  if (type === 'certificate') {
    return ['cert', 'describe', 'events', 'yaml']
  }
  return ['describe', 'events', 'yaml']
}

export function viewLabel(view) {
  return {
    describe: 'Describe',
    logs: 'Logs',
    events: 'Events',
    cilium: 'Cilium',
    cert: 'Cert',
    yaml: 'YAML',
    tree: 'Tree',
  }[view] || view
}

export function viewShortLabel(view) {
  return {
    describe: 'D',
    logs: 'L',
    events: 'E',
    cilium: 'C',
    cert: 'C',
    yaml: 'Y',
    tree: 'T',
  }[view] || '?'
}

export function nodeDisplayTitle(node) {
  const type = nodeType(node) || 'resource'
  const name = nodeName(node) || node?.key || 'unknown'
  return `${type} ${name}`
}

export function nodeRequestParams(node) {
  const meta = node?.metadata || {}
  const key = String(node?.key || '').trim()
  const parsed = parseNodeKey(key)
  const params = new URLSearchParams()

  if (key) {
    params.set('key', key)
  }
  const type = String(node?.type || parsed.type || '').trim()
  const namespace = String(meta.namespace || parsed.namespace || '').trim()
  const name = String(meta.name || parsed.name || '').trim()

  if (type) {
    params.set('type', type)
  }
  params.set('namespace', namespace)
  if (name) {
    params.set('name', name)
  }

  return params
}

function nodeType(node) {
  return String(node?.type || '').trim().toLowerCase()
}

function nodeName(node) {
  return String(node?.metadata?.name || parseNodeKey(String(node?.key || '')).name || '').trim()
}

function parseNodeKey(key) {
  const parts = String(key || '').trim().split('/').filter(Boolean)
  if (parts.length >= 3) {
    return {
      type: parts[0],
      namespace: parts[1],
      name: parts.slice(2).join('/'),
    }
  }
  if (parts.length === 2) {
    return {
      type: parts[0],
      namespace: '',
      name: parts[1],
    }
  }
  return { type: '', namespace: '', name: '' }
}