---
title: "Merge trains"
menuTitle: Merge trains
force_searchable: true
description: Queueing merge requests on a merge train, viewing the queue, and configuring merge train settings
permalink: en/code/documentation/user/merge-trains.html
weight: 86
---

A merge train queues merge requests for a target branch and validates each one against the changes ahead of it before letting it merge. Every queued merge request gets a pipeline that runs on the combination of the target branch, the changes of the merge requests ahead of it in the queue, and its own changes. As long as merge requests land through the train, the target branch only receives code that has been tested in the exact state it will land in.

Merge trains build on merged results pipelines. A merged results pipeline validates one merge request against the current target branch; a merge train validates a whole queue, so two merge requests that pass on their own but break when combined are caught before either lands.

Merge trains are configured for each project separately. Each project has one train per target branch.

## How a merge train works

Merge requests in the queue are ordered by the time they were added, and the oldest one is at the head of the train:

1. A merge request that is set to auto-merge joins the queue at the back of the train.
1. Deckhouse Code creates a temporary Git ref for the merge request that contains the target branch, the changes of every merge request ahead of it in the queue, and its own changes, then starts a pipeline on that ref.
1. When the pipeline of the merge request at the head of the queue succeeds, that merge request merges and leaves the queue.
1. The next merge request becomes the head of the queue. If the merged changes invalidated the state a queued merge request was validated against, its pipeline is discarded and rebuilt on the new state, and the pipelines of all the merge requests behind it are rebuilt too.

A train is not a stored object: it is defined by the merge requests queued for the same target branch of the same project. Positions are recalculated as merge requests leave the queue, so a merge request that merged or was removed no longer affects the positions of the others.

## Prerequisites

- CI/CD must be enabled for the project.
- The CI/CD configuration file must create merge request pipelines. For details, see [Delivery (CI/CD)](/products/code/documentation/user/delivery.html).
- Merged results pipelines must be enabled for the project.
- To change the project settings, you need the **Maintainer** or **Owner** role in the project.

The merge train settings are available only if the feature is enabled for the Deckhouse Code instance. If you do not see them on the "Merge requests" settings page, ask your administrator.

## Enabling merge trains

To enable merge trains:

1. Open the project.
1. Go to "Settings" → "Merge requests".
1. In "Merge options", select the "Enable merged results pipelines" checkbox.
1. Select the "Enable merge trains" checkbox.
1. Select "Save changes".

The following settings appear under "Enable merge trains" and take effect only while it is selected:

| Setting | Description |
|---------|-------------|
| **Merge immediately without restarting the merge train** | Adds the merge-now options described in [Merging immediately](#merging-immediately). The checkbox is unavailable if the project requires fast-forward or semi-linear merges, and while merge train enforcement is selected |
| **Require merge requests to merge through a merge train** | Rejects every direct merge in the project. For details, see [Enforcing merge trains](#enforcing-merge-trains) |
| **Maximum parallel pipelines per merge train** | Limits how many pipelines a train runs at the same time. For details, see [Parallel pipeline limit](#parallel-pipeline-limit) |

Clearing the "Enable merge trains" checkbox does not empty an existing queue in one step. The queue is drained as the refresh cycle reaches each merge request, so queued merge requests can remain visible for a short time after the setting is saved.

## Adding a merge request to a merge train

To add a merge request to a merge train, select "Set to auto-merge" on the merge request page. When merge trains are enabled for the project, auto-merge queues the merge request on the train instead of merging it directly. Depending on the state of the merge request, one of the following options is used:

| Option | Description |
|--------|-------------|
| **Add to merge train** | The merge request joins the queue immediately. This option is used when the merge request can be merged and its pipeline for the current `HEAD` SHA has finished |
| **Add to merge train when all merge checks pass** | The merge request waits until its merge checks pass and then joins the queue |

The text next to the merge button shows which option will be used. If the merge request was already validated by a merge train pipeline, the text is "Re-add to merge train".

If the latest pipeline of the merge request failed, Deckhouse Code asks for confirmation before the merge request is queued.

You can also queue a merge request through the API:

```shell
curl --request POST --header "PRIVATE-TOKEN: <token>" \
  "https://<host>/api/v4/projects/<id>/merge_trains/merge_requests/<merge_request_iid>"
```

| Parameter | Description |
|-----------|-------------|
| `sha` | The SHA you expect the source branch to have. The request is rejected if the branch has moved |
| `squash` | Squashes the commits of the source branch into one when the merge request merges. A train merge also respects the squash setting of the merge request itself |
| `auto_merge` | Queues the merge request when its merge checks pass instead of adding it to the train right away |

The endpoint returns:

- `201` if the merge request joined the queue.
- `202` if the merge request only waits for its merge checks to pass and has not joined the queue yet.
- `400` if merge trains are disabled for the project or the merge request cannot be queued in its current state.
- `401` if the request is not authenticated.
- `403` if your role does not allow you to read the train or to merge the merge request.
- `404` if the project or the merge request does not exist.
- `409` if auto-merge is already set on the merge request.

## Viewing a merge train

To open the merge trains page of a project:

- From a queued merge request: select "View merge train".
- From a merge train pipeline: select "View merge train details" at the top of the pipeline page.

You can also open the page directly at `https://<host>/<namespace>/<project>/-/merge_trains`, or read a train without opening the page through the [API](#reading-a-merge-train-through-the-api).

The page shows:

- A filter that selects the target branch. By default, the filter shows the project default branch.
- The "Active" tab with the merge requests still in the queue, and the "Merged" tab with the merge requests that already merged. Each tab shows how many merge requests it contains.
- A row for every merge request with its pipeline status, title, when it was added or merged, and who added or merged it.

### Merge train position in a merge request

While a merge request is queued, its page shows its position in the queue:

- If nothing is ahead of it: "A new merge train has started, and this merge request is first in the queue."
- Otherwise, its position, for example: "This merge request is number 2 of 5 in the queue."

### Reading a merge train through the API

```shell
# All the trains of a project
curl --header "PRIVATE-TOKEN: <token>" "https://<host>/api/v4/projects/<id>/merge_trains"

# The train of one target branch
curl --header "PRIVATE-TOKEN: <token>" \
  "https://<host>/api/v4/projects/<id>/merge_trains/<target_branch>"

# The queue status of one merge request
curl --header "PRIVATE-TOKEN: <token>" \
  "https://<host>/api/v4/projects/<id>/merge_trains/merge_requests/<merge_request_iid>"
```

The first two endpoints accept the following parameters:

| Parameter | Description |
|-----------|-------------|
| `scope` | Limits the response to the merge requests that are still queued (`active`) or to the merge requests that left the queue (`complete`) |
| `sort` | Orders the merge requests by their position in the queue: `asc` for the oldest first, `desc` for the newest first. The default value is `desc`, so the newest are returned first |

Both endpoints are paginated with the `page` and `per_page` parameters. The response of the first endpoint mixes merge requests of different target branches, so if you need the queue of a single branch, read that train instead.

The GraphQL API also exposes merge trains and their queues for a project. For an overview of the Deckhouse Code interfaces, see [API](/products/code/documentation/user/api.html).

## Queue statuses

Every merge request in a train has one of the following statuses, returned by the REST API and GraphQL. The merge trains page does not show them directly: it groups merge requests into the "Active" and "Merged" tabs.

| Status | Description |
|--------|-------------|
| `idle` | Queued, the pipeline has not started yet |
| `fresh` | The pipeline matches the current merge result |
| `stale` | Something changed ahead in the queue, so the pipeline is rebuilt |
| `merging` | The merge is in progress |
| `merged` | Merged through the train and left the queue |
| `skip_merged` | Merged immediately without restarting the train and left the queue |

Merge requests with the `idle`, `fresh`, and `stale` statuses are shown on the "Active" tab and can be removed from the queue. Merge requests with the `merged` and `skip_merged` statuses are shown on the "Merged" tab. While a merge is in progress, the merge request has the `merging` status and appears on neither tab.

## Removing a merge request from a merge train

To remove a merge request from a merge train, do one of the following:

- Cancel auto-merge on the merge request page.
- Remove the merge request from the queue on the merge trains page: in the merge request row, select the remove icon, then select "Remove from merge train".

Removing a merge request from the queue rebuilds the pipelines of the merge requests behind it, because those pipelines were validated against a queue that no longer exists.

A merge request whose merge is already in progress cannot be removed. Deckhouse Code rejects the removal in this case, and the train finishes the merge.

Deckhouse Code also removes a merge request from the train automatically if the merge request can no longer be merged. For details, see [Merge request dropped from the merge train](#merge-request-dropped-from-the-merge-train).

## Merging immediately

If the "Merge immediately without restarting the merge train" checkbox is selected for the project, the page of a queued merge request offers two additional options next to the merge button:

| Option | Description |
|--------|-------------|
| **Merge now and restart train** | The merge request merges immediately, and the pipelines of the merge requests behind it in the queue are rebuilt with the merged changes |
| **Merge now and don't restart train** | The merge request merges immediately and the train continues without rebuilding. The changes already in the queue are not validated against the merged changes |

Both options require confirmation.

Use "Merge now and don't restart train" only when the merged changes cannot interact with the queued ones. Both options bypass the queue. For that reason, the "Merge immediately without restarting the merge train" checkbox is unavailable when the project requires fast-forward or semi-linear merges, and merge train enforcement removes the options entirely.

A merge request merged with "Merge now and don't restart train" appears on the "Merged" tab with the `skip_merged` status. A merge request merged with "Merge now and restart train" merges directly and does not appear on that tab: the train removes it from the queue as it does with any merge request that is no longer open, and its activity records that removal.

### Merging immediately through the API

The merge endpoint accepts the `skip_merge_train` parameter, which produces the same two behaviors as the options above:

```shell
curl --request PUT --header "PRIVATE-TOKEN: <token>" \
  "https://<host>/api/v4/projects/<id>/merge_requests/<merge_request_iid>/merge?skip_merge_train=true"
```

| Value | Description |
|-------|-------------|
| `true` | The merge request merges immediately and the train continues without rebuilding, as with "Merge now and don't restart train" |
| `false` | The merge request merges immediately and the pipelines of the merge requests behind it in the queue are rebuilt, as with "Merge now and restart train". This is the default value |

The parameter takes effect only if the "Merge immediately without restarting the merge train" checkbox is selected for the project. Otherwise it is ignored. If merge trains are enforced, the merge itself is rejected.

## Enforcing merge trains

Select the "Require merge requests to merge through a merge train" checkbox to reject every direct merge in the project. When enforcement is enabled:

- The merge request page does not offer the merge-now options.
- A merge through the API is rejected unless the merge goes through the train, that is, unless auto-merge is set on the merge request.
- The merges the train itself performs are not affected.

Selecting enforcement also clears and locks the "Merge immediately without restarting the merge train" checkbox: skipping the queue would be rejected at merge time, so it is not offered. Clearing enforcement restores the previously saved value of that checkbox.

Enforcement applies only while merge trains are enabled for the project, so merges are not blocked when trains are disabled.

Enforcement applies to all users, including project owners and administrators.

## Parallel pipeline limit

A train runs several pipelines at once so that the queue does not advance one pipeline at a time. The effective limit is the lower of:

- The instance limit for parallel merge train pipelines, which the administrator sets (default: `20`).
- The project's "Maximum parallel pipelines per merge train" value.

Leave the project field blank to use the instance limit. The project value must be between `1` and `500`. A project can lower the limit but not raise it above the instance limit.

## Merge train pipelines

A merge train pipeline runs on the temporary Git ref that Deckhouse Code builds for the queued merge request, not on the source branch of the merge request. In pipeline lists, such pipelines are labeled `merge train`, which distinguishes them from merged results pipelines.

A finished merge train pipeline cannot be retried: it validated a queue state that no longer exists. To get a new pipeline, add the merge request to the train again.

A merge train pipeline is discarded and replaced when:

- New commits are pushed to the source branch of the merge request.
- A merge request ahead in the queue merges, is removed from the queue, or gets a new pipeline.
- The pipeline of the merge request directly ahead in the queue fails.
- Someone pushes directly to the target branch. In this case, the pipelines of the whole queue are rebuilt, starting from the merge request at the head.

### Running jobs only in merge train pipelines

In a merge train pipeline, the `CI_MERGE_REQUEST_EVENT_TYPE` variable has the value `merge_train`. Use it to run a job only in merge train pipelines:

```yaml
integration-tests:
  script: ./run-integration-tests.sh
  rules:
    - if: $CI_MERGE_REQUEST_EVENT_TYPE == "merge_train"
```

Use the opposite condition to skip a job in merge train pipelines:

```yaml
quick-lint:
  script: ./lint.sh
  rules:
    - if: $CI_MERGE_REQUEST_EVENT_TYPE != "merge_train"
```

## Merge request activity

Deckhouse Code records what happens to a merge request in a train as system notes in the "Activity" section of the merge request:

- The merge request started a train, or was added to the train at a specific position in the queue.
- The merge request was removed from the train by a user.
- The merge request was removed from the train by Deckhouse Code, including the reason.
- Automatic queueing when checks pass was enabled, cancelled, or stopped, including the reason.

If a merge request is removed from the train by Deckhouse Code, its participants also get a to-do item.

## Access to merge trains

Access to merge trains depends on your project role and on the project settings:

| Action | Requirements |
|--------|--------------|
| View the merge trains page and read trains through the API | Merge trains are enabled for the project, and you can read merge requests and pipelines in it. In standard roles, this is available to users with the **Reporter** role or higher |
| Read the merge train status of a single merge request | The same requirements as for reading a train |
| Add a merge request to a train | You can merge the merge request. In standard roles, this is available to users with the **Developer** role or higher |
| Remove a merge request from the queue on the merge trains page | You can cancel pipelines, update the merge request, and merge into the target branch. In standard roles, this is available to users with the **Developer** role or higher |
| Change the merge train settings of a project | **Maintainer** or **Owner** |

Both the merge trains page and the API require you to be signed in. Merge trains are not available to anonymous visitors, even in public projects.

## Audit events

Deckhouse Code records audit events for changes to a project's merge train settings.

| Audit event | Description |
|-------------|-------------|
| `project_cicd_merge_trains_enabled_updated` | The merge trains setting of a project was updated |
| `project_cicd_merge_train_enforced_updated` | The merge train enforcement setting of a project was updated |

## Troubleshooting

### Merge request dropped from the merge train

A merge request that can no longer be merged while its pipeline runs is removed from the queue rather than blocking it. The reason is recorded as a system note in the "Activity" section of the merge request.

The most frequent reasons are:

- Merge trains were disabled for the project.
- The merge request was closed.
- The merge request was marked as a draft.
- The target branch of the merge request was changed.
- The merge request can no longer be merged, for example because of a conflict.
- Auto-merge was cancelled on the merge request.
- The merge train pipeline of the merge request did not succeed.
- The account that queued the merge request was deleted.

While a merge request waits for its merge checks to pass, it is also dropped if it becomes a draft or if its pipeline fails before the checks pass.

Threads and approvals are both checked when a merge request joins the queue. Once it is queued, a thread opened afterwards does not stop the merge, even if the project requires all threads to be resolved: the merge lands exactly what the pipeline validated, and otherwise a single comment would drop the merge request and force everything behind it to rebuild. A required approval that is removed does stop the merge. The merge request keeps its place in the queue until it reaches the head, and is dropped there when its merge is attempted.

Fix the problem stated in the note, then add the merge request to the train again.

### The merge button does not offer merge train options

Check the following:

- Merged results pipelines and merge trains are enabled in the project settings.
- The CI/CD configuration file creates merge request pipelines.
- The merge request has a pipeline for the current `HEAD` SHA of the source branch, and the pipeline has finished.
- Your role allows you to merge the merge request.

### A merge is rejected in a project that requires merge trains

If the "Require merge requests to merge through a merge train" checkbox is selected, a direct merge is rejected, including a merge through the API. Add the merge request to the train instead.

Enforcement applies to every user, so a project owner cannot bypass it. To allow direct merges again, clear the checkbox in the project settings.

### A merge train pipeline cannot be retried

Retrying a finished merge train pipeline is not possible: it validated a queue state that no longer exists. Add the merge request to the train again instead, which creates a pipeline on the current state.

### A merge appears to be stuck

A merge that stops halfway is recovered by a scheduled job: the merge request returns to the queue and the train continues. No manual action is required.

### Merge trains were disabled but the queue is still shown

The queue is drained by the refresh cycle rather than in one step. The merge requests disappear from the page as the cycle reaches them.

### The merge trains page is empty

Check the following:

- The filter shows the branch that the merge requests target.
- Auto-merge is set on the merge requests.
- Merge trains are enabled for the project. If they are disabled, the page is not available even if a queue is left from an earlier state.

### The remove action is not available

The remove action is shown only on the "Active" tab and only if you can cancel pipelines, update the merge request, and merge into the target branch. A merge request whose merge is already in progress cannot be removed.
