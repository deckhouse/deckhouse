---
title: Short-term log storage
description: The built-in in-cluster log storage of Deckhouse Platform based on Grafana Loki. It is integrated with log collection and added to Grafana as a data source.
lang: en
---

The storage is provided by the [`loki`](/modules/loki/) module, whose settings define the disk size, the retention period, the StorageClass and the resources. Logs of system components flow into it after the [log pipeline](glossary.html#log-pipeline) is configured. Log collection from user applications is configured separately. The storage does not support the high availability mode, so important logs are kept long-term in an external storage.

For details, see [Short-term log storage](admin/configuration/logging/storage.html).
