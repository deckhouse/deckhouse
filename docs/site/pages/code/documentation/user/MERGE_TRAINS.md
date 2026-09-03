---
title: "Merge trains"
menuTitle: Merge trains
force_searchable: true
description: Queueing merge requests on a merge train, viewing the queue, and configuring merge train settings
permalink: en/code/documentation/user/merge-trains.html
weight: 86
---

A merge train forms a queue of merge requests targeting the same branch. Before merging, each merge request is validated together with the merge requests ahead of it in the queue. This ensures that only changes validated in their merge order are merged into the target branch.

Merged results pipelines are used for validation. For the first merge request in the queue, such a pipeline validates its changes against the current state of the target branch. For each subsequent merge request, changes from the preceding merge requests in the queue are also taken into account. Therefore, if merge requests pass validation separately but their changes are incompatible, the issue is detected before the changes are merged into the target branch.

Merge trains are configured separately for each project. A separate merge train is used for each target branch in the project.

## How merge trains work

Merge requests are arranged in the queue in the order they were added. Merge requests added earlier are placed at the front of the queue.

Merge requests in the queue are processed as follows:

1. A merge request with auto-merge enabled is added to the end of the queue.
1. Deckhouse Code creates a temporary Git ref for the merge request and runs a pipeline for it. The Git ref contains the state of the target branch, changes from the preceding merge requests in the queue, and changes from the current merge request.
1. If the pipeline for the first merge request in the queue completes successfully, the merge request is merged into the target branch and leaves the queue.
1. The next merge request moves to the front of the queue. If merging the previous merge request makes the state for which the pipeline was run outdated, Deckhouse Code recreates the pipeline for the new state. Pipelines for subsequent merge requests are also recreated.

A merge train is not stored as a separate object. It is formed from merge requests added to the queue for the same target branch of a project. When a merge request is merged or removed from the queue, the positions of the remaining merge requests are recalculated.

## Prerequisites

Before configuring merge trains, make sure the following requirements are met:

* CI/CD is enabled in the project.
* The CI/CD configuration file is configured to create pipelines for merge requests.
* Merged results pipelines are enabled in the project.
* The user has the **Maintainer** or **Owner** role to modify project settings.

Merge train settings are available only if this feature is enabled for the Deckhouse Code instance. If the settings are not available on the "Merge requests" settings page, contact your administrator.

## Enabling merge trains

To enable merge trains:

1. Open the project.
2. Go to "Settings" → "Merge requests".
3. In the "Merge options" section, select "Enable merged results pipelines".
4. Select "Enable merge trains".
5. Click "Save changes".

After merge trains are enabled, the following additional settings become available:

| Setting | Description |
| ------- | ----------- |
| **Merge immediately without restarting the merge train** | Adds the immediate merge options described in ["Merging immediately"](#merging-immediately). This setting is unavailable if the project requires fast-forward or semi-linear merges, or if merging through a merge train is enforced |
| **Require merge requests to merge through a merge train** | Rejects all direct merges in the project. For details, refer to ["Enforcing merge trains"](#enforcing-merge-trains) |
| **Maximum parallel pipelines per merge train** | Limits the number of merge train pipelines that can run simultaneously. For details, refer to ["Limiting parallel pipelines"](#limiting-parallel-pipelines) |

Clearing "Enable merge trains" does not immediately clear the existing queue. Merge requests are removed gradually as the update cycle processes them, so they may remain visible in the queue for some time after the setting is saved.

## Working with the queue

You can add merge requests to the queue, view or remove them, or merge them immediately.

### Adding a merge request to a merge train

You can add a merge request to a merge train using the web interface or API.

{% tabs adding-mr-to-train %}

{% tab "Adding via web UI" %}

To add a merge request to a merge train, click "Set to auto-merge" on the merge request page. If merge trains are enabled in the project, the merge request is added to the merge train instead of being merged directly.

Depending on the state of the merge request, one of the following options is available:

| Option | Description |
| ------ | ----------- |
| **Add to merge train** | Adds the merge request to the queue immediately. This option is available if the merge request is ready to merge and the pipeline for the current `HEAD` SHA has completed |
| **Add to merge train when all merge checks pass** | Adds the merge request to the queue after all merge checks pass |

The option that will be used is displayed next to the merge button. If the merge request has already been validated by a merge train pipeline, "Re-add to merge train" is displayed.

If the latest pipeline for the merge request failed, Deckhouse Code asks for confirmation before adding the merge request to the queue.

{% endtab %}

{% tab "Adding via API" %}

To add a merge request to a merge train via API, run the following request:

```shell
curl --request POST --header "PRIVATE-TOKEN: <TOKEN>" \
  "https://<HOST>/api/v4/projects/<PROJECT_ID>/merge_trains/merge_requests/<MERGE_REQUEST_IID>"
```

Where:

* `<TOKEN>`: Token used to authenticate with Deckhouse Code.
* `<HOST>`: Deckhouse Code instance address.
* `<PROJECT_ID>`: Project ID.
* `<MERGE_REQUEST_IID>`: Internal ID of the merge request within the project.

The following parameters can be specified in the request:

| Parameter | Description |
| --------- | ----------- |
| `sha` | SHA expected in the source branch. If the current SHA does not match the specified value, the request is rejected |
| `squash` | Squashes the source branch commits into a single commit when the merge request is merged. When merging through a merge train, the merge request's squash setting is also taken into account |
| `auto_merge` | Adds the merge request to the queue only after all merge checks pass |

Possible response codes:

| Code | Description |
| ---- | ----------- |
| `201` | Merge request added to the queue |
| `202` | Merge request is waiting for merge checks to pass and has not yet been added to the queue |
| `400` | Merge trains are disabled in the project, or the current state of the merge request does not allow it to be added to the queue |
| `401` | Request was made without authentication |
| `403` | User role does not allow viewing the merge train or merging this merge request |
| `404` | Project or merge request not found |
| `409` | Auto-merge is already enabled for the merge request |

{% endtab %}

{% endtabs %}

### Viewing a merge train

You can get information about a merge train using the web interface or API.

{% tabs viewing-train-info %}

{% tab "Viewing via web UI" %}

You can open the project's merge trains page in one of the following ways:

* On the page of a merge request added to the queue, follow the "View merge train" link.
* On the merge train pipeline page, follow the "View merge train details" link in the page header.
* Open the page directly at `https://<HOST>/<NAMESPACE>/<PROJECT>/-/merge_trains`.

The merge trains page displays:

* A filter for selecting the target branch. The project's default branch is selected initially.
* The "Active" tab with merge requests currently in the queue and the "Merged" tab with merge requests that have already been merged. Each tab shows the number of merge requests.
* Information about each merge request: pipeline status, title, time when the merge request was added to the queue or merged, and the user who added or merged it.

While a merge request is in the queue, its position is displayed on the merge request page:

* For the first merge request: "A new merge train has started, and this merge request is first in the queue".
* For other merge requests, the position in the queue is displayed, for example, "This merge request is number 2 of 5 in the queue".

{% endtab %}

{% tab "Viewing via API" %}

To get information about merge trains via API, run one of the following requests:

```shell
# All merge trains in the project.
curl --header "PRIVATE-TOKEN: <TOKEN>" \
  "https://<HOST>/api/v4/projects/<PROJECT_ID>/merge_trains"

# Merge train for a specific target branch.
curl --header "PRIVATE-TOKEN: <TOKEN>" \
  "https://<HOST>/api/v4/projects/<PROJECT_ID>/merge_trains/<TARGET_BRANCH>"

# Status of a specific merge request in the queue.
curl --header "PRIVATE-TOKEN: <TOKEN>" \
  "https://<HOST>/api/v4/projects/<PROJECT_ID>/merge_trains/merge_requests/<MERGE_REQUEST_IID>"
```

Where:

* `<TOKEN>`: Token used to authenticate with Deckhouse Code.
* `<HOST>`: Deckhouse Code instance address.
* `<PROJECT_ID>`: Project ID.
* `<TARGET_BRANCH>`: Target branch name.
* `<MERGE_REQUEST_IID>`: Internal ID of the merge request within the project.

The first two endpoints accept the following parameters:

| Parameter | Description |
| --------- | ----------- |
| `scope` | Limits the response to merge requests that are still in the queue (`active`) or have already left it (`complete`) |
| `sort` | Specifies the order of merge requests by their position in the queue: `asc` sorts from oldest to newest, and `desc` from newest to oldest. The default value is `desc` |

Both endpoints support pagination using the `page` and `per_page` parameters. The first endpoint returns merge requests for all target branches. To get the queue for a specific branch only, use the second endpoint and specify the branch name.

You can also get information about project merge trains and queues using the GraphQL API.

{% endtab %}

{% endtabs %}

### Removing a merge request from a merge train

To remove a merge request from a merge train, use one of the following methods:

* Cancel auto-merge on the merge request page.
* On the merge trains page, click the remove icon in the merge request row, then click "Remove from merge train".

When a merge request is removed from the queue, pipelines for the merge requests behind it are recreated because the state of the merge train has changed.

A merge request cannot be removed from the merge train if its merge has already started. In this case, Deckhouse Code rejects the removal and completes the merge.

Deckhouse Code also automatically removes a merge request from the merge train if it can no longer be merged. For details, refer to ["Troubleshooting"](#merge-request-removed-from-the-merge-train).

### Merging immediately

You can merge a merge request immediately without waiting for its turn in the merge train. Immediate merging is available through the web interface and API.

{% tabs immediate-merge %}

{% tab "Merging via web UI" %}

If "Merge immediately without restarting the merge train" is enabled in the project, two additional options are available next to the merge button on the page of a merge request in the queue:

| Option | Description |
| ------ | ----------- |
| **Merge now and restart train** | Merges the merge request immediately and recreates the pipelines for subsequent merge requests to include the merged changes |
| **Merge now and don't restart train** | Merges the merge request immediately while existing pipelines continue running without being restarted. As a result, merge requests already in the queue are not validated together with the merged changes |

{% alert level="warning" %}
Use "Merge now and don't restart train" only if the merged changes cannot affect merge requests already in the queue.
{% endalert %}

Both options require confirmation and allow the merge request to be merged outside the established queue order.

Immediate merge options are unavailable if merging through a merge train is enforced in the project. The "Merge now and restart train" setting is also unavailable if the project requires fast-forward or semi-linear merges.

The result of an immediate merge depends on the selected option:

* When merging without restarting the train, the merge request appears on the "Merged" tab with the `skip_merged` status.
* When merging with a train restart, the merge request is merged directly and does not appear on the "Merged" tab. After the merge, it is removed from the queue, and the removal is recorded in the merge request activity.

{% endtab %}

{% tab "Merging via API" %}

To merge a merge request immediately via API, use the merge endpoint with the `skip_merge_train` parameter:

```shell
curl --request PUT --header "PRIVATE-TOKEN: <TOKEN>" \
  "https://<HOST>/api/v4/projects/<PROJECT_ID>/merge_requests/<MERGE_REQUEST_IID>/merge?skip_merge_train=true"
```

Where:

* `<TOKEN>`: Token used to authenticate with Deckhouse Code.
* `<HOST>`: Deckhouse Code instance address.
* `<PROJECT_ID>`: Project ID.
* `<MERGE_REQUEST_IID>`: Internal ID of the merge request within the project.

The `skip_merge_train` parameter accepts the following values:

| Value | Description |
| ----- | ----------- |
| `true` | Merges the merge request immediately without restarting the train |
| `false` | Default. Merges the merge request immediately and recreates the pipelines for subsequent merge requests |

The parameter takes effect only when "Merge immediately without restarting the merge train" is enabled. Otherwise, the parameter is ignored. If merging through a merge train is enforced in the project, the merge request is rejected.

{% endtab %}

{% endtabs %}

## Configuring merge trains

In the project settings, you can enforce merging through a merge train and limit the number of pipelines that can run simultaneously.

### Enforcing merge trains

To prevent merge requests from being merged directly in the project, select "Require merge requests to merge through a merge train".

After this setting is enabled:

* Immediate merge options become unavailable.
* An API merge request is rejected if auto-merge is not enabled for it and it has not been added to a merge train.
* Merges performed by the merge train itself continue to work as usual.

When merge trains are enforced, "Merge immediately without restarting the merge train" is automatically disabled and becomes unavailable. After the settings are saved with merge trains enforced, this setting remains disabled even if enforcement is later turned off. To use it again, enable the setting manually.

Merge train enforcement applies only when merge trains are enabled. If merge trains are disabled, merge requests can be merged directly.

The restriction applies to all users, including project owners and administrators.

### Limiting parallel pipelines

Pipelines for multiple merge requests in a merge train can run simultaneously. The number of pipelines that can run at the same time is determined by two limits:

* The Deckhouse Code instance limit configured by the administrator. The default is `20`.
* The "Maximum parallel pipelines per merge train" project setting.

The lower of the two values is used. For example, if the instance limit is `20` and the project limit is `10`, no more than 10 pipelines can run simultaneously.

To use the instance limit, leave the project setting empty. You can specify a project value from `1` to `500`, but it cannot increase the limit configured for the instance.

## Merge train pipelines

For each merge request in the queue, Deckhouse Code runs a pipeline for a Git ref. This pipeline does not run for the merge request's source branch. In the pipeline list, it is labeled `merge train`, which distinguishes it from a regular merged results pipeline.

A completed merge train pipeline cannot be restarted because the repository state for which it ran may have changed. To run a new pipeline, re-add the merge request to the merge train.

A pipeline is recreated in the following cases:

* A merge request ahead of it in the queue is merged, removed from the queue, or gets a new pipeline.
* The pipeline for the preceding merge request in the queue fails.
* Changes are pushed directly to the target branch. In this case, pipelines for all merge requests in the queue are recreated, starting with the first one.

### Running jobs only in merge train pipelines

In a merge train pipeline, the `CI_MERGE_REQUEST_EVENT_TYPE` variable is set to `merge_train`. You can use it to run a job only in merge train pipelines:

```yaml
integration-tests:
  script: ./run-integration-tests.sh
  rules:
    - if: $CI_MERGE_REQUEST_EVENT_TYPE == "merge_train"
```

To skip a job in merge train pipelines instead, use the opposite condition:

```yaml
quick-lint:
  script: ./lint.sh
  rules:
    - if: $CI_MERGE_REQUEST_EVENT_TYPE != "merge_train"
```

## Merge request statuses in the queue

Each merge request in a merge train has a status that can be retrieved through the REST API or GraphQL API. The statuses are not displayed directly in the web interface. Instead, merge requests are grouped under the "Active" and "Merged" tabs.

| Status | Description |
| ------ | ----------- |
| `idle` | Merge request is in the queue, but its pipeline has not started yet |
| `fresh` | Pipeline corresponds to the current merged result |
| `stale` | The state of the merge train has changed and the pipeline is being recreated |
| `merging` | Merge request is being merged |
| `merged` | Merge request was merged through the merge train and removed from the queue |
| `skip_merged` | Merge request was merged immediately without restarting the merge train and removed from the queue |

Merge requests with the `idle`, `fresh`, and `stale` statuses are displayed on the "Active" tab and can be removed from the queue. Merge requests with the `merged` and `skip_merged` statuses are displayed on the "Merged" tab. A merge request with the `merging` status is not displayed on either tab until the merge is complete.

## Merge request activity

Deckhouse Code records events related to a merge request in a merge train as system notes in the merge request's "Activity" section. The following events are recorded:

* The merge request started a new merge train or was added to an existing merge train at a specific position in the queue.
* The merge request was removed from the merge train by a user.
* The merge request was removed from the merge train by the system, with the reason specified.
* Automatic addition to the merge train after merge checks pass was enabled, canceled, or interrupted, with the reason specified.

If a merge request is removed from the merge train by the system, its participants also receive a corresponding to-do item in their to-do list.

## Access to merge trains

Available merge train actions depend on the user's role and project settings:

| Action | Requirements |
| ------ | ------------ |
| Viewing the merge trains page and retrieving information via API | Merge trains are enabled in the project, and the user has read access to merge requests and pipelines. With standard roles, this is available to users with the **Reporter** role or higher |
| Retrieving the merge train status for a specific merge request | Same requirements as for viewing a merge train |
| Adding a merge request to a merge train | The user has permission to merge the merge request. With standard roles, this is available to users with the **Developer** role or higher |
| Removing a merge request from the queue on the merge trains page | The user has permission to cancel pipelines, modify the merge request, and merge into the target branch. With standard roles, this is available to users with the **Developer** role or higher |
| Modifying merge train settings in the project | **Maintainer** or **Owner** role |

Authentication is required to view merge trains through the web interface or API. Merge trains are not available to anonymous users, even in public projects.

## Audit events

Deckhouse Code records the following audit events when merge train settings are changed in a project:

| Audit event | Description |
| ----------- | ----------- |
| `project_cicd_merge_trains_enabled_updated` | The setting that enables merge trains in the project was changed |
| `project_cicd_merge_train_enforced_updated` | The setting that enforces merging through a merge train was changed |

## Troubleshooting

This section describes common issues with merge trains and how to resolve them.

### Merge request removed from the merge train

If a merge request can no longer be merged while its pipeline is running, Deckhouse Code removes it from the queue to prevent it from blocking other merge requests. The reason for removal is specified in a system note in the "Activity" section.

The most common reasons for removal are:

* Merge trains are disabled in the project.
* New commits were added to the merge request's source branch.
* The merge request was closed.
* The merge request was marked as a draft.
* The target branch of the merge request was changed.
* The merge request can no longer be merged, for example, due to a conflict.
* Auto-merge was canceled for the merge request.
* The merge train pipeline for the merge request failed.
* The account of the user who added the merge request to the queue was deleted.

If a merge request is waiting for merge checks to pass before being added to the merge train, the process is also interrupted if the merge request becomes a draft or its pipeline fails.

Discussions and approvals are checked when a merge request is added to the queue. A new discussion opened afterward does not block the merge, even if the project requires all discussions to be resolved. This preserves the state already validated by the pipeline and avoids recreating pipelines for subsequent merge requests.

If a required approval is revoked, the merge is blocked. The merge request retains its position in the queue but is removed from the merge train when it reaches the front of the queue.

Resolve the issue specified in the system note and re-add the merge request to the merge train.

### Merge button does not offer merge train options

Check the following:

* Merged results pipelines and merge trains are enabled in the project settings.
* The CI/CD configuration file creates pipelines for merge requests.
* The merge request has a completed pipeline for the current `HEAD` SHA of the source branch.
* The user's role allows them to merge the merge request.

### Merge rejected in a project that requires merge trains

If "Require merge requests to merge through a merge train" is enabled, direct merges are rejected, including merges through the API. Instead, add the merge request to the merge train.

The restriction applies to all users, including the project owner. To allow direct merges again, disable this setting.

### Merge train pipeline cannot be restarted

A completed merge train pipeline cannot be restarted because the state for which it ran may have changed. To create a pipeline for the current state, re-add the merge request to the merge train.

### Merge appears to be stuck

If the merge process is interrupted, Deckhouse Code automatically returns the merge request to the queue and continues processing the merge train. No additional action is required.

### Merge trains are disabled, but the queue is still displayed

After merge trains are disabled, merge requests are not removed from the existing queue immediately. They may remain visible on the page for some time while Deckhouse Code gradually clears the queue.

### Merge trains page is empty

Check the following:

* The target branch whose queue you want to view is selected in the filter.
* Auto-merge is enabled for the merge requests.
* Merge trains are enabled in the project. If they are disabled, the page is unavailable even if the queue remains from its previous state.

### Remove action is unavailable

A merge request can be removed from the merge train only on the "Active" tab and only if you have permission to cancel pipelines, modify the merge request, and merge into the target branch.

A merge request cannot be removed from the merge train if its merge has already started.
