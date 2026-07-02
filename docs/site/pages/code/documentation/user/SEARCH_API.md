---
title: "Search API"
menuTitle: Search API
searchable: true
description: "Reference for Deckhouse Code Search REST API with OpenSearch and FE-specific filters"
permalink: en/code/documentation/user/search-api.html
lang: en
weight: 46
---

This page documents the Search REST API implemented in Deckhouse Code.

Source of truth: the Deckhouse Code FE search extension code (not upstream GitLab `doc/api/search.md`, which has different behavior for some filters/scopes).

## Endpoints

- `GET /api/v4/search`
- `GET /api/v4/groups/:id/search` (the `-/` variant `/api/v4/groups/:id/-/search` is also accepted)
- `GET /api/v4/projects/:id/search` (the `-/` variant `/api/v4/projects/:id/-/search` is also accepted)

All endpoints require authentication.

## Scopes and backend

`scope` is required. Supported values differ by endpoint.

| Scope | Instance | Group | Project | Backend when OpenSearch is enabled |
|---|---:|---:|---:|---|
| `projects` | ✅ | ✅ | ❌ | CE/Postgres |
| `users` | ✅ | ✅ | ✅ | CE/Postgres |
| `snippet_titles` | ✅ | ❌ | ❌ | CE/Postgres |
| `issues` | ✅ | ✅ | ✅ | OpenSearch (`advanced`) |
| `work_items` | ✅ | ✅ | ✅ | OpenSearch (`advanced`) |
| `merge_requests` | ✅ | ✅ | ✅ | OpenSearch (`advanced`) |
| `milestones` | ✅ | ✅ | ✅ | OpenSearch (`advanced`) |
| `notes` | ❌ | ❌ | ✅ | OpenSearch (`advanced`) |
| `wiki_blobs` | ❌ | ❌ | ✅ | OpenSearch (`advanced`) |
| `commits` | ❌ | ❌ | ✅ | OpenSearch (`advanced`) |
| `blobs` | ❌ | ❌ | ✅ | OpenSearch (`advanced`) |

Response header `X-Search-Type` returns the resolved search type (`advanced`/other).

## Request parameters

### Common

| Parameter | Type | Required | Endpoints | Notes |
|---|---|---:|---|---|
| `search` | string | ✅ | all | Search query |
| `scope` | string | ✅ | all | See matrix above |
| `confidential` | boolean | ❌ | all | Passed to search service |
| `include_archived` | boolean | ❌ | instance, group | Not available for project endpoint |
| `page` / `per_page` | integer | ❌ | all | Offset pagination |
| `ref` | string | ❌ | project | Branch/tag for project search |
| `state` | string | ❌ | all | `all`, `opened`, `closed`, `merged` |
| `type` | array[string] | ❌ | all | Work item type filter (effective for `work_items`) |

### OpenSearch / FE filter parameters

When parameter scope is invalid, API returns `400` with message:
`<param_name> is supported only for <scope list>`.

| Parameter | Type | Applies to `scope` | Validation |
|---|---|---|---|
| `author_username` | string | `merge_requests` | author filter |
| `exclude_forks` | boolean | `work_items`, `issues` | only in those scopes |
| `fields` | array[string] | `work_items`, `issues` | only `title`; otherwise `400` |
| `label_name` | array[string] | `work_items`, `issues`, `merge_requests` | comma-separated values supported |
| `language` | array[string] | `blobs` | comma-separated values supported |
| `not_author_username` | string | `merge_requests` | author exclusion filter |
| `not_source_branch` | string | `merge_requests` | exclusion filter |
| `not_target_branch` | string | `merge_requests` | exclusion filter |
| `num_context_lines` | integer | `blobs` | range `0..20` |
| `regex` | boolean | `blobs` | query length `3..512` and at least one alphanumeric literal; otherwise `400` |
| `source_branch` | string | `merge_requests` | exact branch filter |
| `target_branch` | string | `merge_requests` | exact branch filter |

## Response headers

- `X-Search-Type`: resolved search type for current request.
- `X-Search-Aggregations`: present only when OpenSearch is enabled and aggregations exist for the requested scope.

`X-Search-Aggregations` is JSON with aggregation buckets. Aggregations are returned for:

- `blobs` (`language` buckets),
- `work_items`/`issues` (`work_item_type_ids` and `labels` buckets),
- `merge_requests` (`labels` buckets).

## Response body

The endpoint returns a JSON array of scope-specific entities:

| Scope | Entity type |
|---|---|
| `issues` | `IssueBasic` |
| `work_items` | `WorkItem` |
| `merge_requests` | `MergeRequestBasic` |
| `milestones` | `Milestone` |
| `notes` | `Note` |
| `commits` | `Commit` |
| `blobs` | `Blob` |
| `wiki_blobs` | `Blob` |
| `projects` | `BasicProjectDetails` |
| `users` | `UserBasic` |
| `snippet_titles` | `Snippet` |

Example response for `scope=issues`:

```json
[
  {
    "id": 1001,
    "iid": 42,
    "title": "Deploy pipeline fails on main",
    "state": "opened",
    "project_id": 7,
    "web_url": "https://gitlab.example.com/my-group/my-project/-/issues/42"
  }
]
```

## Examples

### Instance search: issues/work items with labels and fields

```bash
curl --request GET \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/search?scope=issues&search=deploy&fields=title&label_name=team%3Aplatform&exclude_forks=true"
```

### Group search: merge requests with FE MR filters

```bash
curl --request GET \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/groups/my-group/-/search?scope=merge_requests&search=release&source_branch=release%2F1.2&not_author_username=bot"
```

### Project search: code blobs with regex and context lines

```bash
curl --request GET \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/projects/my-group%2Fmy-project/-/search?scope=blobs&search=deploy.*job&regex=true&num_context_lines=5&language=Ruby"
```

## Error cases (`400`)

### Wrong scope for parameter

Example: `regex=true` with `scope=work_items`:

```json
{
  "message": "regex is supported only for blobs"
}
```

### Invalid regex query constraints

Regex mode requires query length `3..512` and at least one alphanumeric literal.

Example response:

```json
{
  "message": "regex search requires 3-512 chars and at least one alphanumeric literal"
}
```

## Notes on divergence from upstream GitLab docs

Compared to upstream GitLab `doc/api/search.md`, Deckhouse Code FE implementation differs in notable points:

- `fields` is supported only for `work_items` and `issues` (not `merge_requests`).
- `exclude_forks` is scoped to `work_items` and `issues` (not code-search-only semantics).
- Additional FE filters are implemented and documented on this page (`language`, `label_name`, MR branch/author filters, `not_*` filters).
- `X-Search-Aggregations` response header is available when OpenSearch aggregations exist.
