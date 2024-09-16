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

Create a pull request with defaults:

```bash
{{EXAMPLE_REVIEW_DEFAULT}}
```

Create with template overrides:

```bash
{{EXAMPLE_REVIEW_TEMPLATE_OVERRIDES}}
```

Search reviewers, assignees, and labels before review:

```bash
{{EXAMPLE_SEARCH_REVIEWERS}}
{{EXAMPLE_SEARCH_ASSIGNEES}}
{{EXAMPLE_SEARCH_LABELS}}
```

Create with reviewers/assignees:

```bash
{{EXAMPLE_REVIEW_WITH_PARTICIPANTS}}
```

## Supporting Commands

- `fotingo open issue` to open current-branch issue URL.
- `fotingo open pr` to open current-branch PR URL.

## Guidance

- Prefer non-interactive flags (`-y`, `--json`) in automated runs.
- Use explicit flags rather than prompts in non-interactive environments.
- For reviewers, assignees, and labels, run `fotingo search ... --json` first and pass the resolved values into `fotingo review`.
- Use `fotingo --help` (and `<command> --help`) to discover additional workflow actions.
- If required data is missing, run inspect and retry with explicit values.

## Ticket And PR Etiquette

- Write ticket titles as clear outcomes (what changes and where), not vague placeholders.
- Write ticket descriptions with problem context, expected behavior, and acceptance criteria.
- Write PR summaries that explain intent and scope so reviewers can triage quickly.
- Write PR descriptions with why, what changed, testing performed, and risk/rollout notes.
