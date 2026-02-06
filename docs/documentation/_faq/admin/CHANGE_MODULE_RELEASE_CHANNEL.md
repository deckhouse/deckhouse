---
title: How do I change the release channel for a module from source?
lang: en
---

Changing the release channel is possible for modules from the source (since their release cycle is not depends on the DKP release cycle). By default, the release channel for modules is inherited from the global one (specified in the [`settings.releaseChannel`](/modules/deckhouse/configuration.html#parameters-releasechannel) parameter). For more information about update channels, see the [Update Channels](reference/release-channels.html) section.

For modules from the source, the release channel is specified via the ModuleUpdatePolicy resource, which defines the update policy, which is then linked to the module via the `spec.updatePolicy` field in ModuleConfig.

To change the release channel for the module used in the cluster from the source, follow these steps:

1. Create or modify the [ModuleUpdatePolicy](reference/api/cr.html#moduleupdatepolicy) with the desired channel (specify the channel in the `spec.releaseChannel` field).

   Example of a manifest for ModuleUpdatePolicy:

   ```yaml
   apiVersion: deckhouse.io/v1alpha2
   kind: ModuleUpdatePolicy
   metadata:
     name: my-module-policy
   spec:
     releaseChannel: Alpha  # The release channel that must be installed for the module.
     update:
       mode: AutoPatch
       windows: []  # Optional: update windows.
   ```

1. Ensure that the policy has been created using the command:

   ```shell
   d8 k get mup
   ```

   Output example:

   ```console
   NAME               RELEASE CHANNEL   UPDATE MODE
   my-module-policy   Alpha             AutoPatch
   ```

1. Specify the name of the created policy in [ModuleConfig](reference/api/cr.html#moduleconfig) of the module for which the channel is being changed:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: my-module
   spec:
     enabled: true
     updatePolicy: my-module-policy  # ModuleUpdatePolicy name
   ```

When you change the release channel, the module will automatically start receiving versions from the new channel. If there is already a newer version in the new channel, it will be installed according to the configured update mode.

To make sure that the desired release channel is used for the module, use the command:

```shell
d8 k get module my-module -o yaml
```

The update policy used is specified in the `properties.updatePolicy` field. The current module release channel is specified in the `properties.releaseChannel` parameter. Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: my-module
  resourceVersion: "4095616"
  uid: d69fff12-54c4-4949-b82e-7d92f7ecf17a
properties:
  ...
  namespace: my-namespace
  releaseChannel: Alpha # Module release channel.
  requirements:
    deckhouse: '>= 1.71'
  source: deckhouse
  stage: General Availability
  updatePolicy: my-module-policy # Module update policy.
  version: v1.16.10
  weight: 900
  ...
```
