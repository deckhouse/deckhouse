---
title: "The kube-dns module: FAQ"
search: DNS, domain, clusterdomain
---

## How do I replace the cluster domain with minimal downtime?

Add the new domain and save the old one:

1. In the [controlPlaneManager.apiserver.certSANs](../040-control-plane-manager/configuration.html#parameters-apiserver-certsans) section, enter the following parameters:
    - `kubernetes.default.svc.<old clusterDomain>`
    - `kubernetes.default.svc.<new clusterDomain>`
1. In the [kubeDns.clusterDomainAliases](configuration.html#parameters) section, enter:
    - the old clusterDomain.
    - the new clusterDomain.
1. Wait until kube-apiserver is restarted.
1. Replace the old `clusterDomain` with the new one in `dhctl config edit cluster-configuration`

**Important!** If your Kubernetes version is 1.20 and higher, your controllers in the cluster use [advanced ServiceAccount tokens](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#service-account-token-volume-projection) to work with apiserver. Those tokens have extra fields `iss:` and `aud:` that contain `clusterDomain` (e.g. `"iss": "https://kubernetes.default.svc.cluster.local"`). After changing `clusterDomain` apiserver starts to deny queries with old tokens and controllers are bond to provide errors (including deckhouse). The solution is to wait until Kubernetes rotates the tokens (it will be quite fast despite the expiration date) or restart all pods with controllers.

**Important!** If you use [istio](../../modules/110-istio/) module, you have to restart all the application pods under istio control after changing `clusterDomain`.
