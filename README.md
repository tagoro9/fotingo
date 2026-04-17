# Fotingo

A CLI to streamline workflows across Git, GitHub, and Jira.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Overview

Fotingo helps you:

- Start working on Jira issues with consistent branch naming
- Create pull requests with issue context and metadata
- Manage stacked pull request chains, including worktree-aware rebases
- Open related URLs for branch, issue, PR, and repository
- Inspect local branch/issue context as JSON
- Automate command flows with machine-readable output

## Installation

### From source

```bash
go install github.com/tagoro9/fotingo@latest
```

### From binary

Download the latest release from the [releases page](https://github.com/tagoro9/fotingo/releases).

### Homebrew

```bash
brew tap tagoro9/tap
brew install fotingo
xattr -dr com.apple.quarantine /opt/homebrew/bin/fotingo
```

On macOS, Homebrew can preserve the `com.apple.quarantine` attribute on downloaded binaries. Because `fotingo` is a standalone CLI binary distributed outside the App Store, Gatekeeper may block execution until that quarantine attribute is removed. Running `xattr -dr com.apple.quarantine /opt/homebrew/bin/fotingo` clears the attribute so the binary can run normally.

## Quick Start

Prerequisites:

1. GitHub authentication:
   - Fotingo GitHub App installed in the orgs you want to access (it can be installed during the auth flow), or
   - A classic GitHub PAT from `https://github.com/settings/tokens` with `repo` scope
2. Jira authentication:
   - Atlassian API token from `https://id.atlassian.com/manage-profile/security/api-tokens`, or
   - OAuth only in internal binaries compiled with Jira OAuth client credentials
3. Jira account email
4. Jira server URL (for example `https://yourcompany.atlassian.net`)

Jira OAuth client credentials include a client secret and are intended for internal builds only.
Committing or broadly distributing binaries with embedded Jira OAuth client secret is not considered safe.

Basic flow:

```bash
# Authenticate services
fotingo login

# Start work on an issue
fotingo start PROJ-123

# Create a pull request for current branch
fotingo review -y

# Create a child PR stacked on an open parent PR branch
fotingo review -y --branch feature/PROJ-122-parent

# Inspect and refresh the current stacked PR chain
fotingo review stacks
fotingo review stacks sync

# Inspect pull request comments and review conversations
fotingo inspect pr --json

# Refresh selected PR sections or metadata after follow-up changes
fotingo review sync --section changes --section fixed-issues -y

# Open the PR in browser
fotingo open pr
```

For full authentication setup details, see [docs/authentication.md](./docs/authentication.md).

## Common Commands

| Command                 | What it does                                                                                 |
| ----------------------- | -------------------------------------------------------------------------------------------- |
| `fotingo start`         | Creates or checks out a Jira-backed branch, optionally in a dedicated worktree               |
| `fotingo review`        | Creates a pull request with issue context, labels, reviewers, and optional base overrides    |
| `fotingo review sync`   | Refreshes managed PR sections, reviewers, assignees, title, and draft readiness              |
| `fotingo review stacks` | Lists, syncs, and rebases linear stacked PR chains, including branches in separate worktrees |
| `fotingo inspect`       | Prints branch, issue, commit, and pull request metadata for automation                       |
| `fotingo inspect pr`    | Prints PR comments, submitted reviews, and grouped inline review conversations               |
| `fotingo open`          | Opens the related issue, PR, branch, repository, or release page                             |
| `fotingo search`        | Resolves reviewers, assignees, labels, projects, and issue types before command automation   |
| `fotingo config`        | Reads and writes local configuration such as Jira root and worktree parent paths             |
| `fotingo completion`    | Generates shell completion scripts                                                           |

For flags, JSON contracts, and examples, see the [CLI Reference](./docs/cli-reference.md) and [Automation and JSON](./docs/automation-and-json.md) docs.

## Telemetry

Fotingo emits anonymous product telemetry to understand command usage, latency, and failures.

- Enabled by default (`telemetry.enabled: true`)
- Opt out anytime:

```bash
fotingo config set telemetry.enabled false
```

- Telemetry never sends raw tokens, freeform descriptions/titles, branch names, issue IDs, or raw API URLs.

See [docs/telemetry.md](./docs/telemetry.md) for event categories and privacy constraints.

## Documentation

User and maintainer docs live in [`docs/`](./docs/README.md):

- [Breaking Changes v5](./docs/breaking-changes/v5.md)
- [Authentication](./docs/authentication.md)
- [CLI Reference](./docs/cli-reference.md)
- [Configuration](./docs/configuration.md)
- [Telemetry](./docs/telemetry.md)
- [Automation and JSON](./docs/automation-and-json.md)
- [Shell Completion](./docs/shell-completion.md)
- [Exit Codes](./docs/exit-codes.md)
- [Release Operations](./docs/release-operations.md)
- [Homebrew Tap Setup](./docs/homebrew-tap-setup.md)

## Why Fotingo?

Jira-backed development often repeats the same sequence:

1. Pick/assign an issue
2. Move it to `In Progress`
3. Create a correctly named branch
4. Implement and commit
5. Open and enrich a PR
6. Move issue to `In Review` and add PR link

Fotingo turns this into a small set of consistent commands.

## What is a Fotingo?

In Canary Islands Spanish, "fotingo" means an old, rickety car. One origin story links it to Ford's "foot 'n go" phrase from the Model T era. The name fits the CLI goal: minimal friction to get moving.

## Contributing

Contributions are welcome. Open an issue or submit a pull request.

## License

MIT License. See [LICENSE](LICENSE).
