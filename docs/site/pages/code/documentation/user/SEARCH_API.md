---
title: "Search API"
menuTitle: Search API
searchable: true
description: "Reference for Deckhouse Code Search REST API with OpenSearch and FE-specific filters"
permalink: en/code/documentation/user/search-api.html
lang: en
weight: 46
---

Search REST API lets you conduct a search across a Deckhouse Code instance, a specific group, or a project.
To work with the search using web UI, see the [search guide](search.html).

Source of truth: the Deckhouse Code frontend extension (FE) search code (not upstream GitLab `doc/api/search.md`, which has different behavior for some filters/scopes).

## Endpoints

The following endpoints are available for searching:

- `GET /api/v4/search`: Search across a Deckhouse Code instance.
- `GET /api/v4/groups/:id/search` (or `/api/v4/groups/:id/-/search`): Search in a group.
- `GET /api/v4/projects/:id/search` (or `/api/v4/projects/:id/-/search`): Search in a project.

All endpoints require authentication.

## Search scopes

A search scope is defined via the required parameter `scope`. Supported values differ by endpoints:

| `Scope` value | Instance | Group | Project | Backend when OpenSearch is enabled |
|---|---|---|---|---|
| `projects` | ✅ | ✅ | ❌ | CE/PostgreSQL |
| `users` | ✅ | ✅ | ✅ | CE/PostgreSQL |
| `snippet_titles` | ✅ | ❌ | ❌ | CE/PostgreSQL |
| `issues` | ✅ | ✅ | ✅ | OpenSearch (`advanced`) |
| `work_items` | ✅ | ✅ | ✅ | OpenSearch (`advanced`) |
| `merge_requests` | ✅ | ✅ | ✅ | OpenSearch (`advanced`) |
| `milestones` | ✅ | ✅ | ✅ | OpenSearch (`advanced`) |
| `notes` | ❌ | ❌ | ✅ | OpenSearch (`advanced`) |
| `wiki_blobs` | ❌ | ❌ | ✅ | OpenSearch (`advanced`) |
| `commits` | ❌ | ❌ | ✅ | OpenSearch (`advanced`) |
| `blobs` | ❌ | ❌ | ✅ | OpenSearch (`advanced`) |

The response header `X-Search-Type` returns the resolved search type.

## Request parameters

### Common parameters

| Parameter | Type | Required | Endpoints | Notes |
|---|---|---|---|---|
| `search` | string | Yes | All | Search query |
| `scope` | string | Yes | All | Search scope. See the earlier table for available values |
| `confidential` | boolean | No | All | Passed to search service |
| `include_archived` | boolean | No | Instance, group | Not available for searching in a project |
| `page` / `per_page` | integer | No | All | Offset pagination |
| `ref` | string | No | Project | Branch or tag for project search |
| `state` | string | No | All | Object state: `all`, `opened`, `closed`, `merged` |
| `type` | array[string] | No | All | Work item type filter (effective for `work_items`) |

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

## Error cases (400 Bad Request)

### Wrong scope for parameter

Example: `regex=true` with `scope=work_items`:

```json
{
  "message": "regex is supported only for blobs"
}
```

### Invalid regex query constraints

```json
{
  "message": "regex search requires 3-512 chars and at least one alphanumeric literal"
}
```

## Notes on divergence from upstream GitLab docs

- `fields` is supported only for `work_items` and `issues` (not `merge_requests`).
- `exclude_forks` is scoped to `work_items` and `issues`.
- Additional FE filters are implemented: `language`, `label_name`, MR branch/author filters, `not_*` filters.
- `X-Search-Aggregations` response header is available when OpenSearch aggregations exist.
