---
title: Extended monitoring
description: A monitoring mode of the [`extended-monitoring`](/modules/extended-monitoring/) module that adds monitoring of secrets and TLS certificate expiration, collection of cluster events as metrics, checks of image availability in the registry and monitoring of objects in labelled namespaces.
lang: en
---

Objects are monitored in the namespaces that carry the label `extended-monitoring.deckhouse.io/enabled=""`. Image availability is checked for the images used by Deployments, StatefulSets, DaemonSets and CronJobs.

For details, see [Monitoring in Deckhouse Platform](admin/configuration/monitoring/).
