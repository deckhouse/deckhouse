---
title: "How to start module in the DKP cluster?"
permalink: en/module-development/run/
---

This section covers the process of running a configured module in a cluster managed by Deckhouse Kubernetes Platform (DKP).

Follow these steps to run the module in a cluster:

- [Define ModuleSource](#module-source) (the [ModuleSource](../../cr.html#modulesource) resource).
- _(optional)_ Define the [module update policy](#module-update-policy) (the [ModuleUpdatePolicy](../../cr.html#moduleupdatepolicy) resource).
- [Enable the module in the cluster](#enabling-the-module) (the [ModuleConfig](../../cr.html#moduleconfig) resource).
  
## Module source

Create a [ModuleSource](../../cr.html#modulesource) resource to set the source to fetch module information from. This resource will contain the address of the container registry to pull modules from, authentication parameters, and other settings.

An example of a ModuleSource resource:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: example
spec:
  registry:
    repo: registry.example.com/deckhouse/modules
    dockerCfg: <base64 encoded credentials>
```

After the ModuleSource resource is created, DKP will start to perform periodic (every three minutes) data synchronization with the module source (fetching information about the modules available in the source).

Use the following command to check the synchronization status:

```shell
kubectl get ms
```

If the synchronization is successful, you will see output similar to the one below:

```shell
$ kubectl get ms
NAME        COUNT   SYNC   MSG
example     2       16s    Ready
```

If there are synchronization errors, the `MSG` column will contain a general description of the error, e.g.:

```console
$ kubectl get ms
NAME        COUNT   SYNC   MSG
example     2       16s    Some errors occurred. Inspect status for details
```

Detailed error information can be found in the `pullError` field in the status of the ModuleSource resource.

For example, here's how you can get detailed error description from the `example` module source:

```console
$ kubectl get ms example -o jsonpath='{range .status.modules[*]}{.name}{" module error:\n\t"}{.pullError}{"\n"}{end}'
module-1 module error:
  fetch image error: GET https://registry.example.com/v2/deckhouse/modules/module-1/release/manifests/stable: MANIFEST_UNKNOWN: manifest unknown; map[Tag:stable]
module-2 module error:
  fetch image error: GET https://registry.example.com/v2/deckhouse/modules/module-2/release/manifests/stable: MANIFEST_UNKNOWN: manifest unknown; map[Tag:stable]
```

If synchronization is successful, the `.status.modules` field of the ModuleSource resource will contain a list of modules ready to be enabled in the cluster.

Here is an example of how you can get a list of modules available from the `example` module source:

```console
$ kubectl get ms example -o jsonpath='{.status.modules[*].name}'
module-1 module-2
```

The complete list of modules available from all module sources created in the cluster can be retrieved using the following command:

```shell
kubectl get ms  -o jsonpath='{.items[*].status.modules[*].name}'
```

After creating the ModuleSource resource and successful synchronization, _modules_ — [Module](../../cr.html#module) resources should start appearing in the cluster (DKP creates them automatically, you do not need to create them). You can view the list of modules using the following command:

```shell
kubectl get module
```

Example of getting a list of modules:

```console
$ kubectl get module
NAME       WEIGHT   SOURCE   PHASE       ENABLED   READY
module-one                   Available   False     False                      
module-two                   Available   False     False                      
```

To get additional information about the module, use the following command:

```shell
kubectl get module module-one -oyaml
```

Example of output:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  creationTimestamp: "2024-12-12T10:49:40Z"
  generation: 1
  name: module-one
  resourceVersion: "241504954"
  uid: 3ae75474-8e96-4105-a939-6df71cba82d8
properties:
  availableSources:
  - example
status:
  conditions:
  - lastProbeTime: "2024-12-12T10:49:41Z"
    lastTransitionTime: "2024-12-12T10:49:41Z"
    message: disabled
    reason: Disabled
    status: "False"
    type: EnabledByModuleConfig
  - lastProbeTime: "2024-12-12T10:49:41Z"
    lastTransitionTime: "2024-12-12T10:49:41Z"
    status: "False"
    type: EnabledByModuleManager
  - lastProbeTime: "2024-12-16T15:46:26Z"
    lastTransitionTime: "2024-12-12T10:49:41Z"
    message: not installed
    reason: NotInstalled
    status: "False"
    type: IsReady
  phase: Available
```

You can find available sources from which the module can be downloaded in the Module (there is only one in the example).

Next, you need to enable the module. To do this, you need to create a ModuleConfig with the name of the module.

The parameter `enabled` in ModuleConfig is responsible for enabling the module. If the module is available from multiple sources (resource ModuleSource), the required source can be specified in the `source` parameter.

The update policy (the name of the ModuleUpdatePolicy) can be specified in the `updatePolicy` parameter. It is not necessary to specify the update policy; in this case, it will be inherited from the Deckhouse update parameters.

Example of ModuleConfig for enabling the module `module-one` from the source `example`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: module-one
spec:
  enabled: true
  source: example
```

After turning on the module, it should enter the download phase:

```shell
$ kubectl get module module-one
NAME        WEIGHT   SOURCE   PHASE         ENABLED   READY
module-one           example  Downloading   False     False
```

{% alert level="warning" %}
If the module has not entered the download phase, check the module source (ModuleSource), as the module may not be able to download.
{% endalert %}

After a successful download, the module will enter the installation phase (`Installing`):

```shell
$ kubectl get module module-one
NAME        WEIGHT   SOURCE   PHASE         ENABLED   READY
module-one  900      example  Installing    False     False
```

If the module was successfully installed, it will enter the `Ready` phase:

```shell
$ kubectl get module module-one
NAME        WEIGHT   SOURCE   PHASE  ENABLED  READY
module-one  900      example  Ready  True     True
```

Example of a Module object in the cluster when the module has been successfully installed:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  creationTimestamp: "2024-11-18T15:34:15Z"
  generation: 1
  name: module-one
  resourceVersion: "242153004"
  uid: 7111cee7-50cd-4ecf-ba20-d691b13b0f59
properties:
  availableSources:
  - example
  releaseChannel: Stable
  requirements:
    deckhouse: '> v1.63.0'
    kubernetes: '> v1.25.0'
  source: example
  version: v0.7.24
  weight: 910
status:
  conditions:
  - lastProbeTime: "2024-12-12T15:49:35Z"
    lastTransitionTime: "2024-12-12T15:49:35Z"
    status: "True"
    type: EnabledByModuleConfig
  - lastProbeTime: "2024-12-17T09:35:27Z"
    lastTransitionTime: "2024-12-12T15:49:39Z"
    status: "True"
    type: EnabledByModuleManager
  - lastProbeTime: "2024-12-17T09:35:27Z"
    lastTransitionTime: "2024-12-17T09:35:25Z"
    status: "True"
    type: IsReady
  - lastProbeTime: "2024-12-17T09:32:50Z"
    lastTransitionTime: "2024-12-17T09:32:50Z"
    status: "False"
    type: IsOverridden
  hooksState: 'v0.7.24/hooks/moduleVersion.py: ok'
  phase: Ready
```

In the Module, you can see the current installed version of the module, its size, the source from which it was downloaded, its dependencies, and the release channel.

In case of any errors, the module will enter the `Error` phase:

```console
$ kubectl get module module-one
NAME        WEIGHT   SOURCE   PHASE  ENABLED  READY
module-one  910      example  Error  True     Error
```

If the enabled module has several available sources, and a source for the module is not explicitly selected in its ModuleConfig, the module will enter the `Conflict` phase:

```console
$ kubectl get module module-one
NAME        WEIGHT   SOURCE   PHASE     ENABLED  READY
module-one                    Conflict  Fasle    False
```

To resolve the conflict, specify the source of the module (ModuleSource name) explicitly in ModuleConfig.

After downloading the module, the module releases will appear in the cluster — ModuleRelease objects.

You can view the list of releases using the following command:

```shell
kubectl get mr
```

An example of retrieving the list of module releases:

```console
$ kubectl get mr
NAME                       PHASE        UPDATE POLICY   TRANSITIONTIME   MESSAGE
module-one-v0.7.23         Superseded   deckhouse       33h              
module-one-v0.7.24         Deployed     deckhouse       33h              
module-two-v1.2.0          Superseded   deckhouse       48d              
module-two-v1.2.1          Superseded   deckhouse       48d              
module-two-v1.2.3          Deployed     deckhouse       48d              
module-two-v1.2.4          Superseded   deckhouse       44d              
module-two-v1.2.5          Pending      deckhouse       44d              Waiting for the 'release.deckhouse.io/approved: \"true\"' annotation
```

If the module release is in the `Superseded` status, it means that the module release is outdated, and there is a newer release that has replaced it.

{% alert level="warning" %}
If a module release is `Pending`, it means that manual confirmation is required to install it (see [module update policy](#module-update-policy) below). You can confirm the module release using the following command (specify the moduleRelease name):

```shell
kubectl annotate mr <module_release_name> modules.deckhouse.io/approved="true"
```

{% endalert %}

### Switching the module to a different module source

Follow these steps to deploy a module from a different module source:
1. Create a new [ModuleSource resource](#module-source).

1. Specify it in the `source` field in ModuleConfig.

1. Ensure that new module releases (ModuleRelease objects) are created from the new module source in accordance with the update policy:

   ```shell
   kubectl get mr
   ```

## Module update policy

The module update policy refers to the rules that DKP uses to update modules in the cluster. It is set by the [ModuleUpdatePolicy](../../cr.html#moduleupdatepolicy) resource with the following settings:
- module update mode (automatic, manual, updates are disabled);
- the release channel to use for updates;
- time windows for automatic updates during which the module update is permitted.

You do not have to create the ModuleUpdatePolicy resource. If the update policy for a module is not defined (there is no corresponding ModuleUpdatePolicy resource), the update settings match the update settings of DKP (the [update](../../modules/002-deckhouse/configuration.html#parameters-update) parameter of the `deckhouse` module).

Example of the ModuleUpdatePolicy resource, whose update policy allows automatic module updates on Mondays and Wednesdays from 13:30 to 14:00 UTC:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ModuleUpdatePolicy
metadata:
  name: example-update-policy
spec:
  releaseChannel: Alpha
  update:
    mode: Auto
    windows:
    - days:
      - "Mon"
      - "Wed"
      from: "13:30"
      to: "14:00"
```

The update policy is specified in the `updatePolicy` field in ModuleConfig.

## Enabling the module

Before enabling the module, make sure that it can be enabled. Run the following command to list all the available DKP modules:

```shell
kubectl get modules
```

The module must be in the list.

Below is an example of the output:

```console
$ kubectl get module
NAME       WEIGHT   SOURCE   PHASE       ENABLED   READY
...
module-one                   Available   False     False                      
module-two                   Available   False     False     
...
```

It shows that the `module-one` module can be enabled.

If the module is not in the list, check that [module source](#module-source) is defined and the module is listed in the module source. Also check the [update policy](#module-update-policy) of the module (if defined). If the module update policy is not defined, it matches the DKP update policy (the [releaseChannel](../../modules/002-deckhouse/configuration.html#parameters-releasechannel) parameter and the [update](../../modules/002-deckhouse/configuration.html#parameters-update) section of the `deckhouse` module parameters).

You can enable the module similarly to built-in DKP modules using any of the following methods:
- Run the command below (specify the name of the module):

  ```shell
  kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module enable <MODULE_NAME>
  ```

- Create a `ModuleConfig` resource containing the `enabled: true` parameter and module settings..

  Below is an example of a [ModuleConfig](../../cr.html#moduleconfig) that enables and configures the `module-one` module in the cluster:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  metadata:
    name: module-one
  spec:
    enabled: true
    settings:
      parameter: value
    version: 1
  ```

### Troubleshooting

If there were errors while enabling a module in the cluster, you can learn about them as follows:
- View Deckhouse log:

  ```shell
  kubectl -n d8-system logs -l app=deckhouse
  ```

- View the Module object in more detail:

  ```console
  kubectl get module module-one -oyaml
  ```
  
- View the ModuleConfig object of the module.

  Here is an example of the error message for `module-one`:

  ```console
  $ kubectl get moduleconfig module-1
  NAME        ENABLED   VERSION   AGE   MESSAGE
  module-one  true                7s    Ignored: unknown module name
  ```

- View the ModuleSource object.

  Example output if the module source has problems with downloading the module:

  ```console
  $ kubectl get ms
  NAME        COUNT   SYNC   MSG
  example     2       16s    Some errors occurred. Inspect status for details
  ```

Similar to [DeckhouseRelease](../../cr.html#deckhouserelease) (a DKP release resource), modules have a [ModuleRelease](../../cr.html#modulerelease) resource. DKP creates ModuleRelease resources based on what is stored in the container registry. When troubleshooting module issues, check the ModuleRelease available in the cluster as well:

```shell
kubectl get mr
```

Output example:

```console
$ kubectl get mr
NAME                 PHASE        UPDATE POLICY          TRANSITIONTIME   MESSAGE
module-1-v1.23.2     Pending      example-update-policy  3m               Waiting for the 'release.deckhouse.io/approved: "true"' annotation
```

The example output above illustrates ModuleRelease message when the update mode ([update.mode](../../cr.html#moduleupdatepolicy-v1alpha1-spec-update-mode) of the ModuleUpdatePolicy is set to `Manual`. In this case, you must manually confirm the installation of the new module version by adding the `modules.deckhouse.io/approved="true"` annotation to the ModuleRelease:

```shell
kubectl annotate mr module-1-v1.23.2 modules.deckhouse.io/approved="true"
```
