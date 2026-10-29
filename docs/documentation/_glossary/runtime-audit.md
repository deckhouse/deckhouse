---
title: Runtime audit
description: Detection of security threats in a running cluster by analysing Linux kernel events and Kubernetes API audit events.
lang: en
---

Runtime audit in Deckhouse Platform uses two sources of events: Linux kernel events, received through the eBPF driver of the Falco threat detection system, and [Kubernetes API audit](glossary.html#kubernetes-api-audit) events, received through a webhook. It detects attempts to exploit known vulnerabilities, signs of cryptocurrency miners and Kubernetes-specific threats such as shells launched in containers, privileged containers, mounting of insecure paths and reading of sensitive files.

Capturing kernel events requires Linux kernel 5.8 or later with eBPF support.

For details, see [Runtime audit](admin/configuration/security/events/runtime-audit.html).
