---
title: "The monitoring-ping module: usage"
---

## Adding extra IP addresses to monitor

To monitor additional IP addresses, insert their names/hosts into the module config in the deckhouse ConfigMap as follows:

```yaml
monitoringPing: |
  externalTargets:
  - name: google-primary
    host: 8.8.8.8
  - name: yaru
    host: ya.ru
  - host: youtube.com
```

Note that Grafana uses the `name` field to display the related data. If the `name` field is skipped, you must fill in details to the `host` field (mandatory).
