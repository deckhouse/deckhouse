---
title: "The deckhouse module: usage"
---

## Usage

```yaml
deckhouse: |
  logLevel: Debug
  bundle: Minimal
  releaseChannel: RockSolid
```

## Setting up the update mode

> You can also configure node disruption update window in CR [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup) (the `disruptions.automatic.windows` parameter).

Deckhouse will update as soon as a new release will be created if update windows are not set and the update mode is Auto.

Patch versions (e.g. updates from `1.26.1` to `1.26.2`) are installed without confirmation and without taking into account update windows.

### Update windows configuration

Update every day from 8:00 to 15:00 and from 20:00 to 23:00:
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

Update from 13:00 to 18:30 at Tuesday and Saturday:
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
```yaml
deckhouse: |
  ...
  releaseChannel: Stable
  update:
    mode: Manual
```

In this mode, it will be necessary to confirm each minor Deckhouse updates (excluding patch versions).

Manual confirmation of the update to the version `v1.26.0-alpha.6`:
```shell
kubectl patch DeckhouseRelease v1-25-0 --type=merge -p='{"approved": true}'
```
