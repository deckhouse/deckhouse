---
title: "Application service architecture with Istio enabled"
permalink: en/architecture/network/service-with-istio.html
---

## Architecture features

* **Sidecar proxy**:
  Each service Pod gets a sidecar container, a sidecar proxy, with two applications:
  * **Envoy**: Proxies application traffic. It is responsible for implementing all the Istio functionality,
    including routing, authentication, authorization, etc.
  * **Pilot-agent**: A part of Istio that is responsible for keeping the Envoy configuration up to date
    and has a built-in caching DNS server.
* **DNAT settings**:
  * DNAT of incoming and outgoing application requests in the sidecar proxy is configured in each Pod.
    This is done using an additional init container. Thus, traffic will be intercepted transparently for applications.
  * Since incoming traffic is redirected to the sidecar proxy, this also applies to readiness/liveness probes.
    Since the Kubernetes subsystem does not support Mutual TLS probes,
    all existing probes are redirected to a port in the sidecar proxy, which passes them to the application unchanged.
* **Ingress Controller**:
  * Each Pod of an Ingress controller also includes a sidecar proxy, which handles traffic between the controller and services.
  * Incoming traffic from users is handled directly by the controller.
* **Ingress resources**:
  These resources require minimal modification in the form of adding annotations:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"`: The Ingress controller will use the service's ClusterIP
    as upstream instead of the Pod addresses. In this case, traffic balancing between the Pods is handled by the sidecar proxy.
    Use this option only if your service has a ClusterIP.
  * `nginx.ingress.kubernetes.io/upstream-vhost: "myservice.myns.svc"`: The Ingress controller's sidecar proxy makes
    routing decisions based on the Host header. If this annotation is omitted, the controller will leave a header
    with the site address (for example, `Host: example.com`).
* **Services**:
  * Service resources do not require modifications and continue to work without adaptation.
    Applications can still access service addresses like `servicename`, `servicename.myns.svc`, etc.
* **DNS queries**:
  * Internal DNS queries of Pods are transparently redirected to the sidecar proxy
    for processing to resolve DNS names of services from neighboring clusters.

### User request lifecycle

#### Application with Istio disabled

<div data-presentation="../../presentations/istio/request_lifecycle_istio_disabled_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1BtvvtETQENVaWkEpF00zpi7xjFxfWu3ddZmvCF3f2LQ/ --->

#### Application with Istio enabled

<div data-presentation="../../presentations/istio/request_lifecycle_istio_enabled_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1fg_3eVA9JLizZaiN8W5vpkzOE6y9eD-4Iu10At4LN9U/ --->
