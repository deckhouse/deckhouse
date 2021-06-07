---
title: "The user-authz module: configuration"
---

This module is **enabled** by default. To disable it, add the following lines to the `deckhouse` ConfigMap:

```yaml
data:
  userAuthzEnabled: "false"
```

> **Caution!** We strongly do not recommend creating Pods and ReplicaSets – these objects are secondary and should be created by other controllers. Access to creating and modifying Pods and ReplicaSets is disabled.

> **Caution!** Currently, the multi-tenancy mode (namespace-based authorization) is implemented according to a temporary scheme and **isn't guaranteed to be entirely safe and secure**! The `allowAccessToSystemNamespaces` and `limitNamespaces options` in the CR will no longer be applied if the authorization system's webhook is unavailable for some reason. As a result, users will have access to all namespaces. After the webhook availability is restored, the options will become relevant again.

## Parameters

* `enableMultiTenancy` — enable namespace-based authorization.
  * Since this option is implemented via the [Webhook authorization plugin](https://kubernetes.io/docs/reference/access-authn-authz/webhook/), you will need to perform an additional configuration of [kube-apiserver](usage.html#configuring-kube-apiserver). You can use the [control-plane-manager](../../modules/040-control-plane-manager/) module to automate this process.
  * The default value is `false` (i.e., multi-tenancy is disabled).
* `controlPlaneConfigurator` — parameters of the [control-plane-manager](../../modules/040-control-plane-manager/) module.
  * `enabled` — passes parameters for configuring authz-webhook to the control-plane-manager module (see the parameters of the [control-plane-manager](../../modules/040-control-plane-manager/configuration.html#parameters) module).
    * If this parameter is disabled, the control-plane-manager module assumes that Webhook-based authorization is disabled by default. In this case (if no additional settings are provided), the control-plane-manager module will try to delete all references to the Webhook plugin from the manifest (even if you configure the manifest manually).
    * It is set to `true` by default.

All access rights are configured using [Custom Resources](cr.html).
