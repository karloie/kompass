# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.13] - 2026-03-17

### Added
- Added web app interface for interactive cluster exploration in service mode.

### Fixed
- Improved api call strategy for large clusters

## [0.0.12] - 2026-03-15

### Changed
- Reworked the JSON DTO contract to use structured `request.selectors` plus flat graph-level `nodes`, `edges`, and `components` payloads.
- Added interactive HTML rendering for `/api/tree` with template-based markup and external JavaScript assets.
- Updated `make service` to run through `gow` for a faster local service dev loop.
- Updated snapshot workflow so `make snapshot` writes deterministic mock fixtures by default; real-cluster fixtures moved to `make snapshot-real`.

## [0.0.10] - 2026-03-16

### Changed
- Reworked the JSON DTO contract to use structured `request.selectors` plus flat graph-level `nodes`, `edges`, and `components` payloads.
- Added interactive HTML rendering for `/api/tree` with template-based markup and external JavaScript assets.
- Updated `make service` to run through `gow` for a faster local service dev loop.
- Updated snapshot workflow so `make snapshot` writes deterministic mock fixtures by default; real-cluster fixtures moved to `make snapshot-real`.

## [0.0.9] - 2026-03-13

### Changed
- Tree labels are now copy/paste friendly by removing `type: name` punctuation in favor of `type name` formatting.
- Root ordering in graph output is now sorted by `name, kind, namespace` within existing root buckets (workload, standalone, inferred) for faster human scanning.
- Pod and container runtime lines now surface probe health directly in bracketed status output.
- SecretStore views are now rendered in a more intuitive flow (`external source -> provider config -> synced Kubernetes Secret -> usage`) with clearer newcomer-friendly labels.
- ConfigMap sections now surface direct env usage and file-mount usage under each source ConfigMap with clearer intent-first wording.
- Storage sections now use clearer labels (`claim`, `backing volume`, `storage class`, `attachment`) and show PVC mount usage under the claim branch.
- Petshop mock now prominently features the legendary British pop duo tribute.

### Fixed
- Pod phase is now rendered consistently as a bracketed status on pod lines instead of duplicated metadata.
- Endpoint and endpoint-address nodes now render readiness as bracketed status (`[READY]` / `[NOT-READY]`) and avoid duplicated raw readiness fields.
- Mock `petshop-db` data now includes a degraded second replica with endpoint readiness drift to better represent real-world partial failure scenarios.

### Tests
- Updated and expanded tree/pod builder and snapshot fixtures to cover status formatting, secretstore env usage rendering, and degraded mock replica behavior.

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

[0.0.13]: https://github.com/karloie/kompass/releases/tag/v0.0.13
[0.0.12]: https://github.com/karloie/kompass/releases/tag/v0.0.12
[0.0.9]: https://github.com/karloie/kompass/releases/tag/v0.0.9
[0.0.8]: https://github.com/karloie/kompass/releases/tag/v0.0.8
[0.0.7]: https://github.com/karloie/kompass/releases/tag/v0.0.7
