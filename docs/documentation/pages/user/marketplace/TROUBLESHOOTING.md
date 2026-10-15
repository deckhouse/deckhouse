---
title: Troubleshooting
permalink: en/user/marketplace/troubleshooting.html
description: "Diagnose and resolve problems with Marketplace applications in Deckhouse Platform. Verify CRD presence, read Application conditions and summary, inspect logs."
lang: en
search: Application troubleshooting, application conditions, diagnosing applications, application logs
---

In this section, you can learn how to solve typical issues when working with Marketplace applications.

## Verifying Marketplace custom resources

If the `d8 k get applications` command returns an error saying that the resource type doesn't exist, the Marketplace custom resources may not be installed. To check, run the following command:

```bash
d8 k get crd | grep -E 'application|package'
```

Expected output:

<!-- markdownlint-disable MD031 -->
```console
applicationpackages.deckhouse.io                     2026-02-10T14:54:41Z
applicationpackageversions.deckhouse.io              2026-02-10T14:54:41Z
applications.deckhouse.io                            2026-02-10T14:54:41Z
modulepackages.deckhouse.io                          2026-02-10T14:54:41Z
modulepackageversions.deckhouse.io                   2026-02-10T14:54:41Z
packagerepositories.deckhouse.io                     2026-02-10T14:54:41Z
packagerepositoryoperations.deckhouse.io             2026-02-10T14:54:41Z
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

If any resources are missing, contact your cluster administrator. Marketplace requires Deckhouse Platform (DP) version 1.76 or later.

## Quick application diagnostics

The quickest way to understand why an application is not working is to check fields in the `status.summary` section of the Application resource. To run quick diagnostics, run the following command:

```bash
d8 k get applications -n <NAMESPACE> <APPLICATION_NAME> -o yaml | grep -A5 'summary:'
```

Example output:

```yaml
summary:
  state: Updating
  message: "Update is waiting for dependent modules to converge; previous version is still serving"
  tip: "Wait — the previous version is still working. The update will continue automatically once dependent modules converge."
```

For `status.summary` field descriptions, refer to the [Application](../../reference/api/cr.html#application-v1alpha1-status-summary) documentation.

## Reading individual conditions

To get a more detailed view of the application state, use the following command:

```bash
d8 k get applications -n <NAMESPACE> <APPLICATION_NAME> \
  -o jsonpath='{range .status.conditions[*]}{.type}: {.status} ({.reason}) - {.message}{"\n"}{end}'
```

Example output for an update that is waiting:

```console
Installed: True (Installed) -
UpdateInstalled: False (Pending) - waiting for processing
ConfigurationApplied: True (ConfigurationApplied) -
Managed: True (Managed) -
Scaled: True (Scaled) -
Ready: True (Ready) -
```

In this example, `Installed: True` means that the application is running the previously installed version, and `UpdateInstalled: False` with the `Pending` reason means that the update is waiting, for example, for the modules the application depends on. The installed version is shown in the [`status.currentVersion.version`](../../reference/api/cr.html#application-v1alpha1-status-currentversion-version) field.

## Viewing DP logs

If the conditions don't provide enough detail, ask the cluster administrator to check the DP log. The log is available only in the `d8-system` namespace:

```bash
d8 k -n d8-system logs svc/deckhouse-leader -c deckhouse | grep '<NAMESPACE>.<APPLICATION_NAME>'
```

## Viewing application pod logs

The objects of an application are named in the format `d8a-<APPLICATION_NAME>-<SUFFIX>`. To list the Deployments and StatefulSets of the application, run the following command:

```bash
d8 k get deployments,statefulsets -n <NAMESPACE> -l packages.deckhouse.io/instance=<APPLICATION_NAME>
```

The pods of these workloads have names with the same prefix. To list them, run:

```bash
d8 k get pods -n <NAMESPACE> | grep 'd8a-<APPLICATION_NAME>-'
```

To view logs for a specific pod, run:

```bash
d8 k logs -n <NAMESPACE> <POD_NAME>
```

To view logs for a Deployment of the application, run:

```bash
d8 k logs -n <NAMESPACE> deployments/d8a-<APPLICATION_NAME>-<SUFFIX>
```

## Common condition reasons

The following table specifies the most common reasons for listed [conditions](../../reference/api/cr.html#application-v1alpha1-status-conditions-reason).

| Reason | Conditions | Meaning and what to check |
|---|---|---|
| `Pending` | `Installed`, `UpdateInstalled` | DP is waiting to install the version, for example, until the modules from `requirements.modules` of the package are enabled. Ask the administrator to check these modules |
| `RequirementsUnmet` | `Installed` | The cluster doesn't meet the package requirements. The message names the unmet requirement |
| `DownloadFailed` | `Installed`, `UpdateInstalled`, `ConfigurationApplied`, `Managed`, `Ready` | DP can't download the package version. Ask the administrator to check the PackageRepository and access to the repository |
| `LoadFromFilesystemFailed` | `Installed`, `UpdateInstalled`, `Ready` | DP can't read the downloaded package. Contact the package developer |
| `SettingsInvalid` | `Installed`, `UpdateInstalled`, `ConfigurationApplied` | The settings didn't pass validation. Check `spec.settings` against the settings schema of the [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) |
| `HookInitializationFailed`, `HookFailed` | `Installed`, `UpdateInstalled`, `ConfigurationApplied`, `Managed`, `Ready` | A hook of the package failed. The message contains the error |
| `ManifestsApplyFailed` | `Installed`, `UpdateInstalled`, `ConfigurationApplied`, `Managed`, `Ready` | DP can't apply the manifests of the package. Check the message and the events in the namespace |
| `ApplyingManifests`, `SettingsChanged` | `UpdateInstalled`, `ConfigurationApplied`, `Managed`, `Ready` | Not an error. DP is applying the manifests or the changed settings |
| `Reconciling` | `Scaled` | A workload is rolling out. The message names the workload |
| `Degraded` | `Scaled` | A workload failed to roll out. Check the events and the pods of the workload with `d8 k describe` |
| `NoResourceReconciliation` | `Managed` | The application is in maintenance mode (the `spec.maintenance` field) |
| `Deleting` | All | The application is being deleted |
