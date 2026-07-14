---
title: Troubleshooting
permalink: en/user/marketplace/troubleshooting.html
description: "Diagnose and resolve problems with Marketplace applications in Deckhouse Kubernetes Platform. Verify CRD presence, read Application conditions and summary, inspect logs."
lang: en
search: Application troubleshooting, application conditions, diagnosing applications, application conditions, application logs
---

## Verify that Marketplace CRDs are present

If `d8 k get app` returns an error, the Marketplace CRDs may not be installed. To check, run the following command:

```bash
d8 k get crd | grep -E 'application|package'
```

Expected output:

<!-- markdownlint-disable MD031 -->
```console
applicationpackages.deckhouse.io                     2026-02-10T14:54:41Z
applicationpackageversions.deckhouse.io              2026-02-10T14:54:41Z
applications.deckhouse.io                            2026-02-10T14:54:41Z
packagerepositories.deckhouse.io                     2026-02-10T14:54:41Z
packagerepositoryoperations.deckhouse.io             2026-02-10T14:54:41Z
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

If any CRDs are missing, contact your cluster administrator. Marketplace requires DKP version 1.76 or later.

## Read the application summary

The quickest way to understand why an application is not working is to check `status.summary`. Use the following command (the short name `app` can be used):

```bash
d8 k get app -n <NAMESPACE> <APPLICATION_NAME> -o yaml | grep -A5 'summary:'
```

Example output:

```yaml
summary:
  state: Updating
  message: "Update is waiting for dependent modules to converge; previous version is still serving"
  tip: "Waiting until DKP processes all dependent modules to start the update."
```

- **`state`** — current high-level state of the application.
- **`message`** — explains why the application is in this state.
- **`tip`** — what to do to resolve the issue or what DKP is waiting for.

## Read individual conditions

To get a more detailed view of the application state, use the following command:

```bash
d8 k get app -n <NAMESPACE> <APPLICATION_NAME> \
  -o jsonpath='{range .status.conditions[*]}{.type}: {.status} ({.reason}) - {.message}{"\n"}{end}'
```

Example output showing a stuck update:

```yaml
conditions:
  - lastTransitionTime: "2026-02-25T16:39:30Z"
    message: ""
    observedGeneration: 1
    reason: Installed
    status: "True"
    type: Installed
  - lastTransitionTime: "2026-02-25T17:12:25Z"
    message: "Update is waiting for dependent modules to converge"
    observedGeneration: 1
    reason: Pending
    status: "False"
    type: UpdateInstalled
  - lastTransitionTime: "2026-02-25T16:39:30Z"
    message: ""
    observedGeneration: 1
    reason: ConfigurationApplied
    status: "True"
    type: ConfigurationApplied
  - lastTransitionTime: "2026-02-25T16:39:30Z"
    message: ""
    observedGeneration: 1
    reason: Managed
    status: "True"
    type: Managed
  - lastTransitionTime: "2026-02-25T16:39:30Z"
    message: ""
    observedGeneration: 1
    reason: Scaled
    status: "True"
    type: Scaled
  - lastTransitionTime: "2026-02-25T16:39:30Z"
    message: ""
    observedGeneration: 1
    reason: Ready
    status: "True"
    type: Ready
currentVersion:
  version: v0.0.20
```

In this example, `Installed=True` (the application is running on v0.0.20), but `UpdateInstalled=False/Pending` means an update is queued and waiting for a module dependency to settle.

## Check the DKP controller logs

If the status conditions do not provide enough detail for diagnosis, check the controller logs:

```bash
d8 k logs deployments/deckhouse -n d8-system | grep <APPLICATION_NAME>
```

## Check application pod logs

To list the pods created by the application, run the following command:

```bash
d8 k get pods -n <NAMESPACE> -l app.kubernetes.io/instance=<APPLICATION_NAME>
```

To view logs for a specific pod, run:

```bash
d8 k logs -n <NAMESPACE> <POD_NAME>
```

To view logs for a specific deployment prefixed with the instance name, run:

```bash
d8 k logs -n <NAMESPACE> deployments/<APPLICATION_NAME>-<RESOURCE_NAME>
```

## Common conditions and their meaning

| Condition | Status=False reason | What to check |
|---|---|---|
| `Installed` | `InstallFailed` | DKP controller logs, check settings against OpenAPI schema |
| `UpdateInstalled` | `Pending` | Dependent module convergence — check `d8` module conditions |
| `UpdateInstalled` | `UpdateFailed` | Specified `packageVersion` does not exist in the repository — verify with `d8 k get apv -l package=<name>` |
| `ConfigurationApplied` | `ConfigurationFailed` | Settings validation error — check against [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) schema |
| `Scaled` | `NotScaled` | Pods not ready — check pod events with `d8 k describe pod` |
| `Ready` | `NotReady` | One or more conditions above are not satisfied |
