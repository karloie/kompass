# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.0.1] - 2026-03-10

### Added
- Initial vibe release of Kompass
- CLI tool for visualizing Kubernetes resource relationships
- Support for 30+ Kubernetes resource types including:
  - Workloads: Pod, Deployment, ReplicaSet, StatefulSet, DaemonSet, Job, CronJob
  - Networking: Service, Endpoints, EndpointSlice, Ingress, NetworkPolicy, Gateway, HTTPRoute
  - Storage: PersistentVolume, PersistentVolumeClaim, StorageClass
  - RBAC: Role, RoleBinding, ClusterRole, ClusterRoleBinding
  - Config: ConfigMap, Secret, ServiceAccount
  - Cert-manager: Certificate, Issuer, ClusterIssuer
  - Cilium: CiliumNetworkPolicy, CiliumClusterwideNetworkPolicy
- Pod-centric relationship visualization
- Wildcard selector patterns (`*/namespace/*`, `type/*/name`)
- Multiple output formats (ASCII tree, JSON)
- REST API server with `/graph`, `/tree`, `/health` endpoints
- Mock data provider for testing without cluster access
- Rate limiting (100 req/sec) and response caching
- Multi-platform binaries (Linux, macOS, Windows, ARM64)
- Package manager support (deb, rpm, Homebrew)
- Docker container images
- Kubernetes deployment manifests

### Features
- `--context` / `-c` flag for selecting Kubernetes context
- `--namespace` / `-n` flag for namespace filtering
- `--mock` flag for using mock test data
- `--json` flag for JSON output
- `--plain` flag for plain output without emojis
- `--service` flag for starting REST API server
- `--version` / `-v` flag for version information
- `--help` / `-h` flag for usage information

[Unreleased]: https://github.com/karloie/kompass/compare/v0.0.1...HEAD
[0.0.1]: https://github.com/karloie/kompass/releases/tag/v0.0.1
