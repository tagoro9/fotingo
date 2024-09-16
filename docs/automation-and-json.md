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

## Recommended Flags for Automation

| Flag           | Purpose                     |
| -------------- | --------------------------- |
| `--json`       | Machine-readable output     |
| `-y` / `--yes` | Skip prompts                |
| `--quiet`      | Reduce non-essential output |
| `--no-color`   | Disable ANSI color codes    |

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
    "created": true
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
