# Exit Codes

| Code  | Name      | Description                           |
| ----- | --------- | ------------------------------------- |
| `0`   | Success   | Command completed successfully        |
| `1`   | General   | Unclassified or unexpected error      |
| `2`   | Config    | Configuration error                   |
| `3`   | Auth      | Authentication failure                |
| `4`   | Git       | Git operation failure                 |
| `5`   | Jira      | Jira API error                        |
| `6`   | GitHub    | GitHub API error                      |
| `130` | Cancelled | Operation cancelled (Ctrl+C / SIGINT) |

Example:

```bash
fotingo start PROJ-123 -y
case $? in
  0) echo "Success" ;;
  2) echo "Check your configuration" ;;
  3) echo "Authentication failed" ;;
  5) echo "Issue not found in Jira" ;;
  *) echo "Unexpected error" ;;
esac
```
