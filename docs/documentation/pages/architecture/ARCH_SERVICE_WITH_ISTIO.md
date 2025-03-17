---
title: "Application service architecture with Istio enabled"
permalink: en/architecture/service-with-istio.html
---

<!-- transfer from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/istio/#application-service-architecture-with-istio-enabled -->

## Application service architecture with Istio enabled

### Details

* Each service Pod gets a sidecar container — sidecar-proxy. From the technical standpoint, this container contains two applications:
  * **Envoy** proxies service traffic. It is responsible for implementing all the Istio functionality, including routing, authentication, authorization, etc.
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

<div data-presentation="../../presentations/istio/request_lifecycle_istio_disabled_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1BtvvtETQENVaWkEpF00zpi7xjFxfWu3ddZmvCF3f2LQ/ --->

#### Application with Istio turned on

<div data-presentation="../../presentations/istio/request_lifecycle_istio_enabled_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1fg_3eVA9JLizZaiN8W5vpkzOE6y9eD-4Iu10At4LN9U/ --->
