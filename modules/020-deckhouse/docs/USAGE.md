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

## Collect debug info

We always appreciate helping users with debugging complex issues. Please follow these steps so that we can help you:

1. Collect all the necessary information by running the following command:

   ```sh
   kubectl -n d8-system exec deploy/deckhouse \
     -- deckhouse-controller collect-debug-info \
     > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
   ```

2. Send the archive to the [Deckhouse team](https://github.com/deckhouse/deckhouse/issues/new/choose) for further debugging.
