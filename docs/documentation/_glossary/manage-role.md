---
title: Manage role
description: A role of the granular role model for administering Deckhouse Platform or one of its subsystems, that is, cluster-wide resources, the modules of the subsystem and their system namespaces.
lang: en
---

Manage roles belong to the granular [role model](glossary.html#rbac). A manage role grants access to cluster-wide Kubernetes resources, to the ModuleConfig and cluster-wide resources of the modules of its [subsystem](glossary.html#subsystem), and to the [system namespaces](glossary.html#system-namespace) in which these modules run. It does not grant access to the namespaces of user applications.

The name of a manage role has the form `d8:manage:<SUBSYSTEM>:<ACCESS_LEVEL>`, for example `d8:manage:networking:manager`. The subsystem `all` covers all modules. Permissions within a namespace are granted with [use roles](glossary.html#use-role).

For details, see [Granular authorization model](admin/configuration/access/authorization/rbac-experimental.html).
