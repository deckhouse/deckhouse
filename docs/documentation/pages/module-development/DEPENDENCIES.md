---
title: "Dependencies of the Deckhouse Kubernetes Platform modules"
permalink: en/module-development/dependencies/
lang: en
---

This section covers the dependencies you can set for the module.

Dependencies are a set of conditions (requirements) that must be met in order for Deckhouse Kubernetes Platform to be able to run a module.

Deckhouse Kubernetes Platform (DKP) supports the following module dependencies:

- [Deckhouse Kubernetes Platform version](#deckhouse-kubernetes-platform-version-dependency);
- [Kubernetes version](#kubernetes-version-dependency);
- [version of other modules](#dependency-on-the-version-of-other-modules).

### Deckhouse Kubernetes Platform version dependency

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
   root@dev-master-0:~# kubectl get mr
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
   root@dev-master-0:~# kubectl get deckhousereleases.deckhouse.io
   ```

   Output information:

   ```text
   NAME                     PHASE         TRANSITIONTIME   MESSAGE
   v1.73.3                  Skipped       74m
   v1.73.4                  Pending       2m13s            requirements of test are not satisfied: v1.73.4 deckhouse version is not suitable: v1.73.4 is greater than or equal to v1.73.4
   ```

1. **When conducting initial module analyses**  
   Deckhouse checks the current version of DKP and the dependencies of the installed modules. If a mismatch is discovered, the module will be disabled.

### Kubernetes version dependency

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
   root@dev-master-0:~# kubectl get modulereleases.deckhouse.io
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
   root@dev-master-0:~# kubectl get deckhousereleases.deckhouse.io
   ```

   Output information:

   ```text
   NAME                     PHASE         TRANSITIONTIME   MESSAGE
   v1.73.3                  Pending       7s              requirements of test are not satisfied: 1.27 kubernetes version is not suitable: 1.27.0 is less than or equal to 1.28            
   ```

### Dependency on the version of other modules

This dependency defines the list of **enabled** modules and their minimum versions that are required for the module to work. The built-in DKP module version is considered equal to the DKP version.

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

#### Optional module requirements

Use this when your module works alone, but integrates with another module **if it is present**.

**Syntax**

Append `!optional` to the version constraint:

```yaml
requirements:
  modules:
    test1: ">v0.22.1 !optional"
```

**Rules**

- If `test1` is **enabled**, its version **must** satisfy the constraint.
- If `test1` is **disabled**, the constraint is ignored and your module can install and upgrade.
- If you later enable `test1` with a non‑matching version, the enable is denied and the module stays disabled.

**Quick examples**

- `test1` enabled at `v0.21.1` + `>v0.22.1 !optional` → install fails with unmet dependency.
- `test1` disabled + `>v0.22.1 !optional` → install succeeds; the optional requirement is skipped.
- `test` disabled, `test1` enabled at `v0.21.1` + `>v0.22.1 !optional` → enabling `test` is denied.
- `test` enabled with a requirement on `test1`; enabling `test1` at a non‑matching version is denied and `test1` remains disabled.

{% alert level="warning" %}
Enabling or disabling modules may take longer because of extra extender checks.
{% endalert %}

{% alert level="warning" %}
Known limitation: during reconciliation the list of enabled modules may be briefly empty. Rarely this can let an optional check pass incorrectly.
{% endalert %}
