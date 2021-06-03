---
title: "The istio module"
---

This module installs the [Istio service mesh](https://istio.io/).

## Istio features
* Mutual TLS:
  * Traffic between services is encrypted in a straightforward way using SSL;
  * The services automatically authenticate each other via individual client and server certificates;
  * Each service gets its own ID of the <TrustDomain>/ns/<Namespace>/sa/<ServiceAccount> form, where TrustDomain is the cluster domain. Each service can have its own ServiceAccount or use a “default” shared one. The service ID can be used for authorization and other purposes.
* Access authorization between services:
  * You can use the following arguments for defining authorization rules:
    * service IDs (<TrustDomain>/ns/<Namespace>/sa/<ServiceAccount>),
    * namespace,
    * IP ranges,
    * HTTP headers,
    * JWT tokens from application requests.
* Circuit Breaker:
  * Detecting hanging requests and terminating them with an error code;
  * Removing the service from balancing if the error limit is exceeded;
  * Setting limits on the number of TCP connections and requests to the service endpoint.
* Sticky Sessions:
  * Binding requests from end users to the service endpoint.
* Request routing:
  * Canary-deployment — route some of the requests to the new application version;
  * Routing decisions can be based on the following parameters:
    * Host or other headers;
    * uri,
    * Method (GET, POST, etc.).
* Observability:
  * Collecting and visualizing data for tracing service requests using Jaeger;
  * Exporting metrics about traffic between services to Prometheus and visualizing them using Grafana;
  * Visualizing traffic topology and the state of inter-service communications as well as service components using Kiali.

## The application service architecture with Istio enabled
### Details of usage
* Each pod of the service gets a sidecar container — sidecar-proxy. From the technical standpoint, this container contains two applications:
  * Envoy — proxifies service traffic. It is responsible for implementing all the Istio functionality, including routing, authentication, authorization, etc.
  * pilot-agent — a part of Istio. It keeps the Envoy configurations up to date and has a caching DNS server built-in.
* Each pod has a DNAT configured for incoming and outgoing application service requests to the sidecar-proxy. The additional init container is responsible for this. Thus, the traffic is routed transparently for applications.
* Since all incoming service traffic is redirected to the sidecar-proxy, this also applies to the readiness/liveness traffic. The corresponding Kubernetes subsystem cannot probe containers under Mutual TLS. Thus, all the existing probes are automatically reconfigured to use a dedicated sidecar-proxy port that routes traffic to the application unchanged.
* A prepared ingress controller shold be used to receive requests from users or third-party services outside of the cluster:
  * The controller's pods have additional sidecar-proxy containers.
  * Unlike application pods, the ingress controller's sidecar-proxy intercepts only outgoing traffic from the controller to the services. The incoming traffic from the users is handled directly by the controller itself.
* Ingress resources require refinement in the form of adding the following annotations:
    * nginx.ingress.kubernetes.io/service-upstream: "true" — the ingress-nginx controller will use the service's ClusterIP as upstream instead of the pod addresses. Thus, sidecar-proxy now handles traffic balancing between the pods. Use this option only if your service has a ClusterIP.
    * nginx.ingress.kubernetes.io/upstream-vhost: "myservice.myns.svc.cluster-dns-suffix" — the ingress controller's sidecar-proxy makes routing decisions based on the Host header. If this annotation is omitted, the controller will leave a header with the site address (e.g., Host: example.com).
* Resources of the Service type do not require any adaptation and continue to function properly. Applications have access to the addresses of services like servicename, servicename.myns.svc, etc., just like before.
* DNS requests from within the pods are transparently redirected to the sidecar-proxy for processing:
  * This way, domain names of the services in the neighboring clusters can be disassociated from their addresses.

### User request lifecycle
The architecture of the demo service is as follows:
* Namespace — myns.
* The foo pod:
  * Accepts user requests and sends secondary requests to the bar pod.
  * Is linked to the corresponding foo Service.
* The bar pod:
  * Accepts secondary requests from the foo pod and processes them.
  * Is linked to the corresponding bar Service.
* Ingress exposes the foo service via the example.com domain.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vTAPfksFCdlppvmwwRrdlPpeceFEikTfv9aOW3h8YrnRpV5dyKIKMAJeUlRjzsb-0i3Ur388OLcD5Ud/pub?w=1162&h=234)
<!--- Исходник: https://docs.google.com/drawings/d/1JsYtXCl8zbOdZct3SJyQTGC9VuM8kwHjqhlr7J42Uo4/edit --->

1. First, the user sends a request to example.com and that request gets directly to the ingress controller container. The controller:

    * determines that the request should be sent to the foo service in the myns namespace;
    * replaces the Host: header with foo.myns.svc.cluster.local;
    * determines the ClusterIP address;
    * sends a request to it.

1. The pod's DNAT routes the request to the sidecar-proxy.
1. The sidecar-proxy:

    * determines the location of the foo service using the detailed Host header;
    * routes the request to one of the pods combined into the service;
    * decides whether to authorize the request;
    * initiates a TLS session with one of the pods (in our case, there is only one pod) to send a request in the future.

1. When a request arrives at the pod, it is redirected to the sidecar-proxy. The latter establishes the TLS session and accepts the request through it.
1. The request reaches the foo application.
1. The application processes it and initiates the secondary request to the bar service using a partial Host: bar header. For this, it determines the IP address of the Service and connects to it. The request is then redirected to the sidecar-proxy.
1. The sidecar-proxy:

    * Receives a new request and examines its Host header to find out the request's destination. In this case, the Host is not an FQDN, but the sidecar-proxy, unlike the ingress controller proxy, can determine the FQDN by adding a local namespace to it;
    * Routes the request to one of the pods combined into the bar service;
    * Decides whether to authorize the request;
    * Initiates a TLS session with the destination of the request (the bar pod).

1. When a request arrives at the pod, it is redirected to the sidecar-proxy. The latter:

    * establishes the TLS session and accepts the request through it;
    * decides whether to authorize the request;
    * sends the request to the application.

1. The request reaches the bar application.

### Configuring Istio to work with an application
The primary purpose of the configuration is to add the envoy-based "istio-proxy" sidecar container to the application pods. Thus, all traffic will be routed through the sidecar so that Istio can manage it.

The sidecar-injector is a recommended way to add sidecars. Istio can inject sidecar containers into user pods using the [Admission Webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) mechanism. You can configure it using labels and annotations:
* A label attached to a **namespace** — allows the sidecar-injector to identify a group of pods to inject sidecar containers into:
  * `istio-injection=enabled` — use the latest installed version of Istio.
  * `istio.io/rev=v1x8x1` — use a specific version of Istio for a given namespace.
* The `sidecar.istio.io/inject` (`"true"` or `"false"`) **pod** annotation lets you redefine the `sidecarInjectorPolicy` policy locally. These annotationa work only in namespaces to which the above labels are attached.

**Note that** Istio-proxy, running as a sidecar container, consumes resources and adds overhead:
* Each request is DNAT'ed to envoy that processes it and creates another one. The same thing happens on the receiving side.
* Each envoy stores information about all the services in the cluster, thereby consuming memory. The bigger the cluster, the more memory envoy consumes. You can use the [Sidecar](istio-cr.html#sidecar) CustomResource to solve this problem.


## Architecture of the cluster with Istio enabled
The cluster components are divided into two categories:
* control plane — comprises services for managing and configuring the cluster;
* data plane — the application part of Istio; consists of sidecar-proxy containers.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vRt0avuNi0cC_PiZmzuvbuYnFbx8rEyi4lUqB2l4pDIq2j1b3MY3HUeNHKhT3S9EeFC0tQdcY3Q8ydw/pub?w=1314&h=702)
<!--- Исходник: https://docs.google.com/drawings/d/1wXwtPwC4BM9_INjVVoo1WXj5Cc7Wbov2BjxKp84qjkY/edit --->

All data plane services are grouped into a mesh with the following features:
* It has a common namespace for generating service ID in the form <TrustDomain>/ns/<Namespace>/sa/<ServiceAccount>. Each mesh has a TrustDomain ID (in our case, it is the same as the cluster's domain), e.g., mycluster.local/ns/myns/sa/myapp.
* Services within a single mesh can authenticate each other using trusted root certificates.

Control plane components:
* istiod —  is the main service with the following tasks:
    * continuous contact with the Kubernetes API and collecting information about services,
    * processing and validating all Istio-related Custom Resources using the Kubernetes Validating Webhook mechanism,
    * configuring each sidecar proxy individually:
      * generating authorization, routing, balancing rules, etc.,
      * distributing information about other supporting services in the cluster,
      * issuing individual client certificates for implementing Mutual TLS. These certificates are unrelated to the certificates that Kubernetes uses for its own service needs.
    * automatic tuning of manifests that describe application pods via the Kubernetes Mutating Webhook mechanism:
      * injecting the additional sidecar-proxy service container,
      * injecting the additional init container for configuring the network subsystem (configuring DNAT to intercept service traffic),
      * routing readiness and liveness probes through the sidecar-proxy.
* operator — this component installs all the resources required to operate a specific version of the control plane.
* kiali — this dashboard for monitoring and controlling Istio resources as well as user services managed by Istio allows you to:
    * Visualize inter-service connections;
    * Diagnose problem inter-service connections;
    * Diagnose the control plane state.
The ingress controller must be refined to be able to receive user traffic:
* The sidecar-proxy is injected into controller pods. It handles outgoing traffic to the application services only.
* If the application service is managed by Istio, sidecar-proxy establishes a Mutual TLS connection to it.
* If Istio does not manage the application service, the connection to it is established in an unencrypted form.
The istiod controller and sidecar-proxy containers export their own metrics that the cluster-wide Prometheus collects.

## Mutual TLS
Mutual TLS is the main method of mutual service authentication. It is based on the fact that all outgoing requests are verified using the server certificate, and all incoming requests are verified using the client certificate. After the verification is complete, the sidecar-proxy can identify the remote node and use these data for authorization or application purposes.
Mutual TLS is configured for each cluster globally and includes several operating modes:
* Off — Mutual TLS is disabled.
* MutualPermissive — a service can accept both plain text and mutual TLS traffic. Outgoing connections of services managed by Istio are encrypted.
* Mutual — both incoming and outgoing connections are encrypted.
You can redefine this settings at the Namespace level.

## Authorization and the decision-making algorithm
The AuthorizationPolicy resource is responsible for managing authorization. After this resource is created for the service, the following algorithm is used for determining the fate of the request:
* The request is denied if it falls under the DENY policy;
* The request is allowed if there are no ALLOW policies for the service;
* The request is allowed if it falls under the ALLOW policy;
* All other requests are denied.
In other words, if you explicitly deny something, then only this restrictive rule will work. If you explicitly allow something, only explicitly authorized requests will be allowed (however, restrictions will have precedence).
You can use the following arguments for defining authorization rules:
* service IDs and wildcard expressions based on them (`mycluster.local/ns/myns/sa/myapp` or `mycluster.local/*`),
* namespace,
* IP ranges,
* HTTP headers,
* JWT tokens from application requests.


## Federation and multicluster

The module suppoert two deployment models:

* federation
* multicluster

Here are some of their principal differences:
* Federation combines several independent clusters:
  * each cluster has its own naming space (for Namespace, Service, etc.),
  * each cluster has its own network infrastructure and independent address ranges (podSubnetCIDR & serviceSubnetCIDR),
  * services are exposed explicitly.
* Multicluster combines several dependent clusters:
  * all clusters use the shared "flat" network — IP addresses for all pods and services in all clusters are directly routable,
  * all clusters share the single namespace — for any cluster, any service looks like if it is running locally (unless prohibited by the authorization rules).

### Federation
#### Before you begin

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQj76KcY7cqhX_cHscCXdPqzrZk_nip-5vvEeRpB_1A9AXjc64uMq6uEhILn5iw8aUbLQERx1jV1yfp/pub?w=1087&h=626)
<!--- Исходник: https://docs.google.com/drawings/d/1VQ4yZl_39j2WSi7Iif5jn-ItWkjD3_W8uqNPULqEz4A/edit --->

* Federation requires establishing mutual trust between clusters. Thereby, to use federation, you have to make sure that both clusters (say, A and B) trust each other. Technically, this is achieved by exchanging root certificates.
* You also need to share information about public services to use federation. You can do that using ServiceEntry. A service entry defines the public ingress-gateway address of the B cluster so that services of the A cluster can communicate with the bar service in the B cluster.

#### Enabling federation

Enabling federation (via the `istio.federation.enabled = true` module parameter) results in the following activities:
* The ingressgateway service is added to the cluster. Its task is to proxy mTLS traffic coming from outside of the cluster to application services.
* A service gets added to the cluster for exporting the cluster metadata to the outside:
  * the Istio root certificate (no authentication required),
  * the list of public services in the cluster (available only for authenticated requests from neighboring clusters),
  * the list of public addresses of the ingressgateway service. (available only for authenticated requests from neighboring clusters).

#### Managing federation

![resources](https://docs.google.com/drawings/d/e/2PACX-1vT9c5TGwE4MQHxO548h8nrZ8SicSXWNX9KlFl5RmD2BoDce1pnxWj9ZSxZUydOa-9Z7kJMt8WLsdjgZ/pub?w=1393&h=937)
<!--- Исходник: https://docs.google.com/drawings/d/1qNyGLyPUFR2E6qLkDLnqN42sWZzPZ5u782NJJxe-7r8/edit --->

The dedicated deckhouse controller automates the federation management. An instance of this controller runs in each member cluster. The trusts algorithm works as follows:
* The cluster to be trusted (cluster-b):
  * The local controller collects meta information about the cluster and (1) shares it via the standard Ingress:
    * (1a) the public part of the root certificate,
    * (1b) the list of public services in the cluster (public service is identified via a special  `federation.istio.deckhouse.io/public-service=` label),
    * (1c) public ingress-gateway addresses.
* The cluster that trusts (cluster-a):
  * A special IstioFederation (2) resource passes details of the b-cluster to the a-cluster's controller. This resource describes:
    * (2a) the domain prefix of the remote cluster,
    * (2b) the URL where all the meta information about the remote cluster is available (the description of the metadata above).
  * The controller fetches (3) metadata available at the URL and configures the local Istio:
    * (3a) it sets the remote public root certificate as trusted,
    * (3b) for each public service in a remote cluster, it creates a corresponding ServiceEntry resource that contains detailed information about the service coordinates:
      * its hostname - a combination of the name/namespace of the service in the remote cluster (3c) and the cluster's domain suffix (3d),
      * (3e) the public IP address of the remote ingress-gateway.

This algorithm must be implemented in both directions to build mutual trust between clusters. Therefore, to implement a fully-fledged federation, you need to:
* Create a set of IstioFederation resources in each cluster that describe all the other clusters.
* Each public resource must be marked by a `federation.istio.deckhouse.io/public-service=` label.

### Multicluster
#### Before you begin

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQj76KcY7cqhX_cHscCXdPqzrZk_nip-5vvEeRpB_1A9AXjc64uMq6uEhILn5iw8aUbLQERx1jV1yfp/pub?w=1087&h=626)
<!--- Исходник: https://docs.google.com/drawings/d/1VQ4yZl_39j2WSi7Iif5jn-ItWkjD3_W8uqNPULqEz4A/edit --->

* Multicluster requires establishing mutual trust between clusters. Thereby, to implement the multicluster model, you have to make sure that both clusters (say, A and B) trust each other. Technically, this is achieved by exchanging root certificates.
* To collect information about neighboring services, Istio connects directly to the apiserver of the neighboring cluster. This module implements the corresponding communication channel.

#### Enabling multicluster

Enabling the multicluster mode (via the `istio.multicluster.enabled = true` module parameter) results in the following activities:
* A proxy is added to the cluster to expose access to the apiserver using the standard Ingress:
  * The Istio's root certificates of trusted clusters authorize access to this public address. The client's certificate must contain the Subject: CN=deckhouse line.
  * The proxy itself can only access a limited set of resources.
* The module add a service to the cluster that exports the cluster metadata:
  * The Istio root certificate (no authentication required),
  * The public address to connect to the apiserver (available only for authenticated requests from neighboring clusters).

#### Managing multicluster

![resources](https://docs.google.com/drawings/d/e/2PACX-1vTLsBzlI4m9g0BZL13XWHlhUtgSJp7TEEvUuvzYNd_7H-HGz1hSw3CbfC5OR5EyAKppD-g1wMWoeglT/pub?w=1393&h=937)
<!--- Исходник: https://docs.google.com/drawings/d/1aF9BXxQFQpuCj_j3wmMdsVz8vuDOkvQQQ_8UsOmRaGo/edit --->

The dedicated deskhouse controller automates the process of implementing a multicluster. The trust algorithm works as follows::
* The cluster to be trusted (cluster-b):
  * The local controller collects meta information about the cluster and (1) shares it via the standard Ingress:
    * (1a) the public part of the root certificate,
    * (1b) the public address of the apiserver (with a permission to read a limited set of resources; it is only available to clients with a certificate signed by the Istio root certificate (it must also contain the CN=deckhouse line)),
* The cluster that trusts (cluster-a):
  * A special IstioMulticluster (2) resource passes details of the cluster-b to the cluster-a's controller. This resource describes:
    * (2a) the URL where all the meta information about the remote cluster is available (the description of the metadata above).
  * The controller fetches (3) metadata available at the URL and configures the local Istio:
    * (3a) it adds the remote public root certificate to the lisy of trusted certificates,
    * (3b) it creates a kubeconfig to connect to the remote cluster using the public address.
* Now, the isitod that trusts knows how to connect to the API of the neighboring cluster (4). But it will get access only after the similar IstioMulticluster resource is created on the side of the cluster to be trusted (5).
* After mutual trust is established, services can connect to each other directly (6).
The algorithm must be implemented on both sides to establish mutual trust.  Thus, to create a multicluster, you need to:
* Create a set of IstioMulticluster resources in each cluster that describe all other clusters.
