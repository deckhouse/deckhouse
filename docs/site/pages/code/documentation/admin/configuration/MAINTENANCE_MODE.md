---
title: "Maintenance mode"
menuTitle: Maintenance mode
searchable: true
description: Enable maintenance mode to reduce write operations in Deckhouse Code while performing maintenance tasks
permalink: en/code/documentation/admin/configuration/maintenance-mode.html
lang: en
weight: 56
---

Maintenance mode allows administrators to reduce write operations to a minimum while maintenance tasks are performed.
The main goal is to block all external actions that change the internal state.
The internal state includes the PostgreSQL database, and especially files, Git repositories, and container repositories.

When maintenance mode is enabled, in-progress actions finish relatively quickly because no new actions are coming in,
and internal state changes are minimal. In that state, various maintenance tasks are easier.

Maintenance mode allows most external actions that do not change the internal state.
At a high level, HTTP `POST`, `PUT`, `PATCH`, and `DELETE` requests are blocked.

## Enable maintenance mode

Enable maintenance mode as an administrator in one of these ways:

- **Web UI**:
  1. Go to **Admin** → **Settings** → **General**.
  1. Expand the **Maintenance mode** section, and select the **Enable maintenance mode** checkbox. You can optionally add a message for the banner as well.
  1. Select **Save changes**.
- **API**:

  ```shell
  curl --request PUT --header "PRIVATE-TOKEN: $ADMIN_TOKEN" \
    "<instance-url>/api/v4/application/settings?maintenance_mode=true"
  ```

## Disable maintenance mode

Disable maintenance mode in one of these ways:

- **Web UI**:
  1. Go to **Admin** → **Settings** → **General**.
  1. Expand the **Maintenance mode** section, and clear the **Enable maintenance mode** checkbox.
  1. Select **Save changes**.
- **API**:

  ```shell
  curl --request PUT --header "PRIVATE-TOKEN: $ADMIN_TOKEN" \
    "<instance-url>/api/v4/application/settings?maintenance_mode=false"
  ```

## Behavior of features in maintenance mode

When maintenance mode is enabled, a banner is displayed at the top of the page.
The banner can be customized with a specific message.

An error is displayed when a user tries to perform a write operation that isn't allowed.

{% alert level="info" %}
In some cases, visual feedback from an action might be misleading.
For example, when starring a project, the **Star** button changes to show the **Unstar** action.
However, this only updates the UI and doesn't take into account the status of the `POST` request.
{% endalert %}

### Administrator functions

Administrators can edit the application settings. This allows them to disable maintenance mode after it's been enabled.

### Authentication

All users can sign in and out of the instance, but no new users can be created.

If there are LDAP syncs scheduled for that time, they fail because user creation is disabled.
Similarly, user creations based on SAML and other OmniAuth providers fail.

{% alert level="info" %}
When user creation is blocked during an LDAP sign-in, the user sees an error indicating that the instance is in
read-only (maintenance) mode, rather than a generic "access denied" message.
{% endalert %}

### Git actions

All read-only Git operations continue to work, for example `git clone` and `git pull`.
All write operations fail, both through the CLI and the Web IDE, with the error message:
`Git push is not allowed because this instance is currently in (read-only) maintenance mode.`

### Merge requests, issues

All write actions except those mentioned previously fail.
For example, a user cannot update merge requests or issues.

### Incoming email

Creating new issue replies, issues (including new Service Desk issues), and merge requests by email fails.
The incoming mail workers skip processing while maintenance mode is enabled.

### Outgoing email

Notification emails continue to arrive, but emails that require database writes, like resetting the password, do not arrive.

### REST API

For most JSON requests, `POST`, `PUT`, `PATCH`, and `DELETE` are blocked, and the API returns a `503` response
with the error message `system is in maintenance mode` and a `Retry-After` header.

Only the following requests are allowed:

| HTTP request | Allowed routes | Notes |
| ------------ | -------------- | ----- |
| `POST` | `/admin/application_settings/general` | To allow updating application settings in the administrator UI. |
| `PUT` | `/api/v4/application/settings` | To allow updating application settings with the API. |
| `POST` | `/users/sign_in` | To allow users to sign in. |
| `POST` | `/users/sign_out` | To allow users to sign out. |
| `POST` | `/oauth/token` | To allow obtaining OAuth tokens. |
| `POST` | `/admin/session`, `/admin/session/destroy` | To allow Admin Mode for administrators. |
| `POST` | Paths ending with `/compare` | Git revision routes. |
| `POST` | `*.git/git-upload-pack` | To allow Git pull/clone. |
| `POST` | `/api/v4/internal` | Internal API routes. |
| `POST` | `/admin/sidekiq` | To allow management of background jobs in the **Admin** area. |

### GraphQL API

`POST /api/graphql` requests are allowed, but mutations are blocked with the error message
`You cannot perform write operations on a read-only instance`.

### Continuous integration

- No new jobs or pipelines start, scheduled or otherwise.
- Jobs that were already running continue to have a `running` status in the UI, even if they finish running on the runner.
- Jobs in the `running` state for longer than the project's time limit do not time out.
- Pipelines cannot be started, retried, or canceled. No new jobs can be created either.
- The status of the runners in **Admin** → **Runners** isn't updated.

After maintenance mode is disabled, new jobs are picked up again.
Jobs that were in the `running` state before enabling maintenance mode resume, and their logs start updating again.

{% alert level="info" %}
Restart previously `running` pipelines after maintenance mode is turned off.
{% endalert %}

### Deployments

Deployments don't go through because pipelines are unfinished.
Disable auto deploys during maintenance mode, and enable them when it is disabled.

### Container registry

`docker push` fails with the error `denied: requested access to the resource is denied`, but `docker pull` works.

### Package registry

The package registry allows you to install but not publish packages.

### Background jobs

Background jobs (cron jobs, Sidekiq) continue running as is, because background jobs are not automatically disabled.
As background jobs perform operations that can change the internal state of your instance, you may want to disable
some or all of them while maintenance mode is enabled.

To monitor queues and disable jobs:

1. Go to **Admin** → **Monitoring** → **Background jobs**.
1. In the Sidekiq dashboard, select **Cron** and disable jobs individually or all at once by selecting **Disable All**.

### Incident management

Incident management functions are limited.
The creation of alerts and incidents is paused entirely.
Notifications and paging on alerts and incidents are therefore disabled.

### Feature flags

- Development feature flags cannot be turned on or off through the API, but can be toggled through the Rails console.
- The feature flag service responds to feature flag checks, but feature flags cannot be toggled.
