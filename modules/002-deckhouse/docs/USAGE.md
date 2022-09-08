---
title: "The deckhouse module: usage"
---

## Usage

Below is a simple example of the module configuration:

```yaml
deckhouse: |
  logLevel: Debug
  bundle: Minimal
  releaseChannel: RockSolid
```

You can also configure additional parameters.

## Setting up the update mode

Deckhouse will update as soon as a new release will be created if update windows are not set and the update mode is `Auto`.

Patch versions (e.g. updates from `1.26.1` to `1.26.2`) are installed without confirmation and without taking into account update windows.

> You can also configure node disruption update window in CR [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup) (the `disruptions.automatic.windows` parameter).

### Update windows configuration

You can configure the time when Deckhouse will install updates by specifying the following parameters in the module configuration:

```yaml
deckhouse: |
  ...
  releaseChannel: Stable
  update:
    windows: 
      - from: "8:00"
        to: "15:00"
      - from: "20:00"
        to: "23:00"
```

Here updates will be installed every day from 8:00 to 15:00 and from 20:00 to 23:00.

You can also set up updates on certain days, for example, on Tuesdays and Saturdays from 13:00 to 18:30:

```yaml
deckhouse: |
  ...
  releaseChannel: Stable
  update:
    windows: 
      - from: "8:00"
        to: "15:00"
        days:
          - Tue
          - Sat
```

### Manual update confirmation

If necessary, it is possible to enable manual confirmation of updates. This can be done as follows:

```yaml
deckhouse: |
  ...
  releaseChannel: Stable
  update:
    mode: Manual
```

In this mode, it will be necessary to confirm each minor Deckhouse updates (excluding patch versions).

Manual confirmation of the update to the version `v1.26.0`:

```shell
kubectl patch DeckhouseRelease v1-26-0 --type=merge -p='{"approved": true}'
```

### Manual disruption update confirmation

If necessary, it is possible to enable manual confirmation of disruptive updates (updates that change the default values or behavior). This can be done as follows:

```yaml
deckhouse: |
  ...
  releaseChannel: Stable
  update:
    disruptionApprovalMode: Manual
```

In this mode, it will be necessary to confirm each minor disruptive update with an annotation:

```shell
kubectl annotate DeckhouseRelease v1-36-0 release.deckhouse.io/disruption-approved=true
```

### Deckhouse update notification

In `Auto` update mode you can set up a notification of upcoming deckhouse releases, e.g:

```yaml
deckhouse: |
  ...
  update:
    mode: Auto
    notification:
      webhook: https://release-webhook.mydomain.com
```

With these settings, a POST request with `Content-Type: application/json` and the following content will come to the address `https://release-webhook.mydomain.com`:

```json
{
  "version": "1.36", 
  "requirements":  { "k8s": "1.20.0" },
  "changelogLink": "https://github.com/deckhouse/deckhouse/changelog/1.36.md",
  "applyTime": "2023-01-01T14:30:00Z00:00",
  "message": "New Deckhouse Release 1.36 is available. Release will be applied at: Friday, 01-Jan-22 14:30:00 UTC"
}
```

- `version` - A string, the version of the new release
- `requirements` - object, the requirements for the new release
- `changelogLink` - string, a link to the Changelog documentation of the new release
- `applyTime` - string, date and time of the update (calculated by the readiness of the release and your update windows). Format: RFC3339
- `message` - a string, a text message about the availability and time of the update

#### Minimum update notification time

This setting is as follows:

```yaml
deckhouse: |
  ...
  update:
    mode: Auto
    notification:
      webhook: https://release-webhook.mydomain.com
      minimalNotificationTime: 8h
```

and configures the minimum time before the update, in which the notification should come. The Deckhouse update mechanism ensures
that Deckhouse will not update before this time. If you use update windows, the Deckhouse update will happen at the next possible update window.

## Collect debug info

Read [the FAQ](faq.html#how-to-collect-debug-info) to learn more about collecting debug information.
