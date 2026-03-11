# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.8] - 2026-03-12

### Changed
- `/tree` now defaults to plain output in server mode; rich output can be requested via the `plain` query parameter.
- Plain tree rendering keeps emoji markers while removing ANSI styling.
- Simplified JSON response contracts by removing per-graph duplicated node references and relying on response-level `nodes` maps.
- Simplified tree contract shape: `response.trees` now contains tree nodes directly.
- Aligned response struct field order to place `nodes` first in graph/tree responses for clearer shared-node contract readability.
- Added CI workflow for branch and PR.

### Fixed
- ReplicaSet ownership inference now correctly falls back to selector-vs-pod-label matching when owner references are missing.
- `/tree` now returns proper `500` responses on provider/inference failures instead of `200` with error text.
- Server request handling no longer mutates shared client namespace between requests.

### Docs
- Added `--debug` / `-d` flag documentation and aligned `--plain` wording to "without ANSI colors".

### Tests
- Expanded test coverage for `pkg/graph` and `pkg/kube`, including cache/client/loaders/core utilities and graph inference paths.

## [0.0.7] - 2026-03-11

### Fixed
- Improved `--json` performance for large clusters by reducing response-building overhead in graph output paths.
- Resolved smaller stability bugs in server/CLI handling and provider edge cases.

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

[Unreleased]: https://github.com/karloie/kompass/compare/v0.0.8...HEAD
[0.0.8]: https://github.com/karloie/kompass/releases/tag/v0.0.8
[0.0.7]: https://github.com/karloie/kompass/releases/tag/v0.0.7
