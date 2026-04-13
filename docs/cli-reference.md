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

# Start in a worktree under a specific parent directory
fotingo start PROJ-123 --worktree-path .claude/worktrees

# Start in non-interactive mode
fotingo start PROJ-123 -y
```

Flags:

| Flag              | Short | Description                                         |
| ----------------- | ----- | --------------------------------------------------- |
| `--title`         | `-t`  | Create a new issue with this title                  |
| `--description`   | `-d`  | Description for new issue                           |
| `--project`       | `-p`  | Project key for new issue (required with `--title`) |
| `--kind`          | `-k`  | Issue type: Story, Bug, Task, SubTask, Epic         |
| `--parent`        | `-a`  | Parent issue for sub-tasks                          |
| `--epic`          | `-e`  | Epic issue key to link                              |
| `--labels`        | `-l`  | Labels to add (repeatable)                          |
| `--no-branch`     | `-n`  | Set issue status without creating/switching branch  |
| `--worktree`      |       | Create the issue branch in a new sibling worktree   |
| `--worktree-path` |       | Parent directory for the created linked worktree    |
| `--interactive`   | `-i`  | Interactive create flow                             |

Notes:

- In worktree mode, `start` prints the created branch name and worktree folder, and JSON output includes `branch.name` plus `branch.worktreePath`.
- Worktree directory names use the hardcoded `fotingo-wt-<branch>` format.
- `--worktree-path` implies worktree mode and overrides `git.worktree.path` for one invocation.

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

# Target a non-default base branch
fotingo review --branch release/2026.04

# Create a child PR stacked on an open parent PR branch
fotingo review --branch feature/PROJ-122-parent

# Inspect the current stack
fotingo review stacks

# Refresh stacked PR sections across the current stack
fotingo review stacks sync

# Rebase stack branches in their local worktrees
fotingo review stacks rebase

# Rebase and push rewritten stack branches with force-with-lease
fotingo review stacks rebase --push

# Sync only the changes and fixed-issues sections on an existing PR
fotingo review sync --section changes --section fixed-issues -y

# Update review metadata on an existing PR
fotingo review sync -r alice --remove-reviewers team/platform -a bob --remove-assignee carol --ready-for-review -y
```

Notes:

- Use the global `--branch` / `-b` flag to override the pull request base branch when the PR should target something other than the repository default branch.
- When `--branch` targets a branch that already has an open PR, `review` treats the new PR as a stacked child, creates or reuses stack metadata, and updates the `Stacked PRs` section across the open stack.
- Use `--template-summary` and `--template-description` to fill the default PR template sections.
- `--template-description` expands escaped `\n`, `\r\n`, and `\t`, which makes multiline scripted descriptions reliable.
- Use `--description` when you want to replace the entire PR body instead of filling template placeholders.
- Use `fotingo review sync --section ...` when you only want to refresh specific managed PR sections. Supported section values are `summary`, `description`, `fixed-issues`, and `changes`; shell completion suggests them.
- Use `fotingo review sync -r ... --remove-reviewers ... -a ... --remove-assignee ...` to update reviewers and assignees on an existing PR.
- Use `fotingo review sync --ready-for-review` to move an existing draft PR to ready for review without recreating it.
- Resolve reviewers, assignees, and labels ahead of time with `fotingo search ... --json` when scripting review creation.
- Stacked PR sections render Jira keys and PR links. The current PR is marked after the order number with `👉`.

#### `review stacks`

Inspect and manage the stacked pull request chain for the current branch.

```bash
fotingo review stacks [command] [flags]
```

Subcommands:

| Command                               | Description                                                                                |
| ------------------------------------- | ------------------------------------------------------------------------------------------ |
| `fotingo review stacks`               | Print the current branch's stack members in root-to-leaf order, marking the current PR     |
| `fotingo review stacks sync`          | Recompute and update the deterministic stacked PR section for every open PR in the stack   |
| `fotingo review stacks rebase`        | Rebase local stack branches in order, using each branch's existing worktree when available |
| `fotingo review stacks rebase --push` | Push rebased stack branches with `--force-with-lease` after successful local rebases       |

Notes:

- Stack commands default to the stack that contains the current branch's open PR.
- `review stacks` prints a terminal table with clickable Jira and pull request labels in terminals that support links, not Markdown table syntax.
- `review stacks sync` does not open an editor; it only rewrites the marker-owned `stacked-prs` section.
- `review stacks rebase` requires every branch that will be rebased to have a clean local worktree and stops at the first rebase conflict.
- Branches in separate linked worktrees are supported; fotingo discovers them with Git worktree metadata and runs each rebase in that branch's worktree.
- Branching stacks are not supported in this iteration. If one PR branch has multiple stack children, fotingo fails before updating PR bodies.

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

`fotingo open issue` resolves Jira issues from the current branch context. If the branch
name includes an issue key, that key is preferred first, and commit-linked issue keys from
the branch are also considered. Interactive runs will prompt when multiple linked issues are
found; `--json` and other non-interactive runs return an ambiguity error listing the candidates.

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
fotingo inspect pr [flags]
```

Examples:

```bash
fotingo inspect
fotingo inspect --branch feature/PROJ-123-my-feature
fotingo inspect --issue PROJ-123
fotingo inspect pr --json
fotingo inspect pr --branch feature/PROJ-123-my-feature --json
```

Branch inspection now includes pull request metadata when the inspected branch already has an
open PR, including the PR title, description, and URL, so automation can read branch, issue,
commit, and PR context from a single JSON call.

Flags:

| Flag       | Short | Description               |
| ---------- | ----- | ------------------------- |
| `--branch` | `-b`  | Inspect a specific branch |
| `--issue`  | `-i`  | Inspect a specific issue  |

`fotingo inspect pr` returns the open pull request for the inspected branch plus top-level comments and submitted reviews with their grouped inline conversations. It supports `--branch`/`-b` and returns a JSON error when the branch has no open pull request.

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
