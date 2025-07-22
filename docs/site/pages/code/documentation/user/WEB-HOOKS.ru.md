---
title: "Веб-хуки"
menuTitle: Веб-хуки
force_searchable: true
description: Веб-хуки
permalink: ru/code/documentation/user/web-hooks.html
lang: ru
weight: 50
---
## Вебхуки (webhooks)

Вебхуки представляют собой событийно-ориентированный способ интеграции с внешними сервисами. Они позволяют автоматически отправлять HTTP-запросы при наступлении событий в системе.

Основные возможности вебхуков:

- Поддержка событий: Push, Merge Request, Issue, Pipeline, Release и другие.
- Настройка запросов: выбор метода (POST, PUT), формат JSON-пейлоада и настройка заголовков.
- Обеспечение безопасности: использование Secret Token, поддержка SSL/TLS и фильтрация событий.
- Поддержка на уровне отдельных проектов, групп и всей системы.
- Интеграция с CI/CD, системами мониторинга, чатами и таск-менеджерами.
- Автоматические повторы (Retry) при сбоях соединения.

## Веб-хуки проектов

- Присутствует в CE-версии GitLab.

## Веб-хуки групп

Чтобы добавить веб-хук в группу, необходимо, находясь на странице группы, нажать **Настройки => Веб-хуки**.  
Далее необходимо выбрать отслеживаемые события. В веб-хуках группы доступны все события из проектов, а также дополнительно:
- События участников
- События проекта
- События подгруппы

### События

> Если у автора не указан публичный e-mail, то e-mail будет приходить со значением `"[УДАЛЕНО]"`.

#### События участников

Срабатывает при создании, удалении или изменении участников группы или проекта.

##### Создание

Заголовки запроса:

```text
X-Gitlab-Event: Member Hook
```

Пример тела запроса:

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

##### Изменение

Заголовки запроса:

```text
X-Gitlab-Event: Member Hook
```

Пример тела запроса:

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

##### удаление

Заголовки запроса:

```text
X-Gitlab-Event: Member Hook
```

Пример тела запроса:

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

#### События проекта

Срабатывает при создании или удалении проектов в группе и подгрупах

##### Создание

Заголовки запроса:

```text
X-Gitlab-Event: Project Hook
```

Тело запроса:

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

##### Удаление

Заголовки запроса:

```text
X-Gitlab-Event: Project Hook
```

Тело запроса:

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

#### События подгруп

Срабатывает при создании или удалении подгруппы

##### Создание

Заголовки запроса:

```text
X-Gitlab-Event: Subgroup Hook
```

Тело запроса:

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

##### Удаление

Заголовки запроса:

```text
X-Gitlab-Event: Subgroup Hook
```

Тело запроса:

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
