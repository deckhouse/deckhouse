---
title: "API поиска"
menuTitle: API поиска
searchable: true
description: "Справочник по Search REST API в Deckhouse Code: OpenSearch и FE-фильтры"
permalink: ru/code/documentation/user/search-api.html
lang: ru
weight: 46
---

На этой странице описан Search REST API в Deckhouse Code.
Для работы с поиском в интерфейсе используйте [руководство по поиску](/code/documentation/user/search.html).

Источник истины: код FE-расширения (frontend extension) поиска Deckhouse Code (а не upstream GitLab `doc/api/search.md`, где часть семантики фильтров/скоупов отличается).

## Эндпоинты

- `GET /api/v4/search`
- `GET /api/v4/groups/:id/search` (вариант с `-/`: `/api/v4/groups/:id/-/search` тоже принимается)
- `GET /api/v4/projects/:id/search` (вариант с `-/`: `/api/v4/projects/:id/-/search` тоже принимается)

Все эндпоинты требуют аутентификации.

## Скоупы и бэкенд

Параметр `scope` обязателен. Поддерживаемые значения зависят от эндпоинта.

| `scope` | Инстанс | Группа | Проект | Бэкенд при включённом OpenSearch |
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
| `scope` | string | ✅ | все | Описано в матрице выше |
| `confidential` | boolean | ❌ | все | Передаётся в службу поиска |
| `include_archived` | boolean | ❌ | инстанс, группа | Для эндпоинта проекта недоступен |
| `page` / `per_page` | integer | ❌ | все | Пагинация со смещением (offset) |
| `ref` | string | ❌ | проект | Ветка или тег для поиска в проекте |
| `state` | string | ❌ | все | `all`, `opened`, `closed`, `merged` |
| `type` | array[string] | ❌ | все | Фильтр типа work item (фактически для `work_items`) |

### Параметры OpenSearch и FE-фильтры

Если параметр передан для неподдерживаемого `scope`, API возвращает `400`:
`<param_name> is supported only for <scope list>`.

| Параметр | Тип | Применяется к `scope` | Ограничения |
|---|---|---|---|
| `author_username` | string | `merge_requests` | фильтр по автору |
| `exclude_forks` | boolean | `work_items`, `issues` | только в этих `scope` |
| `fields` | array[string] | `work_items`, `issues` | допустимо только `title`; иначе `400` |
| `label_name` | array[string] | `work_items`, `issues`, `merge_requests` | поддерживаются значения через запятую |
| `language` | array[string] | `blobs` | поддерживаются значения через запятую |
| `not_author_username` | string | `merge_requests` | исключение по автору |
| `not_source_branch` | string | `merge_requests` | исключающий фильтр |
| `not_target_branch` | string | `merge_requests` | исключающий фильтр |
| `num_context_lines` | integer | `blobs` | диапазон `0..20` |
| `regex` | boolean | `blobs` | длина запроса `3..512` и минимум один буквенно-цифровой литерал; иначе `400` |
| `source_branch` | string | `merge_requests` | точный фильтр по исходной ветке |
| `target_branch` | string | `merge_requests` | точный фильтр по целевой ветке |

## Заголовки ответа

- `X-Search-Type`: итоговый тип поиска для запроса.
- `X-Search-Aggregations`: присутствует только когда OpenSearch включён и для выбранного `scope` есть агрегаты.

`X-Search-Aggregations` — JSON с корзинами агрегатов. Агрегаты возвращаются для:

- `blobs` (`language` buckets),
- `work_items`/`issues` (`work_item_type_ids` и `labels` buckets),
- `merge_requests` (`labels` buckets).

## Тело ответа

Эндпоинт возвращает JSON-массив объектов, тип которых зависит от `scope`:

| `scope` | Тип объекта |
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

## Примеры

### Поиск по экземпляру: issues/work items с метками и полями

```bash
curl --request GET \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/search?scope=issues&search=deploy&fields=title&label_name=team%3Aplatform&exclude_forks=true"
```

### Поиск по группе: MR с FE-фильтрами

```bash
curl --request GET \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/groups/my-group/-/search?scope=merge_requests&search=release&source_branch=release%2F1.2&not_author_username=bot"
```

### Поиск по проекту: blobs с regex и контекстными строками

```bash
curl --request GET \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/projects/my-group%2Fmy-project/-/search?scope=blobs&search=deploy.*job&regex=true&num_context_lines=5&language=Ruby"
```

## Ошибки (400 Bad Request)

Тексты полей `message` в примерах ниже возвращаются API на английском языке.

### Неверный `scope` для параметра

Пример: `regex=true` с `scope=work_items`:

```json
{
  "message": "regex is supported only for blobs"
}
```

### Нарушение ограничений regex-запроса

Regex-режим требует длины запроса `3..512` и хотя бы одного буквенно-цифрового литерала.

```json
{
  "message": "regex search requires 3-512 chars and at least one alphanumeric literal"
}
```

## Отличия от upstream GitLab

По сравнению с upstream GitLab `doc/api/search.md`, FE-реализация Deckhouse Code отличается:

- `fields` поддерживается только для `work_items` и `issues` (не для `merge_requests`).
- `exclude_forks` ограничен `work_items` и `issues`.
- Добавлены FE-фильтры: `language`, `label_name`, фильтры по ветке/автору MR, `not_*`-фильтры.
- Заголовок ответа `X-Search-Aggregations` доступен при наличии агрегатов OpenSearch.
