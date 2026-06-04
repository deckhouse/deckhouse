---
title: "External status checks in merge requests"
menuTitle: External status checks
force_searchable: true
description: Viewing and retrying external status checks in merge requests
permalink: en/code/documentation/user/external-status-checks.html
weight: 85
---

External status checks show the result of checks performed by services outside Deckhouse Code. A project maintainer can configure these checks for a project and define which target branches they apply to.

Use external status checks on the merge request page to understand whether external systems have approved the current merge request state.

## How external status checks work

When a merge request event occurs, Deckhouse Code sends information about the merge request to every external status check that applies to the target branch. The external service checks the merge request and sends a result back to Deckhouse Code.

A check result always applies to the current `HEAD` SHA of the source branch. If new commits are pushed to the merge request, the previous result no longer applies to the new state and Deckhouse Code waits for a new result.

External status checks can affect merging only if the project setting **Status checks must succeed** is enabled. If the setting is disabled, the widget still shows check results, but failed or pending checks do not block merging.

## External status checks widget

The **External status checks** widget is shown on the merge request page when the merge request has applicable external checks.

The widget shows:

- Check name.
- Check status.
- External service URL, if your role allows viewing it.
- Error details for failed checks, if Deckhouse Code has an error reason.
- **Retry** action for failed checks, if your role allows retrying them.

The widget helps authors and reviewers understand which external systems still need to respond before the merge request can be merged.

## Check statuses

External status checks in the widget can have the following statuses:

| Status | Description |
|--------|-------------|
| `pending` | Deckhouse Code is waiting for a response from the external service. |
| `passed` | The external service approved the current merge request state. |
| `failed` | The external service rejected the merge request, the request to the external service failed, or the timeout expired. |

If a check is `failed`, move the pointer over the failed check to view the error reason. The reason can show an external service error, a connection problem, a blocked URL, or an automatic timeout.

## Pending checks

A check stays in `pending` while Deckhouse Code waits for the external service response.

While at least one check is `pending`, the merge request widget refreshes the external status checks approximately every 10 seconds. Polling stops when no checks are in `pending`.

A pending check can become `failed` automatically if the external service does not respond before the configured timeout. The timeout is configured at the project level or for a specific external status check.

## Failed checks

A check can become `failed` when:

- The external service returns `failed` for the current merge request `HEAD` SHA.
- Deckhouse Code cannot send a request to the external service.
- The external service URL is blocked or unavailable.
- The external service does not respond before the timeout expires.

If **Status checks must succeed** is enabled for the project, a failed check blocks merging until the check passes or the project settings are changed.

## Retry a failed check

You can retry a failed external status check if you have the **Developer** role or higher in the project.

Retry is available only for failed checks that belong to the current `HEAD` SHA of the merge request source branch.

To retry a failed check:

1. Open the merge request.
1. Find the **External status checks** widget.
1. In the failed check row, select **Retry**.

After retry, Deckhouse Code changes the check back to `pending` and sends the current merge request payload to the external service again.

Use retry after the external service issue has been fixed or when the failure was temporary.

## Check URLs

The widget can show the external service URL for a check. The URL is visible only to users whose role allows viewing external status check URLs. In standard project roles, this is available to users with the **Developer** role or higher.

If you cannot see the URL, you can still see the check status if your role allows viewing status check responses.

## Access to check results

Access to external status check information depends on your project role:

| Action | Minimum role |
|--------|--------------|
| View check responses in the widget | **Reporter** |
| View the external service URL | **Developer** |
| Retry a failed check | **Developer** |

For internal projects, authenticated users who can read merge requests in the target project can also view status check responses.

## Branch scope

A project can have checks that apply to:

- All branches.
- All protected branches.
- Selected protected branches.

If a merge request target branch does not match a check scope, the check is not shown in the merge request widget.

When the target branch changes, Deckhouse Code recalculates the applicable checks. Checks that no longer apply are removed from the merge request, and newly applicable checks are added in the `pending` status.

## Troubleshooting

### Merge request is blocked by an external status check

Check the following:

- The **Status checks must succeed** checkbox is enabled.
- The check applies to the merge request target branch.
- The external service sent `passed` for the current `HEAD` SHA.
- The timeout did not expire before the external service callback.

### Check failed because of timeout

Check the following:

- The project-level **Status checks timeout** value.
- The individual **Timeout minutes** value of the check.
- The external service response time.
- Whether the check should be retried after the external service is fixed.

### A check stays pending

A check can stay pending while Deckhouse Code waits for the external service.

Check the following:

- Whether the external service is available.
- Whether the service can process the merge request payload.
- Whether the service sends a response for the current `HEAD` SHA.
- Whether the configured timeout is long enough for the service to finish processing.

If the timeout expires, the check becomes `failed`.

### Request to the external service failed

Check the following:

- The URL in **API to check**.
- Network access from Deckhouse Code to the external service.
- Whether the URL uses `http` or `https`.
- The error text in the merge request widget.

### Retry is not available

The **Retry** action is shown only when all of the following conditions are true:

- The check has the `failed` status.
- The check belongs to the current merge request `HEAD` SHA.
- You have the **Developer** role or higher in the project.

If retry is not available, ask a project maintainer to check the project settings and your role.

### The external service URL is not visible

The URL is hidden if your role does not allow viewing external status check URLs. Ask a project maintainer if you need access to the external service URL.
