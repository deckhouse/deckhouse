---
title: "Maintenance mode"
menuTitle: Maintenance mode
searchable: true
description: Enabling maintenance mode to reduce write operations in Deckhouse Code while performing maintenance tasks
permalink: en/code/documentation/admin/configuration/maintenance-mode.html
lang: en
weight: 56
---

Maintenance mode allows administrators to reduce write operations to a minimum while maintenance tasks are performed.
The main goal is to block all external actions that change the internal state of the PostgreSQL database, as well as files, Git repositories, and container image registries.

When maintenance mode is enabled, in-progress actions finish shortly  because no new actions are coming in,
and internal state changes are minimal. In that state, various maintenance tasks are easier.

Maintenance mode allows most external actions that do not change the internal state.
At the HTTP level, requests with `POST`, `PUT`, `PATCH`, and `DELETE` methods are blocked.

## Enabling maintenance mode

Enable maintenance mode as an administrator in one of these ways:

- **Via Web UI**:
  1. Go to "Admin" → "Settings" → "General".
  1. Expand the "Maintenance mode" section, and select the "Enable maintenance mode" checkbox.
  1. If necessary, add a message to be displayed in the banner.
  1. Select "Save changes".
- **Via API**:

  Run the following request:

  ```shell
  curl --request PUT --header "PRIVATE-TOKEN: $ADMIN_TOKEN" \
    "<instance-url>/api/v4/application/settings?maintenance_mode=true"
  ```

## Disabling maintenance mode

Disable maintenance mode in one of these ways:

- **Via Web UI**:
  1. Go to "Admin" → "Settings" → "General".
  1. Expand the "Maintenance mode" section and clear the "Enable maintenance mode" checkbox.
  1. Select "Save changes".
- **Via API**:

  Run the following request:

  ```shell
  curl --request PUT --header "PRIVATE-TOKEN: $ADMIN_TOKEN" \
    "<instance-url>/api/v4/application/settings?maintenance_mode=false"
  ```

## Maintenance mode considerations

When maintenance mode is enabled, a banner is displayed at the top of the interface.
If necessary, the banner message can be customized.

When trying to perform an unallowed write operation, the user gets an error message.

{% alert level="info" %}
In some cases, visual feedback from an action might be misleading.
For example, when starring a project, the "Star" button changes to "Unstar".
This is due to the UI being updated prior to obtaining the result of the `POST` request.
{% endalert %}

### Administrator actions

Administrators can still edit the application settings. This allows them to disable maintenance mode after it's been enabled.

### Authentication

All users can sign in and out of the system, but the new user creation is blocked.

If an LDAP synchronization is scheduled during the maintenance, it will fail because user creation is disabled.
Similarly, a new user creation based on SAML and other OmniAuth providers will fail as well.

{% alert level="info" %}
When user creation is blocked during an LDAP sign-in, the user sees an error message indicating that the instance is in
read-only (maintenance) mode, rather than a generic message about the access being denied.
{% endalert %}

### Git actions

All read-only Git operations continue to work, including `git clone` and `git pull`.

All write operations fail, both through the CLI and the Web IDE, accompanied by the following message:

```console
Git push is not allowed because this instance is currently in (read-only) maintenance mode.
```

### Merge requests and issues

All write actions besides the exceptions fail.
For example, a user cannot edit merge requests or issues.

### Incoming email

Creating new issue replies, issues (including new Service Desk issues), and merge requests by email fails.
The incoming mail workers skip processing while maintenance mode is enabled.

### Outgoing email

Notification emails continue to arrive, but emails that require database writes, like resetting the password, do not arrive.

### REST API

For most JSON requests, the `POST`, `PUT`, `PATCH`, and `DELETE` methods are blocked. The API returns a `503` response
with the error message `system is in maintenance mode` and a `Retry-After` header.

Only the following requests are allowed:

| HTTP request | Allowed routes | Notes |
| ------------ | -------------- | ----- |
| `POST` | `/admin/application_settings/general` | Updating application settings in the administrator UI |
| `PUT` | `/api/v4/application/settings` | Updating application settings via the API |
| `POST` | `/users/sign_in` | Sign in for users |
| `POST` | `/users/sign_out` | Sign out for users |
| `POST` | `/oauth/token` | Obtaining OAuth tokens |
| `POST` | `/admin/session`, `/admin/session/destroy` | Using Admin Mode for administrators |
| `POST` | Paths ending with `/compare` | Git revision routes |
| `POST` | `*.git/git-upload-pack` | Running `git pull` and `git clone` |
| `POST` | `/api/v4/internal` | Internal API routes |
| `POST` | `/admin/sidekiq` | Management of background jobs in the "Admin" area |

### GraphQL API

The `POST /api/graphql` requests are allowed, but GraphQL mutations are blocked with the following error message: `You cannot perform write operations on a read-only instance`.

### Continuous integration

During the maintenance, the following restrictions apply:

- No new jobs or pipelines start, scheduled or otherwise.
- Jobs that were already running when the maintenance was enabled continue to have a `running` status in the UI, even if they finish running on the runner.
- Jobs in the `running` state do not time out if the project's time limit is exceeded.
- Pipelines cannot be started, retried, or canceled. No new jobs in the existing pipelines can be created either.
- The status of the runners in the "Admin" → "Runners" section isn't updated.

After maintenance mode is disabled, new jobs are picked up again.

Jobs that were in the `running` state before enabling maintenance mode resume, and their logs start updating again.

{% alert level="info" %}
When the maintenance mode is disabled, it is recommended that you restart pipelines that were in the `running` state when the maintenance was enabled.
{% endalert %}

### Deployments

Deployments don't go through because associated pipelines can't be finished.
Disable automated deployments during maintenance mode, and enable them when it is disabled.

### Container registry

The `docker push` command fails with the following error message:

```console
denied: requested access to the resource is denied
```

However, the `docker pull` command still works.

### Package registry

The package registry allows you to install but not publish packages.

### Background jobs

Background jobs (including cron and Sidekiq jobs) continue running as is, because background jobs are not automatically disabled.

As background jobs perform operations that can change the internal state of your instance, you may want to disable
some or all of them while maintenance mode is enabled.

To monitor queues and disable jobs:

1. Go to "Admin" → "Monitoring" → "Background jobs".
1. In the Sidekiq dashboard, select "Cron" and disable jobs individually or all at once by selecting "Disable All".

### Incident management

Incident management functions are limited.
The creation of alerts and incidents is paused entirely, while associated notifications are disabled.

### Feature flags

- Development feature flags cannot be turned on or off through the API, but can be toggled through the Rails console.
- The feature flag service responds to feature flag checks, but feature flags cannot be toggled.
