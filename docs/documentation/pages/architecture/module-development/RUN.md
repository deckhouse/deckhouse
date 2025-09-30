---
title: "How to run and verify a module in the DKP cluster"
permalink: en/architecture/module-development/run/
---

This section describes the process of running a module in a Deckhouse Kubernetes Platform (DKP) cluster, as well as connecting Deckhouse Module Tools for setting up validation and metrics collection.

Follow these steps to run the module in a cluster:

- [Define ModuleSource](#module-source) (the [ModuleSource](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulesource) resource).
- _(optional)_ Define the [module update policy](#module-update-policy) (the [ModuleUpdatePolicy](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleupdatepolicy) resource).
- [Enable the module in the cluster](#enabling-the-module) (the [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) resource).
  
## Module source

Create a [ModuleSource](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulesource) resource to set the source to fetch module information from. This resource will contain the address of the container registry to pull modules from, authentication parameters, and other settings.

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
d8 k get ms
```

If the synchronization is successful, you will see output similar to the one below:

```shell
$ d8 k get ms
NAME        COUNT   SYNC   MSG
example     2       16s    Ready
```

If there are synchronization errors, the `MSG` column will contain a general description of the error, e.g.:

```console
$ d8 k get ms
NAME        COUNT   SYNC   MSG
example     2       16s    Some errors occurred. Inspect status for details
```

Detailed error information can be found in the `pullError` field in the status of the ModuleSource resource.

For example, here's how you can get detailed error description from the `example` module source:

```console
$ d8 k get ms example -o jsonpath='{range .status.modules[*]}{.name}{" module error:\n\t"}{.pullError}{"\n"}{end}'
module-1 module error:
  fetch image error: GET https://registry.example.com/v2/deckhouse/modules/module-1/release/manifests/stable: MANIFEST_UNKNOWN: manifest unknown; map[Tag:stable]
module-2 module error:
  fetch image error: GET https://registry.example.com/v2/deckhouse/modules/module-2/release/manifests/stable: MANIFEST_UNKNOWN: manifest unknown; map[Tag:stable]
```

If synchronization is successful, the `.status.modules` field of the ModuleSource resource will contain a list of modules ready to be enabled in the cluster.

Here is an example of how you can get a list of modules available from the `example` module source:

```console
$ d8 k get ms example -o jsonpath='{.status.modules[*].name}'
module-1 module-2
```

The complete list of modules available from all module sources created in the cluster can be retrieved using the following command:

```shell
d8 k get ms  -o jsonpath='{.items[*].status.modules[*].name}'
```

After creating the ModuleSource resource and successful synchronization, _modules_ — [Module](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#module) resources should start appearing in the cluster (DKP creates them automatically, you do not need to create them). You can view the list of modules using the following command:

```shell
d8 k get module
```

Example of getting a list of modules:

```console
$ d8 k get module
NAME       STAGE    SOURCE   PHASE       ENABLED   READY
module-one                   Available   False     False                      
module-two                   Available   False     False                      
```

To get additional information about the module, use the following command:

```shell
d8 k get module module-one -oyaml
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

{% alert level="warning" %}
If there are mandatory parameters in the module configuration and the module is enabled without specifying them, a configuration validation error will occur. In this case, the `D8DeckhouseModuleValidationError` alert will appear, and the module will not be successfully activated.

To get more details, use the following command:

```shell
d8 k get mr -l module=<MODULE_NAME>
```

Make sure to specify the required configuration parameters in `ModuleConfig` according to the module’s documentation.
{% endalert %}

After turning on the module, it should enter the download phase:

```shell
$ d8 k get module module-one
NAME        STAGE    SOURCE   PHASE         ENABLED   READY
module-one           example  Downloading   False     False
```

{% alert level="warning" %}
If the module has not entered the download phase, check the module source (ModuleSource), as the module may not be able to download.
{% endalert %}

After a successful download, the module will enter the installation phase (`Installing`):

```shell
$ d8 k get module module-one
NAME        STAGE    SOURCE   PHASE         ENABLED   READY
module-one           example  Installing    False     False
```

If the module was successfully installed, it will enter the `Ready` phase:

```shell
$ d8 k get module module-one
NAME        STAGE    SOURCE   PHASE  ENABLED  READY
module-one           example  Ready  True     True
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
$ d8 k get module module-one
NAME        STAGE    SOURCE   PHASE  ENABLED  READY
module-one           example  Error  True     Error
```

If the enabled module has several available sources, and a source for the module is not explicitly selected in its ModuleConfig, the module will enter the `Conflict` phase:

```console
$ d8 k get module module-one
NAME        STAGE    SOURCE   PHASE     ENABLED  READY
module-one                    Conflict  False    False
```

To resolve the conflict, specify the source of the module (ModuleSource name) explicitly in ModuleConfig.

After downloading the module, the module releases will appear in the cluster — ModuleRelease objects.

You can view the list of releases using the following command:

```shell
d8 k get mr
```

An example of retrieving the list of module releases:

```console
$ d8 k get mr
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
d8 k annotate mr <module_release_name> modules.deckhouse.io/approved="true"
```

{% endalert %}

### Switching the module to a different module source

Follow these steps to deploy a module from a different module source:
1. Create a new [ModuleSource resource](#module-source).

1. Specify it in the `source` field in ModuleConfig.

1. Ensure that new module releases (ModuleRelease objects) are created from the new module source in accordance with the update policy:

   ```shell
   d8 k get mr
   ```

## Module update policy

The module update policy refers to the rules that DKP uses to update modules in the cluster. It is set by the [ModuleUpdatePolicy](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleupdatepolicy) resource with the following settings:
- module update mode (automatic, manual, updates are disabled);
- the release channel to use for updates;
- time windows for automatic updates during which the module update is permitted.

You do not have to create the ModuleUpdatePolicy resource. If the update policy for a module is not defined (there is no corresponding ModuleUpdatePolicy resource), the update settings match the update settings of DKP (the [update](/modules/deckhouse/configuration.html#parameters-update) parameter of the `deckhouse` module).

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
d8 k get modules
```

The module must be in the list.

Below is an example of the output:

```console
$ d8 k get module
NAME       STAGE    SOURCE   PHASE       ENABLED   READY
...
module-one                   Available   False     False                      
module-two                   Available   False     False     
...
```

It shows that the `module-one` module can be enabled.

If the module is not in the list, check that [module source](#module-source) is defined and the module is listed in the module source. Also check the [update policy](#module-update-policy) of the module (if defined). If the module update policy is not defined, it matches the DKP update policy (the [releaseChannel](/modules/deckhouse/configuration.html#parameters-releasechannel) parameter and the [update](/modules/deckhouse/configuration.html#parameters-update) section of the `deckhouse` module parameters).

You can enable the module similarly to built-in DKP modules using any of the following methods:
- Run the command below (specify the name of the module):

  ```shell
  d8 platform module enable <MODULE_NAME>
  ```

- Create a `ModuleConfig` resource containing the `enabled: true` parameter and module settings..

  Below is an example of a [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) that enables and configures the `module-one` module in the cluster:

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
  d8 k -n d8-system logs -l app=deckhouse
  ```

- View the Module object in more detail:

  ```console
  d8 k get module module-one -oyaml
  ```
  
- View the ModuleConfig object of the module.

  Here is an example of the error message for `module-one`:

  ```console
  $ d8 k get moduleconfig module-1
  NAME        ENABLED   VERSION   AGE   MESSAGE
  module-one  true                7s    Ignored: unknown module name
  ```

- View the ModuleSource object.

  Example output if the module source has problems with downloading the module:

  ```console
  $ d8 k get ms
  NAME        COUNT   SYNC   MSG
  example     2       16s    Some errors occurred. Inspect status for details
  ```

Similar to [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) (a DKP release resource), modules have a [ModuleRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulerelease) resource. DKP creates ModuleRelease resources based on what is stored in the container registry. When troubleshooting module issues, check the ModuleRelease available in the cluster as well:

```shell
d8 k get mr
```

Output example:

```console
$ d8 k get mr
NAME                 PHASE        UPDATE POLICY          TRANSITIONTIME   MESSAGE
module-1-v1.23.2     Pending      example-update-policy  3m               Waiting for the 'release.deckhouse.io/approved: "true"' annotation
```

The example output above illustrates ModuleRelease message when the update mode ([update.mode](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleupdatepolicy-v1alpha2-spec) of the ModuleUpdatePolicy is set to `Manual`. In this case, you must manually confirm the installation of the new module version by adding the `modules.deckhouse.io/approved="true"` annotation to the ModuleRelease:

```shell
d8 k annotate mr module-1-v1.23.2 modules.deckhouse.io/approved="true"
```

## Integrating Deckhouse Module Tools for Module Validation

To enable automatic validation of the module structure and, if needed, metrics reporting, you can integrate Deckhouse Module Tools (DMT) into your build process.

### For GitHub Projects

A dedicated [GitHub Action](https://github.com/deckhouse/modules-actions/blob/main/lint/action.yml) is available for integrating the DMT into your module.

To connect the DMT, add the following step to your build workflow configuration in `[project].github/workflows/build.yml`:

{% raw %}

```yaml
jobs:
  lint:
    runs-on: ubuntu-latest
    continue-on-error: true
    name: Linting
    steps:
      - uses: actions/checkout@v4
      - uses: deckhouse/modules-actions/lint@main
      env:
         DMT_METRICS_URL: ${{ secrets.DMT_METRICS_URL }}
         DMT_METRICS_TOKEN: ${{ secrets.DMT_METRICS_TOKEN }}
```

{% endraw %}

The `DMT_METRICS_URL` and `DMT_METRICS_TOKEN` variables are optional. If set, the DMT will send telemetry to the specified endpoint.

> If the module resides in the `deckhouse` GitHub organization, these variables will be automatically populated from the configured secrets.

A complete example of the configuration can be found in the [build_dev.yml](https://github.com/deckhouse/csi-nfs/blob/main/.github/workflows/build_dev.yml#L39C1-L42C62) file.

To simplify your setup, you can also use the provided [configuration templates](https://github.com/deckhouse/modules-actions/blob/main/.examples/build.yml).

### For GitLab Projects

For GitLab projects, ready-to-use templates are available and can be included in your `.gitlab-ci.yml` file to automatically configure the build and validation processes:

- **Setup**: [Setup configuration template](https://github.com/deckhouse/modules-gitlab-ci/blob/main/templates/Setup.gitlab-ci.yml)
- **Build**: [Build process configuration template](https://github.com/deckhouse/modules-gitlab-ci/blob/main/templates/Build.gitlab-ci.yml)

#### Steps to connect

1. In your project's `.gitlab-ci.yml` file, add references to the templates:

    ```yaml
    include:
      - remote: https://raw.githubusercontent.com/deckhouse/modules-gitlab-ci/refs/heads/main/templates/Setup.gitlab-ci.yml
      - remote: https://raw.githubusercontent.com/deckhouse/modules-gitlab-ci/refs/heads/main/templates/Build.gitlab-ci.yml
    ```

   Example of template inclusion:  
   [GitLab `.gitlab-ci.yml`, line 2](https://fox.flant.com/deckhouse/flant-integration/-/blob/main/.gitlab-ci.yml?ref_type=heads#L2)

1. After adding the templates, in the same `.gitlab-ci.yml` configuration, add a step to perform the check:

    ```yaml
    Lint:
      extends: .lint
    ```

   For an example of how to add a check step, see [GitLab `.gitlab-ci.yml`, line 48](https://fox.flant.com/deckhouse/flant-integration/-/blob/main/.gitlab-ci.yml?ref_type=heads#L48).

> If your project is hosted in the [https://fox.flant.com/deckhouse](https://fox.flant.com/deckhouse) group, the metrics variables are already configured. No additional setup is required.
