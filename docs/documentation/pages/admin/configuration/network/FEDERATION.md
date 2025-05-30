---
title: "Federation"
permalink: en/admin/configuration/network/cluster-federation.html
---

## Federation of Istio funds (Service Mesh)

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/istio/#federation -->

### Requirements for clusters

* Each cluster must have a unique domain in the [`clusterDomain`](../../reference/cr/clusterconfiguration/#clusterconfiguration-clusterdomain) parameter of the resource [_ClusterConfiguration_](../../reference/cr/clusterconfiguration/). Please note that none of the clusters should use the domain `cluster.local`, which is the default setting.

  > `cluster.local` is an unmodified alias for the local cluster domain.
  > When specifying `cluster.local` as a principals in the AuthorizationPolicy, it will always refer to the local cluster, even if there is another cluster in the mesh with [`clusterDomain`](../../reference/cr/clusterconfiguration/#clusterconfiguration-clusterdomain) explicitly defined as `cluster.local`.
  > [source](https://istio.io/latest/docs/tasks/security/authorization/authz-td-migration/#best-practices)

* Pod and Service subnets in the [`podSubnetCIDR`](../../reference/cr/clusterconfiguration/#clusterconfiguration-podsubnetcidr) and [`serviceSubnetCIDR`](../../reference/cr/clusterconfiguration/#clusterconfiguration-servicesubnetcidr) parameters of the resource [_ClusterConfiguration_](../../reference/cr/clusterconfiguration/) must be unique for each federation member.

  > - When analyzing HTTP and HTTPS traffic _(in istio terminology)_, you can identify them and decide on further routing or blocking based on their headers.
  > - At the same time, when analyzing TCP traffic _(in istio terminology)_, it is possible to identify them and decide on further routing or blocking based only on their destination IP address or port number.
  >
  > If the IP addresses of services or pods in different clusters match, requests from other pods in other clusters may mistakenly fall under the istio's rules.
  > The intersection of subnets of services and pods is strictly prohibited in `single-network` mode, and is acceptable but not recommended in `multi-networks` mode.
  > [source](https://istio.io/latest/docs/ops/deployment/deployment-models/#single-network )
  >
  > - In the single-network mode, pods from different clusters can communicate with each other directly.
  > - In the multi-networks mode, pods from different clusters can only communicate with each other if they use the Istio-gateway.

### General principles of federation

* Federation requires mutual trust between clusters. Thereby, to use federation, you have to make sure that both clusters (say, A and B) trust each other. From a technical point of view, this is achieved by a mutual exchange of root certificates.
* You also need to share information about government services to use the federation. You can do that using ServiceEntry. A service entry defines the public ingress-gateway address of the B cluster so that services of the A cluster can communicate with the bar service in the B cluster.

<div data-presentation="../../presentations/istio/federation_common_principles_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1klrLIXqe-zl9Dspbsu9nTI1a1nD3v7HHQqIN4iqF00s/ --->

### Enabling the federation

Enabling federation (via the `istio.federation.enabled = true` module parameter) results in the following activities:

* The `ingressgateway` service is added to the cluster. Its task is to proxy mTLS traffic coming from outside of the cluster to application services.
* A service gets added to the cluster that exports the following cluster metadata to the outside:
  * Istio root certificate (accessible without authentication).
  * List of public services in the cluster (available only for authenticated requests from neighboring clusters).
  * List of public addresses of the `ingressgateway` service (available only for authenticated requests from neighboring clusters).

### Managing the federation

<div data-presentation="../../presentations/istio/federation_istio_federation_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1dYOeYKGaGOsgskWCDDcVJfXcMC9iQ4cvaCkhyqrDKgg/ --->

To establish a federation, you must:

* Create a set of `IstioFederation` resources in each cluster that describe all the other clusters.
  * After successful auto-negotiation between clusters, the status of `IstioFederation` resource will be filled with neighbour's public and private metadata (`status.metadataCache.public` and `status.metadataCache.private`).
* Add the `federation.istio.deckhouse.io/public-service: ""` label to each resource(`service`) that is considered public within the federation.
  * In the other federation clusters, a corresponding `ServiceEntry` will be created for each `service`, leading to the `ingressgateway` of the original cluster.

> It is important, that in these `services`, in the `.spec.ports` section, each port must have the `name` field filled.

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/istio/#federation -->

### Example of configuring a federation of two clusters

> Available in Enterprise Edition only.

Use custom resource IstioFederation to customize the federation using Istio tools.

Cluster A:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: cluster-b
spec:
  metadataEndpoint: https://istio.k8s-b.example.com/metadata/
  trustDomain: cluster-b.local
```

Cluster B:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: cluster-a
spec:
  metadataEndpoint: https://istio.k8s-a.example.com/metadata/
  trustDomain: cluster-a.local
```
