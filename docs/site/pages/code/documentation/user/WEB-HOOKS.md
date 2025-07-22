---
title: "Webhooks"
menuTitle: Webhooks
force_searchable: true
description: Webhooks
permalink: ru/code/documentation/user/web-hooks.html
lang: en
weight: 50
---
## Webhooks

Webhooks are an event-driven way to integrate with external services.  
They allow you to automatically send HTTP requests when events occur in the system.

**Main features of webhooks:**

- Support for events: Push, Merge Request, Issue, Pipeline, Release, and more.
- Request configuration: choose method (POST, PUT), JSON payload format, and custom headers.
- Security: use of Secret Token, SSL/TLS support, and event filtering.
- Support at the level of individual projects, groups, and the entire system.
- Integration with CI/CD, monitoring systems, chats, and task managers.
- Automatic retries in case of connection failures.

## Project Webhooks

- Available in the GitLab CE version.

## Group Webhooks

To add a webhook to a group, go to the group page and click **Settings => Webhooks**.  
Then, select the events that will trigger the webhook. Group webhooks support all project events, and additionally:
- Member events
- Project events
- Subgroup events

### Events

If the user does not have a public email specified, the email will be received as `"[REDACTED]"`.

#### Member Events

Triggered when members of a group or project are created, removed, or modified.

##### Creation

Request headers:

```text
X-Gitlab-Event: Member Hook
```

Body:

```json
{
  "created_at": "2025-07-02T15:23:25Z",
  "updated_at": "2025-07-02T15:35:51Z",
  "group_name": "agriculture",
  "group_path": "agriculture",
  "group_id": 1130,
  "user_username": "reported_user_barabara",
  "user_name": "Estella Gleason",
  "user_email": "[DELETED]",
  "user_id": 58,
  "group_access": "Guest",
  "expires_at": "2025-07-09T00:00:00Z",
  "event_name": "user_add_to_group"
}

```

##### Update

Request headers:

```text
X-Gitlab-Event: Member Hook
```

Body:

```json
{
  "created_at": "2025-07-02T15:23:25Z",
  "updated_at": "2025-07-02T15:36:21Z",
  "group_name": "agriculture",
  "group_path": "agriculture",
  "group_id": 1130,
  "user_username": "reported_user_barabara",
  "user_name": "Estella Gleason",
  "user_email": "[DELETED]",
  "user_id": 58,
  "group_access": "Guest",
  "expires_at": null,
  "event_name": "user_update_for_group"
}

```

##### Delete

Request headers:

```text
X-Gitlab-Event: Member Hook
```

Body:

```json
{
  "created_at": "2025-07-02T15:23:25Z",
  "updated_at": "2025-07-02T15:36:21Z",
  "group_name": "agriculture",
  "group_path": "agriculture",
  "group_id": 1130,
  "user_username": "reported_user_barabara",
  "user_name": "Estella Gleason",
  "user_email": "[DELETED]",
  "user_id": 58,
  "group_access": "Guest",
  "expires_at": null,
  "event_name": "user_remove_from_group"
}

```

#### Project events

Triggered when project created or deleted

##### Create

Request headers:

```text
X-Gitlab-Event: Project Hook
```

Body:

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
      "email": "[DELETED]"
    }
  ],
  "project_visibility": "private"
}
```

##### Delete

Request headers:

```text
X-Gitlab-Event: Project Hook
```

Body:

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

#### Subgroups event

Triggered when subgroup created or deleted

##### Create

Request headers:

```text
X-Gitlab-Event: Subgroup Hook
```

Body:

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

##### Delete

Request headers:

```text
X-Gitlab-Event: Subgroup Hook
```

Body:

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
