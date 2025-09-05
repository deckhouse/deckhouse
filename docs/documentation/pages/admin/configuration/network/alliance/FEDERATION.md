---
title: "Federation"
permalink: en/admin/configuration/network/alliance/federation.html
---

## Federation with Istio (Service Mesh)

{% alert level="info" %}
Available only in DKP Enterprise Edition (EE).
{% endalert %}

### Requirements for clusters

* Each cluster must have a unique domain in the [`clusterDomain`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain) parameter of the ClusterConfiguration resource.
  Note that none of the clusters should use the domain `cluster.local`, which is the default setting.

  > `cluster.local` can't be used as it's an unmodified alias for the local cluster domain.
  > When specifying `cluster.local` as a principals in the AuthorizationPolicy,
  > it will always refer to the local cluster, even if there is another cluster in the mesh with [`clusterDomain`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain) explicitly defined as `cluster.local`
  > (for details, refer to the [Istio documentation](https://istio.io/latest/docs/tasks/security/authorization/authz-td-migration/#best-practices)).

* Pod and Service subnets in the [`podSubnetCIDR`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-podsubnetcidr) and [`serviceSubnetCIDR`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-servicesubnetcidr) parameters of the ClusterConfiguration resource must be unique for each federation member.

  > When analyzing traffic, Istio uses:
  > - For HTTP and HTTPS requests — headers.
  > - For TCP requests — destination IP address and port number.
  >
  > If the IP addresses of services or pods in different clusters match,
  > requests from other pods in other clusters may mistakenly fall under the istio's rules.
  > The intersection of subnets of services and pods is strictly prohibited in single-network mode,
  > and is acceptable but not recommended in multi-networks mode.
  > For details, refer to the [Istio documentation](https://istio.io/latest/docs/ops/deployment/deployment-models/#single-network).
  >
  > - In the single-network mode, pods from different clusters can communicate with each other directly.
  > - In the multi-networks mode, pods from different clusters can only communicate with each other if they use the Istio-gateway.

### General principles of federation

* Federation requires mutual trust between clusters.
  It requires a mutual root certificate exchange: cluster A must trust cluster B, and vice versa.
* Configuring inter-cluster access to services requires exchanging information about public services.
  To expose the bar service from cluster B to cluster A, you must create a ServiceEntry resource in cluster A,
  defining the Ingress gateway public address of cluster B.

<div data-presentation="../../../../presentations/istio/federation_common_principles_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1klrLIXqe-zl9Dspbsu9nTI1a1nD3v7HHQqIN4iqF00s/ --->

### Enabling the federation — created services

Enabling federation (via the `istio.federation.enabled = true` module parameter) results in the following:

* The `ingressgateway` service is added to the cluster.
  Its task is to proxy mTLS traffic coming from outside to application services.
* Another added service exports the following cluster metadata to the outside:
  * Istio root certificate (accessible without authentication).
  * List of public services in the cluster (available only for authenticated requests from neighboring clusters).
  * List of public addresses of the `ingressgateway` service
    (available only for authenticated requests from neighboring clusters).

### Managing the federation

<div data-presentation="../../../../presentations/istio/federation_istio_federation_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1dYOeYKGaGOsgskWCDDcVJfXcMC9iQ4cvaCkhyqrDKgg/ --->

To establish a federation, you must:

* Create a set of [IstioFederation](/modules/istio/cr.html#istiofederation) resources in each cluster
  that describe all the other clusters.
  * After successful auto-negotiation between clusters,
    the IstioFederation resource will be filled with necessary service data in `status.metadataCache.public` and `status.metadataCache.private`.
* Add the `federation.istio.deckhouse.io/public-service: ""` label to each Service resource
  that is considered public within the federation.
  * In the other federation clusters, a corresponding ServiceEntry will be created for each Service,
    leading to the `ingressgateway` of the original cluster.

> **Important**. Ensure that the `name` field in the `.spec.ports` section of the Services resource is filled in for each port.
> Otherwise, there may be issues in the federation's work.

### Example of configuring a federation of two clusters

To set up a federation with Istio, use the [IstioFederation](/modules/istio/cr.html#istiofederation) custom resource.

Cluster A:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: cluster-a
spec:
  metadataEndpoint: https://istio.k8s-a.example.com/metadata/
  trustDomain: cluster-a.local
```

Cluster B:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: cluster-b
spec:
  metadataEndpoint: https://istio.k8s-b.example.com/metadata/
  trustDomain: cluster-b.local
```
