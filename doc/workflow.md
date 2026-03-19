# Workflow Spec

## Scope

Files:
1. .github/workflows/ci.yml
2. .github/workflows/release.yml
3. .github/workflows/docker.yml
4. .github/workflows/goreleaser.yml

## Responsibilities

1. release.yml: create and optionally push next tag, build and optionally push Docker image.
2. docker.yml: build and push Docker image for a provided or pushed tag.
3. goreleaser.yml: build and optionally publish GoReleaser artifacts and Homebrew.
4. ci.yml: run code checks and workflow validation.

## Paths

### Golden path: PR merge to main

1. PR opens with label: release:patch | release:minor | release:major.
2. PR merges to main.
3. release.yml auto-runs on push to main.
4. Parses PR labels or commits since last tag (conventional commits fallback).
5. Computes next version, creates tag vX.Y.Z, pushes.
6. Tag push auto-triggers docker.yml and goreleaser.yml.

### Fallback: manual release

1. release.yml workflow_dispatch with manual bump input.
2. Same behavior as golden path.
3. Use when golden path label is missed.

### Emergency: re-release latest tag

1. re-release.yml workflow_dispatch.
2. Defaults to latest v* tag (non-interactive).
3. Re-publishes docker and goreleaser artifacts.
4. No tag creation, no version bump.
5. Use for hotfixed code landed in main after tag push.

## Triggers and Inputs

release.yml:
1. triggers:
   1. push to main (golden path)
   2. workflow_dispatch (manual fallback)
2. inputs (manual only):
   1. bump: patch | minor | major

re-release.yml:
1. trigger: workflow_dispatch
2. inputs: none (defaults to latest tag)

docker.yml:
1. triggers:
	1. push tags v*
	2. workflow_dispatch
2. input (manual):
	1. tag (required, vX.Y.Z)

goreleaser.yml:
1. triggers:
	1. push tags v*
	2. workflow_dispatch
2. inputs (manual):
	1. tag (required, vX.Y.Z)
	2. publish: false | true (default false)

ci.yml:
1. triggers:
	1. push branches **
	2. workflow_dispatch

## Publish Rules

1. Golden path (push to main): auto-publish Docker and trigger GoReleaser publish.
2. Manual release: auto-publish Docker and trigger GoReleaser publish.
3. Re-release: always publishes Docker and GoReleaser (no skip option).
4. Tag push events from release.yml auto-trigger docker.yml and goreleaser.yml.

## Required Secrets

release.yml / re-release.yml publish:
1. DOCKERHUB_USERNAME
2. DOCKERHUB_TOKEN

re-release.yml (also needs):
1. HOMEBREW_TAP_GITHUB_TOKEN

goreleaser.yml with publish=true:
1. HOMEBREW_TAP_GITHUB_TOKEN

docker.yml publish:
1. DOCKERHUB_USERNAME
2. DOCKERHUB_TOKEN

## Local Validation

Required before workflow PR merge:
1. make wf-lint
2. make wf-plan-release
3. make wf-plan-goreleaser
4. make wf-plan-rerelease

Optional deeper checks:
1. make wf-test-release
2. make wf-test-goreleaser
3. make wf-test-rerelease

## Release Runbook

Golden path (automatic on PR merge):
1. Add label release:patch|minor|major to PR.
2. Merge PR to main.
3. release.yml auto-runs, parses label or commits.
4. Tag created and pushed -> docker.yml and goreleaser.yml auto-run.
5. Check summaries: Docker published, GoReleaser/Homebrew published.

Manual fallback (rare):
1. Run release.yml workflow_dispatch with bump input.
2. Same publish flow as golden path.

Emergency re-release (hotfix after tag):
1. Run re-release.yml workflow_dispatch.
2. Defaults to latest v* tag (non-interactive).
3. Docker and GoReleaser/Homebrew re-published for that tag.

## Invariants

1. Golden path (push to main) always publishes when version detected.
2. Manual release requires explicit bump input.
3. Re-release always publishes latest tag (no skip option).
4. release.yml does not run GoReleaser directly.
5. All Docker/GoReleaser publish happens via tag-triggered workflows.
