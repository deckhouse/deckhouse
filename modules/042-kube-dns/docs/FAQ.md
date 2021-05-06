---
title: "The kube-dns module: FAQ"
search: DNS, domain, clusterdomain
---

## How do I replace the cluster domain without downtime?

Add the new domain and save the old one:

1. In the `controlPlaneManager.apiserver.certSANs` section, enter the following parameters:
    - `kubernetes.default.svc.<old clusterDomain>`
    - `kubernetes.default.svc.<new clusterDomain>`
1. In the [kubeDns.clusterDomainAliases](configuration.html#parameters) section, enter:
    - the old clusterDomain.
    - the new clusterDomain.
1. Wait until kube-apiserver is restarted.
1. Replace the old clusterDomain with the new one in `dhctl config edit cluster-configuration`
