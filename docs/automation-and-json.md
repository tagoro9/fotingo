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
fotingo review stacks --json
fotingo review stacks sync --json
fotingo open pr --json
fotingo inspect
fotingo inspect pr --json
```

When `start` runs with `--worktree`, `--worktree-path`, or `git.worktree.enabled=true`, automation should read both `branch.name` and `branch.worktreePath` from the JSON result.
`start` auto-stashes only tracked or staged changes; untracked-only files and directories remain untouched.

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

When that base branch already has an open PR, `fotingo review -y --branch <parent-branch>` creates a stacked child PR and updates the stack section across the open stack. To refresh stack sections later without prompting:

```bash
fotingo review stacks sync --json
```

To rebase a stack whose branches may live in separate local worktrees:

```bash
fotingo review stacks rebase --json
fotingo review stacks rebase --push --json
```

Automation should run stack rebase only when it is prepared to handle conflicts. The command requires clean worktrees before starting and stops at the first failed rebase; `--push` is the explicit opt-in for force-with-lease pushes.

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
    "worktreePath": "/workspace/repo/.claude/worktrees/fotingo-wt-b-proj-123_fix_login_bug"
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

### `review stacks` success

```json
{
  "success": true,
  "stack": {
    "stackId": "owner/repo#12",
    "currentBranch": "feature/PROJ-124-leaf",
    "currentPullRequest": 14,
    "members": [
      {
        "number": 12,
        "url": "https://github.com/owner/repo/pull/12",
        "title": "[PROJ-122] Parent",
        "jiraKey": "PROJ-122",
        "jiraUrl": "https://jira.example.com/browse/PROJ-122",
        "headRef": "feature/PROJ-122-parent",
        "baseRef": "main",
        "status": "🟢"
      },
      {
        "number": 14,
        "url": "https://github.com/owner/repo/pull/14",
        "title": "[PROJ-124] Leaf",
        "jiraKey": "PROJ-124",
        "jiraUrl": "https://jira.example.com/browse/PROJ-124",
        "headRef": "feature/PROJ-124-leaf",
        "baseRef": "feature/PROJ-122-parent",
        "status": "🟢",
        "current": true
      }
    ]
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

### `inspect pr` success

```json
{
  "branch": {
    "name": "b/PROJ-123_fix_login_bug"
  },
  "pullRequest": {
    "number": 42,
    "url": "https://github.com/owner/repo/pull/42",
    "apiUrl": "https://api.github.com/repos/owner/repo/pulls/42",
    "title": "[PROJ-123] Fix login bug",
    "description": "PR body",
    "draft": false,
    "state": "open"
  },
  "comments": [
    {
      "id": 101,
      "author": "alice",
      "body": "Top-level PR comment",
      "htmlUrl": "https://github.com/owner/repo/pull/42#issuecomment-101",
      "createdAt": "2026-04-11T10:00:00Z"
    }
  ],
  "reviews": [
    {
      "id": 201,
      "author": "bob",
      "state": "COMMENTED",
      "body": "Review body",
      "submittedAt": "2026-04-11T10:05:00Z",
      "conversations": [
        {
          "id": "review-comment-301",
          "comments": [
            {
              "id": 301,
              "reviewId": 201,
              "author": "bob",
              "body": "Please adjust this line",
              "path": "internal/example.go",
              "line": 10,
              "conversationId": "review-comment-301"
            }
          ]
        }
      ]
    }
  ]
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
