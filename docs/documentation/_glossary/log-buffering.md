---
title: Log buffering
description: Temporary accumulation of logs in a buffer before they are sent to the storage system.
lang: en
---

Buffering keeps logs during temporary connection issues with the storage and sends them when the connection is restored, smooths load spikes on the storage and reduces the number of network requests by sending logs in batches. It is configured with the `buffer` parameter of [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) in the [log pipeline](glossary.html#log-pipeline).

For details, see [Log collection and delivery](admin/configuration/logging/delivery.html).
