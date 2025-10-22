---
title: "The user-authz module: configuration"
---

> **Caution!** We strongly do not recommend creating Pods and ReplicaSets – these objects are secondary and should be created by other controllers. Access to creating and modifying Pods and ReplicaSets is disabled.
>
> **Caution!** Currently, the multi-tenancy mode (namespace-based authorization) is implemented according to a temporary scheme and **isn't guaranteed to be entirely safe and secure**! The `allowAccessToSystemNamespaces`, `namespaceSelector` and `limitNamespaces` options in the custom resource will no longer be applied if the authorization system's webhook is unavailable for some reason. As a result, users will have access to all namespaces. After the webhook availability is restored, the options will become relevant again.

All access rights are configured using [Custom Resources](cr.html).

<!-- SCHEMA -->
