---
title: Overview
permalink: en/admin/configuration/logging/
description: "Configure logging in Deckhouse Kubernetes Platform with built-in log collection, delivery, and storage. Centralized logging solution for cluster monitoring and troubleshooting."
---

Deckhouse Kubernetes Platform (DKP) provides built-in tools for log collection,
delivery, and short-term storage.

DKP logging capabilities:

- Collect logs from cluster pods and nodes.
- Process logs, including metadata enrichment and message filtering.
- Deliver to various storage and analysis systems, including Loki, Elasticsearch, Splunk, and others.
- Short-term log storage in the cluster with search and visualization capabilities through Grafana.

The following sections describe how to configure log collection and delivery,
as well as organize their short-term storage within the cluster.
