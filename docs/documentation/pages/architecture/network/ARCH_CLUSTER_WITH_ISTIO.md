---
title: "Cluster architecture with Istio enabled"
permalink: en/architecture/network/cluster-with-istio.html
---

The cluster components are divided into two categories:

* **Control plane**: Managing and maintaining services. The control plane usually refers to istiod Pods.
* **Data plane**: Istio application part composed of a set of sidecar-proxy containers.

![Cluster architecture with Istio enabled](../../images/istio/istio-architecture.svg)
<!--- Source: https://docs.google.com/drawings/d/1wXwtPwC4BM9_INjVVoo1WXj5Cc7Wbov2BjxKp84qjkY/edit --->

All Data Plane services are grouped into a mesh with the following features:

* It has a common namespace for generating service ID in the form `<TrustDomain>/ns/<Namespace>/sa/<ServiceAccount>`.
  Each service mesh has a TrustDomain ID (in our case, it is the same as the cluster domain),
  for example, `mycluster.local/ns/myns/sa/myapp`.
* Authentication of services within a single service mesh using trusted root certificates.

Control plane components:

* **istiod**: The main service with the following tasks:
  * Continuous connection to the Kubernetes API and collecting information about services.
  * Processing and validating all Istio-related custom resources using the Kubernetes Validating Webhook mechanism.
  * Configuring each sidecar proxy individually:
    * Generating authorization, routing, balancing rules, etc.
    * Distributing information about other application services in the cluster.
    * Issuing individual client certificates for implementing Mutual TLS.
      These certificates are unrelated to the certificates that Kubernetes uses for its own service needs.
  * Automatic tuning of manifests that describe application Pods via the Kubernetes Mutating Webhook mechanism:
    * Injecting an additional sidecar-proxy service container.
    * Injecting an additional init container for configuring the network subsystem
      (configuring DNAT to intercept application traffic).
    * Routing readiness and liveness probes through the sidecar-proxy.
* **operator**: Installs all the resources required to operate a specific version of the control plane.
* **kiali**: Dashboard for monitoring and controlling Istio resources as well as user services managed by Istio:
  * Visualizing relationships between services.
  * Diagnosing problematic relationships.
  * Diagnosing control plane health.

With Istio enabled, the Ingress controller behavior is changed as follows:

* A sidecar proxy is added to the controller Pods.
  It only handles traffic from the controller to the application services (the [`enableIstioSidecar`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) parameter of the IngressNginxController resource).
* Services not managed by Istio continue to function as before, requests to them are not intercepted by the controller sidecar.
* Requests to services running under Istio are intercepted by the sidecar
  and processed according to Istio rules (read more about [activating Istio to work with an application](../../user/network/app_istio_activation.html)).

The istiod controller and sidecar-proxy containers export their own metrics that the cluster-wide Prometheus collects.

## Estimating overhead

Using Istio will incur additional resource costs for both control plane (istiod controller) and data plane (istio sidecars).

### Control plane

The istiod controller continuously monitors the cluster configuration,
compiles the settings for the data plane istio sidecars and distributes them over the network.
Therefore, the more applications and their instances, the more services, and the more frequently this configuration changes,
the more computational resources are required and the greater the load on the network.
Two approaches are supported to reduce the load on controller instances:

* Horizontal scaling (the [`controlPlane.replicasManagement`](/modules/istio/configuration.html#parameters-controlplane-replicasmanagement) parameter
  of the module) — the more controller instances, the fewer instances of istio sidecars to serve for each controller
  and the less CPU and network load.
* Data-plane segmentation using the [Sidecar](/modules/istio/istio-cr.html#sidecar) resource (recommended approach)
  — the smaller the scope of an individual istio sidecar, the less data in the data plane needs to be updated and the less CPU and network load.

A rough estimate of overhead for a control plane instance
that serves 1000 services and 2000 istio sidecars is 1 vCPU and 1.5 GB RAM.

### Data plane

The consumption of data plane resources (istio sidecars) is affected by many factors:

* Number of connections
* The intensity of requests
* Size of requests and responses
* Protocol (HTTP/TCP)
* Number of CPU cores
* Complexity of the service mesh configuration

A rough estimate of the overhead for an istio sidecar instance is 0.5 vCPU for 1000 requests/sec and 50 MB RAM.

Istio sidecars also increase latency in network requests, about 2.5 ms per request.
