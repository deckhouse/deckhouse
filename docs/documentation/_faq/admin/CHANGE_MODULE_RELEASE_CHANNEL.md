---
title: How do I change the release channel for a module?
lang: en
---

A module can be built-in to DKP or connected from a module source (defined using [ModuleSource](reference/api/cr.html#modulesource)). Built-in modules have a common release cycle with DKP and are updated together with DKP. **The release channel of a built-in module always matches the DKP release channel.** A module connected from a source has its own release cycle, which is independent of the DKP release cycle. **The release channel of a module connected from a source can be changed.**

Below is the process of changing the release channel for a module connected from a source.

By default, the release channel for modules is inherited from the DKP release channel (specified in the [`releaseChannel`](/modules/deckhouse/configuration.html#parameters-releasechannel) parameter of the `deckhouse` ModuleConfig). For more information about release channels, see the [Release Channels](architecture/module-development/versioning/#release-channels) section.

For modules from a source, the release channel is specified using [ModuleUpdatePolicy](reference/api/cr.html#moduleupdatepolicy), which is then _linked_ to the module via the `updatePolicy` parameter in ModuleConfig.

To change the release channel for a module from a source, follow these steps:

1. Define the module update policy.

   Create a [ModuleUpdatePolicy](reference/api/cr.html#moduleupdatepolicy) where you specify the release channel in the `releaseChannel` parameter.

   Example ModuleUpdatePolicy:

   ```yaml
   apiVersion: deckhouse.io/v1alpha2
   kind: ModuleUpdatePolicy
   metadata:
     name: my-module-policy
   spec:
     releaseChannel: Alpha
     # If necessary, specify the update mode and update windows.
     # update:
     #   mode: AutoPatch
     #   windows: []
   ```

   Ensure that the policy has been created:

   ```shell
   d8 k get mup my-module-policy
   ```

   Output example:

   ```console
   NAME               RELEASE CHANNEL   UPDATE MODE
   my-module-policy   Alpha             AutoPatch
   ```

1. Link the update policy to the module.

   Specify the name of the created update policy in the [updatePolicy](reference/api/cr.html#moduleconfig-v1alpha1-spec-updatepolicy) parameter of the corresponding module's ModuleConfig.

   To edit the ModuleConfig, use the command (specify the module name):

   ```shell
   d8 k edit mc my-module
   ```

   Example ModuleConfig:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: my-module
   spec:
     enabled: true
     # ModuleUpdatePolicy name
     updatePolicy: my-module-policy
   ```

When you change the module's release channel, its version will change according to the configured update mode.

To view the current release channel of the module and other information about the module's state in the cluster, use the corresponding [Module](reference/api/cr.html#module) object.

Example command to get information about the module:

```shell
d8 k get module my-module -o yaml
```

The update policy used will be specified in the `properties.updatePolicy` field, and the current release channel in the `properties.releaseChannel` field. Example output:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: my-module
  # ...
properties:
  # ...
  releaseChannel: Alpha # Module release channel.
  updatePolicy: my-module-policy # Module update policy.
  version: v1.16.10  # Module version.
  # ...
```
