---
title: "Loki: FAQ"
type:
  - instruction
search: Loki
---

{% raw %}

## Alert LokiDiscardedSamplesWarning

The **`LokiDiscardedSamplesWarning`** alert fires when **Loki** discards log samples during ingestion because they **fail validation** or **exceed ingestion limits**.

This means that the **log shipper** is trying to send data that Loki rejects.

---

### Possible causes

1. **Ingestion or stream size limits exceeded**
- Loki enforces limits on:
  - the number of unique labels per stream;
  - the length of label names and values;
  - the number of streams per client;
  - the batch size during ingestion.

A detailed description of each reason can be found in the [official Loki documentation](https://grafana.com/docs/loki/latest/operations/request-validation-rate-limits/#validation-errors).

---

### What to do

1. **Check the reason in the metric**
- The alert includes the following field:

  ```text
  Samples are discarded because of "{{ $labels.reason }}"
  ```

  This field indicates which limit Loki hit (`reason`).

2. **Find and adjust the corresponding limit in the ModuleConfig**
  - Go to **MC** and edit the limit by its name (`reason`) in Loki’s configuration. The list of tunable parameters is available in the [Loki module configuration documentation](configuration.html#parameters-lokiconfig).

- For example:
  - `rate_limited` → increase `ingestionRateMB`, or `ingestionBurstSizeMB` if short ingestion spikes are expected. Alternatively, consider limiting the amount of logs sent on the application side.

{% endraw %}
