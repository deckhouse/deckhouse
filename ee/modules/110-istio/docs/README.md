---
title: "The istio module"
---

## What problems Istio fixes for you

[Istio](https://istio.io/) is an implementation of the Service Mesh approach, a framework for managing network traffic on a centralized basis.

Among other things, Istio solves the following tasks in a transparent for applications way:

* [Mutual TLS](#Mutual-TLS):
  * Mutual trusted service authentication;
  * Traffic encryption;
* [Access authorization between services](#Authorization):
* [Request routing](#Request-routing):
  * Canary deployments and A/B testing: send part of the requests to the new application version;
* [Managing request balancing between service Endpoints](#Managing-request-balancing-between-service-Endpoints):
  * Circuit Breaker:
    * Temporarily excluding endpoints from balancing if the error limit is exceeded;
    * Setting limits on the number of TCP connections and the number of requests per endpoint;
    * Detecting abnormal requests and terminating them with an error code;
  * Sticky Sessions:
    * Binding requests from end users to the service endpoint;
  * Locality Failover — prioritizing endpoints in the local availability zone;
* [Observability](Observability):
  * Collecting and visualizing data for tracing service requests using Jaeger;
  * Exporting metrics about traffic between services to Prometheus and visualizing them using Grafana;
  * Visualizing traffic topology and the state of inter-service communications as well as service components using Kiali;
* [Organizing a multi-datacenter cluster by joining clusters into a single Service Mesh (multicluster)](#multicluster);
* [Grouping isolated clusters into a federation with the ability to provide native (in the Service Mesh sense) access to selected services.](#Federation).

We recommend watching the [video](https://www.youtube.com/watch?v=9CUfaeT3T-A) in which we explain the term, examine the Istio architecture, and estimate the overhead.

## Mutual TLS

Mutual TLS is the main method of mutual service authentication. It is based on the fact that all outgoing requests are verified using the server certificate, and all incoming requests are verified using the client certificate. After the verification is complete, the sidecar-proxy can identify the remote node and use these data for authorization or auxiliary purposes.

Each service gets its own identifier of the following format: `<TrustDomain>/ns/<Namespace>/sa/<ServiceAccount>` where `TrustDomain` is the cluster domain in our case. You can assign your own ServiceAccount to each service or use the regular “default” one. The service ID can be used for authorization and other purposes. This is the identifier used as a name to validate against in TLS certificates.

Each cluster has a [global configuration of Mutual TLS](configuration.html#parameters-tlsmode) that includes several operating modes:
* `Off` — Mutual TLS is disabled;
* `MutualPermissive` — a service can accept both plain text and mutual TLS traffic. Outgoing service connections managed by Istio are encrypted;
* `Mutual` — both incoming and outgoing connections are established in encrypted form only.

You can redefine this settings at the Namespace level.

## Authorization

The [AuthorizationPolicy](istio-cr.html#authorizationpolicy) resource is responsible for managing authorization. Once this resource is created for the service, the following algorithm is used for determining the fate of the request:

* The request is denied if it falls under the DENY policy;
* The request is allowed if there are no ALLOW policies for the service;
* The request is allowed if it falls under the ALLOW policy.
* In all other cases, the request is denied.

In other words, if you explicitly deny something, then only this restrictive rule will work. On the other hand, if you explicitly allow something, only explicitly authorized requests would be allowed (however, restrictions will have precedence).

You can use the following arguments for defining authorization rules:
* service IDs and wildcard expressions based on them (`mycluster.local/ns/myns/sa/myapp` or `mycluster.local/*`);
* namespace;
* IP ranges;
* HTTP headers;
* JWT tokens.

## Request routing

[VirtualService](istio-cr.html#virtualservice) is the main resource for routing control; it allows you to override the destination of an HTTP or TCP request. Routing decisions can be based on the following parameters:
* Host or other headers;
* uri;
* Method (GET, POST, etc.);
* Pod labels or the namespace of the request source;
* dst-IP or dst-port for non-HTTP requests.

## Managing request balancing between service Endpoints

[DestinationRule](istio-cr.html#destinationrule) is the main resource for managing request balancing; it allows you to configure the details of requests leaving the Pods:
* limits/timeouts for TCP;
* balancing algorithms between Endpoints;
* rules for detecting problems on the Endpoint side to take it out of balancing;
* encryption details.

**Caution!** All customizable limits apply to each client Pod individually (on a per Pod basis)! Suppose you limited a service to one TCP connection. In this case, if you have three client Pods, the service will get three incoming connections.

## Observability
### Tracing

Istio makes it possible to collect application traces and inject trace headers if there are none. In doing so, however, you have to keep in mind the following:
* If a request initiates secondary requests for a service, they must inherit the trace headers by means of the application.
* You will need to install Jaeger to collect and display traces.

### Grafana

The standard module bundle includes the following additional dashboards:
* for evaluating the throughput and success of requests/responses between applications;
* for evaluating control-plane performance and load.

### Kiali

Kiali is a tool for visualizing your application's service tree. It allows you to quickly assess the situation in the network connectivity by visualizing the requests and their quantitative characteristics directly on the scheme.

## Architecture of the cluster with Istio enabled

The cluster components are divided into two categories:
* control plane — managing and maintaining services; "сontrol-plane" usually refers to istiod pods;
* data plane — mediating and controlling all network communication between microservices, it is composed of a set of sidecar-proxy containers.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vRt0avuNi0cC_PiZmzuvbuYnFbx8rEyi4lUqB2l4pDIq2j1b3MY3HUeNHKhT3S9EeFC0tQdcY3Q8ydw/pub?w=1314&h=702)
<!--- Source: https://docs.google.com/drawings/d/1wXwtPwC4BM9_INjVVoo1WXj5Cc7Wbov2BjxKp84qjkY/edit --->

All data plane services are grouped into a mesh with the following features:
* It has a common namespace for generating service ID in the form <TrustDomain>/ns/<Namespace>/sa/<ServiceAccount>. Each mesh has a TrustDomain ID (in our case, it is the same as the cluster domain), e.g., `mycluster.local/ns/myns/sa/myapp`;
* Services within a single mesh can authenticate each other using trusted root certificates;

Control plane components:
* istiod is the main service with the following tasks:
    * continuous connection to the Kubernetes API and collecting information about services;
    * processing and validating all Istio-related Custom Resources using the Kubernetes Validating Webhook mechanism;
    * configuring each sidecar proxy individually:
      * generating authorization, routing, balancing rules, etc.;
      * distributing information about other application services in the cluster;
      * issuing individual client certificates for implementing Mutual TLS. These certificates are unrelated to the certificates that Kubernetes uses for its own service needs;
    * automatic tuning of manifests that describe application pods via the Kubernetes Mutating Webhook mechanism:
      * injecting an additional sidecar-proxy service container;
      * injecting an additional init container for configuring the network subsystem (configuring DNAT to intercept application traffic);
      * routing readiness and liveness probes through the sidecar-proxy;
* operator: this component installs all the resources required to operate a specific version of the control plane;
* kiali: this dashboard for monitoring and controlling Istio resources as well as user services managed by Istio allows you:
    * visualize inter-service connections;
    * diagnose problem inter-service connections;
    * diagnose the control plane state.

The ingress controller must be refined to receive user traffic:
* You need to add sidecar-proxy to the controller Pods. It only handles traffic from the controller to the application services (the IngressNginxController [`enableIstioSidecar`](https://deckhouse.io/en/documentation/v1/modules/402-ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) parameter of the IngressNginxController resource);
* Services not managed by Istio continue to function as before, requests to them are not intercepted by the controller sidecar;
* Requests to services running under Istio are intercepted by the sidecar and processed according to Istio rules (see [Activating Istio to work with the application](#activating-istio-to-work-with-the-application)).

The istiod controller and sidecar-proxy containers export their own metrics that the cluster-wide Prometheus collects.

## Application service architecture with Istio enabled

### Details

* Each service pod gets a sidecar container — sidecar-proxy. From the technical standpoint, this container contains two applications:
  * Envoy proxifies service traffic. It is responsible for implementing all the Istio functionality, including routing, authentication, authorization, etc;
  * pilot-agent is a part of Istio. It keeps the Envoy configurations up to date and has a built-in caching DNS server;
* Each pod has a DNAT configured for incoming and outgoing service requests to the sidecar-proxy. The additional init container is used for that. Thus, the traffic is routed transparently for applications.
* Since incoming service traffic is redirected to the sidecar-proxy, this also applies to the readiness/liveness traffic. The Kubernetes subsystem that does this doesn't know how to probe containers under Mutual TLS. Thus, all the existing probes are automatically reconfigured to use a dedicated sidecar-proxy port that routes traffic to the application unchanged;
* You have to configure the Ingress controller to receive requests from outside the cluster:
  * The controller's pods have additional sidecar-proxy containers;
  * Unlike application pods, the Ingress controller's sidecar-proxy intercepts only outgoing traffic from the controller to the services. The incoming traffic from the users is handled directly by the controller itself;
* Ingress resources require refinement in the form of adding annotations:
    * `nginx.ingress.kubernetes.io/service-upstream: "true"` — the ingress-nginx controller will use the service's ClusterIP as upstream instead of the Pod addresses. In this case, traffic balancing between the Pods is handled by the sidecar-proxy. ИUse this option only if your service has a ClusterIP;
    * `nginx.ingress.kubernetes.io/upstream-vhost: "myservice.myns.svc.cluster-dns-suffix"` — the ingress controller's sidecar-proxy makes routing decisions based on the Host header. If this annotation is omitted, the controller will leave a header with the site address (e.g., `Host: example.com`);
* Resources of the Service type do not require any adaptation and continue to function properly. Just like before, applications have access to service addresses like servicename, servicename.myns.svc, etc;
* DNS requests from within the pods are transparently redirected to the sidecar-proxy for processing:
  * This way, domain names of the services in the neighboring clusters can be disassociated from their addresses.

### User request lifecycle

#### Application with Istio turned off

<iframe src="https://docs.google.com/presentation/d/e/2PACX-1vSN2yCNumnHC-Q9sgQ7LstaLuG8lWjYkvKrN27zNM4P8JxejasMeCazGIX5zYNSLuv6DieoXgI1Mx7u/embed?start=false&loop=false&delayms=3000" frameborder="0" width="960" height="569" allowfullscreen="true" mozallowfullscreen="true" webkitallowfullscreen="true"></iframe>
<!--- Source: https://docs.google.com/presentation/d/1_lw3EyDNTFTYNirqEfrRANnEAVjGhrOCdFJc-zCOuvs/edit --->
<p class="text text_alt" style="color: #2A5EFF">
  <img src="/images/icons/arrow-up.svg" alt="" style="width: 25px;margin-left: 59px;position: relative;top: -2px;">
  Control presentation
</p>

#### Application with Istio turned on

<iframe src="https://docs.google.com/presentation/d/e/2PACX-1vSBqX8-US32uDhUYKpra4Co9rYsh9wqbhUV2pMh69WC-daXwW7CYeaofH_yhDOl4pdN-tO5pIPDMqtw/embed?start=false&loop=false&delayms=3000" frameborder="0" width="960" height="569" allowfullscreen="true" mozallowfullscreen="true" webkitallowfullscreen="true"></iframe>
<!--- Source: https://docs.google.com/presentation/d/1gQfX9ge2vhp74yF5LOfpdK2nY47l_4DIvk6px_tAMPU/edit --->
<p class="text text_alt" style="color: #2A5EFF">
  <img src="/images/icons/arrow-up.svg" alt="" style="width: 25px;margin-left: 59px;position: relative;top: -2px;">
  Control presentation
</p>

## Activating Istio to work with the application

The main purpose of the activation is to add a sidecar container to the application Pods so that Istio can manage the traffic.

The sidecar-injector is a recommended way to add sidecars. Istio can inject sidecar containers into user pods using the [Admission Webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) mechanism. You can configure it using labels and annotations:
* A label attached to a **namespace** allows the sidecar-injector to identify a group of Pods to inject sidecar containers into:
  * `istio-injection=enabled` — use the latest installed version of Istio;
  * `istio.io/rev=v1x13` — use the specific Istio version for a given namespace;
* The `sidecar.istio.io/inject` (`"true"` or `"false"`) **Pod** annotation lets you redefine the `sidecarInjectorPolicy` policy locally. These annotations work only in namespaces to which the above labels are attached.

**Note that** Istio-proxy, running as a sidecar container, consumes resources and adds overhead:
* Each request is DNAT'ed to envoy that processes it and creates another one. The same thing happens on the receiving side.
* Each envoy stores information about all the services in the cluster, thereby consuming memory. The bigger the cluster, the more memory envoy consumes. You can use the [Sidecar](istio-cr.html#sidecar) CustomResource to solve this problem.

It is also important to get the ingress controller and the application's Ingress resources ready:
* enable [`enableIstioSidecar`](https://deckhouse.io/en/documentation/v1/modules/402-ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) of the IngressNginxController resource;
* add annotations to the application's Ingress resources:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — the ingress-nginx controller will use the service's ClusterIP as upstream instead of the Pod addresses. In this case, traffic balancing between the Pods is now hndled by the sidecar-proxy. Use this option only if your service has a ClusterIP;
  * `nginx.ingress.kubernetes.io/upstream-vhost: "myservice.myns.svc.cluster-dns-suffix"` — the ingress controller's sidecar-proxy makes routing decisions based on the Host header. If this annotation is omitted, the controller will leave a header with the site address (e.g., `Host: example.com`).


## Federation and multicluster

Deckhouse supports two schemes of inter-cluster interaction:

* federation
* multicluster

Below are their fundamental differences:
* The federation aggregates multiple sovereign (independent) clusters:
  * each cluster has its own namespace (for Namespace, Service, etc.);
  * access to individual services between clusters is clearly defined;
* The multicluster aggregates co-dependent clusters:
  * cluster namespaces are shared — each service is available to neighboring clusters as if it were running in a local cluster (unless authorization rules prohibit that).

### Federation

#### General principles

* Federation requires mutual trust between clusters. Thereby, to use federation, you have to make sure that both clusters (say, A and B) trust each other. From a technical point of view, this is achieved by a mutual exchange of root certificates;
* You also need to share information about government services to use the federation. You can do that using ServiceEntry. A service entry defines the public ingress-gateway address of the B cluster so that services of the A cluster can communicate with the bar service in the B cluster.

<iframe src="https://docs.google.com/presentation/d/e/2PACX-1vRGnHBdHyQq7xGP3v3kaUCsMkfBGXun5NJb4R6nRQtjOlrq4BSyZ4hIUbA92JN4OCJcoR5A3M6VCtS8/embed?start=false&loop=false&delayms=3000" frameborder="0" width="960" height="569" allowfullscreen="true" mozallowfullscreen="true" webkitallowfullscreen="true"></iframe>
<!--- Source: https://docs.google.com/presentation/d/1EI2MQMuVCGACnLNBXMGVDNJVhwU3vJYtVcHhrWfjLDc/edit --->
<p class="text text_alt" style="color: #2A5EFF">
  <img src="/images/icons/arrow-up.svg" alt="" style="width: 25px;margin-left: 59px;position: relative;top: -2px;">
  Control presentation
</p>


#### Enabling the federation

Enabling federation (via the `istio.federation.enabled = true` module parameter) results in the following activities:
* The ingressgateway service is added to the cluster. Its task is to proxy mTLS traffic coming from outside of the cluster to application services;
* A service gets added to the cluster that exports the following cluster metadata to the outside:
  * Istio root certificate (accessible without authentication);
  * list of public services in the cluster (available only for authenticated requests from neighboring clusters);
  * list of public addresses of the ingressgateway service (available only for authenticated requests from neighboring clusters).

#### Managing the federation

<iframe src="https://docs.google.com/presentation/d/e/2PACX-1vQtxDlMcQFmJT7Jc1HDose3KXwe8dGqLs_C1JSoKg0Dv6tZq9a2nibRPZh9Yihy4UoyXMHKBAFKZDIM/embed?start=false&loop=false&delayms=3000" frameborder="0" width="960" height="569" allowfullscreen="true" mozallowfullscreen="true" webkitallowfullscreen="true"></iframe>
<!--- Source: https://docs.google.com/presentation/d/1MpmtwJwvSL32EdwOUNpJ6GjgWt0gplzjqL8OOprNqvc/edit --->
<p class="text text_alt" style="color: #2A5EFF">
  <img src="/images/icons/arrow-up.svg" alt="" style="width: 25px;margin-left: 59px;position: relative;top: -2px;">
  Control presentation
</p>

To establish a federation, you must:
* Create a set of IstioFederation resources in each cluster that describe all the other clusters;
* Add the `federation.istio.deckhouse.io/public-service=` label to each resource that is considered public within the federation.

### Multicluster

#### General principles

<iframe src="https://docs.google.com/presentation/d/e/2PACX-1vQBozUYrpJ3Qzk4BWxkkAtiHuJjvG3dL0K43ZdQy6dJjkSToEAZT_2pqVlpv4vjdlmgBv16pH9juBY1/embed?start=false&loop=false&delayms=3000" frameborder="0" width="960" height="569" allowfullscreen="true" mozallowfullscreen="true" webkitallowfullscreen="true"></iframe>
<!--- Source: https://docs.google.com/presentation/d/1WeNrp0Ni2Tz3_Az0f45rkWRUZxZUDx93Om5MB3sEod8/edit --->
<p class="text text_alt" style="color: #2A5EFF">
  <img src="/images/icons/arrow-up.svg" alt="" style="width: 25px;margin-left: 59px;position: relative;top: -2px;">
  Control presentation
</p>

* Multicluster requires mutual trust between clusters. Thereby, to use multiclustering, you have to make sure that both clusters (say, A and B) trust each other. From a technical point of view, this is achieved by a mutual exchange of root certificates;
* Istio connects directly to the apiserver of the neighboring cluster to gather information about its services. This Deckhouse module takes care of the corresponding communication channel.

#### Enabling the multicluster

Enabling the multicluster (via the `istio.multicluster.enabled = true` module parameter) results in the following activities:
* A proxy is added to the cluster to publish access to the apiserver via the standard Ingress:
  * Access through this public address is secured by  authorization based on Bearer tokens signed with trusted keys. Deckhouse automatically exchanges trusted public keys during the mutual configuration of the multicluster;
  * The proxy itself has read-only access to a limited set of resources;
* A service gets added to the cluster that exports the following cluster metadata to the outside:
  * Istio root certificate (accessible without authentication);
  * The public apiserver address (available only for authenticated requests from neighboring clusters);
  * List of public addresses of the ingressgateway service (available only for authenticated requests from neighboring clusters);
  * Server public keys to authenticate requests to apiserver and to private metadata (see above).

#### Managing the multicluster

<iframe src="https://docs.google.com/presentation/d/e/2PACX-1vSg7WC5U6u8hpVKQFFOKRo8b1NwIhzXXMx26gNNrWekAcTvZOVT4-nzTAnzPnjzlAfFSYL5-U4_Qa1h/embed?start=false&loop=false&delayms=3000" frameborder="0" width="960" height="569" allowfullscreen="true" mozallowfullscreen="true" webkitallowfullscreen="true"></iframe>
<!--- Source: https://docs.google.com/presentation/d/1D3nuoC0okJQRCOY4teJ6p598Bd4JwPXZT5cdG0hW8Hc/edit --->
<p class="text text_alt" style="color: #2A5EFF">
  <img src="/images/icons/arrow-up.svg" alt="" style="width: 25px;margin-left: 59px;position: relative;top: -2px;">
  Control presentation
</p>

To create a multicluster, you need:
* Create a set of IstioMulticluster resources in each cluster that describe all the other clusters.

## Estimating overhead

ПA rough estimate of overhead when using Istio is available at https://istio.io/latest/docs/ops/deployment/performance-and-scalability/.
You can use the [Sidecar](istio-cr.html#sidecar) resource to limit resource consumption by limiting the field of view of a sidecar.
