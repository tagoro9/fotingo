# CLI Reference

## Commands

### `login`

Authenticate GitHub and Jira credentials.

```bash
fotingo login
```

Notes:

- GitHub OAuth requires the Fotingo GitHub App to be installed in the orgs you want to access, and it can be installed during the auth flow.
- GitHub token auth expects a classic PAT with `repo` scope from `https://github.com/settings/tokens`.
- Jira token auth expects an Atlassian API token from `https://id.atlassian.com/manage-profile/security/api-tokens`.
- Jira OAuth requires binaries compiled with Jira OAuth client credentials and is intended for internal builds only.
- Committing or broadly distributing binaries with embedded Jira OAuth client secret is not considered safe.

### `ai setup`

Install fotingo agent skills for supported AI providers (`cursor`, `codex`, `claude-code`).

```bash
fotingo ai setup [flags]
```

Examples:

```bash
# Select providers interactively (TTY)
fotingo ai setup

# Install selected providers at project scope
fotingo ai setup --agent codex --agent cursor --scope project

# Install all providers for user scope
fotingo ai setup --all --scope user

# Preview without writing files
fotingo ai setup --all --dry-run
```

Flags:

| Flag        | Description                                                        |
| ----------- | ------------------------------------------------------------------ |
| `--agent`   | Provider to install (repeatable: `cursor`, `codex`, `claude-code`) |
| `--all`     | Install all supported providers                                    |
| `--scope`   | Installation scope: `project` or `user`                            |
| `--dry-run` | Show planned writes without modifying files                        |
| `--force`   | Overwrite existing generated skill files                           |

### `start`

Start working on a Jira issue by setting it to `In Progress` and creating a feature branch.

```bash
fotingo start [issueId] [flags]
```

Examples:

```bash
# Start working on an existing issue
fotingo start PROJ-123

# Select from your open issues interactively
fotingo start

# Create a new issue and start working on it
fotingo start -t "Fix login bug" -p PROJ -k Bug

# Create a sub-task under a parent issue
fotingo start -t "Implement feature" -p PROJ -a PROJ-100

# Start an issue without creating a branch
fotingo start PROJ-123 --no-branch

# Start in a new sibling worktree
fotingo start PROJ-123 --worktree

# Start in non-interactive mode
fotingo start PROJ-123 -y
```

Flags:

| Flag            | Short | Description                                         |
| --------------- | ----- | --------------------------------------------------- |
| `--title`       | `-t`  | Create a new issue with this title                  |
| `--description` | `-d`  | Description for new issue                           |
| `--project`     | `-p`  | Project key for new issue (required with `--title`) |
| `--kind`        | `-k`  | Issue type: Story, Bug, Task, SubTask, Epic         |
| `--parent`      | `-a`  | Parent issue for sub-tasks                          |
| `--epic`        | `-e`  | Epic issue key to link                              |
| `--labels`      | `-l`  | Labels to add (repeatable)                          |
| `--no-branch`   | `-n`  | Set issue status without creating/switching branch  |
| `--worktree`    |       | Create the issue branch in a new sibling worktree   |
| `--interactive` | `-i`  | Interactive create flow                             |

### `review`

Create a pull request for the current branch.

```bash
fotingo review [flags]
```

Examples:

```bash
# Create a PR with Jira integration
fotingo review

# Draft PR
fotingo review --draft

# Skip Jira integration
fotingo review --simple

# Add labels/reviewers/assignees
fotingo review -l bug -r alice -a bob

# Fill the default Summary and Description sections
fotingo review --template-summary "Fix auth bug" --template-description "Why: clearer auth failures.\n\nWhat changed:\n- improve copy\n- add telemetry"

# Replace the full PR body from stdin
printf '## Summary\n\nFix auth bug\n\n## Description\n\nCustom reviewer notes.\n' | fotingo review --description -

# Custom title
fotingo review --title "Fix auth bug"
```

Notes:

- Use `--template-summary` and `--template-description` to fill the default PR template sections.
- `--template-description` expands escaped `\n`, `\r\n`, and `\t`, which makes multiline scripted descriptions reliable.
- Use `--description` when you want to replace the entire PR body instead of filling template placeholders.
- Resolve reviewers, assignees, and labels ahead of time with `fotingo search ... --json` when scripting review creation.

Flags:

| Flag                     | Short | Description                                                          |
| ------------------------ | ----- | -------------------------------------------------------------------- |
| `--draft`                | `-d`  | Create a draft pull request                                          |
| `--labels`               | `-l`  | Labels to add (repeatable)                                           |
| `--reviewers`            | `-r`  | Reviewers to request (repeatable)                                    |
| `--assignee`             | `-a`  | Assignees to add (repeatable)                                        |
| `--simple`               | `-s`  | Skip Jira integration and create a GitHub-only PR                    |
| `--title`                |       | Override the generated PR title                                      |
| `--description`          |       | Override the entire PR body (`-` to read stdin)                      |
| `--template-summary`     |       | Override the default `Summary` section placeholder                   |
| `--template-description` |       | Override the default `Description` section; expands escaped newlines |

### `open`

Open project-related URLs in your browser.

```bash
fotingo open [branch|issue|pr|repo]
```

Examples:

```bash
fotingo open branch
fotingo open issue
fotingo open pr
fotingo open repo
fotingo open pr --json
```

### `inspect`

Output execution context as JSON.

```bash
fotingo inspect [flags]
```

Flags:

| Flag       | Short | Description               |
| ---------- | ----- | ------------------------- |
| `--branch` | `-b`  | Inspect a specific branch |
| `--issue`  | `-i`  | Inspect a specific issue  |

### `completion`

Generate shell completion scripts.

```bash
fotingo completion [bash|zsh|fish|powershell]
```

## Global Flags

| Flag         | Short | Description                       |
| ------------ | ----- | --------------------------------- |
| `--branch`   | `-b`  | Base branch override              |
| `--yes`      | `-y`  | Accept defaults without prompting |
| `--json`     |       | Output machine-readable JSON      |
| `--quiet`    |       | Suppress non-essential output     |
| `--verbose`  | `-v`  | Include verbose logs              |
| `--debug`    |       | Include debug logs                |
| `--no-color` |       | Disable ANSI colors               |
