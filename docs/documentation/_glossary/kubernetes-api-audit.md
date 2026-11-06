---
title: Kubernetes API audit
description: Logging of requests to the Kubernetes API server according to an audit policy. By default the log is written to the `/var/log/kube-audit/audit.log` file on master nodes.
lang: en
---

Kubernetes API audit tracks requests to the API server and helps to investigate unexpected behaviour and to meet security requirements. Deckhouse Platform configures a basic audit policy by default: it skips frequent service updates such as Endpoints, Events and leader election and logs changes of pods, nodes, ServiceAccount objects and operations in system namespaces. The policy can be extended with custom rules.

The audit log is a source for [runtime audit](glossary.html#runtime-audit) and for [security events](glossary.html#security-event).

For details, see [Kubernetes API event audit](admin/configuration/security/events/kubernetes-api-audit.html).
