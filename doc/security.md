# Kompass Security

## Overview

- Default: no auth on localhost:8080 (safe by binding)
- Operator can expose to 0.0.0.0 and set OIDC or Basic Auth
- Authentication is enforced in shared deployments; authorization is currently not user-scoped.

## Configuration

`KOMPASS_AUTH_MODE` (default: `none`)
- `none`: no auth
- `oidc`: OIDC (requires KOMPASS_OIDC_ISSUER_URL, KOMPASS_OIDC_CLIENT_ID, KOMPASS_OIDC_CLIENT_SECRET, KOMPASS_OIDC_REDIRECT_URI)
- `basic`: HTTP Basic Auth (requires KOMPASS_BASIC_AUTH_USER, KOMPASS_BASIC_AUTH_HASH)

**KOMPASS_BASIC_AUTH_HASH**: bcrypt hash of password (e.g., `$2a$12$...`). Salt is embedded in hash.

Optional: `KOMPASS_REQUIRE_SECURE_CONNECTION=true` (recommended for non-localhost)

## Behavior

- **localhost:8080 (default)**: no auth middleware
- **0.0.0.0 without mode set**: error on startup
- **0.0.0.0 + OIDC mode**: redirect browser to login, return 401 for API
- **0.0.0.0 + Basic mode**: require Authorization header on all requests
- **OIDC session**: secure httpOnly cookie after callback
- All authenticated users currently see the same cluster view (no per-user filtering)

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

- Auth middleware on routes
- OIDC login/callback/logout
- Basic Auth fallback
- 401/403 responses
- Secret value redaction in API output
