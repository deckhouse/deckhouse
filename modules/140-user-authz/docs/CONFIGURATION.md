---
title: "The user-authz module: configuration"
---

{% include module-bundle.liquid %}

> **Caution!** We strongly do not recommend creating Pods and ReplicaSets – these objects are secondary and should be created by other controllers. Access to creating and modifying Pods and ReplicaSets is disabled.
>
> **Caution!** Currently, the multi-tenancy mode (namespace-based authorization) is implemented according to a temporary scheme and **isn't guaranteed to be entirely safe and secure**! The `allowAccessToSystemNamespaces` and `limitNamespaces options` in the CR will no longer be applied if the authorization system's webhook is unavailable for some reason. As a result, users will have access to all namespaces. After the webhook availability is restored, the options will become relevant again.

## Parameters

<!-- SCHEMA -->

All access rights are configured using [Custom Resources](cr.html).
