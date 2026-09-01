---
title: Logging
permalink: en/virtualization-platform/documentation/admin/platform-management/logging/
---

Deckhouse Virtualization Platform (DVP) provides built-in tools for collecting,
delivering, and short-term storage of logs.

DVP logging capabilities:

- Collecting logs from pods and cluster nodes.
- Log processing, including metadata enrichment and message filtering.
- Delivery to various storage and analysis systems, including Loki, Elasticsearch, Splunk, and others.
- Short-term log storage in the cluster with search and visualization capabilities through Grafana.
- View current logs without deploying a storage system, using [lightweight logs](lightweight.html).

The following sections describe how to:

- Configure log collection and delivery.
- Organize their short-term storage within the cluster.
- Use lightweight logs to view the current state of pods without extra resource costs.
