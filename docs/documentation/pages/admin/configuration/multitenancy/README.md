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
  * **Resource access management**: Administrators can control which cluster-wide resources
  (such as StorageClass, ClusterIssuer, ClusterRole, and LoadBalancerClass) each project may use,
  and set per-project default values. For details, refer to [grants section in the usage guide](multitenancy/project-management.html#managing-access-to-cluster-wide-resources).

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

## Additional namespaces

A project is not limited to a single namespace: if an application needs several namespaces (for example, a separate one for a cache or a queue), they can be added to the project as [additional namespaces](multitenancy/project-management.html#additional-project-namespaces). The project's access grants and namespaced template policies (network isolation, log shipping) automatically apply to every namespace of the project, not just the main one.
