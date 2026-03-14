---
title: Network subsystem
permalink: en/architecture/network/
search: network, network subsystem
description: Architecture of the Network subsystem in Deckhouse Kubernetes Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

This subsection describes the architecture of the Network subsystem of Deckhouse Kubernetes Platform (DKP).

The Network subsystem includes the following modules:

* [`kube-dns`](/modules/kube-dns/): Installs CoreDNS components for DNS management in the Kubernetes cluster.
* [`node-local-dns`](/modules/node-local-dns/): Deploys a caching DNS server on each cluster node and exports DNS metrics to Prometheus for analyzing DNS operation in the cluster on the [Grafana dashboard](/modules/node-local-dns/#grafana-dashboard). The architecture of the caching DNS server is described on the [corresponding page](dns-caching.html) of this subsection.
* [`kube-proxy`](/modules/kube-proxy/): Manages the kube-proxy components responsible for networking and load balancing within the cluster.
* [`cni-cilium`](/modules/cni-cilium/): Provides cluster networking using the CNI Cilium plugin.
* [`ingress-nginx`](/modules/ingress-nginx/): Installs and manages the [Ingress NGINX Controller](https://kubernetes.github.io/ingress-nginx/) using custom resources. The module architecture is described on the [corresponding page](ingress-nginx.html) of this subsection.
* [`metallb`](/modules/metallb/): Implements the LoadBalancer mechanism for Services in bare-metal clusters.

The subsection also describes:

* [Cluster architecture with Istio enabled](cluster-with-istio.html)
* [Application service architecture with Istio enabled](service-with-istio.html)
