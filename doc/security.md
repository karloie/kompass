# Kompass Security

## Overview

- Default: no auth on localhost:8080 (safe by binding)
- Operator can expose to 0.0.0.0

## Secret Redaction

Kompass redacts secret values in API responses to avoid exposing raw secret content in UI/API output.

Current behavior:
- Secret values in `data` and `stringData` are replaced with `<SECRET>`.
- `stringData` is removed from the response payload.
- Secret key names are preserved, and `keyCount` is added.

Scope:
- Applies to secret resources loaded for graph/tree responses.
- Applies when fetching an individual secret resource.
- Applies in both cluster mode and mock mode.

What is still visible:
- Secret metadata (name, namespace, labels/annotations where present).
- Secret type and key names.
- References to secrets from workloads and other resources.

Non-goal:
- Redaction reduces accidental exposure in Kompass output, but it is not a replacement for authentication, authorization, or Kubernetes RBAC.

## Current Implementation

- 401/403 responses
- Secret value redaction in API output
