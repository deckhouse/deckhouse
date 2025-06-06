---
title: "Architecture of the cluster with Istio enabled"
permalink: en/architecture/network/cluster-with-istio.html
---

<!-- transferred from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/istio/#architecture-of-the-cluster-with-istio-enabled -->

The cluster components are divided into two categories:

* control plane — managing and maintaining services; "control-plane" usually refers to istiod Pods;
* data plane — mediating and controlling all network communication between microservices, it is composed of a set of sidecar-proxy containers.

![Architecture of the cluster with Istio enabled](../../images/istio/istio-architecture.svg)
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
  * Diagnose problem inter-service connections.
  * Diagnose the control plane state.

The Ingress controller must be refined to receive user traffic:

* You need to add sidecar-proxy to the controller Pods. It only handles traffic from the controller to the application services (the [`enableIstioSidecar`](../ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) parameter of the `IngressNginxController` resource).
* Services not managed by Istio continue to function as before, requests to them are not intercepted by the controller sidecar.
* Requests to services running under Istio are intercepted by the sidecar and processed according to Istio rules (read more about [activating Istio to work with the application](#activating-istio-to-work-with-the-application)).

The istiod controller and sidecar-proxy containers export their own metrics that the cluster-wide Prometheus collects.
