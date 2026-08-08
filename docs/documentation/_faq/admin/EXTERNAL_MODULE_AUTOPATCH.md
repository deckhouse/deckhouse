---
title: How do I configure an external module to update to patch versions only?
lang: en
---

By default, if a module has no dedicated update policy, the update mode and windows are inherited from the DKP settings.

If DKP is set to the `AutoPatch` mode, the external module will also automatically receive only patch versions within the current minor version. Moving to a new minor version will require manual approval. In this case, no additional configuration is needed.

For more information about DKP update modes, see [Configuring updates](admin/configuration/update/configuration.html#update-modes).

If you need to manage the module update mode independently of DKP, create a [ModuleUpdatePolicy](reference/api/cr.html#moduleupdatepolicy) with `update.mode: AutoPatch` and link it to the module via the `updatePolicy` parameter in ModuleConfig:

1. Create an update policy.

   Example ModuleUpdatePolicy:

   ```yaml
   apiVersion: deckhouse.io/v1alpha2
   kind: ModuleUpdatePolicy
   metadata:
     name: my-update-policy
   spec:
     releaseChannel: Stable
     update:
       mode: AutoPatch
   ```

   Ensure that the policy has been created:

   ```shell
   d8 k get mup my-update-policy
   ```

1. Link the policy to the module.

   Specify the policy name in the [updatePolicy](reference/api/cr.html#moduleconfig-v1alpha1-spec-updatepolicy) parameter of the module's ModuleConfig:

   ```shell
   d8 k edit mc module
   ```

   Example ModuleConfig:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: module
   spec:
     enabled: true
     updatePolicy: my-update-policy
   ```

In the `AutoPatch` mode, module patch versions (for example, from `v1.16.1` to `v1.16.2`) are applied automatically, taking update windows into account if they are configured. To move to a new minor version (for example, from `v1.16.*` to `v1.17.*`), approve the corresponding [ModuleRelease](reference/api/cr.html#modulerelease):

```shell
d8 k annotate mr module-v1.17.0 modules.deckhouse.io/approved="true"
```

Or using the [`d8`](/products/kubernetes-platform/documentation/v1/cli/d8/) CLI:

```shell
d8 system module approve module v1.17.0
```
