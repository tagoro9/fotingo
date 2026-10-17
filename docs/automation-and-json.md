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
fotingo issue comment -i PROJ-123 --body-file note.txt --json
fotingo issue label -i PROJ-123 --add ai --json
```

When `start` runs with `--worktree`, `--worktree-path`, or `git.worktree.enabled=true`, automation should read both `branch.name` and `branch.worktreePath` from the JSON result.
`start` auto-stashes only tracked or staged changes; untracked-only files and directories remain untouched.
`start` refreshes only the default branch when needed and avoids shallow-fetch side effects during that preflight step.

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

## Recommended Flags for Automation

| Flag           | Purpose                     |
| -------------- | --------------------------- |
| `--json`       | Machine-readable output     |
| `-y` / `--yes` | Skip prompts                |
| `--quiet`      | Reduce non-essential output |
| `--no-color`   | Disable ANSI color codes    |

## Standalone Jira Comments and Labels

Use explicit issue keys to comment or add labels before a repository is available,
or when taking over a PR without running `fotingo review`. These commands need
only Jira credentials and do not resolve Git or PR context. Do not run `start` or
`review sync` merely to post a comment or label an issue: those workflows have
additional status or PR-body effects.

```bash
fotingo issue comment -i PROJ-123 --body-file - --json <<'NOTE'
Which repository should contain this change?

Please include the expected behavior and any constraints.
NOTE
fotingo issue label -i PROJ-123 --add ai --add needs-clarification --json
```

Use `--body-file -` for multiline input and `--body-file path` for saved text.
Comment text is literal, so template expressions are preserved rather than
expanded. Exactly one text argument or `--body-file` is required. Issue keys must
match `^[A-Z][A-Z0-9]+-\d+$`; invalid keys fail before Jira initialization.

Label updates preserve existing labels and can be repeated safely. Comment creation
is not idempotent: if a request has an uncertain outcome, verify whether the comment
was posted before retrying. Success objects for these two commands have no
`success` wrapper; failures exit non-zero and use the standard JSON error format.
HTTP 403 errors report permission denied, while HTTP 404 errors report an issue
that is missing or not visible to the account.

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

## Issue-Tracker Workflow Overrides In Automation

Use `--tracker-labels` for labels applied when Fotingo moves an issue through
its workflow. This is separate from `review --labels`, which applies GitHub
pull-request labels:

```bash
fotingo start PROJ-123 -y --tracker-labels active
fotingo review -y --tracker-labels ready-for-review
```

Use `--tracker-comment` to override the configured issue-tracker PR-link comment for one
review. Pass an empty value only when the automation should suppress that
comment:

```bash
fotingo review -y --tracker-comment '{{.Issue.Key}}: {{.PullRequest.URL}}'
fotingo review -y --tracker-comment ""
```

When that base branch already has an open PR, `fotingo review -y --branch <parent-branch>` creates a stacked child PR and creates or extends the GitHub-native stack. To refresh native stack state later without prompting:

```bash
fotingo review stacks sync --json
```

To rebase a stack whose branches may live in separate local worktrees:

```bash
fotingo review stacks rebase --json
fotingo review stacks rebase --push --json
```

Automation should run stack rebase only when it is prepared to handle conflicts. The command requires clean worktrees before starting and stops at the first failed rebase. If earlier stack PRs have been merged, remaining PRs are rebased onto the native stack base branch. `--push` is the explicit opt-in for force-with-lease pushes.

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

### `issue comment` success

```json
{
  "issueKey": "PROJ-123",
  "commentId": "10234",
  "url": "https://jira.example.com/browse/PROJ-123?focusedCommentId=10234"
}
```

### `issue label` success

The labels array includes both pre-existing and newly added labels.

```json
{
  "issueKey": "PROJ-123",
  "labels": ["existing", "ai", "backend"]
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
