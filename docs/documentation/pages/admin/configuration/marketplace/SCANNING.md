---
title: Scanning
permalink: en/admin/configuration/marketplace/scanning.html
description: "Monitor and manage package repository scanning operations in Deckhouse Kubernetes Platform Marketplace. View scan history, check progress, and trigger manual scans with PackageRepositoryOperation."
---

Deckhouse Kubernetes Platform (DKP) uses [PackageRepositoryOperation](../../../reference/api/cr.html#packagerepositoryoperation) to scan package registries. Each scan operation discovers new package versions and creates or updates [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) objects. Operations are created automatically on a schedule or can be created manually when needed.

## Viewing scan operations

To view all scanning operations, use the command (you can use `pro` as a shorthand for `packagerepositoryoperations`):

```bash
d8 k get pro
```

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
| `Completed` | Whether the operation finished (`True` / `False`) |
| `MSG` | Message from the `Completed` condition |
| `CompletionTime` | Time when the operation completed |

To filter operations by a specific repository, use the following command:

```bash
d8 k get pro -l packages.deckhouse.io/repository=<REPO_NAME>
```

## Inspecting a scan operation

For full scan results including per-package details, use the following command:

```bash
d8 k get pro <operation-name> -o yaml
```

Key status fields:

| Field | Description |
|---|---|
| `status.startTime` | When the operation started |
| `status.completionTime` | When the operation completed |
| `status.packages.total` | Total packages found |
| `status.packages.processedOverall` | Total packages successfully processed |
| `status.packages.newVersionsOverall` | Total new versions across all packages |
| `status.packages.processed[]` | Per-package results: `name`, `type`, `foundVersions`, `newVersions` |
| `status.packages.failed[]` | Packages with errors: `name`, `errors[]` with `version` and `message` |
| `status.packages.discovered[]` | Packages first seen in this operation |

An example command for viewing packages with errors:

```bash
d8 k get pro <operation-name> \
  -o jsonpath='{range .status.packages.failed[*]}{.name}: {range .errors[*]}{.version} - {.message}{"\n"}{end}{end}'
```

## Triggering a manual scan

By default, each [PackageRepository](../../../reference/api/cr.html#packagerepository) creates a new scan operation **6 hours** after the previous one completed (configurable via `spec.scanInterval`).

To scan immediately, create a PackageRepositoryOperation manually with the following example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PackageRepositoryOperation
spec:
  packageRepositoryName: my-registry
  type: Update
  update:
    fullScan: true
    timeout: 5m
```

{% alert level="info" %}
Use `generateName` instead of `name` so you can create multiple operations without name conflicts.
{% endalert %}

Alternatively, to create a scan operation, you can use the following command:

```bash
d8 system package scan <REPO_NAME>
```

This command creates a PackageRepositoryOperation with `spec.type: Update` and `spec.update.fullScan: true`.

### `fullScan` flag

| Value | Behavior |
|---|---|
| `true` | Re-checks all tags in the registry, including those already known |
| `false` (default) | Only processes tags added since the last scan (incremental) |

Use `fullScan: true` when you suspect the registry was modified outside the normal workflow, or when [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) objects are missing versions that exist in the registry.
