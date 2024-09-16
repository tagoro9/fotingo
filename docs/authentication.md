# Authentication

Use `fotingo login` to authenticate GitHub and Jira interactively.

```bash
fotingo login
```

## GitHub

`fotingo` supports OAuth and API token auth for GitHub.

- OAuth flow requires the Fotingo GitHub App to be installed in the orgs you want to access.
- The app can be installed during the auth flow when prompted.
- If you choose token auth, use a classic personal access token (PAT):
  - Create it at `https://github.com/settings/tokens`
  - Token type: classic PAT
  - Required scope: `repo`

## Jira

For Jira token auth, use an Atlassian API token and your Atlassian account email.

- Create API token at `https://id.atlassian.com/manage-profile/security/api-tokens`
- Use your Jira/Atlassian account email when prompted
- Configure or enter Jira site URL (for example `https://yourcompany.atlassian.net`)

Jira OAuth support requires binaries compiled with Jira OAuth credentials (client ID and client secret).
This is intended for internal builds only. Committing or distributing binaries with embedded Jira OAuth client secret is not considered safe.

## Environment Variables

You can also provide credentials non-interactively:

- `GITHUB_TOKEN` for GitHub token auth
- `FOTINGO_JIRA_USER_LOGIN` for Jira account email
- `FOTINGO_JIRA_USER_TOKEN` for Jira API token
- `FOTINGO_JIRA_ROOT` for Jira site URL
