---
title: "OpenSearch API"
menuTitle: OpenSearch API
searchable: true
description: "Admin REST API for OpenSearch index recreation and indexing queue stats"
permalink: en/code/documentation/admin/configuration/opensearch-api.html
lang: en
weight: 38
---

This page documents Deckhouse Code admin OpenSearch endpoints.
For user-facing search parameters, see ["Search API"](../../user/search-api.html).

## Permissions

- `POST /api/v4/admin/opensearch/recreate_indices`: Admin only (`authenticated_as_admin!`).
- `GET /api/v4/admin/opensearch/indexing_queue_stats`: Authenticated user with permission `read_admin_search_indexing_queue_stats` on `:global`.

## POST /api/v4/admin/opensearch/recreate_indices

Synchronously recreates OpenSearch index(es) and enqueues background reindex jobs.

### Request body

| Field | Type | Required | Allowed values |
|---|---|---|---|
| `schema_class` | string | Yes | `recreate_all`, `Search::Opensearch::IndicesSchema::Code`, `Search::Opensearch::IndicesSchema::Wiki`, `Search::Opensearch::IndicesSchema::Note`, `Search::Opensearch::IndicesSchema::Milestone`, `Search::Opensearch::IndicesSchema::WorkItem`, `Search::Opensearch::IndicesSchema::MergeRequest` |

### Responses

- `202 Accepted`:

  

- `400 Bad Request` (for example, if OpenSearch is disabled or there is a service error):

  

### Request example

```bash
curl --request POST \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --header "Content-Type: application/json" \
  --data '{"schema_class":"recreate_all"}' \
  --url "https://gitlab.example.com/api/v4/admin/opensearch/recreate_indices"
```

## GET /api/v4/admin/opensearch/indexing_queue_stats

Returns Sidekiq queue stats for OpenSearch indexing.

### Response (200 OK)

```json
{
  "total": 42,
  "updated_at": "2026-07-01T12:34:56.789Z"
}
```

Fields:

- `total`: Total number of indexing jobs in the queue.
- `updated_at`: ISO8601 timestamp with milliseconds (or `null`).

### Request example

```bash
curl --request GET \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/admin/opensearch/indexing_queue_stats"
```
