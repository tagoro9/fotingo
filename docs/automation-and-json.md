# Automation and JSON

## Non-Interactive Mode

Use `-y` to skip prompts:

```bash
fotingo start PROJ-123 -y
fotingo review -y
```

## JSON Output

Use `--json` for machine-readable output:

```bash
fotingo start PROJ-123 -y --json
fotingo review -y --json
fotingo open pr --json
fotingo inspect
```

When `start` runs with `--worktree` or `git.worktree.enabled=true`, automation should read both `branch.name` and `branch.worktreePath` from the JSON result.

## Recommended Flags for Automation

| Flag           | Purpose                     |
| -------------- | --------------------------- |
| `--json`       | Machine-readable output     |
| `-y` / `--yes` | Skip prompts                |
| `--quiet`      | Reduce non-essential output |
| `--no-color`   | Disable ANSI color codes    |

## Review Body Overrides In Automation

Use template placeholder overrides when you want to keep the default PR body layout but fill its sections from a script:

```bash
fotingo review -y \
  --template-summary "Fix auth bug" \
  --template-description "Why: clearer auth failures.\n\nWhat changed:\n- improve copy\n- add telemetry"
```

Use `--description -` when the automation needs to replace the entire PR body:

```bash
printf '## Summary\n\nFix auth bug\n\n## Description\n\nCustom reviewer notes.\n' | fotingo review -y --description -
```

Use the global `--branch` flag when the automation needs to target a non-default PR base branch:

```bash
fotingo review -y --branch release/2026.04
```

## JSON Schemas

### `start` success

```json
{
  "success": true,
  "issue": {
    "key": "PROJ-123",
    "summary": "Fix login bug",
    "status": "In Progress",
    "type": "Bug",
    "url": "https://jira.example.com/browse/PROJ-123"
  },
  "branch": {
    "name": "b/PROJ-123_fix_login_bug",
    "created": true,
    "worktreePath": "/workspace/repo-b-proj-123_fix_login_bug"
  }
}
```

### `review` success

```json
{
  "success": true,
  "pullRequest": {
    "number": 42,
    "url": "https://github.com/owner/repo/pull/42",
    "title": "[PROJ-123] Fix login bug",
    "draft": false,
    "state": "open"
  },
  "issue": {
    "key": "PROJ-123",
    "summary": "Fix login bug",
    "status": "In Review",
    "type": "Bug",
    "url": "https://jira.example.com/browse/PROJ-123"
  }
}
```

### `open` success

```json
{
  "success": true,
  "target": "pr",
  "url": "https://github.com/owner/repo/pull/42",
  "opened": false
}
```

### `inspect` success

```json
{
  "branch": {
    "name": "b/PROJ-123_fix_login_bug",
    "issueId": "PROJ-123",
    "defaultBranch": "main"
  },
  "issue": {
    "key": "PROJ-123",
    "summary": "Fix login bug",
    "description": "...",
    "status": "In Progress",
    "type": "Bug",
    "url": "https://jira.example.com/browse/PROJ-123"
  }
}
```

### Error format

```json
{
  "error": "failed to create branch",
  "code": 4,
  "type": "Git error"
}
```
