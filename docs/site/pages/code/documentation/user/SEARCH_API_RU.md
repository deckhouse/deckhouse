---
title: "Search API"
menuTitle: Search API
searchable: true
description: "Справочник по Search REST API в Deckhouse Code: OpenSearch и FE-фильтры"
permalink: ru/code/documentation/user/search-api.html
lang: ru
weight: 46
---

На этой странице описан Search REST API в Deckhouse Code.

Источник истины: код FE-расширения поиска Deckhouse Code (а не upstream GitLab `doc/api/search.md`, где часть семантики фильтров/скоупов отличается).

## Эндпоинты

- `GET /api/v4/search`
- `GET /api/v4/groups/:id/search` (вариант с `-/`: `/api/v4/groups/:id/-/search` тоже принимается)
- `GET /api/v4/projects/:id/search` (вариант с `-/`: `/api/v4/projects/:id/-/search` тоже принимается)

Все эндпоинты требуют аутентификацию.

## Скоупы и backend

Параметр `scope` обязателен. Поддерживаемые значения зависят от эндпоинта.

| Scope | Instance | Group | Project | Backend при включенном OpenSearch |
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

В заголовке ответа `X-Search-Type` возвращается итоговый тип поиска (`advanced`/другой).

## Параметры запроса

### Общие

| Параметр | Тип | Обязательный | Эндпоинты | Примечание |
|---|---|---:|---|---|
| `search` | string | ✅ | все | Поисковый запрос |
| `scope` | string | ✅ | все | См. матрицу выше |
| `confidential` | boolean | ❌ | все | Передается в search service |
| `include_archived` | boolean | ❌ | instance, group | Для project-эндпоинта недоступен |
| `page` / `per_page` | integer | ❌ | все | Пагинация offset-based |
| `ref` | string | ❌ | project | Ветка/тег для поиска в проекте |
| `state` | string | ❌ | все | `all`, `opened`, `closed`, `merged` |
| `type` | array[string] | ❌ | все | Фильтр типа work item (фактически для `work_items`) |

### Параметры OpenSearch / FE-фильтры

Если параметр передан для неподдерживаемого scope, API возвращает `400`:
`<param_name> is supported only for <scope list>`.

| Параметр | Тип | Применяется к `scope` | Валидация |
|---|---|---|---|
| `author_username` | string | `merge_requests` | фильтр по автору |
| `exclude_forks` | boolean | `work_items`, `issues` | только в этих scope |
| `fields` | array[string] | `work_items`, `issues` | допустимо только `title`; иначе `400` |
| `label_name` | array[string] | `work_items`, `issues`, `merge_requests` | поддерживается comma-separated |
| `language` | array[string] | `blobs` | поддерживается comma-separated |
| `not_author_username` | string | `merge_requests` | исключение по автору |
| `not_source_branch` | string | `merge_requests` | исключающий фильтр |
| `not_target_branch` | string | `merge_requests` | исключающий фильтр |
| `num_context_lines` | integer | `blobs` | диапазон `0..20` |
| `regex` | boolean | `blobs` | длина запроса `3..512` и минимум один буквенно-цифровой литерал; иначе `400` |
| `source_branch` | string | `merge_requests` | точный фильтр по source branch |
| `target_branch` | string | `merge_requests` | точный фильтр по target branch |

## Заголовки ответа

- `X-Search-Type`: итоговый тип поиска для запроса.
- `X-Search-Aggregations`: присутствует только когда OpenSearch включен и для выбранного scope есть агрегаты.

`X-Search-Aggregations` — JSON с корзинами агрегатов. Агрегаты возвращаются для:

- `blobs` (`language` buckets),
- `work_items`/`issues` (`work_item_type_ids` и `labels` buckets),
- `merge_requests` (`labels` buckets).

## Тело ответа

Эндпоинт возвращает JSON-массив объектов, тип которых зависит от scope:

| Scope | Тип объекта |
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

Пример ответа для `scope=issues`:

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

## Примеры

### Instance search: issues/work items с labels и fields

```bash
curl --request GET \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/search?scope=issues&search=deploy&fields=title&label_name=team%3Aplatform&exclude_forks=true"
```

### Group search: merge requests с FE MR-фильтрами

```bash
curl --request GET \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/groups/my-group/-/search?scope=merge_requests&search=release&source_branch=release%2F1.2&not_author_username=bot"
```

### Project search: code blobs с regex и context lines

```bash
curl --request GET \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/projects/my-group%2Fmy-project/-/search?scope=blobs&search=deploy.*job&regex=true&num_context_lines=5&language=Ruby"
```

## Ошибки (`400`)

### Неверный scope для параметра

Пример: `regex=true` с `scope=work_items`:

```json
{
  "message": "regex is supported only for blobs"
}
```

### Невалидные ограничения regex-запроса

В regex-режиме запрос должен иметь длину `3..512` и содержать минимум один буквенно-цифровой литерал.

Пример ответа:

```json
{
  "message": "regex search requires 3-512 chars and at least one alphanumeric literal"
}
```

## Расхождения с upstream GitLab docs

Относительно upstream GitLab `doc/api/search.md` в Deckhouse Code FE-реализация отличается:

- `fields` поддерживается только для `work_items` и `issues` (не для `merge_requests`).
- `exclude_forks` ограничен `work_items` и `issues` (а не только code search).
- Реализованы дополнительные FE-фильтры (`language`, `label_name`, MR branch/author, `not_*`).
- Добавлен заголовок ответа `X-Search-Aggregations` при наличии агрегатов OpenSearch.
