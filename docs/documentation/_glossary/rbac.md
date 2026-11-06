---
title: Role model
description: Role-based access control (RBAC), a mechanism that defines which users or services can perform specific actions within projects or the system.
lang: en
---

Deckhouse Platform supports two role models: the granular one, which is recommended and is built on [use roles](glossary.html#use-role) and [manage roles](glossary.html#manage-role), and the basic one with [high-level roles](glossary.html#high-level-role). Both models are supported by the [`user-authz`](/modules/user-authz/) module and can be used at the same time: the permissions they grant are summed up.

For details, see [Authorization](admin/configuration/access/authorization/).
