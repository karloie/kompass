# 🧭 Kompass

<img src="doc/vibecoded.png" width="120" alt="Vibe Coded Badge" align="right">

[![Go Reference](https://pkg.go.dev/badge/github.com/karloie/kompass.svg)](https://pkg.go.dev/github.com/karloie/kompass)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/karloie/kompass)](go.mod)
[![Homebrew Version](https://img.shields.io/badge/dynamic/regex?url=https%3A%2F%2Fraw.githubusercontent.com%2Fkarloie%2Fhomebrew-tap%2Fmain%2Fkompass.rb&search=version%20%22(%3F%3Cversion%3E%5B%5E%22%5D%2B)%22&replace=%24%3Cversion%3E&label=homebrew)](https://github.com/karloie/homebrew-tap)
[![Docker Pulls](https://img.shields.io/docker/pulls/karloie/kompass)](https://hub.docker.com/r/karloie/kompass)

Kompass helps you learn Kubernetes by visualizing Kubernetes resource relationships. Query with patterns, get directed graphs showing how pods, services, deployments, and 30+ resource types connect in real clusters.

## Features

- **Pod-centric visualization** - Focus on pod relationships and dependencies
- **Wildcard selectors** - Patterns like `*/namespace/*`, `deployment/*/frontend`
- **30+ Kubernetes resources** - Workloads, networking, storage, RBAC, cert-manager, Cilium
- **Multiple formats** - ASCII tree, JSON
- **Dependency-aware ordering** - Controllers and related resources are grouped for readable topology
- **Go library** - Use in your tools
- **REST API** - Programmatic access
- **Mock data** - Test without cluster

## Supported Resources

- **Workloads**: Pod, Deployment, ReplicaSet, StatefulSet, DaemonSet, Job, CronJob  
- **Networking**: Service, Endpoints, EndpointSlice, Ingress, NetworkPolicy, Gateway, HTTPRoute, CiliumNetworkPolicy  
- **Storage**: PersistentVolume, PersistentVolumeClaim, StorageClass, VolumeAttachment, CSIDriver, CSINode  
- **Config**: ConfigMap, Secret, ServiceAccount  
- **RBAC**: Role, RoleBinding, ClusterRole, ClusterRoleBinding  
- **Cert**: Certificate, Issuer, ClusterIssuer  
- **Other**: HorizontalPodAutoscaler, Node

## Quick Start

> **Prerequisite:** For CLI usage against a real cluster, you need a valid Kubernetes client configuration (kubeconfig) for your target context. `kubectl` is the most common way to set this up, but any valid kubeconfig source works.

```bash
# List all pods in current namespace
kompass
```

### Example Output

```
🚀 deployment petshop-db [AVAILABLE] {available=1, current=2, namespace=petshop, ready=1, replicas=2, strategy=RollingUpdate, updated=2}
├─ 🕸 ciliumnetworkpolicy petshop-db {egress=false, ingress=true}
│  ├─ 👉 cnp-ingress fromendpoint {matchLabels=app.kubernetes.io/name:petshop-tennant}
│  │  └─ 🤝 service petshop-tennant {ports=[8080/TCP], selector=[app.kubernetes.io/instance=petshop-tennant app.kubernetes.io/name=petshop-tennant], type=ClusterIP}
│  ├─ 👉 cnp-ingress fromendpoint {matchLabels=app.kubernetes.io/name:petshop-frontend-girls}
│  │  ├─ 🤝 service petshop-lowe {loadBalancerIP=[boys.petshop.com west-end-girls.petshop.com its-a-sin.petshop.com always-on-my-mind.petshop.com go-west.petshop.com opportunities.petshop.com], ports=[443/TCP], selector=[app.kubernetes.io/instance=petshop-frontend-girls app.kubernetes.io/name=petshop-frontend-girls], type=LoadBalancer}
│  │  └─ 🤝 service petshop-frontend-girls {ports=[8080/TCP], selector=[app.kubernetes.io/instance=petshop-frontend-girls app.kubernetes.io/name=petshop-frontend-girls], type=ClusterIP}
│  ├─ 👉 cnp-ingress fromentities {entities=[cluster]}
│  ├─ 🎯 endpointselector label app.kubernetes.io/instance=petshop-db
│  └─ 🎯 endpointselector label app.kubernetes.io/name=petshop-db
├─ 🧩 replicaset petshop-db-5cb9cd8b74 [PROGRESSING] {available=1, current=2, ready=1, replicas=2}
│  ├─ 🫛 pod petshop-db-5cb9cd8b74-pqhk9 [RUNNING] {nodeName=psb-01-worker-055ceed2, podIP=10.244.9.90}
│  │  └─ 🧊 container app [RUNNING, LIVENESS=PASSING, READINESS=READY, STARTUP=STARTED]
│  │     └─ 🐋 image docker-hub/neo4j:5.26.20-community-ubi9 {id=docker-hub/neo4j@sha256:mock-petshop-db}
│  └─ 🫛 pod petshop-db-5cb9cd8b74-qx7m2 [PENDING] {nodeName=psb-01-worker-055ceed2, podIP=10.244.9.91}
│     └─ 🧊 container app [WAITING, LIVENESS=UNKNOWN, READINESS=NOT-READY, STARTUP=NOT-STARTED] {reason=CrashLoopBackOff, restarts=6}
│        └─ 🐋 image docker-hub/neo4j:5.26.20-community-ubi9 {id=docker-hub/neo4j@sha256:mock-petshop-db-bad}
├─ 🤝 service petshop-db-service {ports=[7474/TCP 7687/TCP], selector=[app.kubernetes.io/instance=petshop-db app.kubernetes.io/name=petshop-db], type=ClusterIP}
│  ├─ 📍 endpoints petshop-db-service
│  │  └─ 🔗 subset {ports=[petshop-db-bolt:7687/TCP petshop-db-http:7474/TCP]}
│  │     ├─ 🔌 address [READY] {ip=10.244.9.90, nodeName=psb-01-worker-055ceed2, targetName=petshop-db-5cb9cd8b74-pqhk9}
│  │     └─ 🔌 address [NOT-READY] {ip=10.244.9.91, nodeName=psb-01-worker-055ceed2, targetName=petshop-db-5cb9cd8b74-qx7m2}
│  └─ 📍 endpointslice petshop-db-service-672nq {addressType=IPv4, port=7474, portName=petshop-db-http, protocol=TCP}
│     ├─ 🔌 endpoint [READY] {address=10.244.9.90, nodeName=psb-01-worker-055ceed2, serving=true, targetName=petshop-db-5cb9cd8b74-pqhk9}
│     └─ 🔌 endpoint [NOT-READY] {address=10.244.9.91, nodeName=psb-01-worker-055ceed2, targetName=petshop-db-5cb9cd8b74-qx7m2}
└─ 📑 spec
   ├─ ♻️ environment
   │  └─ 💬 NEO_DB_PASSWORD=<SECRET> {key=PSB-DATABASE-PASSWORD, secretStore=petshop-db-vault}
   ├─ 🐋 image docker-hub/neo4j:5.26.20-community-ubi9 {pullPolicy=Always}
   ├─ 💓 livenessprobe {failureThreshold=40, periodSeconds=5, port=7687, timeoutSeconds=10, type=tcpSocket}
   ├─ 🛡️ podsecuritycontext {fsGroup=7474, runAsGroup=7474, runAsUser=7474}
   ├─ 🔌 ports
   │  └─ ⇄ port http {containerPort=8080, protocol=TCP}
   ├─ ✅ readinessprobe {failureThreshold=20, periodSeconds=5, port=7687, timeoutSeconds=10, type=tcpSocket}
   ├─ 🔧 resources {limits=memory:1Gi, requests=cpu:100m memory:128Mi}
   ├─ 🔒 secrets
   │  └─ 🔐 external secret source secrets-store.csi.k8s.io {driver=secrets-store.csi.k8s.io, secretProviderClass=petshop-db-vault}
   │     ├─ 🔏 provider config petshop-db-vault {provider=azure, secretObjects=1}
   │     └─ 🔒 synced secret petshop-db-secrets {keys=1, type=Opaque}
   │        └─ 💬 NEO_DB_PASSWORD=<SECRET> {key=PSB-DATABASE-PASSWORD}
   ├─ 🛡️ securitycontext {allowPrivilegeEscalation=false, capabilitiesDrop=[ALL], readOnlyRootFilesystem=false, runAsNonRoot=true, runAsUser=7474, seccompProfile=RuntimeDefault}
   ├─ ▶️ startupprobe {failureThreshold=1000, periodSeconds=5, port=7687, type=tcpSocket}
   └─ 💾 storage
      └─ 📀 claim petshop-db-data [BOUND] {accessModes=[ReadWriteOnce], capacity=3Gi, phase=Bound, storage=3Gi, storageClass=standard, volumeName=pvc-dbde64d2-ef2b-4cb7-ae0d-a3b07cb7e522}
         └─ 💿 backing volume pvc-dbde64d2-ef2b-4cb7-ae0d-a3b07cb7e522 {accessModes=[ReadWriteOnce], capacity=3Gi, phase=Bound, reclaimPolicy=Delete, storageClass=standard, volumeMode=Filesystem}
            ├─ 🗂️ storage class standard {allowVolumeExpansion=true, provisioner=kubernetes.io/gce-pd, reclaimPolicy=Delete, volumeBindingMode=Immediate}
            └─ 📎 attachment csi-959030a095e12a5c5224b5fe15796d0ad6ae46b1099c05fc09f46e08a6f47359 {attached=true, attacher=pd.csi.storage.gke.io, nodeName=psb-01-worker-055ceed2, pvName=pvc-dbde64d2-ef2b-4cb7-ae0d-a3b07cb7e522}
```

### Basic Commands

```bash
# Exact resource
kompass --mock deployment/petshop/petshop-kafka

# Multiple resources
kompass --mock */petshop/* */kafka-system/*

# Wildcard patterns
kompass '*/myapp/*'              # All resources in myapp namespace
kompass 'pod/*/*-api'            # All pods ending with -api
kompass 'deployment/prod/*'      # All deployments in prod namespace
```

### Resource Selector Format

- `name` - Resource in default namespace
- `namespace/name` - Resource in namespace
- `type/namespace/name` - Exact resource
- `*/namespace/*` - All in namespace

### CLI Options

| Flag | Short | Description |
|------|-------|-------------|
| `--context <name>` | `-c` | Kubernetes context |
| `--namespace <name>` | `-n` | Namespace |
| `--mock` | | Use mock data |
| `--json` | | JSON output |
| `--plain` | | Plain output without ANSI colors |
| `--debug` | `-d` | Enable debug logging |
| `--service [addr]` | | Start API server (`localhost:8080`) |
| `--version` | `-v` | Show version |
| `--help` | `-h` | Show help |

## Installation

### Go Install

```bash
go install github.com/karloie/kompass/cmd/kompass@latest
```

### Homebrew (macOS/Linux)

```bash
brew install karloie/tap/kompass
```

### Binary Download

Download pre-built binaries from [GitHub Releases](https://github.com/karloie/kompass/releases)

### Package Managers

```bash
# Debian/Ubuntu
wget https://github.com/karloie/kompass/releases/latest/download/kompass_<version>_linux_amd64.deb
sudo dpkg -i kompass_<version>_linux_amd64.deb

# Red Hat/Fedora/CentOS
wget https://github.com/karloie/kompass/releases/latest/download/kompass_<version>_linux_amd64.rpm
sudo rpm -i kompass_<version>_linux_amd64.rpm
```

### Container

The Docker image is intended to be deployed inside a Kubernetes cluster as a service running the REST API. For local CLI usage, install the binary instead.

The image also includes `kubectl`, `cilium`, and `hubble` binaries so pod-level diagnostics (for example TUI netpol/hubble pages when running interactively) can run in-cluster without installing extra tools.

```bash
# Run API server
docker run -p 8080:8080 karloie/kompass:latest --service 0.0.0.0:8080

# With mock data
docker run -p 8080:8080 karloie/kompass:latest --service 0.0.0.0:8080 --mock
```

> **Note:** Direct cluster access from containers may fail with OIDC or exec-based authentication plugins (AWS, GKE, Azure). Use binary installation for CLI usage.

### Kubernetes Deployment (Recommended)

Use the included manifest for an in-cluster API deployment with service account + read-only RBAC:

```bash
kubectl apply -f deploy/kompass-k8s.yaml
kubectl -n kompass rollout status deploy/kompass
```

Forward locally and query:

```bash
kubectl -n kompass port-forward svc/kompass 8080:8080

# In another terminal
curl "http://localhost:8080/api/healthz"
curl "http://localhost:8080/api/graph?namespace=default&selector=*/default/*"
```

Manifest location: `deploy/kompass-k8s.yaml`

## API Server Usage

### Starting the Server

```bash
# Start on default port (8080)
kompass --service

# Custom port
kompass --service localhost:9090

# Bind to specific interface
kompass --service 0.0.0.0:8080

# With specific namespace and context
kompass --service --namespace production --context prod

# Using mock data
kompass --service --mock

# Enable debug logging
kompass --debug '*/petshop/*'
```

### Available Output Formats

- **JSON Graph** - Flat graph JSON with `nodes`, `edges`, and `components` (`/api/graph`)
- **JSON Tree** - Tree-oriented JSON with `trees` plus shared `nodes` (`/api/tree`, `Accept: application/json`)
- **Text Tree** - ASCII tree rendering (`/api/tree`, `Accept: text/plain`)
- **HTML Tree** - interactive HTML tree (`/api/tree`, `Accept: text/html`)

HTML tree UI features:

- Namespace dropdown that reloads the tree for the selected namespace
- Client-side filter with wildcard (`*`, `?`) and negate (`!`) support
- URL-synced filter query via `q=`

### REST API

Endpoints accept query parameters:

| Parameter | Description |
|-----------|-------------|
| `selector` | Resource selector (comma-separated, optional) |
| `namespace` | Target namespace |
| `mock` | Use mock data when set to `mock` |
| `q` | HTML tree filter query (`Accept: text/html`) |
| `static` | Hide namespace selector in HTML output (`Accept: text/html`) |

Graph and tree JSON responses include request metadata under `request.selectors` as an array.

### API Examples

```bash
# JSON graph
curl "http://my.service.net/api/graph?selector=deployment/myapp/frontend&namespace=default"

# JSON graph in mock mode
curl "http://my.service.net/api/graph?mock=mock&selector=*/petshop/*"

# JSON tree
curl -H "Accept: application/json" "http://my.service.net/api/tree?namespace=production&selector=pod/production/myapp"

# ASCII tree
curl -H "Accept: text/plain" "http://my.service.net/api/tree?namespace=production&selector=pod/production/myapp"

# HTML tree
curl -H "Accept: text/html" "http://my.service.net/api/tree?namespace=production"

# HTML tree with prefilled filter query
curl -H "Accept: text/html" "http://my.service.net/api/tree?namespace=production&q=kafka*"

# Static/embed HTML tree (no namespace switcher)
curl -H "Accept: text/html" "http://my.service.net/api/tree?namespace=production&static=1"

# Cache metadata
curl "http://my.service.net/api/metadata"

# Health check
curl "http://my.service.net/api/healthz"  # Liveness
curl "http://my.service.net/api/readyz"   # Readiness
```

## Development

### Building

```bash
make build
```

### Service Modes

```bash
# Dev mode (hot reload)
make dev

# Standard build
make build

# Release build
make build-release
```

Runtime behavior:

- `kompass --service` serves API endpoints on `localhost:8080` by default.
- Use `kompass --service 0.0.0.0:8080` to publish on all interfaces.

Snapshot behavior:

- `make snapshot` writes deterministic mock fixtures (`testdata/fixtures/mock.json`, `testdata/fixtures/mock.txt`).
- `make snapshot-real` writes real-cluster fixtures (`testdata/fixtures/real.json`, `testdata/fixtures/real.txt`).

### Running Tests

```bash
make test
```

### Coverage Report

```bash
make coverage
```

### Running Locally

```bash
# With mock data
make mock

# Against real cluster
make real
```

## License

MIT - see [LICENSE](LICENSE)
