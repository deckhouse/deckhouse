---
title: "The kube-dns module: FAQ"
search: DNS, domain, clusterdomain
---

## How do I replace the cluster domain with minimal downtime?

Add a new domain and retain the previous one. To do this, modify the configuration parameters:

1. In the [controlPlaneManager.apiserver](../control-plane-manager/configuration.html):

   - [controlPlaneManager.apiserver.certSANs](../control-plane-manager/configuration.html#parameters-apiserver-certsans),
   - [apiserver.serviceAccount.additionalAPIAudiences](../control-plane-manager/configuration.html#parameters-apiserver-serviceaccount-additionalapiaudiences),
   - [apiserver.serviceAccount.additionalAPIIssuers](../control-plane-manager/configuration.html#parameters-apiserver-serviceaccount-additionalapiissuers).

   Example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     enabled: true
     version: 1
     settings:
       apiserver:
         certSANs:
          - kubernetes.default.svc.<old clusterDomain>
          - kubernetes.default.svc.<new clusterDomain>
         serviceAccount:
           additionalAPIAudiences:
           - https://kubernetes.default.svc.<old clusterDomain>
           - https://kubernetes.default.svc.<new clusterDomain>
           additionalAPIIssuers:
           - https://kubernetes.default.svc.<old clusterDomain>
           - https://kubernetes.default.svc.<new clusterDomain>
   ```

1. In the [kubeDns.clusterDomainAliases](configuration.html#parameters):

   Example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: kube-dns
   spec:
     version: 1
     enabled: true
     settings:
       clusterDomainAliases:
         - <old clusterDomain>
         - <new clusterDomain>
   ```

1. Wait until the `kube-apiserver` has restarted.
1. Replace the previous `clusterDomain` with the new one. To do this, execute the command:

   ```bash
   d8 platform edit cluster-configuration
   ```

**Important!** If your Kubernetes version is 1.20 and higher, your controllers in the cluster use [advanced ServiceAccount tokens](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#service-account-token-volume-projection) to work with API server. Those tokens have extra fields `iss:` and `aud:` that contain `clusterDomain` (e.g. `"iss": "https://kubernetes.default.svc.cluster.local"`). After changing the clusterDomain, the API server will start issuing tokens with the new service-account-issuer, but thanks to the configuration of additionalAPIAudiences and additionalAPIIssuers, the apiserver will continue to accept the old tokens.
After 48 minutes (80% of 3607 seconds), Kubernetes will begin to refresh the issued tokens, and the new service-account-issuer will be used for the updated tokens. After 90 minutes (3607 seconds plus a short buffer) following the kube-apiserver restart, you can remove the serviceAccount configuration from the control-plane-manager configuration.

**Important!** If you use [istio](../../modules/istio/) module, you have to restart all the application pods under istio control after changing `clusterDomain`.

## How to Increase the Number of kube-dns pods?

Deckhouse distributes kube-dns pods based on the following principles. It searches for nodes with the labels node-role.deckhouse.io/ and node-role.kubernetes.io/, then applies the following rules:

* If there are nodes with the kube-dns role in the cluster, the number of replicas is calculated as the sum of these nodes and the master nodes.
* If kube-dns nodes are absent, the system searches for nodes with the system role, and the number of replicas is then determined as the sum of system nodes and master nodes.
* If only master nodes are present in the cluster, the number of kube-dns replicas will be equal to the number of masters.
