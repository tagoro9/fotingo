# GitHub profile lookup efficiency

## Cause

The `github::get_user_profile` telemetry operation represents `GET /users/{login}`,
not authenticated-user lookup (`GET /user`, recorded as `get_current_user`).
Collaborator and organization-member list responses commonly omit display names.
Previously, each cold list fetch enriched up to 500 missing names with individual
REST profile requests. Search ranks candidates only after loading their names, so
even a command returning five matches could trigger hundreds of requests.

Persistent profile caching helps subsequent runs on the same machine. Its default
TTL is 30 days, but it cannot eliminate cold-start requests in fresh containers or
when the cache is unavailable or not writable.

## Approaches considered

| Approach                        | Benefit                                                          | Stateless limitation                                                              |
| ------------------------------- | ---------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| Longer TTL or shared disk cache | Reuses previous results                                          | Requires retained state; existing TTL is already 30 days                          |
| Conditional REST requests       | A successful authenticated 304 can save primary rate-limit quota | Still sends one request per profile and requires retained validators and payloads |
| Concurrent REST requests        | Can shorten elapsed time                                         | Does not reduce request count and increases concurrency pressure                  |
| Login-only matching             | Eliminates enrichment requests                                   | Loses full-name search                                                            |
| Lazy login-first resolution     | Can skip names for exact-login inputs                            | Requires a wider search refactor to preserve full-name fuzzy ranking              |
| Batched GraphQL profile queries | Preserves names with fewer requests                              | Requires GraphQL access; login matching remains available if enrichment fails     |

GitHub recommends avoiding unnecessary API calls and supports GraphQL requests
that return only the fields needed. GraphQL aliases allow multiple `user(login:)`
lookups in one query; variables keep login values out of the query text.

A GraphQL-only inventory connection could reduce requests further, but would
replace the existing membership retrieval and pagination path. This change retains
the REST candidate inventories and their visibility behavior and batches only
optional display-name enrichment.

## Implemented behavior

Cold enrichment queries `login` and `name` for up to 50 unique missing logins per
GraphQL request, preserving the existing ceiling of 500 profiles per list fetch.
Existing names and cached profiles are applied first. Overlapping collaborator and
member lists share a case-insensitive, TTL-bound, per-client memory cache, even if
the persistent cache cannot be opened. Concurrent enrichment calls on that client
share the same work. Successful nameless profiles are cached, too.

For a cold list of 500 missing names, profile enrichment takes 10 requests instead
of 500. With ordinary 100-user REST pagination, the total is 5 list requests plus
10 profile batches, compared with 5 plus 500 previously. These are HTTP request
counts, not guarantees about GraphQL quota cost. Actual savings depend on list
size, cached data, and overlap.

GraphQL errors do not trigger an individual REST fallback. Unresolved users remain
selectable by login, successful partial results are preserved, and failed requests
are not cached as successfully resolved empty profiles. A request-wide failure
stops additional enrichment attempts on that client. Incomplete inventories are
not written to the persistent candidate cache after enrichment errors, so a fresh
invocation can retry without waiting for the inventory TTL to expire.
If GraphQL is unavailable or token permissions are insufficient, full-name matching
for unresolved users is unavailable until names can be loaded successfully.

Persistent candidate-list and profile caching retain their existing TTL settings
and cache-hit behavior. Cached candidate lists use available cached names without
fresh enrichment requests. For disposable jobs, retaining `cache.path` can further
reduce requests, but batching does not depend on retained state.

Telemetry records a physical profile batch as `github::get_user_profiles`; the
legacy singular operation remains the name for an individual REST profile lookup.
The distinction reflects a real reduction in HTTP requests. Query text, login
values, and response names are not included in telemetry. GitHub.com uses
`/graphql`; GitHub Enterprise uses `/api/graphql`.

## Primary sources

- [GitHub REST API best practices](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api)
- [Migrating from REST to GraphQL](https://docs.github.com/en/graphql/guides/migrating-from-rest-to-graphql)
- [GraphQL aliases and variables](https://graphql.org/learn/queries/)
- [GitHub GraphQL authentication and requests](https://docs.github.com/en/graphql/guides/forming-calls-with-graphql)
- [GitHub GraphQL rate and query limits](https://docs.github.com/en/graphql/overview/rate-limits-and-query-limits-for-the-graphql-api)
