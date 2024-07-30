# How to disable alert rule during Deckhouse update

When deckhouse is updating you probably want to suppress some alerts (like PodIsNotReady)
This could be done by adding the annotation `d8_ignore_on_update: "true"` to a PrometheusRule

Example:

```yaml
- name: d8.deckhouse.availability
  rules:
  - alert: D8DeckhouseSelfTargetDown
    expr: max by (job) (up{job="deckhouse", scrape_source="self"} == 0)
    annotations:
      d8_ignore_on_update: "true"
      summary: Prometheus is unable to scrape Deckhouse metrics.
```

## How does it works

This annotations will mutate the rule expression inside Prometheus:

```text
expr: ($expr) and on() ((max(d8_is_updating) != 1) or on() absent(d8_is_updating))
```

`d8_is_updating` metric set inside update_deckhouse_image hook also become absent when Deckhouse pod is ready.
