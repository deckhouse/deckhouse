---
title: "Multicluster"
permalink: en/admin/network/multicluster.html
---

## Multicluster of Istio funds

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/istio/#multicluster -->

### Requirements for clusters

* Cluster domains in the [`clusterDomain`](../../installing/configuration.html#clusterconfiguration-clusterdomain) parameter of the resource [_ClusterConfiguration_](../../installing/configuration.html#clusterconfiguration) must be the same for all multicluster members. The default value is `cluster.local`.

* Pod and Service subnets in the [`podSubnetCIDR`](../../installing/configuration.html#clusterconfiguration-podsubnetcidr) and [`serviceSubnetCIDR`](../../installing/configuration.html#clusterconfiguration-servicesubnetcidr) parameters of the resource [_ClusterConfiguration_](../../installing/configuration.html#clusterconfiguration) must be unique for each multicluster member.

  > - When analyzing HTTP and HTTPS traffic *(in istio terminology)*, you can identify them and decide on further routing or blocking based on their headers.
  > - At the same time, when analyzing TCP traffic *(in istio terminology)*, it is possible to identify them and decide on further routing or blocking based only on their destination IP address or port number.s
  >
  > If the IP addresses of services or pods in different clusters match, requests from other pods in other clusters may mistakenly fall under the istio's rules.
  > The intersection of subnets of services and pods is strictly prohibited in `single-network` mode, and is acceptable but not recommended in `multi-networks` mode.
  > [source](https://istio.io/latest/docs/ops/deployment/deployment-models/#single-network )
  >
  > - In the single-network mode, pods from different clusters can communicate with each other directly.
  > - In the multi-networks mode, pods from different clusters can only communicate with each other if they use the Istio-gateway.

### General principles

<div data-presentation="../../presentations/istio/multicluster_common_principles_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1fmVDf-6yDSCEHhg_2vSvZcRkLSkQtUYrE6MISjZdb8Q/ --->

* Multicluster requires mutual trust between clusters. Thereby, to use multiclustering, you have to make sure that both clusters (say, A and B) trust each other. From a technical point of view, this is achieved by a mutual exchange of root certificates.
* Istio connects directly to the API server of the neighboring cluster to gather information about its services. This Deckhouse module takes care of the corresponding communication channel.

### Enabling the multicluster

Enabling the multicluster (via the `istio.multicluster.enabled = true` module parameter) results in the following activities:

* A proxy is added to the cluster to publish access to the API server via the standard Ingress resource:
  * Access through this public address is secured by  authorization based on Bearer tokens signed with trusted keys. Deckhouse automatically exchanges trusted public keys during the mutual configuration of the multicluster.
  * The proxy itself has read-only access to a limited set of resources.
* A service gets added to the cluster that exports the following cluster metadata to the outside:
  * Istio root certificate (accessible without authentication).
  * The public API server address (available only for authenticated requests from neighboring clusters).
  * List of public addresses of the `ingressgateway` service (available only for authenticated requests from neighboring clusters).
  * Server public keys to authenticate requests to API server and to private metadata (see above).

### Managing the multicluster

<div data-presentation="../../presentations/istio/multicluster_istio_multicluster_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1fy3jIynIPTrJ5Whn4eqQxeLk7ORtipDxBWP3By4buoc/ --->

To create a multicluster, you need to create a set of `IstioMulticluster` resources in each cluster that describe all the other clusters.

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/istio/#multicluster -->

### Example of configuring a multicluster of two clusters

> Available in Enterprise Edition only.

Use the IstioMulticluster resource to configure a multicluster using Istio tools.

Cluster A:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: cluster-b
spec:
  metadataEndpoint: https://istio.k8s-b.example.com/metadata/
```

Cluster B:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: cluster-a
spec:
  metadataEndpoint: https://istio.k8s-a.example.com/metadata/
```

## Multicluster of Cilium funds

Need content
