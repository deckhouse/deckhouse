---
title: "External status checks for merge requests"
menuTitle: External status checks
searchable: true
description: Configuring external status checks for merge requests
permalink: en/code/documentation/admin/external-status-checks.html
lang: en
weight: 60
---

External status checks let project administrators send merge request data to an external service and use the service response as an additional merge condition. Use them when a project must pass an external compliance check, quality gate, or security validation before a merge request can be merged.

External status checks are configured for each project separately and are not shared between projects.

## Accessing the settings

To configure external status checks, open the project and go to **Settings** → **Merge requests**.

The page contains two related configuration areas:

- **Merge checks** — project-level settings that affect mergeability.
- **External status checks** — a table for creating, updating, and deleting external status check services.

Users need permission to manage merge request settings in the project. In standard roles, this is available to users with the **Maintainer** or **Owner** role.

## Merge checks settings

### Status checks must succeed

The **Status checks must succeed** checkbox blocks merge requests until all applicable external status checks have the `passed` status.

If the checkbox is cleared, external status checks are still shown in merge requests but do not block merging.

### Status checks timeout

The **Status checks timeout** field sets the default timeout for project status checks.

Behavior:

- The default value is `5` minutes.
- The value must be `1` minute or greater.
- The value applies to checks without an individual timeout.
- If the external service does not respond before the timeout expires, the check response becomes `failed`.

## External status checks table

The **External status checks** table shows all status check services configured for the project.

The table contains:

- The status check name.
- The external service URL.
- The target branch scope.
- The status check timeout or the project default timeout.
- The HMAC shared secret status.
- **Update** and **Delete** actions.

If a check does not have an individual timeout, the table shows the project default timeout with the `(default)` label.

## Adding an external status check

To add an external status check:

1. Open the project.
1. Go to **Settings** → **Merge requests**.
1. In **External status checks**, select **Add external status check**.
1. Fill in the fields.
1. Select **New external status check**.

| Field | Description |
|-------|-------------|
| **Service name** | Name of the external service. The value is required, must be unique in the project, and must not exceed 255 characters. |
| **API to check** | URL of the external service endpoint. The value is required, must be unique in the project, and must use the `http` or `https` protocol. |
| **Target branch** | Scope that defines which merge request target branches use the check. |
| **Timeout minutes** | Individual timeout for this check. If set, it overrides the project-level **Status checks timeout** value. |
| **HMAC Shared Secret** | Optional secret used to sign requests sent from Deckhouse Code to the external service. |

After a check is created, Deckhouse Code creates `pending` check responses for matching open merge requests. Requests to the external service are not sent for those existing merge requests. These responses become `failed` after the timeout expires unless a user retries the check.

For new merge request events, Deckhouse Code sends requests to the matching external status check services automatically.

## Target branch scope

The **Target branch** field defines which merge requests use the status check.

| Scope | Description |
|-------|-------------|
| **All branches** | Applies the check to merge requests targeting any branch. |
| **All protected branches** | Applies the check to merge requests targeting protected branches. |
| Selected protected branches | Applies the check only to merge requests targeting the selected protected branches. Wildcard protected branches are supported. |

When the target branch of a merge request changes, Deckhouse Code recalculates the applicable checks. Responses for checks that no longer apply are removed, and missing `pending` responses are created for newly applicable checks.

## HMAC shared secret

If **HMAC Shared Secret** is set, Deckhouse Code adds the `X-Gitlab-Signature` header to requests sent to the external service.

The header value is an HMAC-SHA256 digest of the request body, generated with the shared secret.

The secret is stored encrypted. After it is saved, the UI only shows that a secret exists. To replace the secret:

1. In the status check row, select **Update**.
1. Select **Edit secret**.
1. Enter a new value.
1. Select **Update status check**.

## Check lifecycle

When a merge request event occurs, Deckhouse Code sends a merge request payload to every applicable external status check service. The payload includes the `external_approval_rule` object with the check `id`, `name`, and `external_url`.

A check response can have one of the following statuses:

| Status | Description |
|--------|-------------|
| `pending` | Deckhouse Code is waiting for the external service response. |
| `passed` | The external service approved the merge request state. |
| `failed` | The external service rejected the merge request state, the request to the external URL failed, or the timeout expired. |

The external service updates a response by using the API endpoint:

```text
POST /projects/:id/merge_requests/:merge_request_iid/status_check_responses
```

The request must include:

- `external_status_check_id` — status check ID.
- `sha` — current `HEAD` SHA of the merge request source branch.
- `status` — `passed` or `failed`.

Deckhouse Code does not update the response if:

- `sha` does not match the current source branch `HEAD`.
- The response is no longer in the `pending` status.
- The timeout has already expired.

## Timeouts

Timeout counting starts when Deckhouse Code sends the request to the external service. If a user retries a failed check, timeout counting starts from the retry time.

Timeout values are selected in this order:

1. The **Timeout minutes** value of the status check.
1. The project-level **Status checks timeout** value.

A background worker checks pending responses every minute. When a response exceeds its timeout, Deckhouse Code changes its status to `failed` and records the reason as `Automatically closed after timeout`.

If the request to the external service fails, Deckhouse Code changes the response status to `failed` and stores the error reason. Users can see the error reason in the merge request widget.

## Deleting an external status check

To delete a check:

1. Open the project.
1. Go to **Settings** → **Merge requests**.
1. In **External status checks**, select **Delete** for the required check.
1. Confirm the deletion.

After deletion, the check no longer applies to project merge requests.

## Audit events

Deckhouse Code records audit events for status check management and response changes.

| Audit event | Description |
|-------------|-------------|
| `external_status_check_created` | An external status check was created. |
| `external_status_check_updated` | An external status check was updated. |
| `external_status_check_destroyed` | An external status check was deleted. |
| `external_status_check_response_updated` | A response was changed by an external callback, retry, request failure, or timeout. |

## Permissions

| Action | Minimum role |
|--------|--------------|
| Create, update, delete, or list project status check services | **Maintainer** or **Owner** |
| Send a status check callback | **Developer** |
| View status check responses in the merge request widget | **Reporter** |
| View the external status check URL in the merge request widget | **Developer** or auditor |
| Retry a failed check | **Developer** |

For internal projects, authenticated users who can read merge requests in the target project can also read status check responses.

## Troubleshooting

### Duplicate name or URL error

A project cannot have two external status checks with the same **Service name** or **API to check** value. Use a unique value for each check.
