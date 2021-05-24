---
title: "The dashboard module"
---

This module installs Kubernetes Dashboard [Web UI](https://github.com/kubernetes/dashboard) that allows you to manage applications running in the cluster. It is integrated with [user-authn](../../modules/150-user-authn/) and [user-authz](../../modules/140-user-authz/) modules (access to the cluster is based on the user ID and credentials).

When working over http, the module has minimum rights according to the `User` role defined in the [user-authz](../../modules/140-user-authz/) module.

> Note that you cannot enable the dashboard module if `user-authz` is disabled.

Kubernetes Dashboard, among other things, allows you to:
- manage pods and other high-level resources;
- exec to containers via the web console for debugging;
- browse logs of specific containers.
