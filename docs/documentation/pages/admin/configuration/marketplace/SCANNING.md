---
title: Scanning
permalink: en/admin/configuration/marketplace/scanning.html
description: "Monitor and manage package repository scanning operations in Deckhouse Platform Marketplace. View scan history, check progress, and trigger manual scans with PackageRepositoryOperation."
---

Deckhouse Platform (DP) uses [PackageRepositoryOperation](../../../reference/api/cr.html#packagerepositoryoperation) objects to scan package repositories. Each scan operation discovers new package versions and creates [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) objects for them. DP creates operations automatically, and you can also create them manually.

## Viewing scan operations

To view the scan operations, use the following command (you can use `pro` as a short name for `packagerepositoryoperations`):

```bash
d8 k get pro
```

DP keeps the 10 most recent operations of each repository and deletes older ones.

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME                   COUNT   COMPLETED   MSG   COMPLETIONTIME
test-scan-1780052895   23      True              3h38m
test-scan-1780053890   23      True              3h22m
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Output columns:

| Column | Description |
|---|---|
| `Count` | Total number of packages found during the scan |
| `Completed` | Whether the operation has finished (`True` / `False`). A failed operation is completed too: the result is the reason of the `Completed` condition, `ScanSucceeded` or `ScanFailed` |
| `MSG` | Message of the `Completed` condition, for example, the scan error |
| `CompletionTime` | Time when the operation completed |

To filter operations by repository, use the following command:

```bash
d8 k get pro -l packages.deckhouse.io/repository=<REPOSITORY_NAME>
```

## Inspecting a scan operation

For full scan results including per-package details, use the following command:

```bash
d8 k get pro <OPERATION_NAME> -o yaml
```

Key status fields:

| Field | Description |
|---|---|
| `status.startTime` | When the operation started |
| `status.completionTime` | When the operation completed |
| `status.packages.total` | Total number of packages found |
| `status.packages.processedOverall` | Number of packages processed so far, including the failed ones |
| `status.packages.newVersionsOverall` | Total number of new versions across all packages |
| `status.packages.processed[]` | Per-package results: `name`, `type`, `foundVersions`, `newVersions` |
| `status.packages.failed[]` | Packages with errors: `name`, `errors[]` with `version` and `message` |
| `status.packages.discovered[]` | Packages that are still waiting to be processed. The list is empty after the operation completes |

An example command for viewing packages with errors:

```bash
d8 k get pro <OPERATION_NAME> \
  -o jsonpath='{range .status.packages.failed[*]}{.name}: {range .errors[*]}{.version} - {.message}{"\n"}{end}{end}'
```

## Triggering a manual scan

DP starts a scan of a [PackageRepository](../../../reference/api/cr.html#packagerepository) when the resource is created, when its spec changes, when DP restarts, and then every `spec.scanInterval` (6 hours by default, at least 3 minutes). A scan is skipped if the previous operation of the repository hasn't completed yet.

To scan immediately, create a PackageRepositoryOperation manually, for example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PackageRepositoryOperation
metadata:
  generateName: my-registry-scan-manual-
spec:
  packageRepositoryName: my-registry
  type: Update
  update:
    fullScan: true
```

{% alert level="info" %}
The `generateName` field gives every operation a unique name. It works only with `d8 k create`, not with `d8 k apply`.
{% endalert %}

Alternatively, to create a scan operation, you can use the following command:

```bash
d8 system package scan <REPOSITORY_NAME>
```

This command creates a PackageRepositoryOperation with `spec.type: Update` and `spec.update.fullScan: true`.

### `fullScan` parameter

| Value | Behavior |
|---|---|
| `true` | Lists all semver tags of each package and creates the versions that are missing in the cluster. Versions that already exist are not read again |
| `false` (default) | Processes only the tags whose version is higher than the latest version of the package already processed in the cluster |

The automatic operations of a repository are full until the first successful scan and incremental after it.

Use `fullScan: true` if a version was published after a higher one, for example, a patch for an older minor version, or when [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) objects are missing versions that exist in the container registry.
