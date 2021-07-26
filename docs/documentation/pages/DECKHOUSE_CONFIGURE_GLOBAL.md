---
title: "Global configuration"
permalink: en/deckhouse-configure-global.html
---

## What do I need to configure?

We recommend specifying the `modules.publicDomainTemplate` parameter.

```yaml
global: |
  modules:
    publicDomainTemplate: "%s.kube.company.my"
```

## Parameters

* `modules` — parameters of service components:
  * `publicDomainTemplate` (recommended) — the template with the `%s` key as the dynamic string parameter. Deckhouse will use this template for creating DNS records. The latter are necessary for the internal needs of DH and the operation of the modules. **Do not use** DNS names (nor do create Ingress resources) that match this template to avoid conflicts with the Ingress resources created by Deckhouse. An example of the template: `%s.kube.company.my`. If this parameter is omitted, no Ingress resources will be created.
  * `ingressClass` — the class of the Ingress controller used for service components.
    * It is set to `nginx`  by default.
  * `placement` — parameters regulating the layout of the Deckhouse components.
    * `customTolerationKeys` — a list of custom toleration keys; use them to allow the deployment of some critical add-ons (such as cni and csi) on dedicated nodes.
      * An example:

        ```yaml
        customTolerationKeys:
        - dedicated.example.com
        - node-dedicated.example.com/master
        ```
  * `https` — the HTTPS implementation used by the service components.
    * `mode` — the HTTPS usage mode:
      * `Disabled` — in this mode, all service components use HTTP only (some modules may not work, e.g., [user-authn](modules/150-user-authn/));
      * `CertManager` — all service components use HTTPS and get a certificate from the clusterissuer defined in the `certManager.clusterIssuerName` parameter;
      * `CustomCertificate` — all service components use HTTPS using the certificate from the `d8-system` namespace;
      * `OnlyInURI` — all service components use HTTP (in the expectation that an HTTPS load balancer runs in front of them and terminates HTTPS).
      * By default, the `CertManager` mode is enabled.
    * `certManager`
      * `clusterIssuerName` — what ClusterIssuer to use for service components (currently, `letsencrypt`, `letsencrypt-staging`, `selfsigned` are available; also, you can define your own).
        * By default, `letsencrypt` is used.
    * `customCertificate`
      * `secretName` - the name of the secret in the `d8-system` namespace to use with system components (this secret must have the [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets) format).
        * It is set to `false` by default.
  * `resourcesRequests` - the amount of CPU and memory allocated to service components.
    * `everyNode` - system components running on every cluster node (usually DaemonSets).
      * `cpu` – the combined CPU requests for all the components on each node (default: 300m).
      * `memory` – the combined memory requests for all the components on each node (default: 512Mi).
    * `masterNode` - system components (control plane and system components on the master nodes).
      * `cpu` – the combined CPU requests for the system components on master nodes in addition to `everyNode`.
        * For the Deckhouse-controlled cluster, the default value is calculated automatically: `.status.allocatable.cpu` of the smallest master node (no more than 4 cores) minus `everyNode`.
        * For a managed cluster, the default value is 1 core minus `everyNode`.
      * `memory` – the total amount of memory allocated to system components on master nodes in addition to `everyNode`.
        * For the Deckhouse-managed cluster, the default value is calculated automatically: `.status.allocatable.memory` of the smallest master node (no more than 8GiB) minus `everyNode`.
        * For a managed cluster, the default value is 1GiB minus `everyNode`.
      * **Caution!** Deckhouse does not manage control plane components in managed clusters, so all resources are allocated to the system components.
* `storageClass` —  the storage class to use with all service components (prometheus, grafana, openvpn, ...).
    * By default, it is set to null. In this case, service components use `cluster.defaultStorageClass` (which is determined automatically) or `emptyDir` (if `cluster.defaultStorageClass` isn't defined).
    * Use this parameter only in exceptional circumstances.
* `highAvailability` — a global switch to enable the HA mode for modules that support it. The parameter is not defined by default; the decision is made based on the `global.discovery.clusterControlPlaneIsHighlyAvailable` parameter.
