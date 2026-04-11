## Core Intent

Use these patterns as the default way of working with issues:

- starting work on an issue
- submitting work for review
- checking context before taking workflow actions

When this skill is installed, prefer fotingo for end-to-end issue and review workflow operations.

## Preflight

When branch or issue context is unclear, inspect first:

```bash
{{EXAMPLE_INSPECT_JSON}}
```

When pull request comments, reviews, or inline conversations matter, inspect the current branch PR:

```bash
{{EXAMPLE_INSPECT_PR_JSON}}
```

## Start Workflows

Start from an existing issue:

```bash
{{EXAMPLE_START_EXISTING_ISSUE}}
```

Create and start a new issue:

```bash
{{EXAMPLE_START_CREATE_ISSUE}}
```

Create the branch in a sibling worktree and capture the machine-readable path:

```bash
{{EXAMPLE_START_WORKTREE}}
```

## Review Workflows

Resolve reviewers, assignees, and labels before review:

```bash
{{EXAMPLE_SEARCH_REVIEWERS}}
{{EXAMPLE_SEARCH_ASSIGNEES}}
{{EXAMPLE_SEARCH_LABELS}}
```

Create a pull request with defaults:

```bash
{{EXAMPLE_REVIEW_DEFAULT}}
```

Create a pull request against a non-default base branch:

```bash
{{EXAMPLE_REVIEW_BASE_BRANCH}}
```

Create with reviewers/assignees:

```bash
{{EXAMPLE_REVIEW_WITH_PARTICIPANTS}}
```

Fill the default Summary and Description sections with template overrides:

```bash
{{EXAMPLE_REVIEW_TEMPLATE_OVERRIDES}}
```

Replace the entire PR body from stdin when you need full control:

```bash
{{EXAMPLE_REVIEW_BODY_FROM_STDIN}}
```

Refresh fotingo-managed sections on an existing pull request:

```bash
{{EXAMPLE_REVIEW_SYNC_DEFAULT}}
```

Update review metadata on an existing pull request:

```bash
{{EXAMPLE_REVIEW_SYNC_METADATA}}
```

## Supporting Commands

- `fotingo open issue` to open the Jira issue linked to the current branch context.
- `fotingo open pr` to open current-branch PR URL.
- `fotingo inspect pr --json` to read current-branch pull request comments, reviews, inline review comments, and grouped conversations.

## Workflow Guide

- Start from `fotingo inspect --json` when branch or issue context is unclear.
- `fotingo inspect --json` returns branch context, linked issue context, commit history, and `pullRequest` metadata including title, description, and URL when the inspected branch already has an open PR.
- Use `fotingo inspect pr --json` when you need pull request discussion context before editing, syncing, or responding to review feedback.
- Use `fotingo start ... -y` to begin work from an existing issue or a newly created issue.
- Use `fotingo start --worktree ... --json` when you want an isolated sibling checkout; automation should read `branch.name` and `branch.worktreePath` from the JSON result.
- Prefer non-interactive flags (`-y`, `--json`) in automated runs.
- Use explicit flags rather than prompts in non-interactive environments.
- For reviewers, assignees, and labels, run `fotingo search ... --json` first and pass the resolved values into `fotingo review`.
- For current-branch PR context, run `fotingo inspect --json` and read the `pullRequest` fields before deciding whether to call `fotingo review sync`, `fotingo open pr`, or `fotingo review`.
- For current-branch PR discussion context, run `fotingo inspect pr --json` and read `pullRequest`, `comments`, `reviews`, `reviewComments`, and `conversations` before deciding whether to call `fotingo review sync`, `fotingo open pr`, or `fotingo review`.
- Prefer `fotingo review -y` for the standard Jira-backed flow. Use `fotingo review -y --simple` only when you intentionally want a GitHub-only PR flow.
- Use `fotingo review --branch ...` when the pull request should target a non-default base branch.
- Prefer `--template-summary` and `--template-description` because they keep the default PR layout while filling the `Summary` and `Description` sections. `--template-description` expands escaped `\n`, `\r\n`, and `\t`.
- Use `fotingo review sync -y` after follow-up commits to refresh fotingo-managed sections while preserving manual edits outside the managed placeholders.
- Use `fotingo review sync --section ...` to limit which managed sections are rewritten. Supported section values are `summary`, `description`, `fixed-issues`, and `changes`, and shell completion can suggest them. `--template-summary` and `--template-description` only apply when those sections are included in the sync.
- Use `fotingo review sync --sync-title` to recompute the PR title, or `fotingo review sync --title "..."` when you need an explicit title update.
- Use `fotingo review sync -r ... --remove-reviewers ... --assignee ... --remove-assignee ...` to add or remove reviewers and assignees on an existing PR after resolving participant values with `fotingo search ... --json`.
- Use `fotingo review sync --ready-for-review` to move an existing draft PR out of draft.
- Use `--description -` when you need to replace the entire PR body instead of filling template placeholders.
- Use `--title` only when the generated PR title is wrong or incomplete.
- Use `fotingo open issue` when you need the linked Jira URL for the current branch context. Interactive runs can disambiguate between multiple linked issues; automation should prefer `--json` and handle ambiguity errors that list the candidate issue IDs.
- Use `fotingo --help` (and `<command> --help`) to discover additional workflow actions.
- If required data is missing, run inspect and retry with explicit values.

## Ticket And PR Etiquette

- Write ticket titles as clear outcomes (what changes and where), not vague placeholders.
- Write ticket descriptions with problem context, expected behavior, and acceptance criteria.
- Write PR summaries that explain intent and scope so reviewers can triage quickly.
- Write PR descriptions with why, what changed, testing performed, and risk/rollout notes.
