---
title: Namespace
description: A Kubernetes mechanism for grouping objects logically.
lang: en
---

Examples of Kubernetes objects that can be grouped by a namespace: Pod, Deployment, Service, Ingress, ConfigMap.

Namespaces are often used for isolation, but without additional configuration they do not provide it, unlike [projects](glossary.html#project) in Deckhouse Platform.

Namespaces are divided into [system](glossary.html#system-namespace) and [user](glossary.html#user-namespace) namespaces.
