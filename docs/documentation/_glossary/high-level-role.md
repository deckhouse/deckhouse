---
title: High-level role
description: A role of the basic role model of Deckhouse Platform, provided by the [`user-authz`](/modules/user-authz/) module on top of Kubernetes RBAC. User, PrivilegedUser, Editor, Admin, ClusterEditor, ClusterAdmin and SuperAdmin.
lang: en
---

High-level roles belong to the basic [role model](glossary.html#rbac). The [`user-authz`](/modules/user-authz/) module creates them as a set of ClusterRole objects and grants them to users and groups with [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) and [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule). Each role extends the previous one: `User` can view pods, logs and Deployments, `Admin` manages RBAC in a [namespace](glossary.html#namespace), `ClusterAdmin` has full access to cluster-wide resources.

The recommended granular role model uses [use roles](glossary.html#use-role) and [manage roles](glossary.html#manage-role) instead of high-level roles. The models are not compatible, automatic conversion is impossible, but they can be used at the same time: the permissions of both models are summed up.

For details, see [Basic authorization model](admin/configuration/access/authorization/rbac-current.html).
