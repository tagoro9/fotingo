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

## Start Workflows

Start from an existing issue:

```bash
{{EXAMPLE_START_EXISTING_ISSUE}}
```

Create and start a new issue:

```bash
{{EXAMPLE_START_CREATE_ISSUE}}
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

## Supporting Commands

- `fotingo open issue` to open current-branch issue URL.
- `fotingo open pr` to open current-branch PR URL.

## Workflow Guide

- Start from `fotingo inspect --json` when branch or issue context is unclear.
- Use `fotingo start ... -y` to begin work from an existing issue or a newly created issue.
- Prefer non-interactive flags (`-y`, `--json`) in automated runs.
- Use explicit flags rather than prompts in non-interactive environments.
- For reviewers, assignees, and labels, run `fotingo search ... --json` first and pass the resolved values into `fotingo review`.
- Prefer `fotingo review -y` for the standard Jira-backed flow. Use `fotingo review -y --simple` only when you intentionally want a GitHub-only PR flow.
- Prefer `--template-summary` and `--template-description` because they keep the default PR layout while filling the `Summary` and `Description` sections. `--template-description` expands escaped `\n`, `\r\n`, and `\t`.
- Use `--description -` when you need to replace the entire PR body instead of filling template placeholders.
- Use `--title` only when the generated PR title is wrong or incomplete.
- Use `fotingo --help` (and `<command> --help`) to discover additional workflow actions.
- If required data is missing, run inspect and retry with explicit values.

## Ticket And PR Etiquette

- Write ticket titles as clear outcomes (what changes and where), not vague placeholders.
- Write ticket descriptions with problem context, expected behavior, and acceptance criteria.
- Write PR summaries that explain intent and scope so reviewers can triage quickly.
- Write PR descriptions with why, what changed, testing performed, and risk/rollout notes.
