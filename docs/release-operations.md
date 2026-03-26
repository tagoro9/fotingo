# Release Operations

This document is the maintainer runbook for snapshot builds, prereleases, and stable releases.

## Prerequisites

You need:

- `goreleaser` v2+
- GitHub repository write access
- Access to repository secrets for release workflows

## Required Secrets

The release and prerelease workflows require the following secrets:

- `HOMEBREW_TAP_GITHUB_TOKEN`: PAT with write access to `tagoro9/homebrew-tap`
- `FOTINGO_JIRA_OAUTH_CLIENT_ID`: Jira OAuth client ID injected into binaries
- `FOTINGO_JIRA_OAUTH_CLIENT_SECRET`: Jira OAuth client secret injected into binaries
- `FOTINGO_GH_OAUTH_CLIENT_ID`: GitHub OAuth client ID injected into binaries

`GITHUB_TOKEN` is provided automatically by GitHub Actions and is used for publishing GitHub Releases.

## Local Snapshot Build

Validate configuration and build a host-target snapshot:

```bash
goreleaser check --config .goreleaser.yaml
goreleaser build --single-target --snapshot --clean --config .goreleaser.yaml
```

Or use:

```bash
script/build
```

GoReleaser publishes a single `fotingo` CLI artifact compiled with
`-tags fotingo_org_only_participants`, so participant resolution uses only
organization members and teams by default in release binaries.

## Release Triggers

- Prerelease: pull request events (`opened`, `labeled`, `synchronize`) when the PR has label `prerelease`
- Stable release: pushes to `main`, pushes to branches matching `v*`, or manual `workflow_dispatch`

Workflows:

- `prerelease.yaml` runs on PR activity and only executes jobs when label `prerelease` is present
- `release.yaml` runs on pushes to `main`/`v*` branches and manual dispatch

## Publishing a Prerelease

Add the `prerelease` label to the pull request and push commits as needed.

The `Prerelease` workflow publishes GitHub prerelease assets and updates the Homebrew formula.

## Publishing a Stable Release

Push to `main` (or dispatch the workflow manually) and the `Release` workflow performs the release flow.

## Injected Build Metadata

GoReleaser injects this metadata into `pkg/commands`:

- `Version={{.Version}}`
- `GitCommit={{.ShortCommit}}`
- `BuildTime={{.CommitDate}}`
- `Platform={{.Os}}/{{.Arch}}`

## Troubleshooting

### `goreleaser check` fails

- Ensure `.goreleaser.yaml` is valid YAML.
- Ensure templates and ldflags entries reference valid package symbols.

### Homebrew update fails

- Confirm `HOMEBREW_TAP_GITHUB_TOKEN` has write access to `tagoro9/homebrew-tap`.
- Confirm `tagoro9/homebrew-tap` has a default branch named `main`.

### OAuth values are empty in binary

- Confirm all OAuth secrets are configured in workflow settings.
- Confirm ldflags keys in `.goreleaser.yaml` match variable names in:
  - `internal/jira/oauth.go`
  - `internal/github/github.go`
