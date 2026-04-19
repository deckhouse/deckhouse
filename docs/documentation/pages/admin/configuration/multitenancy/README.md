---
title: Multitenancy
permalink: en/admin/multitenancy.html
description: Multitenancy
---

Multitenancy is the ability to create isolated environments (projects) within a Kubernetes cluster.
Projects are similar to namespaces but offer more capabilities.
While namespaces are used for logical separation of resources in Kubernetes,
they do not, for example, restrict network communication, pod resource consumption, or host directory mounts.
These limitations make namespaces insufficient for modern development needs.
By default, namespaces also do not include logging, auditing, and vulnerability scanning.

Using projects helps address these limitations and offers the following benefits:

* For platform administrators:
  * **Consistency**: Administrators can create projects using a shared template,
  which ensures consistency and simplifies management.
  * **Security**: Projects ensure isolation for resources and access policies between different tenants,
  supporting a secure multitenant environment.
  * **Resource consumption**: Administrators can easily set resource quotas and limits for each project
  to prevent resource overconsumption.

* For platform users:
  * **Immediate start**: Developers can request projects created per templates from administrators
  to quickly start developing new applications.
  * **Isolation**: Each project provides an isolated environment,
  allowing developers to deploy and test their applications without affecting others.

{% alert level="warning" %}
[Secret copying](/modules/secret-copier/) across all namespaces is incompatible with projects in multitenancy mode.

This mode creates isolated environments for users within their projects,
while [`secret-copier`](/modules/secret-copier/) automatically distributes secrets to all namespaces.
If sensitive data is present in a user’s private environment,
it could lead to a data leak and a security model breach.
{% endalert %}

## Limitations

Projects has several limitations:

- Creating more than one namespace within a project is not supported. If you need multiple namespaces, create a separate project for each of them.
- Template resources are applied only to a single namespace whose name matches the project name.
