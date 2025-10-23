---
title: FAQ
permalink: en/admin/configuration/update/faq.html
description: "Frequently asked questions about Deckhouse Kubernetes Platform updates. Update troubleshooting, configuration, and best practices for platform maintenance."
---

## How can I apply an update immediately, bypassing update windows, canary releases, and manual update mode?

To apply a Deckhouse Kubernetes Platform (DKP) update immediately,
add the annotation `release.deckhouse.io/apply-now: "true"` to the corresponding [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) resource.

{% alert level="warning" %}
This will bypass update windows, [canary release settings](../../../user/network/canary-deployment.html), and the [manual cluster update mode](configuration.html#manual-update-approval).
The update will be applied immediately after the annotation is set.
{% endalert %}

Example command to set the annotation and skip update windows for version `v1.56.2`:

```shell
d8 k annotate deckhousereleases v1.56.2 release.deckhouse.io/apply-now="true"
```

Example of a resource with the annotation set:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  annotations:
    release.deckhouse.io/apply-now: "true"
...
```

## How can I tell that an update is in progress?

During an update:

- The [`DeckhouseUpdating`](../../../reference/alerts.html#monitoring-deckhouse-deckhouseupdating) alert is active.
- The `deckhouse` Pod is not in the `Ready` state.
  If the Pod stays in a non-`Ready` state for a long time, it may indicate an issue with DKP that requires investigation.

## How can I tell that the update was successful?

If the [`DeckhouseUpdating`](../../../reference/alerts.html#monitoring-deckhouse-deckhouseupdating) alert is gone, the update has finished.

You can also check the status of DKP releases in the cluster with the following command:

```shell
d8 k get deckhouserelease
```

Example output:

```console
NAME       PHASE        TRANSITIONTIME   MESSAGE
v1.46.8    Superseded   13d
v1.46.9    Superseded   11d
v1.47.0    Superseded   4h12m
v1.47.1    Deployed     4h12m
```

The `Deployed` status means the cluster has switched to the corresponding version,
but it doesn’t guarantee that the update has been successful.

To ensure the update completed successfully, check the state of the `deckhouse` Pod with the following command:

```shell
d8 k -n d8-system get pods -l app=deckhouse
```

Example output:

```console
NAME                   READY  STATUS   RESTARTS  AGE
deckhouse-7844b47bcd-qtbx9  1/1   Running  0       1d
```

- If the Pod is `Running` and shows `1/1` under `READY`, it means the update completed successfully.
- If the Pod is `Running` but shows `0/1` under `READY`, it means the update is still in progress.
  If it stays like this for more than 20–30 minutes, it may indicate a problem with DKP that requires investigation.
- If the Pod is not `Running`, it may indicate a problem with DKP that requires investigation.

### If something goes wrong

- Check the logs using the following command:

  ```shell
  d8 k -n d8-system logs -f -l app=deckhouse | jq -Rr 'fromjson? | .msg'
  ```

- Collect debug information and contact [DKP technical support](/tech-support/).
- Ask for help from the [community](/community/).

## How can I know when a new DKP version is available for the cluster?

As soon as a new version appears on the configured release channel:

- The [`DeckhouseReleaseIsWaitingManualApproval`](../../../reference/alerts.html#monitoring-deckhouse-deckhousereleaseiswaitingmanualapproval) alert will appear if the cluster is in [manual update mode](configuration.html#manual-update-approval).
- A new [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) custom resource will be created.
  To see the list of releases, run `d8 k get deckhousereleases`.
  If the new version is in `Pending` state, it means it hasn’t been installed yet. Possible reasons:
  - [Manual update mode](configuration.html#manual-update-approval) is enabled.
  - Automatic update mode is enabled and [update windows](configuration.html#update-windows) are scheduled, but the window hasn’t started yet.
  - Automatic update mode is enabled and update windows are not scheduled,
    but the update is delayed by a random period to reduce load on the container image registry.
    The `status.message` field of the DeckhouseRelease resource will show a corresponding message.
  - The [`update.notification.minimalNotificationTime`](/modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) parameter is set, and the delay period hasn’t elapsed.

## How can I receive information about upcoming updates in advance?

You can get information about upcoming minor DKP version updates on the release channel in one of the following ways:

- Enable [manual update mode](configuration.html#manual-update-approval).
  A new [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) resource and the [`DeckhouseReleaseIsWaitingManualApproval`](../../../reference/alerts.html#monitoring-deckhouse-deckhousereleaseiswaitingmanualapproval) alert will appear when a new version is available.
- Enable [automatic update mode](configuration.html#automatic-update-mode) and set a delay using the [`minimalNotificationTime`](/modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) parameter.
  A new [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) resource will appear when a new version is available.
  If you also set a webhook URL in the [`update.notification.webhook`](/modules/deckhouse/configuration.html#parameters-update-notification-webhook) parameter,
  a notification will be sent about the upcoming update.

## How can I check DKP versions available in different release channels?

For information about the current DKP versions across all release channels, visit [releases.deckhouse.io](https://releases.deckhouse.io).

## What should I do if DKP is not receiving updates from the configured channel?

- Ensure the [correct release channel](../../../architecture/updating.html#release-channels) is configured.
- Check that DNS resolution for the Deckhouse image registry is working correctly.
  
  Get and compare the IP addresses of `registry.deckhouse.io` from both a node and the `deckhouse` Pod.
  They must match.

  Example of obtaining an IP of `registry.deckhouse.io` from a node:

  ```shell
  getent ahosts registry.deckhouse.io
  ```

  Example output:

  ```console
  185.193.90.38    STREAM registry.deckhouse.io
  185.193.90.38    DGRAM
  185.193.90.38    RAW
  ```

  Example of obtaining an IP of `registry.deckhouse.io` from the `deckhouse` Pod:

  ```shell
  d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- getent ahosts registry.deckhouse.io
  ```

  Example output:

  ```console
  185.193.90.38    STREAM registry.deckhouse.io
  185.193.90.38    DGRAM  registry.deckhouse.io
  ```

  If the resulted IPs do not match, check DNS settings on the node.
  Pay attention to the `search` domain list in `/etc/resolv.conf`, which affects name resolution in the `deckhouse` Pod.
  If the `search` parameter in `/etc/resolv.conf` file specifies a domain with wildcard DNS resolution configured,
  this may lead to incorrect IP address resolution for the Deckhouse image registry (see example below).

{% offtopic title="Example DNS settings that may cause issues resolving the Deckhouse image registry IP address…" %}

Below is an example of how DNS settings may result in different resolution behavior on the node and in a Kubernetes Pod:

- Example of `/etc/resolv.conf` on the node:

  ```text
  nameserver 10.0.0.10
  search company.my
  ```

  > On nodes, the default `ndot` setting is **1** (`options ndots:1`), while in Kubernetes Pods, it’s **5**.
  > This causes different resolution logic for DNS names with 5 or fewer dots on a node and on the Pod.

- The DNS zone `company.my` has a wildcard entry `*.company.my` that resolves to `10.0.0.100`.
  This means any undefined DNS name in the `company.my` zone resolves to `10.0.0.100`.

Taking into account the `search` parameter in `/etc/resolv.conf`, when accessing `registry.deckhouse.io` from a node,
the system will attempt to resolve the IP address for `registry.deckhouse.io`
(because it considers it fully qualified due to the default `options ndots:1` setting).

However, when accessing `registry.deckhouse.io` from a Kubernetes Pod,
considering the `options ndots:5` setting used by default in Kubernetes and the `search` parameter,
the system will first attempt to resolve the name `registry.deckhouse.io.company.my`.
This name will resolve to the IP address `10.0.0.100` because,
according to the `company.my` DNS zone's wildcard configuration,
`*.company.my` is resolved to `10.0.0.100`.
As a result, the Pod will fail to connect to the `registry.deckhouse.io` host and will be unable to download information about available Deckhouse updates.
{% endofftopic %}
