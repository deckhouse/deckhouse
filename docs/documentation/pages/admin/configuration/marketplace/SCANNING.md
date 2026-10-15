---
title: Scanning
permalink: en/admin/configuration/marketplace/scanning.html
description: "Monitor and manage package repository scanning operations in Deckhouse Platform Marketplace. View scan history, check progress, and trigger manual scans with PackageRepositoryOperation."
---

Deckhouse Platform (DP) uses [PackageRepositoryOperation](../../../reference/api/cr.html#packagerepositoryoperation) objects to configure package repository scanning. Each scan operation discovers new package versions and creates [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) objects for them. DP creates operations automatically, but you can also create them manually.

## Viewing scan operations

To view a list of scan operations, use the following command (you can use `pro` instead of PackageRepositoryOperation):

```bash
d8 k get pro
```

DP keeps the 10 most recent operations of each repository and deletes operations if that threshold is exceeded, starting from the oldest ones.

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
| `Completed` | Whether the operation has finished (`True` or `False`). A failed operation is considered completed too. The result is defined by the reason of the `Completed` condition: `ScanSucceeded` or `ScanFailed` |
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

For status field descriptions, refer to the [PackageRepositoryOperation](../../../reference/api/cr.html#packagerepositoryoperation-v1alpha1-status) documentation.

An example command for viewing packages with errors:

```bash
d8 k get pro <OPERATION_NAME> \
  -o jsonpath='{range .status.packages.failed[*]}{.name}: {range .errors[*]}{.version} - {.message}{"\n"}{end}{end}'
```

## Triggering a manual scan

DP starts a scan of a [PackageRepository](../../../reference/api/cr.html#packagerepository) when the resource is created, when its specification is modified, when DP restarts, and then every [`spec.scanInterval`](../../../reference/api/cr.html#packagerepository-v1alpha1-spec-scaninterval) (6 hours by default, at least 3 minutes). A scan is skipped if the previous operation of the repository hasn't completed yet.

To start the scanning immediately, create a PackageRepositoryOperation manually, for example:

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
The `generateName` field gives every operation a unique name. It works only with `d8 k create`, but not with `d8 k apply`.
{% endalert %}

Alternatively, to create a scan operation, use the following command:

```bash
d8 system package scan <REPOSITORY_NAME>
```

This command creates a PackageRepositoryOperation with [`spec.type: Update`](../../../reference/api/cr.html#packagerepositoryoperation-v1alpha1-spec-type) and [`spec.update.fullScan: true`](../../../reference/api/cr.html#packagerepositoryoperation-v1alpha1-spec-update-fullscan).

### Full scanning

To run a full scanning of a package repository, set the [`spec.update.fullScan: true`](../../../reference/api/cr.html#packagerepositoryoperation-v1alpha1-spec-update-fullscan) parameter in a PackageRepositoryOperation. In this mode, all available package versions are scanned, not only the ones higher than discovered previously.

With `spec.update.fullScan: false` (by default), only the SemVer tags with a version higher than the package version already processed in the cluster are processed.

Automatic operations perform a full repository scan until the first successful scan. After that, automatic scans process only versions newer than the latest package version already processed.

Use the full scanning in the following cases:

- A package version was published after a newer version, for example, a patch for a previous minor version.
- [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) resources are missing from the cluster for versions available in the package repository.
