---
title: "Dependencies of the Deckhouse Kubernetes Platform modules"
permalink: en/architecture/module-development/dependencies/
lang: en
---

This section covers the dependencies you can set for the module.

Dependencies are a set of conditions (requirements) that must be met in order for Deckhouse Kubernetes Platform to be able to run a module.

Deckhouse Kubernetes Platform (DKP) supports the following module dependencies:

- [Deckhouse Kubernetes Platform version](#deckhouse-kubernetes-platform-version-dependency);
- [Kubernetes version](#kubernetes-version-dependency);
- [version of other modules](#dependency-on-the-version-of-other-modules).

## Deckhouse Kubernetes Platform version dependency

This dependency defines the minimum or maximum DKP version with which the module is compatible.

An example of setting up a dependency for Kubernetes 1.27 and higher in the `module.yaml` file:

```yaml
name: test
weight: 901
requirements:
    deckhouse: ">= 1.61"
```

{% alert level="info" %}
For testing, you can set the `TEST_EXTENDER_DECKHOUSE_VERSION` environment variable to imitate the desired version of Deckhouse Kubernetes Platform.
{% endalert %}

Deckhouse checks whether the dependency is met in the following cases:

1. **When installing or upgrading a module**
   If the DKP version does not meet the requirements specified in the release module dependencies, the latter will not be installed or upgraded.

   Below is an example of the ModuleRelease resource for which the DKP version does not meet the module requirements:

   ```console
   root@dev-master-0:~# d8 k get mr
   ```

   Output information:

   ```text
      NAME                     PHASE        UPDATE POLICY   TRANSITIONTIME   MESSAGE
      test-v0.8.3              Pending      test-alpha      2m30s            requirements are not satisfied: current deckhouse version is not suitable: 1.0.0 is less than or equal to v1.64.0 
   ```

1. **When upgrading Deckhouse Kubernetes Platform**
   Deckhouse checks if the new DKP version matches the dependencies of the installed and active modules. If at least one module is not compatible with the new version, the DKP upgrade will not be performed.

   Below is an example of the DeckhouseRelease resource for which the DKP version does not meet the module requirements:

   ```console
   root@dev-master-0:~# d8 k get deckhousereleases.deckhouse.io
   ```

   Output information:

   ```text
   NAME                     PHASE         TRANSITIONTIME   MESSAGE
   v1.73.3                  Skipped       74m
   v1.73.4                  Pending       2m13s            requirements of test are not satisfied: v1.73.4 deckhouse version is not suitable: v1.73.4 is greater than or equal to v1.73.4
   ```

1. **When conducting initial module analyses**
   Deckhouse checks the current version of DKP and the dependencies of the installed modules. If a mismatch is discovered, the module will be disabled.

## Kubernetes version dependency

This dependency defines the minimum or maximum Kubernetes version with which the module is compatible.

Here is how you can enable the Kubernetes version dependency for Kubernetes 1.28 and higher in the `module.yaml` file:

```yaml
name: test
weight: 901
requirements:
    kubernetes: ">= 1.28"
```

{% alert level="info" %}
For testing, you can set the `TEST_EXTENDER_KUBERNETES_VERSION` environment variable to imitate the desired version of Deckhouse Kubernetes Platform.
{% endalert %}

Deckhouse checks whether the dependency is met in the following cases:

1. **When installing or upgrading a module**
   If the Kubernetes version does not meet the requirements specified in the release module dependencies, the latter will not be installed or upgraded.
  
   Below is an example of the ModuleRelease resource for which the Kubernetes version does not meet the module requirements:

   ```console
   root@dev-master-0:~# d8 k get modulereleases.deckhouse.io
   ```

   Output information:

   ```text
   NAME                          PHASE        UPDATE POLICY   TRANSITIONTIME   MESSAGE
   test-v0.8.2                   Pending      test-alpha      24m              requirements are not satisfied: current kubernetes version is not suitable: 1.29.6 is less than or equal to 1.29
   virtualization-v.0.0.0-dev4   Deployed      deckhouse      142d
   ```

1. **When upgrading Kubernetes**
   Deckhouse examines the dependencies of active modules, and if at least one module is incompatible with the new Kubernetes version, the version upgrade will not proceed.

   Below is an example of the output you may encounter when a module is incompatible with a newer version of Kubernetes:

   ```console
   root@dev-master-0:~# d8 platform edit cluster-configuration
   ```

   Output information:

   ```text
   Save cluster-configuration back to the Kubernetes cluster
   Update cluster-configuration secret
   Attempt 1 of 5 |
           Update cluster-configuration secret failed, next attempt will be in 5s"
           Error: admission webhook "kubernetes-version.deckhouse-webhook.deckhouse.io" denied the request: requirements of test are not satisfied: 1.27 kubernetes version is not suitable: 1.27.0 is less than or equal to 1.28
   ```

1. **When conducting initial module analyses**
   If the Kubernetes version does not conform to the dependencies of the modules that are already installed, DKP will disable those modules.

1. **When upgrading Deckhouse Kubernetes Platform**
   Deckhouse checks the default Kubernetes version value for DKP and if it is not compatible with the active modules, the DKP update will not be carried out.

   Below is an example of the DeckhouseRelease resource for which the Kubernetes version does not meet the module requirements:

   ```console
   root@dev-master-0:~# d8 k get deckhousereleases.deckhouse.io
   ```

   Output information:

   ```text
   NAME                     PHASE         TRANSITIONTIME   MESSAGE
   v1.73.3                  Pending       7s              requirements of test are not satisfied: 1.27 kubernetes version is not suitable: 1.27.0 is less than or equal to 1.28            
   ```

## Dependency on the version of other modules

Dependencies on other modules describe the conditions for enabling, updating, and disabling a module.
A module in the Deckhouse Kubernetes Platform may have required and optional dependencies on versions of other modules.

### Required dependencies

This dependency defines the list of **enabled** modules and their versions that are required for the module to work.

{% alert level="info" %}
The built-in DKP module version is considered equal to the DKP version.
{% endalert %}

If you need to specify that some module is simply enabled, no matter what version, then you can use the following syntax (using the `user-authn` module as an example):

```yaml
requirements:
  modules:
    user-authn: ">= 0.0.0"
```

Example of setting up a dependency on three modules:

```yaml
name: hello-world
requirements:
  modules:
    ingress-nginx: '> 1.67.0'
    node-local-dns: '>= 0.0.0'
    operator-trivy: '> v1.64.0'
```

### Optional module requirements

{% alert level="danger" %}
Optional dependencies for modules are available for Deckhouse Kubernetes Platform starting with version 1.73.
If you need to use them for a module, [set a Deckhouse Kubernetes Platform version dependency](#deckhouse-kubernetes-platform-version-dependency) 1.73 or higher for that module.
{% endalert %}

Use this when your module works alone, but integrates with another module **if it is present**.

{% alert level="info" %}
Optional dependencies may affect the ability to enable, disable, and update both modules: the dependent module and the module on which it depends.
{% endalert %}

To mark a requirement as optional, add `!optional` to the version constraint string:

```yaml
name: prometheus
requirements:
  modules:
    test: ">v0.22.1 !optional"
```

> The following sections describes restrictions on the use of optional module dependencies and provides examples of settings where:
>
> - `prometheus` — the target module for which an optional dependency is specified;
> - `test` — a module that can be used in conjunction with the target module.

#### Restrictions on enabling and disabling prometheus when there is an optional dependency on test

When there is an optional dependency on the version of another module, the target module has the following restrictions on enabling and disabling:

1. If `test` is not enabled in the cluster, `prometheus` can be enabled.

   **Example:** `test` is disabled + `prometheus` has an optional requirement `test: ">v0.22.1 !optional"` → `prometheus` will be enabled, the requirement is skipped.

1. If `test` is already enabled in the cluster, enabling `prometheus` with `requirements` is only possible if `requirements` specifies requirements that the current version of `test` meets.

   **Example:** `test` version `v0.21.1` is enabled in the cluster + an optional requirement `test: ">v0.22.1 !optional"` is set for `prometheus` → installing/enabling `prometheus` will result in a dependency mismatch error (the current version of `test` does not match `requirements`).

1. If `test` is disabled in the cluster, `prometheus` will remain enabled.

   **Example:** `test` is enabled in the cluster + an optional requirement `test: ">v0.22.1 !optional"` is set for `prometheus` → disabling `test` will be successful, `prometheus` will not be disabled.

#### Restrictions on updating prometheus when there is an optional dependency on test

When there is an optional dependency on the version of another module, the target module has the following update restrictions:

1. `prometheus` can be updated even if the cluster does not have the `test` module.

   **Example:** `test` is disabled + `prometheus` has an optional requirement `test: ">v0.22.1 !optional"` → `prometheus` will be updated.

1. The update for `prometheus` will be blocked until `test` specified in `requirements` is updated in the cluster.

   **Example:** `test` is enabled + an optional requirement `test: ">v0.22.1 !optional"` is set for `prometheus` → `prometheus` will not be updated until `test` is updated.

#### Restrictions on enabling test, whose version is specified in prometheus dependencies

If the `prometheus` module is enabled, it is not possible to enable `test`, whose version does not match the expression specified in `requirements` for `prometheus`.

**Example:** `prometheus` is included + an optional requirement `test: ">v0.22.1 !optional"` is specified for `prometheus` → an attempt to include `test v0.21.1` will result in a dependency mismatch error (the version `test v0.21.1` does not match the condition in `requirements` for `prometheus`).

#### Restrictions on updating test, whose version is specified in prometheus dependencies

If prometheus and test are included in the cluster, test can only be updated to a version that meets the requirements specified in requirements for prometheus.

**Example:** The `prometheus` and `test` modules are included in the cluster + an optional requirement `test: ‘=v0.22.1 !optional’` is set for `prometheus` + an attempt to update `test` to version `0.23.1` → `test` will not be updated because the required version does not meet the `requirements` for `prometheus`.

{% alert level="warning" %}
- Enabling or disabling modules may take longer because of extra extender checks.
- Known limitation: during reconciliation the list of enabled modules may briefly be empty, which in rare cases lets an optional dependency check pass incorrectly. If this happens, retry the operation.
{% endalert %}
