---
title: "Architecture of the cluster with Istio enabled"
permalink: en/architecture/network/cluster-with-istio.html
---

<!-- transferred from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/istio/#architecture-of-the-cluster-with-istio-enabled -->

The cluster components are divided into two categories:

* Control Plane — managing and maintaining services; "control-plane" usually refers to istiod Pods;
* Data Plane — mediating and controlling all network communication between microservices, it is composed of a set of sidecar-proxy containers.

![Architecture of the cluster with Istio enabled](../../images/istio/istio-architecture.svg)
<!--- Source: https://docs.google.com/drawings/d/1wXwtPwC4BM9_INjVVoo1WXj5Cc7Wbov2BjxKp84qjkY/edit --->

All Data Plane services are grouped into a mesh with the following features:

* It has a common namespace for generating service ID in the form `<TrustDomain>/ns/<Namespace>/sa/<ServiceAccount>`. Each mesh has a TrustDomain ID (in our case, it is the same as the cluster domain), e.g. `mycluster.local/ns/myns/sa/myapp`.
* Authentication of services within a single service mesh using trusted root certificates.

Control Plane components:

* **istiod** — the main service with the following tasks:
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
* **operator** — installs all the resources required to operate a specific version of the Control Plane.
* **kiali** — dashboard for monitoring and controlling Istio resources as well as user services managed by Istio:
  * visualizing relationships between services.
  * diagnosing problematic relationships.
  * diagnosing Control Plane health.

The Ingress controller must be refined to receive user traffic:

* You need to add sidecar-proxy to the controller Pods. It only handles traffic from the controller to the application services (the [`enableIstioSidecar`](../../modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) parameter of the `IngressNginxController` resource).
* Services not managed by Istio continue to function as before, requests to them are not intercepted by the controller sidecar.
* Requests to services running under Istio are intercepted by the sidecar and processed according to Istio rules (read more about [activating Istio to work with the application](../../user/network/app_istio_activation.html)).

The istiod controller and sidecar-proxy containers export their own metrics that the cluster-wide Prometheus collects.

## Estimating overhead

Using Istio will incur additional resource costs for both **control-plane** (istiod controller) and **data-plane** (istio-sidecars).

### control-plane

The istiod controller continuously monitors the cluster configuration, compiles the settings for the istio-sidecars and distributes them over the network. Accordingly, the more applications and their instances, the more services, and the more frequently this configuration changes, the more computational resources are required and the greater the load on the network. Two approaches are supported to reduce the load on controller instances:

* horizontal scaling (module configuration [`controlPlane.replicasManagement`](../../modules/istio/configuration.html#parameters-controlplane-replicasmanagement)) — the more controller instances, the fewer instances of istio-sidecars to serve for each controller and the less CPU and network load.
* data-plane segmentation using the [Sidecar](../../modules/istio/istio-cr.html#sidecar) resource (recommended approach) — the smaller the scope of an individual istio-sidecar, the less data in the data-plane needs to be updated and the less CPU and network overhead.

A rough estimate of overhead for a control-plane instance that serves 1000 services and 2000 istio-sidecars is 1 vCPU and 1.5 GB RAM.

### data-plane

The consumption of data-plane resources (istio-sidecar) is affected by many factors:

* number of connections,
* the intensity of requests,
* size of requests and responses,
* protocol (HTTP/TCP),
* number of CPU cores,
* complexity of Service Mesh configuration.

A rough estimate of the overhead for an istio-sidecar instance is 0.5 vCPU for 1000 requests/sec and 50 MB RAM.
istio-sidecars also increase latency in network requests — about 2.5ms per request.
