---
title: "The istio module"
webIfaces:
- name: istio
---

## Compatibility table for supported versions

| Istio version | [K8S versions supported by Istio](https://istio.io/latest/docs/releases/supported-releases/#support-status-of-istio-releases) |          Status in D8          |
|:-------------:|:-----------------------------------------------------------------------------------------------------------------------------:|:------------------------------:|
|     1.16      |                                                1.22<sup>*</sup>, 1.23<sup>*</sup>, 1.24<sup>*</sup>, 1.25                                                | Deprecated and will be deleted |
|     1.19      |                                                    1.25, 1.26, 1.27, 1.28                                                     |           Supported            |

<sup>*</sup> — the Kubernetes version **is NOT supported** in the current Deckhouse Kubernetes Platform release.

## What issues does Istio help to resolve?

[Istio](https://istio.io/) is a framework for managing network traffic on a centralized basis that implements the Service Mesh approach.

Among other things, Istio solves the following tasks in a transparent for applications way:

* [Using Mutual TLS:](#mutual-tls)
  * Mutual trusted service authentication.
  * Traffic encryption.
* [Access authorization between services.](#authorization)
* [Request routing:](#request-routing)
  * Canary deployments and A/B testing: send part of the requests to the new application version.
* [Managing request balancing between service Endpoints:](#managing-request-balancing-between-service-endpoints)
  * Circuit Breaker:
    * Temporarily excluding endpoints from balancing if the error limit is exceeded.
    * Setting limits on the number of TCP connections and the number of requests per endpoint.
    * Detecting abnormal requests and terminating them with an error code (HTTP request timeout).
  * Sticky Sessions:
    * Binding requests from end users to the service endpoint.
  * Locality Failover — prioritizing endpoints in the local availability zone.
  * gRPC services load-balancing.
* [Improving Observability](#observability):
  * Collecting and visualizing data for tracing service requests using Jaeger.
  * Exporting metrics about traffic between services to Prometheus and visualizing them using Grafana.
  * Visualizing traffic topology and the state of inter-service communications as well as service components using Kiali.
* [Organizing a multi-datacenter cluster by joining clusters into a single Service Mesh (multicluster).](#multicluster)
* [Grouping isolated clusters into a federation with the ability to provide native (in the Service Mesh sense) access to selected services.](#federation)

## Mutual TLS

Mutual TLS is the main method of mutual service authentication. It is based on the fact that all outgoing requests are verified using the server certificate, and all incoming requests are verified using the client certificate. After the verification is complete, the sidecar-proxy can identify the remote node and use these data for authorization or auxiliary purposes.

Each service gets its own identifier of the following format: `<TrustDomain>/ns/<Namespace>/sa/<ServiceAccount>` where `TrustDomain` is the cluster domain in our case. You can assign your own ServiceAccount to each service or use the regular “default” one. The service ID can be used for authorization and other purposes. This is the identifier used as a name to validate against in TLS certificates.

You can redefine this settings at the Namespace level.

## Authorization

The [AuthorizationPolicy](istio-cr.html#authorizationpolicy) resource is responsible for managing authorization. Once this resource is created for the service, the following algorithm is used for determining the fate of the request:

* The request is denied if it falls under the DENY policy.
* The request is allowed if there are no ALLOW policies for the service.
* The request is allowed if it falls under the ALLOW policy.
* In all other cases, the request is denied.

In other words, if you explicitly deny something, then only this restrictive rule will work. On the other hand, if you explicitly allow something, only explicitly authorized requests would be allowed (however, restrictions will have precedence).

You can use the following arguments for defining authorization rules:

* service IDs and wildcard expressions based on them (`mycluster.local/ns/myns/sa/myapp` or `mycluster.local/*`)
* namespace
* IP ranges
* HTTP headers
* JWT tokens

## Request routing

[VirtualService](istio-cr.html#virtualservice) is the main resource for routing control; it allows you to override the destination of an HTTP or TCP request. Routing decisions can be based on the following parameters:

* Host or other headers
* URI
* method (GET, POST, etc.)
* Pod labels or the namespace of the request source
* dst-IP or dst-port for non-HTTP requests

## Managing request balancing between service Endpoints

[DestinationRule](istio-cr.html#destinationrule) is the main resource for managing request balancing; it allows you to configure the details of requests leaving the Pods:

* limits/timeouts for TCP
* balancing algorithms between Endpoints
* rules for detecting problems on the Endpoint side to take it out of balancing
* encryption details

> **Caution!** All customizable limits apply to each client Pod individually (on a per Pod basis)! Suppose you limited a service to one TCP connection. In this case, if you have three client Pods, the service will get three incoming connections.

## Observability

### Tracing

Istio makes it possible to collect application traces and inject trace headers if there are none. In doing so, however, you have to keep in mind the following:

* If a request initiates secondary requests for a service, they must inherit the trace headers by means of the application.
* You will need to install Jaeger to collect and display traces.

### Grafana

The standard module bundle includes the following additional dashboards:

* Dashboard for evaluating the throughput and success of requests/responses between applications.
* Dashboard for evaluating control plane performance and load.

### Kiali

Kiali is a tool for visualizing your application's service tree. It allows you to quickly assess the situation in the network connectivity by visualizing the requests and their quantitative characteristics directly on the scheme.

## Architecture of the cluster with Istio enabled

The cluster components are divided into two categories:

* control plane — managing and maintaining services; "control-plane" usually refers to istiod Pods;
* data plane — mediating and controlling all network communication between microservices, it is composed of a set of sidecar-proxy containers.

![Architecture of the cluster with Istio enabled](../../images/110-istio/istio-architecture.svg)
<!--- Source: https://docs.google.com/drawings/d/1wXwtPwC4BM9_INjVVoo1WXj5Cc7Wbov2BjxKp84qjkY/edit --->

All data plane services are grouped into a mesh with the following features:

* It has a common namespace for generating service ID in the form `<TrustDomain>/ns/<Namespace>/sa/<ServiceAccount>`. Each mesh has a TrustDomain ID (in our case, it is the same as the cluster domain), e.g. `mycluster.local/ns/myns/sa/myapp`.
* Services within a single mesh can authenticate each other using trusted root certificates.

Control plane components:

* `istiod` — the main service with the following tasks:
  * Continuous connection to the Kubernetes API and collecting information about services.
  * Processing and validating all Istio-related Custom Resources using the Kubernetes Validating Webhook mechanism.
  * Configuring each sidecar proxy individually:
    * Generating authorization, routing, balancing rules, etc..
    * Distributing information about other application services in the cluster.
    * Issuing individual client certificates for implementing Mutual TLS. These certificates are unrelated to the certificates that Kubernetes uses for its own service needs.
  * Automatic tuning of manifests that describe application Pods via the Kubernetes Mutating Webhook mechanism:
    * Injecting an additional sidecar-proxy service container.
    * Injecting an additional init container for configuring the network subsystem (configuring DNAT to intercept application traffic).
    * Routing readiness and liveness probes through the sidecar-proxy.
* `operator` — installs all the resources required to operate a specific version of the control plane.
* `kiali` — dashboard for monitoring and controlling Istio resources as well as user services managed by Istio that allows you:
  * Visualize inter-service connections.
  * Viagnose problem inter-service connections.
  * Diagnose the control plane state.

The Ingress controller must be refined to receive user traffic:

* You need to add sidecar-proxy to the controller Pods. It only handles traffic from the controller to the application services (the [`enableIstioSidecar`](../402-ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) parameter of the `IngressNginxController` resource).
* Services not managed by Istio continue to function as before, requests to them are not intercepted by the controller sidecar.
* Requests to services running under Istio are intercepted by the sidecar and processed according to Istio rules (read more about [activating Istio to work with the application](#activating-istio-to-work-with-the-application)).

The istiod controller and sidecar-proxy containers export their own metrics that the cluster-wide Prometheus collects.

## Application service architecture with Istio enabled

### Details

* Each service Pod gets a sidecar container — sidecar-proxy. From the technical standpoint, this container contains two applications:
  * **Envoy** proxifies service traffic. It is responsible for implementing all the Istio functionality, including routing, authentication, authorization, etc.
  * **pilot-agent** is a part of Istio. It keeps the Envoy configurations up to date and has a built-in caching DNS server.
* Each Pod has a DNAT configured for incoming and outgoing service requests to the sidecar-proxy. The additional init container is used for that. Thus, the traffic is routed transparently for applications.
* Since incoming service traffic is redirected to the sidecar-proxy, this also applies to the readiness/liveness traffic. The Kubernetes subsystem that does this doesn't know how to probe containers under Mutual TLS. Thus, all the existing probes are automatically reconfigured to use a dedicated sidecar-proxy port that routes traffic to the application unchanged.
* You have to configure the Ingress controller to receive requests from outside the cluster:
  * The controller's Pods have additional sidecar-proxy containers.
  * Unlike application Pods, the Ingress controller's sidecar-proxy intercepts only outgoing traffic from the controller to the services. The incoming traffic from the users is handled directly by the controller itself;
* Ingress resources require refinement in the form of adding annotations:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — the Ingress controller will use the service's ClusterIP as upstream instead of the Pod addresses. In this case, traffic balancing between the Pods is handled by the sidecar-proxy. Use this option only if your service has a ClusterIP.
  * `nginx.ingress.kubernetes.io/upstream-vhost: "myservice.myns.svc"` — the Ingress controller's sidecar-proxy makes routing decisions based on the Host header. If this annotation is omitted, the controller will leave a header with the site address (e.g. `Host: example.com`).
* Resources of the Service type do not require any adaptation and continue to function properly. Just like before, applications have access to service addresses like `servicename`, `servicename.myns.svc`, etc;
* DNS requests from within the Pods are transparently redirected to the sidecar-proxy for processing:
  * This way, domain names of the services in the neighboring clusters can be disassociated from their addresses.

### User request lifecycle

#### Application with Istio turned off

<div data-presentation="../../presentations/110-istio/request_lifecycle_istio_disabled_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1BtvvtETQENVaWkEpF00zpi7xjFxfWu3ddZmvCF3f2LQ/ --->

#### Application with Istio turned on

<div data-presentation="../../presentations/110-istio/request_lifecycle_istio_enabled_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1fg_3eVA9JLizZaiN8W5vpkzOE6y9eD-4Iu10At4LN9U/ --->

## Activating Istio to work with the application

The main purpose of the activation is to add a sidecar container to the application Pods so that Istio can manage the traffic.

The sidecar-injector is a recommended way to add sidecars. Istio can inject sidecar containers into user Pods using the [Admission Webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) mechanism. You can configure it using labels and annotations:

* A label attached to a namespace allows the sidecar-injector to identify a group of Pods to inject sidecar containers into:
  * `istio-injection=enabled` — use the global version of Istio (`spec.settings.globalVersion` in `ModuleConfig`);
  * `istio.io/rev=v1x16` — use the specific Istio version for a given namespace.
* The `sidecar.istio.io/inject` (`"true"` or `"false"`) **Pod** annotation lets you redefine the `sidecarInjectorPolicy` policy locally. These annotations work only in namespaces to which the above labels are attached.

It is also possible to add the sidecar to an individual pod in namespace without the `istio-injection=enabled` or `istio.io/rev=vXxYZ` labels by setting the `sidecar.istio.io/inject=true` Pod label.

**Note!** Istio-proxy, running as a sidecar container, consumes resources and adds overhead:

* Each request is DNAT'ed to Envoy that processes it and creates another one. The same thing happens on the receiving side.
* Each Envoy stores information about all the services in the cluster, thereby consuming memory. The bigger the cluster, the more memory Envoy consumes. You can use the [Sidecar](istio-cr.html#sidecar) CustomResource to solve this problem.

It is also important to get the Ingress controller and the application's Ingress resources ready:

* Enable [`enableIstioSidecar`](../402-ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) of the `IngressNginxController` resource.
* Add annotations to the application's Ingress resources:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — the Ingress controller will use the service's ClusterIP as upstream instead of the Pod addresses. In this case, traffic balancing between the Pods is now handled by the sidecar-proxy. Use this option only if your service has a ClusterIP.
  * `nginx.ingress.kubernetes.io/upstream-vhost: "myservice.myns.svc"` — the Ingress controller's sidecar-proxy makes routing decisions based on the Host header. If this annotation is omitted, the controller will leave a header with the site address (e.g. `Host: example.com`).

## Federation and multicluster

> Available in Enterprise Edition only.

Deckhouse supports two schemes of inter-cluster interaction:

* [federation](#federation)
* [multicluster](#multicluster)

Below are their fundamental differences:

* The federation aggregates multiple sovereign (independent) clusters:
  * each cluster has its own namespace (for Namespace, Service, etc.);
  * access to individual services between clusters is clearly defined.
* The multicluster aggregates co-dependent clusters:
  * cluster namespaces are shared — each service is available to neighboring clusters as if it were running in a local cluster (unless authorization rules prohibit that).

### Federation

#### Requirements for clusters

* Each cluster must have a unique domain in the [`clusterDomain`](../../installing/configuration.html#clusterconfiguration-clusterdomain) parameter of the resource [_ClusterConfiguration_](../../installing/configuration.html#clusterconfiguration). The default value is `cluster.local`.
* Pod and Service subnets in the [`podSubnetCIDR`](../../installing/configuration.html#clusterconfiguration-podsubnetcidr) and [`serviceSubnetCIDR`](../../installing/configuration.html#clusterconfiguration-servicesubnetcidr) parameters of the resource [_ClusterConfiguration_](../../installing/configuration.html#clusterconfiguration) can be the same.

#### General principles of federation

* Federation requires mutual trust between clusters. Thereby, to use federation, you have to make sure that both clusters (say, A and B) trust each other. From a technical point of view, this is achieved by a mutual exchange of root certificates.
* You also need to share information about government services to use the federation. You can do that using ServiceEntry. A service entry defines the public ingress-gateway address of the B cluster so that services of the A cluster can communicate with the bar service in the B cluster.

<div data-presentation="../../presentations/110-istio/federation_common_principles_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1klrLIXqe-zl9Dspbsu9nTI1a1nD3v7HHQqIN4iqF00s/ --->

#### Enabling the federation

Enabling federation (via the `istio.federation.enabled = true` module parameter) results in the following activities:

* The `ingressgateway` service is added to the cluster. Its task is to proxy mTLS traffic coming from outside of the cluster to application services.
* A service gets added to the cluster that exports the following cluster metadata to the outside:
  * Istio root certificate (accessible without authentication).
  * List of public services in the cluster (available only for authenticated requests from neighboring clusters).
  * List of public addresses of the `ingressgateway` service (available only for authenticated requests from neighboring clusters).

#### Managing the federation

<div data-presentation="../../presentations/110-istio/federation_istio_federation_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1dYOeYKGaGOsgskWCDDcVJfXcMC9iQ4cvaCkhyqrDKgg/ --->

To establish a federation, you must:

* Create a set of `IstioFederation` resources in each cluster that describe all the other clusters.
  * After successful auto-negotiation between clusters, the status of `IstioFederation` resoure will be filled with neighbour's public and private metadata (`status.metadataCache.public` and `status.metadataCache.private`).
* Add the `federation.istio.deckhouse.io/public-service=` label to each resource(`service`) that is considered public within the federation.
* In the other federation clusters, a corresponding `ServiceEntry` will be created for each `service`, leading to the `ingressgateway` of the original cluster.
* ```

> It is important, that in these `services`, in the `.spec.ports` section, each port must have the `name` field filled.

### Multicluster

#### Requirements for clusters

* Cluster domains in the [`clusterDomain`](../../installing/configuration.html#clusterconfiguration-clusterdomain) parameter of the resource [_ClusterConfiguration_](../../installing/configuration.html#clusterconfiguration) must be the same for all multicluster members. The default value is `cluster.local`.
* Pod and Service subnets in the [`podSubnetCIDR`](../../installing/configuration.html#clusterconfiguration-podsubnetcidr) and [`serviceSubnetCIDR`](../../installing/configuration.html#clusterconfiguration-servicesubnetcidr) parameters of the resource [_ClusterConfiguration_](../../installing/configuration.html#clusterconfiguration) must be unique for each multicluster member.

#### General principles

<div data-presentation="../../presentations/110-istio/multicluster_common_principles_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1fmVDf-6yDSCEHhg_2vSvZcRkLSkQtUYrE6MISjZdb8Q/ --->

* Multicluster requires mutual trust between clusters. Thereby, to use multiclustering, you have to make sure that both clusters (say, A and B) trust each other. From a technical point of view, this is achieved by a mutual exchange of root certificates.
* Istio connects directly to the API server of the neighboring cluster to gather information about its services. This Deckhouse module takes care of the corresponding communication channel.

#### Enabling the multicluster

Enabling the multicluster (via the `istio.multicluster.enabled = true` module parameter) results in the following activities:

* A proxy is added to the cluster to publish access to the API server via the standard Ingress resource:
  * Access through this public address is secured by  authorization based on Bearer tokens signed with trusted keys. Deckhouse automatically exchanges trusted public keys during the mutual configuration of the multicluster.
  * The proxy itself has read-only access to a limited set of resources.
* A service gets added to the cluster that exports the following cluster metadata to the outside:
  * Istio root certificate (accessible without authentication).
  * The public API server address (available only for authenticated requests from neighboring clusters).
  * List of public addresses of the `ingressgateway` service (available only for authenticated requests from neighboring clusters).
  * Server public keys to authenticate requests to API server and to private metadata (see above).

#### Managing the multicluster

<div data-presentation="../../presentations/110-istio/multicluster_istio_multicluster_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1fy3jIynIPTrJ5Whn4eqQxeLk7ORtipDxBWP3By4buoc/ --->

To create a multicluster, you need to create a set of `IstioMulticluster` resources in each cluster that describe all the other clusters.

## Estimating overhead

A rough estimate of overhead when using Istio is available [here](https://istio.io/v1.19/docs/ops/deployment/performance-and-scalability/).
You can use the [Sidecar](istio-cr.html#sidecar) resource to limit resource consumption by limiting the field of view of a sidecar.
