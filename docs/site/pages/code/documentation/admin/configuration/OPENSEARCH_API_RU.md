---
title: "OpenSearch API"
menuTitle: OpenSearch API
searchable: true
description: "Административный REST API для пересоздания индексов OpenSearch и статистики очереди индексации"
permalink: ru/code/documentation/admin/configuration/opensearch-api.html
lang: ru
weight: 38
---

На этой странице описаны административные OpenSearch-эндпоинты Deckhouse Code.

## Права доступа

- `POST /admin/opensearch/recreate_indices`: только администратор (`authenticated_as_admin!`).
- `GET /admin/opensearch/indexing_queue_stats`: аутентифицированный пользователь с правом `read_admin_search_indexing_queue_stats` на `:global`.

## POST `/api/v4/admin/opensearch/recreate_indices`

Синхронно пересоздает индекс(ы) OpenSearch и ставит фоновые задачи реиндексации.

### Тело запроса

| Поле | Тип | Обязательное | Допустимые значения |
|---|---|---:|---|
| `schema_class` | string | ✅ | `recreate_all`, `Search::Opensearch::IndicesSchema::Code`, `Search::Opensearch::IndicesSchema::Wiki`, `Search::Opensearch::IndicesSchema::Note`, `Search::Opensearch::IndicesSchema::Milestone`, `Search::Opensearch::IndicesSchema::WorkItem`, `Search::Opensearch::IndicesSchema::MergeRequest` |

### Ответы

- `202 Accepted`

```json
{
  "message": "OpenSearch indices were reset; reindex jobs were enqueued."
}
```

- `400 Bad Request` (например, OpenSearch выключен или сервис вернул ошибку)

```json
{
  "message": "OpenSearch is disabled"
}
```

### Пример

```bash
curl --request POST \
  --header "PRIVATE-TOKEN: <admin_token>" \
  --header "Content-Type: application/json" \
  --data '{"schema_class":"recreate_all"}' \
  --url "https://code.example.com/api/v4/admin/opensearch/recreate_indices"
```

## GET `/api/v4/admin/opensearch/indexing_queue_stats`

Возвращает статистику Sidekiq-очереди индексации OpenSearch.

### Ответ (`200 OK`)

```json
{
  "total": 42,
  "updated_at": "2026-07-01T12:34:56.789Z"
}
```

Поля:

- `total` — общее количество задач индексации в очереди.
- `updated_at` — timestamp ISO8601 с миллисекундами (или `null`).

### Пример

```bash
curl --request GET \
  --header "PRIVATE-TOKEN: <token>" \
  --url "https://code.example.com/api/v4/admin/opensearch/indexing_queue_stats"
```
