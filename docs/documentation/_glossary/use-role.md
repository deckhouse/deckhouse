---
title: Use role
description: A role of the granular role model for granting users permissions within one namespace, that is, access to namespaced module resources and standard namespaced Kubernetes resources.
lang: en
---

Use roles belong to the granular [role model](glossary.html#rbac). They are intended for users, typically application developers, who do not manage the cluster or its modules but need to create Ingress resources, configure authentication for applications or collect logs in their [namespace](glossary.html#namespace).

The [`user-authz`](/modules/user-authz/) module creates the roles `d8:use:role:viewer`, `d8:use:role:user`, `d8:use:role:manager` and `d8:use:role:admin`. A use role is granted only with RoleBinding. Administrator permissions are granted with [manage roles](glossary.html#manage-role).

For details, see [Granular authorization model](admin/configuration/access/authorization/rbac-experimental.html).
