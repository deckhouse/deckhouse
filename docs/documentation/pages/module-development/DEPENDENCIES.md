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
- [cluster installation status](#cluster-installation-status-dependency).

### Deckhouse Kubernetes Platform version dependency

This dependency defines the minimum or maximum DKP version with which the module is compatible.

Here's how you can set the module dependency in the `module.yaml` file: during installation, the module will require DKP version 1.61 or higher:

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
   NAME                     PHASE        UPDATE POLICY   TRANSITIONTIME   MESSAGE
   test-v0.8.3              Pending      test-alpha      2m30s            requirements are not satisfied: current deckhouse version is not suitable: 1.0.0 is less than or equal to v1.64.0 
   ```

1. **When upgrading Deckhouse Kubernetes Platform**  
   Deckhouse checks if the new DKP version matches the dependencies of the installed and active modules. If at least one module is not compatible with the new version, the DKP upgrade will not be performed.

   Below is an example of the DeckhouseRelease resource for which the DKP version does not meet the module requirements:

   ```console
   root@dev-master-0:~# kubectl get deckhousereleases.deckhouse.io
   NAME                     PHASE         TRANSITIONTIME   MESSAGE
   v1.73.3                  Skipped       74m
   v1.73.4                  Pending       2m13s            requirements of test are not satisfied: v1.73.4 deckhouse version is not suitable: v1.73.4 is greater than or equal to v1.73.4
   ```

1. **When conducting initial module analyses**  
   Deckhouse checks the current version of DKP and the dependencies of the installed modules. If a mismatch is discovered, the module will be disabled.

### Kubernetes version dependency

This dependency defines the minimum or maximum Kubernetes version with which the module is compatible.

Here is how you can enable the Kubernetes version dependency in the `module.yaml` file:

```yaml
name: test
weight: 901
requirements:
    kubernetes: ">= 1.27"
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
   NAME                          PHASE        UPDATE POLICY   TRANSITIONTIME   MESSAGE
   test-v0.8.2                   Pending      test-alpha      24m              requirements are not satisfied: current kubernetes version is not suitable: 1.29.6 is less than or equal to 1.29
   virtualization-v.0.0.0-dev4   Deployed      deckhouse      142d
   ```

1. **When upgrading Kubernetes**  
   Deckhouse examines the dependencies of active modules, and if at least one module is incompatible with the new Kubernetes version, the version upgrade will not proceed.

   Below is an example of the output you may encounter when a module is incompatible with a newer version of Kubernetes:

   ```console
   root@dev-master-0:~# kubectl -n d8-system exec -it deployment/deckhouse -c deckhouse -- deckhouse-controller edit cluster-configuration
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
   NAME                     PHASE         TRANSITIONTIME   MESSAGE
   v1.73.3                  Pending       7s              requirements of test are not satisfied: 1.27 kubernetes version is not suitable: 1.27.0 is less than or equal to 1.28            
   ```

### Cluster installation status dependency

This dependency indicates that the module requires a cluster whose installation and configuration is complete. The dependency can only be set for built-in DKP modules.

Here is how you can enable this dependency in the `module.yaml` file:

```yaml
name: ingress-nginx
weight: 402
description: |
    Ingress controller for nginx
    https://kubernetes.github.io/ingress-nginx
requirements:
    bootstrapped: true
```

This check is carried out only once - during the initial module analysis. If the cluster installation and configuration is not complete, the module will not be enabled.
