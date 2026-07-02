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

## Permissions

- `POST /api/v4/admin/opensearch/recreate_indices`: admin only (`authenticated_as_admin!`).
- `GET /api/v4/admin/opensearch/indexing_queue_stats`: authenticated user with permission `read_admin_search_indexing_queue_stats` on `:global`.

## POST `/api/v4/admin/opensearch/recreate_indices`

Synchronously recreates OpenSearch index(es) and enqueues background reindex jobs.

### Request body

| Field | Type | Required | Allowed values |
|---|---|---:|---|
| `schema_class` | string | ✅ | `recreate_all`, `Search::Opensearch::IndicesSchema::Code`, `Search::Opensearch::IndicesSchema::Wiki`, `Search::Opensearch::IndicesSchema::Note`, `Search::Opensearch::IndicesSchema::Milestone`, `Search::Opensearch::IndicesSchema::WorkItem`, `Search::Opensearch::IndicesSchema::MergeRequest` |

### Responses

- `202 Accepted`

```json
{
  "message": "OpenSearch indices were reset; reindex jobs were enqueued."
}
```

- `400 Bad Request` (for example, OpenSearch disabled or service error)

```json
{
  "message": "OpenSearch is disabled"
}
```

### Example

```bash
curl --request POST \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --header "Content-Type: application/json" \
  --data '{"schema_class":"recreate_all"}' \
  --url "https://gitlab.example.com/api/v4/admin/opensearch/recreate_indices"
```

## GET `/api/v4/admin/opensearch/indexing_queue_stats`

Returns Sidekiq queue stats for OpenSearch indexing.

### Response (`200 OK`)

```json
{
  "total": 42,
  "updated_at": "2026-07-01T12:34:56.789Z"
}
```

Fields:

- `total` — total number of queued indexing jobs.
- `updated_at` — ISO8601 timestamp with milliseconds (or `null`).

### Example

```bash
curl --request GET \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/admin/opensearch/indexing_queue_stats"
```
