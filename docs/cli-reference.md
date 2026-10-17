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

| Flag               | Short | Description                                         |
| ------------------ | ----- | --------------------------------------------------- |
| `--title`          | `-t`  | Create a new issue with this title                  |
| `--description`    | `-d`  | Description for new issue                           |
| `--project`        | `-p`  | Project key for new issue (required with `--title`) |
| `--kind`           | `-k`  | Issue type: Story, Bug, Task, SubTask, Epic         |
| `--parent`         | `-a`  | Parent issue for sub-tasks                          |
| `--epic`           | `-e`  | Epic issue key to link                              |
| `--labels`         | `-l`  | Labels to add (repeatable)                          |
| `--tracker-labels` |       | Labels to add to the issue when work starts         |
| `--no-branch`      | `-n`  | Skip branch creation or checkout                    |
| `--worktree`       |       | Create the issue branch in a new sibling worktree   |
| `--worktree-path`  |       | Parent directory for the created linked worktree    |
| `--interactive`    | `-i`  | Interactive create flow                             |

Notes:

- In worktree mode, `start` prints the created branch name and worktree folder, and JSON output includes `branch.name` plus `branch.worktreePath`.
- Worktree directory names use the hardcoded `fotingo-wt-<branch>` format.
- `start` auto-stashes only tracked or staged changes. Untracked-only files and directories stay in place.
- `start` resolves and refreshes only the repository default branch during branch preflight. If the local remote-tracking ref is already current, fotingo skips the fetch; when a refresh is needed, it uses a targeted non-shallow fetch.
- `--worktree-path` implies worktree mode and overrides `git.worktree.path` for one invocation.

### `issue create`

Create tickets without starting work using `fotingo issue create`. This command
requires only Jira access, preserves the initial status and assignment, and does
not apply in-progress labels or create branches:

```bash
fotingo issue create -p PROJ -k Task -t "Plan follow-up" -y --json
```

Optional flags: `--description` (`-d`), `--labels` (`-l`), `--parent` (`-a`), and
`--epic` (`-e`). Parent and epic links use the same resolution as `start`;
`--parent` implies a sub-task. JSON returns `issueKey`, `summary`, `status`,
`type`, and `url` without a success wrapper. Use `start -t ... -p ...` when
creation should also begin work.

### `issue comment`

Post a literal plain-text comment to an explicit Jira issue key. This command uses
only Jira credentials and works outside a Git repository; it does not resolve a
branch, remote, or pull request, or change the issue status.

```bash
fotingo issue comment -i PROJ-123 'A question or progress note'
fotingo issue comment -i PROJ-123 --body-file note.txt --json
fotingo issue comment -i PROJ-123 --body-file - --json < note.txt
```

| Flag          | Short | Description                                        |
| ------------- | ----- | -------------------------------------------------- |
| `--issue`     | `-i`  | Required issue key matching `^[A-Z][A-Z0-9]+-\d+$` |
| `--body-file` |       | Read comment text from a file; `-` reads stdin     |

Supply exactly one text argument or `--body-file`. Empty or whitespace-only bodies
are rejected. Text is literal, including `{{...}}`: it is not rendered as a Go
template or interpreted as Markdown or Jira wiki markup. Blank lines separate ADF
paragraphs, and single newlines are preserved as line breaks.

With `--json`, success returns `issueKey`, `commentId`, and a `url` focused on the
new comment. Failures exit non-zero and return a JSON error. A 403 reports permission
denied; a 404 reports that the issue is missing or not visible to the account.
Invalid keys are rejected before initializing Jira.

### `issue label`

Add labels to an explicit Jira issue key without changing status or posting a
comment. This command also works outside a Git repository and does not interact
with branches or pull requests.

```bash
fotingo issue label -i PROJ-123 --add ai --add backend --json
```

| Flag      | Short | Description                                        |
| --------- | ----- | -------------------------------------------------- |
| `--issue` | `-i`  | Required issue key matching `^[A-Z][A-Z0-9]+-\d+$` |
| `--add`   |       | Label to add; repeat for multiple labels           |

At least one `--add` is required. Labels must be non-empty and contain no whitespace.
Updates are additive and idempotent: existing labels remain, and adding an existing
label is a no-op. JSON success returns `issueKey` and the complete `labels` list
after the update. Key validation and JSON error behavior match `issue comment`.
Use this command for standalone Jira labels; `review --labels` sets GitHub labels,
and `start` or `review --tracker-labels` applies labels during status transitions.

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

# Add workflow labels to linked issues and override the tracker comment
fotingo review --tracker-labels ready-for-review --tracker-comment '{{.Issue.Key}}: {{.PullRequest.URL}}'

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
- When `--branch` targets a branch that already has an open PR, `review` treats the new PR as a stacked child and creates or extends the GitHub-native pull request stack.
- Use `--template-summary` and `--template-description` to fill the default PR template sections.
- `--template-description` expands escaped `\n`, `\r\n`, and `\t`, which makes multiline scripted descriptions reliable.
- Use `--description` when you want to replace the entire PR body instead of filling template placeholders.
- Use `--tracker-labels` for issues that move to `In Review`; `--labels` applies GitHub pull-request labels.
- Use `--tracker-comment` to override the issue-tracker PR-link comment for one review. Pass an empty value only to suppress the comment.
- Comment templates are checked against linked issue data before PR creation or sync updates. A failed status transition does not prevent posting the PR-link comment; labels still require a successful transition. Late rendering failures produce visible warnings.
- Use `fotingo review sync --section ...` when you only want to refresh specific managed PR sections. Supported section values are `summary`, `description`, `fixed-issues`, and `changes`; shell completion suggests them.
- Use `fotingo review sync -r ... --remove-reviewers ... -a ... --remove-assignee ...` to update reviewers and assignees on an existing PR.
- Use `fotingo review sync --ready-for-review` to move an existing draft PR to ready for review without recreating it.
- Resolve reviewers, assignees, and labels ahead of time with `fotingo search ... --json` when scripting review creation.
- Stack membership and ordering are provided by GitHub's native pull request stack API.

#### `review stacks`

Inspect and manage the stacked pull request chain for the current branch.

```bash
fotingo review stacks [command] [flags]
```

Subcommands:

| Command                               | Description                                                                                |
| ------------------------------------- | ------------------------------------------------------------------------------------------ |
| `fotingo review stacks`               | Print the current branch's stack members in root-to-leaf order, marking the current PR     |
| `fotingo review stacks sync`          | Refresh native stack state without editing pull request bodies                             |
| `fotingo review stacks rebase`        | Rebase local stack branches in order, using each branch's existing worktree when available |
| `fotingo review stacks rebase --push` | Push rebased stack branches with `--force-with-lease` after successful local rebases       |

Notes:

- Stack commands default to the stack that contains the current branch's open PR.
- `review stacks` prints a terminal table with clickable Jira and pull request labels in terminals that support links, not Markdown table syntax.
- `review stacks sync` does not open an editor or rewrite pull request bodies.
- `review stacks rebase` requires every branch that will be rebased to have a clean local worktree and stops at the first rebase conflict. If earlier stack PRs have been merged, the remaining PRs are rebased onto the native stack's base branch.
- Branches in separate linked worktrees are supported; fotingo discovers them with Git worktree metadata and runs each rebase in that branch's worktree.
- Branching stacks are not supported in this iteration. If one PR branch has multiple stack children, fotingo fails before updating PR bodies.

Flags:

| Flag                     | Short | Description                                                            |
| ------------------------ | ----- | ---------------------------------------------------------------------- |
| `--draft`                | `-d`  | Create a draft pull request                                            |
| `--labels`               | `-l`  | Labels to add (repeatable)                                             |
| `--tracker-labels`       |       | Labels to add to linked issues moved to In Review                      |
| `--tracker-comment`      |       | Override the issue-tracker comment; pass an empty value to suppress it |
| `--reviewers`            | `-r`  | Reviewers to request (repeatable)                                      |
| `--assignee`             | `-a`  | Assignees to add (repeatable)                                          |
| `--simple`               | `-s`  | Skip Jira integration and create a GitHub-only PR                      |
| `--title`                |       | Override the generated PR title                                        |
| `--description`          |       | Override the entire PR body (`-` to read stdin)                        |
| `--template-summary`     |       | Override the default `Summary` section placeholder                     |
| `--template-description` |       | Override the default `Description` section; expands escaped newlines   |

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
