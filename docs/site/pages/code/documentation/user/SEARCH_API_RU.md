---
title: "API поиска"
menuTitle: API поиска
searchable: true
description: "Справочник по Search REST API в Deckhouse Code: OpenSearch и FE-фильтры"
permalink: ru/code/documentation/user/search-api.html
lang: ru
weight: 46
---

Search REST API позволяет выполнять поиск по инстансу Deckhouse Code, отдельной группе или проекту.
Для работы с поиском через веб-интерфейс используйте [руководство по поиску](search.html).

Источник истины: код FE-расширения (frontend extension) поиска Deckhouse Code (а не upstream GitLab `doc/api/search.md`, где часть семантики фильтров и значений `scope` отличается).

## Эндпоинты

Для поиска доступны следующие эндпоинты:

- `GET /api/v4/search` — поиск по инстансу Deckhouse Code;
- `GET /api/v4/groups/:id/search` (или `/api/v4/groups/:id/-/search`) — поиск по группе;
- `GET /api/v4/projects/:id/search` (или `/api/v4/projects/:id/-/search`) — поиск по проекту.

Все эндпоинты требуют аутентификации.

## Области поиска

Область поиска задаётся обязательным параметром `scope`. Поддерживаемые значения зависят от эндпоинта:

| Значение `scope` | Инстанс | Группа | Проект | Бэкенд при включённом OpenSearch |
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

В заголовке ответа `X-Search-Type` возвращается фактически использованный тип поиска.

## Параметры запроса

### Общие параметры

| Параметр | Тип | Обязательный | Эндпоинты | Примечание |
|---|---|---|---|---|
| `search` | string | Да | Все | Поисковый запрос |
| `scope` | string | Да | Все | Область поиска. Доступные значения описаны в таблице выше |
| `confidential` | boolean | Нет | Все | Передаётся в службу поиска |
| `include_archived` | boolean | Нет | Инстанс, группа | Параметр недоступен для поиска по проекту |
| `page` / `per_page` | integer | Нет | Все | Постраничный вывод со смещением (offset) |
| `ref` | string | Нет | Проект | Ветка или тег для поиска в проекте |
| `state` | string | Нет | Все | Состояние объекта: `all`, `opened`, `closed`, `merged` |
| `type` | array[string] | Нет | Все | Фильтр типа work item (фактически применяется при `scope=work_items`) |

### Параметры OpenSearch и FE-фильтры

Поддержка дополнительных параметров зависит от выбранной области поиска.
Если параметр передан с неподдерживаемым значением `scope`, API возвращает ответ `400` с сообщением `<param_name> is supported only for <scope list>`.

| Параметр | Тип | Применяется к `scope` | Ограничения |
|---|---|---|---|
| `author_username` | string | `merge_requests` | Фильтр по автору |
| `exclude_forks` | boolean | `work_items`, `issues` | Только в этих `scope` |
| `fields` | array[string] | `work_items`, `issues` | Поддерживается только значение `title`. Для других значений API возвращает `400` |
| `label_name` | array[string] | `work_items`, `issues`, `merge_requests` | Поддерживаются значения через запятую |
| `language` | array[string] | `blobs` | Поддерживаются значения через запятую |
| `not_author_username` | string | `merge_requests` | Исключение по автору |
| `not_source_branch` | string | `merge_requests` | Исключающий фильтр |
| `not_target_branch` | string | `merge_requests` | Исключающий фильтр |
| `num_context_lines` | integer | `blobs` | Поддерживается диапазон `0..20` |
| `regex` | boolean | `blobs` | Длина запроса `3..512` и хотя бы один буквенно-цифровой литерал в запросе, иначе API возвращает `400` |
| `source_branch` | string | `merge_requests` | Точный фильтр по исходной ветке |
| `target_branch` | string | `merge_requests` | Точный фильтр по целевой ветке |

## Заголовки ответа

API может возвращать следующие заголовки:

- `X-Search-Type` — фактически использованный тип поиска;
- `X-Search-Aggregations` — присутствует только когда OpenSearch включён и для выбранной области поиска доступны агрегаты.

Состав агрегатов зависит от значения `scope`:

| Значение `scope` | Агрегаты |
| ---------------- | -------- |
| `blobs` | `language` |
| `work_items`, `issues` | `work_item_type_ids`, `labels` |
| `merge_requests` | `labels` |

## Тело ответа

Эндпоинт возвращает JSON-массив объектов, тип которых зависит от выбранной области поиска:

| Значение `scope` | Тип объекта |
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

### Поиск по инстансу: issues/work items с метками и полями

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

### Неверная область поиска для параметра

Пример текста ошибки при некорректном использовании `regex=true` с `scope=work_items`:

```json
{
  "message": "regex is supported only for blobs"
}
```

### Нарушение ограничений regex-запроса

Regex-режим требует длины запроса `3..512` и хотя бы одного буквенно-цифрового литерала.
Пример текста ошибки при несоблюдении требований:

```json
{
  "message": "regex search requires 3-512 chars and at least one alphanumeric literal"
}
```

## Отличия от upstream GitLab

По сравнению с upstream GitLab `doc/api/search.md` FE-реализация Deckhouse Code отличается:

- параметр `fields` поддерживается только для областей поиска `work_items` и `issues` (не для `merge_requests`);
- параметр `exclude_forks` поддерживается только для областей поиска `work_items` и `issues`;
- добавлены FE-фильтры: `language`, `label_name`, фильтры по ветке и автору MR, а также исключающие `not_*`-фильтры;
- заголовок ответа `X-Search-Aggregations` возвращается при наличии агрегатов OpenSearch.
