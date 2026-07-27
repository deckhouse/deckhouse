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

### OpenSearch and FE filter parameters

Support for additional parameters depends on the selected search scope.
If a parameter is submitted with invalid `scope` values, API returns the `400` response with the message `<param_name> is supported only for <scope list>`.

| Parameter | Type | Applies to `scope` | Restrictions |
|---|---|---|---|
| `author_username` | string | `merge_requests` | Author filter |
| `exclude_forks` | boolean | `work_items`, `issues` | Only in these `scope` values |
| `fields` | array[string] | `work_items`, `issues` | Only `title` is supported. For other values, API returns `400` |
| `label_name` | array[string] | `work_items`, `issues`, `merge_requests` | Comma-separated values are supported |
| `language` | array[string] | `blobs` | Comma-separated values are supported |
| `not_author_username` | string | `merge_requests` | Author exclusion filter |
| `not_source_branch` | string | `merge_requests` | Exclusion filter |
| `not_target_branch` | string | `merge_requests` | Exclusion filter |
| `num_context_lines` | integer | `blobs` | Supported range `0..20` |
| `regex` | boolean | `blobs` | Query length `3..512` and at least one alphanumeric literal; otherwise API returns `400` |
| `source_branch` | string | `merge_requests` | Exact branch filter |
| `target_branch` | string | `merge_requests` | Exact branch filter |

## Response headers

The API can return the following headers:

- `X-Search-Type`: Resolved search type for current request.
- `X-Search-Aggregations`: Present only when OpenSearch is enabled and aggregations exist for the requested scope.

The aggregation scope depends on the `scope` value:

| `Scope` value | Aggregations |
| ------------- | ------------ |
| `blobs` | `language` |
| `work_items`, `issues` | `work_item_type_ids`, `labels` |
| `merge_requests` | `labels` |

## Response body

The endpoint returns a JSON array of scope-specific entities:

| `Scope` value | Entity type |
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

Error message example when `regex=true` is incorrectly used together with `scope=work_items`:

```json
{
  "message": "regex is supported only for blobs"
}
```

### Invalid regex query constraints

Regex mode requires a query length of `3..512` and at least one alphanumeric literal.
The following is an error message example when the requirements are not met:

```json
{
  "message": "regex search requires 3-512 chars and at least one alphanumeric literal"
}
```

## Notes on divergence from upstream GitLab docs

The FE implementation of Deckhouse Code is different from the upstream GitLab `doc/api/search.md`. The differences are as follows:

- The `fields` parameter is supported only for `work_items` and `issues` search scopes (not `merge_requests`).
- The `exclude_forks` parameter is supported only for `work_items` and `issues` search scopes.
- Additional FE filters are implemented: `language`, `label_name`, MR branch and author filters, and `not_*` filters.
- The `X-Search-Aggregations` response header is returned when OpenSearch aggregations exist.
