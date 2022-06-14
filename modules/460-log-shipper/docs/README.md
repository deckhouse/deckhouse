---
title: "The log-shipper module"
---

Module is created for creating log-pipeline on nodes with [Custom Resources](cr.html).

You can store logs using log-pipeline to Loki/Elasticsearch/Logstash storages.

### Log filters

There is a couple of filters to reduce the number of lines sent to the destination — `log filter` and `label filter`.

![log-shipper pipeline](../../images/460-log-shipper/log_shipper_pipeline.png)

They are executed right after concatenating lines together with the multiline log parser.

1. `label filter` — rules are executed against the metadata of a message. Fields in metadata (or labels) come from a source, so for different sources, we will have different fields for filtering. These rules are useful, for example, to drop messages from a particular container and for Pods with/without a label.
2. `log filter` — rules are executed against a message. It is possible to drop messages based on their JSON fields or, if a message is not JSON-formatted, use regex to exclude lines.

Both filters have the same structured configuration:
* `field` — the source of data to filter (most of the time it is a value of a label or a JSON field).
* `operator` — action to apply to a value of the field. Possible options are In, NotIn, Regex, NotRegex, Exists, DoesNotExist.
* `values` — this option has a different meanings for different operations:
  * DoesNotExist, Exists — not supported;
  * In, NotIn — a value of a field must / mustn't be in the list of provided values;
  * Regex, NotRegex — a value of a field must match any or mustn't match all the provided regexes (values).

You can find examples in the [Usage](usage.html) section of the documentation.

> NOTE: Extra labels are added on the `Destination` stage of the pipeline, so it is impossible to run queries against them.
