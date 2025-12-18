---
title: "The loki module: FAQ"
type:
  - instruction
search: Loki, LokiDiscardedSamplesWarning
description: "FAQ about the Loki module: troubleshooting LokiDiscardedSamplesWarning alert, resolving log ingestion issues, and configuring Loki limits"
---

{% raw %}

## Why is LokiDiscardedSamplesWarning alert triggered?

The [`LokiDiscardedSamplesWarning`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#loki-lokidiscardedsampleswarning) alert is triggered when Loki discards logs during data ingestion because they fail validation or exceed configured limits.

In other words, `log-shipper` is trying to send data that Loki rejects.

### Possible causes

Most likely, a limit on the size or number of streams has been exceeded.

Loki enforces limits on:

- The number of unique labels per stream
- The length of label names and values
- The number of streams per client
- The batch size during ingestion

For a detailed description of each limit, refer to the [official Loki documentation](https://grafana.com/docs/loki/latest/operations/request-validation-rate-limits/#validation-errors).

### Resolving the issue

1. Check the reason in the alert description.

   ```text
   Samples are being discarded because of "{{ $labels.reason }}"...
   ```

   The `{{ $labels.reason }}` field indicates which limit was exceeded and why Loki is rejecting logs.

1. Adjust the corresponding limit in the `loki` module configuration.

   For a description of configurable parameters, refer to the [module documentation](configuration.html#parameters-lokiconfig).

   For example, if the reason is `rate_limited`, increase the `ingestionRateMB` or `ingestionBurstSizeMB` values if temporary ingestion spikes are expected.
   Alternatively, you can limit the amount of logs sent on the application side.

{% endraw %}
