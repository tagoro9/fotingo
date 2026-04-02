# Configuration

## Environment Variables

| Variable                  | Description                                                       |
| ------------------------- | ----------------------------------------------------------------- |
| `FOTINGO_JIRA_ROOT`       | Jira server URL (for example `https://yourcompany.atlassian.net`) |
| `FOTINGO_JIRA_USER_LOGIN` | Jira username (email)                                             |
| `FOTINGO_JIRA_USER_TOKEN` | Jira API token                                                    |
| `FOTINGO_GIT_REMOTE`      | Git remote name (default: `origin`)                               |
| `GITHUB_TOKEN`            | GitHub classic PAT with `repo` scope                              |

If `jira.root` / `FOTINGO_JIRA_ROOT` is not set, interactive Jira-backed commands prompt for it and persist it.

## Configuration File

Fotingo resolves config from:

1. `.fotingo.yaml` in the current directory
2. `~/.config/fotingo/config.yaml` as the canonical user config file

Example:

```yaml
git:
  remote: origin
  branchTemplate: "{{.Issue.ShortName}}/{{.Issue.Info}}_{{.Issue.SanitizedSummary}}"
  worktree:
    enabled: false

github:
  token: ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

jira:
  root: https://yourcompany.atlassian.net
```

## Key Properties

| Path                            | Description                                    |
| ------------------------------- | ---------------------------------------------- |
| `git.branchTemplate`            | Template for branch names                      |
| `git.remote`                    | Git remote name                                |
| `git.worktree.enabled`          | Create `start` branches in sibling worktrees   |
| `github.token`                  | GitHub OAuth token or classic PAT              |
| `github.cache.labelsTTL`        | Labels cache TTL                               |
| `github.cache.collaboratorsTTL` | Collaborators cache TTL                        |
| `github.cache.orgMembersTTL`    | Organization members cache TTL                 |
| `github.cache.teamsTTL`         | Organization teams cache TTL                   |
| `github.cache.userProfilesTTL`  | GitHub user profile cache TTL                  |
| `jira.root`                     | Jira server URL                                |
| `jira.user.login`               | Jira username                                  |
| `jira.user.token`               | Jira API token                                 |
| `jira.cache.issueTypesTTL`      | Jira issue types cache TTL                     |
| `cache.path`                    | Override cache DB path                         |
| `telemetry.enabled`             | Enable anonymous telemetry (`true` by default) |

Jira OAuth site metadata (`siteId`) is derived internally and cached by `jira.root`; it is not a user-managed config key.

## Telemetry Opt-Out

Disable telemetry:

```bash
fotingo config set telemetry.enabled false
```

Re-enable telemetry:

```bash
fotingo config set telemetry.enabled true
```

Token setup references:

- GitHub token auth: create a classic PAT at `https://github.com/settings/tokens` with `repo` scope.
- Jira token auth: create an Atlassian API token at `https://id.atlassian.com/manage-profile/security/api-tokens`.

## Templates

### Branch Template (`git.branchTemplate`)

| Placeholder                   | Description                                 |
| ----------------------------- | ------------------------------------------- |
| `{{.Issue.ShortName}}`        | Issue type short name (`f`, `b`, `t`, etc.) |
| `{{.Issue.Info}}`             | Issue key (`PROJ-123`)                      |
| `{{.Issue.SanitizedSummary}}` | Issue summary sanitized for branch names    |

### Pull Request Template

PR template resolution order:

1. `.github/PULL_REQUEST_TEMPLATE/fotingo.md`
2. Standard GitHub PR template locations (`.github/pull_request_template.md`, `.github/PULL_REQUEST_TEMPLATE.md`, or `pull_request_template.md`)
3. Built-in default template

The default template uses fotingo-managed HTML comment markers for `summary`, `description`, `fixed-issues`, and `changes`, plus the `{fotingo.banner}` placeholder.

Repository templates can use the same marker pairs:

- `<!-- fotingo:start summary -->` / `<!-- fotingo:end summary -->`
- `<!-- fotingo:start description -->` / `<!-- fotingo:end description -->`
- `<!-- fotingo:start fixed-issues -->` / `<!-- fotingo:end fixed-issues -->`
- `<!-- fotingo:start changes -->` / `<!-- fotingo:end changes -->`

Legacy managed placeholders such as `{summary}`, `{description}`, `{fixedIssues}`, and `{changes}` are still supported for backward compatibility, but they are deprecated.
