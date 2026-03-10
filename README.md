# 🧭 Kompass

<img src="doc/vibecoded.png" width="120" alt="Vibe Coded Badge" align="right">

[![Go Reference](https://pkg.go.dev/badge/github.com/karloie/kompass.svg)](https://pkg.go.dev/github.com/karloie/kompass)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/karloie/kompass)](go.mod)
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
🚀 deployment: petshop-web [AVAILABLE] {available=1, current=1, namespace=petshop, ready=1, replicas=1, strategy=RollingUpdate, updated=1}
├─ 📜 ciliumnetworkpolicy: petshop-web {egress=false, ingress=true}
│  ├─ 👉 cnp-ingress: fromentities {entities=[cluster]}
│  ├─ 🎯 endpointselector: label app.kubernetes.io/instance=petshop-web
│  ├─ 🎯 endpointselector: label app.kubernetes.io/name=petshop-web
│  ├─ 👈 cnp-egress: toendpoint {matchLabels=app.kubernetes.io/name:petshop-db}
│  │  └─ 🖥️ service: petshop-db-service {ports=[7474/TCP 7687/TCP], selector=[app.kubernetes.io/instance=petshop-db app.kubernetes.io/name=petshop-db], type=ClusterIP}
│  └─ 👈 cnp-egress: tofqdn {matchName=am.kpt.petshop.com}
├─ 🎮 replicaset: petshop-web-598696998b [READY] {available=1, current=1, ready=1, replicas=1}
│  ├─ 🫛 pod: petshop-web-598696998b-xxxxx {nodeName=bunny-01-worker-055ceed2, phase=Running, podIP=10.244.9.240}
│  │  └─ 🧊 container: app
│  │     └─ 🐋 image: petshop/petshop-web:main_20260226_100330
│  └─ 🫛 pod: petshop-web-598696998b-yyyyy {phase=Running, podIP=10.244.9.250}
│     └─ 🧊 container: app
│        └─ 🐋 image: petshop/petshop-web:main_20260226_100330
├─ 🖥️ service: petshop-web {ports=[8080/TCP], selector=[app.kubernetes.io/instance=petshop-web app.kubernetes.io/name=petshop-web], type=ClusterIP}
│  ├─ 📍 endpoints: petshop-web
│  │  └─ 🔗 subset: {ports=[petshop-web-http:8080/TCP]}
│  │     └─ 🔌 address: {ip=10.244.9.240, nodeName=bunny-01-worker-055ceed2, targetName=petshop-web-598696998b-xxxxx}
│  ├─ 📍 endpointslice: petshop-web-abcde {addressType=IPv4, port=8080, portName=petshop-web-http, protocol=TCP}
│  │  └─ 🔌 endpoint: {address=10.244.9.240, nodeName=bunny-01-worker-055ceed2, ready=true, serving=true, targetName=petshop-web-598696998b-xxxxx}
│  └─ 🔄 httproute: petshop-web {hostnames=[petshop-web.bunny.los.petshop.com petshop-web.bunny.petshop.com], paths=[/*], service=petshop-web:8080/TCP (ClusterIP)}
└─ 📄 spec
   ├─ 🔤 envars
   │  ├─ 💬 env: LOG_LEVEL=info
   │  ├─ 💬 env: NEO4J_PASSWORD=<SECRET> {key=PETSHOP-DATABASE-PASSWORD}
   │  │  └─ 🔒 secret: petshop-web-secrets {keys=2, type=Opaque}
   │  ├─ 💬 env: NEO4J_URI=bolt://petshop-db-service:7687
   │  ├─ 💬 env: NEO4J_USERNAME=neo4j
   │  ├─ 💬 env: OIDC_CLIENT_ID=petshop-web
   │  ├─ 💬 env: OIDC_CLIENT_SECRET=<SECRET> {key=PETSHOP-WEB-CLIENT-SECRET}
   │  │  └─ 🔒 secret: petshop-web-secrets {keys=2, type=Opaque}
   │  ├─ 💬 env: OIDC_ISSUER_URL=https://am.kpt.petshop.com/am/oauth2/realms/root/realms/intranett
   │  ├─ 💬 env: OIDC_REDIRECT_URL=https://petshop-web.bunny.petshop.com/auth/callback
   │  └─ 💬 env: REQUIRE_SECURE_CONNECTION=true
   ├─ 🐋 image: petshop/petshop-web:main_20260226_100330 {pullPolicy=Always}
   ├─ 📂 mounts
   │  ├─ 📁 mount: {mount=/tmp, volume=emptyDir}
   │  └─ 📁 mount: {mount=/mnt/secrets, readOnly=true, volume=secrets-store.csi.k8s.io}
   ├─ 🔌 ports
   │  └─ ⇄ port: http {containerPort=8080, protocol=TCP}
   ├─ 💾 resources: {limits=memory:512Mi, requests=cpu:100m memory:128Mi}
   └─ 🛡️ securitycontext: {allowPrivilegeEscalation=false, capabilitiesDrop=[ALL], readOnlyRootFilesystem=false, runAsNonRoot=true, runAsUser=1000, seccompProfile=RuntimeDefault}
```

### Basic Commands

```bash
# Exact resource
kompass deployment/myapp/frontend

# Multiple resources
kompass pod/default/nginx service/default/nginx

# Wildcard patterns
kompass '*/myapp/*'              # All resources in myapp namespace
kompass 'pod/*/*-api'            # All pods ending with -api
kompass 'deployment/prod/*'      # All deployments in prod namespace
```

### Resource Selector Format

- `name` - Resource in default namespace
- `namespace/name` - Resource in namespace
- `type/name` - Typed resource
- `type/namespace/name` - Exact resource
- `*/namespace/*` - All in namespace

### CLI Options

| Flag | Short | Description |
|------|-------|-------------|
| `--context <name>` | `-c` | Kubernetes context |
| `--namespace <name>` | `-n` | Namespace |
| `--mock` | | Use mock data |
| `--json` | | JSON output |
| `--plain` | | Plain output |
| `--service [addr]` | | Start server (`:8080`) |
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

The Docker image is designed for running the REST API server. For CLI usage, install the binary instead.

```bash
# Run API server
docker run -p 8080:8080 karloie/kompass:latest --service

# With mock data
docker run -p 8080:8080 karloie/kompass:latest --service --mock
```

> **Note:** Direct cluster access from containers may fail with OIDC or exec-based authentication plugins (AWS, GKE, Azure). Use binary installation for CLI usage.

## API Server Usage

### Starting the Server

```bash
# Start on default port (8080)
kompass --service

# Custom port
kompass --service :9090

# Bind to specific interface
kompass --service 0.0.0.0:8080

# With specific namespace and context
kompass --service --namespace production --context prod

# Using mock data
kompass --service --mock
```

### Available Output Formats

- **Text** - ASCII tree (`/tree`)
- **JSON** - Raw graph data (`/graph`)

### REST API

Endpoints accept query parameters:

| Parameter | Description |
|-----------|-------------|
| `selector` | Resource selector (comma-separated) |
| `namespace` | Target namespace |
| `mock` | Use mock data |

### API Examples

```bash
# JSON graph
curl "http://localhost:8080/graph?selector=deployment/myapp/frontend&namespace=default"

# ASCII tree
curl "http://localhost:8080/tree?namespace=production&selector=pod/production/myapp"

# Health check
curl "http://localhost:8080/healthz"  # Liveness
curl "http://localhost:8080/readyz"   # Readiness
```

## Development

### Building

```bash
make build
```

### Running Tests

```bash
make test
```

### Coverage Report

```bash
make cover
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
