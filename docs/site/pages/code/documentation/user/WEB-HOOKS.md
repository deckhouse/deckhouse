---
title: "Webhooks"
menuTitle: Webhooks
force_searchable: true
description: Webhooks
permalink: en/code/documentation/user/web-hooks.html
lang: en
weight: 50
---

Webhooks are an event-driven integration mechanism with external systems. They enable automatic sending of HTTP requests when specific events occur in Deckhouse Code:

- Support for a wide range of events: Push, Merge Request, Issue, Pipeline, Release, and others.
- Request configuration: choice of method (POST, PUT), JSON format, header customization.
- Security features: Secret Token, SSL/TLS, event filtering.
- Support at the project, group, and instance levels.
- Integration with CI/CD, monitoring, messaging platforms, and tracking systems.
- Retry mechanism for connection failures.

> Project webhooks are supported in GitLab CE.

## Group webhooks

To add a webhook at the group level, open the group page and navigate to "Settings" → "Webhooks". Then select the desired events. Group webhooks support all project events plus:

- Member events;
- Project events;
- Subgroup events.

> If the user has no public email specified, the email in the request body will appear as "[REDACTED]".  
> Member events trigger upon creation, modification, or deletion of group or project members.

## Creating group webhooks

Request header:

```console
X-Gitlab-Event: Member Hook
```

Example request body:

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

### Updating group webhooks

Request header:

```text
X-Gitlab-Event: Member Hook
```

Example request body:

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

### Deleting group webhooks

Request header:

```text
X-Gitlab-Event: Member Hook
```

Example request body:

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

## Project events

Trigger on creation or deletion of projects in groups and subgroups.

### Creating project webhooks

Request header:

```text
X-Gitlab-Event: Project Hook
```

Example request body:

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

### Deleting project webhooks

Request header:

```text
X-Gitlab-Event: Project Hook
```

Example request body:

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

## Subgroup events

Trigger on creation or deletion of subgroups.

### Creating subgroup webhooks

Request header:

```text
X-Gitlab-Event: Subgroup Hook
```

Example request body:

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

### Deleting subgroup webhooks

Request header:

```text
X-Gitlab-Event: Subgroup Hook
```

Example request body:

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
