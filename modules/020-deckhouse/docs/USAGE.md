---
title: "The deckhouse module: usage"
---

### Usage

```yaml
deckhouse: |
  logLevel: Debug
  bundle: Minimal
  releaseChannel: RockSolid
```

### Update windows configuration

Update every day from 8:00 to 15:00 and from 20:00 to 23:00
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

Update from 13:00 to 18:30 at Tuesday and Saturday
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

If update windows is not set - Deckhouse will update as soon as new release will be created

---
*Manual update confirmation*
```yaml
deckhouse: |
  ...
  releaseChannel: Stable
  update:
    mode: Manual
```

You will have to approve every Deckhouse's release (except patch). For example:
```sh
kubectl patch DeckhouseRelease v1-25-0 --type=merge -p='{"approved": true}'
```

*Attention* Patch versions (1.25.1, 1.25.2, 1.25.3, etc) will be installed without approve and out of update windows
