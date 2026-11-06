---
title: Security event
description: A structured record of an action or fact that is significant from an information security perspective, extracted from the logs of applications and Kubernetes infrastructure components.
lang: en
---

Deckhouse Platform collects security events from container sources, that is, application logs in pods, and from cluster sources, such as the [Kubernetes API audit](glossary.html#kubernetes-api-audit) log and node authentication logs. The [`log-shipper`](/modules/log-shipper/) module pre-selects log records, and the [`security-events-manager`](/modules/security-events-manager/) module parses them, brings them to a unified format with a mandatory set of attributes, enriches them with context and delivers them to storage and analytics systems such as Loki, Elasticsearch, Kafka or Splunk.

For details, see [Security events](admin/configuration/security/events/security-events.html).
