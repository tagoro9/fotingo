# Telemetry

Fotingo uses PostHog for anonymous usage analytics.

## Defaults and Control

- Default: enabled
- Config key: `telemetry.enabled`
- Disable:

```bash
fotingo config set telemetry.enabled false
```

## What Is Tracked

### Command lifecycle events

- `fotingo.command.started`
- `fotingo.command.completed`
- `fotingo.command.error`
- `fotingo.command.crashed`

Properties include command identity, global mode flags, safe option metadata (booleans/counts/enums), duration, and exit code.

### Integration events

- `fotingo.integration.call`

Properties include service (`github`/`jira`), stable operation name, duration, status bucket, and success.

### UI events

- `fotingo.ui.update_banner.shown`

## Privacy Constraints

Telemetry is allowlist-based and excludes sensitive/freeform values. Fotingo does not send:

- access tokens or secrets
- issue titles/descriptions
- PR freeform body content
- branch names
- issue keys/IDs
- raw API URLs

For repeated/sensitive options (for example reviewers/assignees/labels), telemetry sends counts only.

## Identity

Telemetry uses an anonymous installation ID persisted locally and used as `distinct_id`.
It is generated once per installation and is not derived from email, hostname, or repository metadata.
