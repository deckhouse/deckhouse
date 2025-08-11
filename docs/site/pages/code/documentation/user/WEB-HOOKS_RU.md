---
title: "Вебхуки"
menuTitle: Вебхуки
force_searchable: true
description: Вебхуки
permalink: ru/code/documentation/user/web-hooks.html
lang: ru
weight: 50
---

Вебхуки — это событийно-ориентированный механизм интеграции с внешними системами. Они позволяют автоматически отправлять HTTP-запросы при возникновении определённых событий в Deckhouse Code:

- Поддержка широкого спектра событий: Push, Merge Request, Issue, Pipeline, Release и др.
- Настройка запросов: выбор метода (POST, PUT), формат JSON, настройка заголовков.
- Безопасность: Secret Token, SSL/TLS, фильтрация по событиям.
- Поддержка на уровне проектов, групп и всей инстанции.
- Интеграция с CI/CD, мониторингом, мессенджерами и системами трекинга.
- Механизм повторных попыток при сбоях соединения (Retry).

> Вебхуки проектов поддерживаются в GitLab CE.

## Вебхуки групп

Чтобы добавить вебхук на уровне группы, откройте страницу группы и перейдите в «Настройки» → «Вебхуки». Далее выберите интересующие события. Вебхуки групп поддерживают все события из проектов, а также:

- События участников;
- События проектов;
- События подгрупп.

> Если у пользователя не указан публичный email, в теле запроса email будет отображаться как `"[УДАЛЕНО]"`.
>
> События участников срабатывают при создании, изменении или удалении участников группы или проекта.

## Создание вебхуков групп

Заголовок запроса:

```console
X-Gitlab-Event: Member Hook
```

Пример запроса:

```json
{
  "created_at": "2025-07-02T15:23:25Z",
  "updated_at": "2025-07-02T15:35:51Z",
  "group_name": "agriculture",
  "group_path": "agriculture",
  "group_id": 1130,
  "user_username": "reported_user_barabara",
  "user_name": "Estella Gleason",
  "user_email": "[УДАЛЕНО]",
  "user_id": 58,
  "group_access": "Guest",
  "expires_at": "2025-07-09T00:00:00Z",
  "event_name": "user_add_to_group"
}

```

### Изменение вебхуков групп

Заголовок запроса:

```console
X-Gitlab-Event: Member Hook
```

Пример запроса:

```json
{
  "created_at": "2025-07-02T15:23:25Z",
  "updated_at": "2025-07-02T15:36:21Z",
  "group_name": "agriculture",
  "group_path": "agriculture",
  "group_id": 1130,
  "user_username": "reported_user_barabara",
  "user_name": "Estella Gleason",
  "user_email": "[УДАЛЕНО]",
  "user_id": 58,
  "group_access": "Guest",
  "expires_at": null,
  "event_name": "user_update_for_group"
}

```

### Удаление вебхуков групп

Заголовок запроса:

```console
X-Gitlab-Event: Member Hook
```

Пример запроса:

```json
{
  "created_at": "2025-07-02T15:23:25Z",
  "updated_at": "2025-07-02T15:36:21Z",
  "group_name": "agriculture",
  "group_path": "agriculture",
  "group_id": 1130,
  "user_username": "reported_user_barabara",
  "user_name": "Estella Gleason",
  "user_email": "[УДАЛЕНО]",
  "user_id": 58,
  "group_access": "Guest",
  "expires_at": null,
  "event_name": "user_remove_from_group"
}

```

## События проекта

Срабатывает при создании или удалении проектов в группе и подгруппах.

### Создание вебхуков проекта

Заголовок запроса:

```console
X-Gitlab-Event: Project Hook
```

Пример запроса:

```json
{
  "event_name": "project_create",
  "created_at": "2025-07-02T15:40:09Z",
  "updated_at": "2025-07-02T15:40:09Z",
  "name": "rspec",
  "path": "rspec",
  "path_with_namespace": "flant-development/agriculture/rspec",
  "project_id": 28,
  "project_namespace_id": 1130,
  "owners": [
    {
      "name": "Administrator",
      "email": "[УДАЛЕНО]"
    }
  ],
  "project_visibility": "private"
}
```

### Удаление вебхуков проекта

Заголовок запроса:

```console
X-Gitlab-Event: Project Hook
```

Пример запроса:

```json
{
  "event_name": "project_destroy",
  "created_at": "2025-07-02T15:40:09Z",
  "updated_at": "2025-07-02T15:42:04Z",
  "name": "rspec",
  "path": "rspec",
  "path_with_namespace": "flant-development/agriculture/rspec",
  "project_id": 28,
  "project_namespace_id": 1130,
  "owners": [
    {
      "name": "Administrator",
      "email": "[REDACTED]"
    }
  ],
  "project_visibility": "private"
}
```

## События подгрупп

Срабатывает при создании или удалении подгруппы.

### Создание вебхуков подгрупп

Заголовок запроса:

```console
X-Gitlab-Event: Subgroup Hook
```

Пример запроса:

```json
{
  "created_at": "2025-07-02T15:44:02Z",
  "updated_at": "2025-07-02T15:44:02Z",
  "event_name": "subgroup_create",
  "name": "finances",
  "path": "finances",
  "full_path": "flant-development/finances",
  "group_id": 1659,
  "parent_group_id": 1123,
  "parent_name": "Flant development",
  "parent_path": "flant-development",
  "parent_full_path": "flant-development"
}
```

### Удаление вебхуков подгрупп

Заголовок запроса:

```console
X-Gitlab-Event: Subgroup Hook
```

Пример запроса:

```json
{
  "created_at": "2025-07-02T15:44:02Z",
  "updated_at": "2025-07-02T15:44:02Z",
  "event_name": "subgroup_destroy",
  "name": "finances",
  "path": "finances",
  "full_path": "flant-development/finances",
  "group_id": 1659,
  "parent_group_id": 1123,
  "parent_name": "Flant development",
  "parent_path": "flant-development",
  "parent_full_path": "flant-development"
}
```
