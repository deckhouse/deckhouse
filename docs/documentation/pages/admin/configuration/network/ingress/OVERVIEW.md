---
title: "Incoming traffic balancing"
permalink: en/admin/configuration/network/ingress/
description: "Configure ingress load balancing in Deckhouse Kubernetes Platform with NLB and ALB. Traffic routing, SSL termination, and application-level load balancing setup."
---

This section describes the approaches to balancing incoming traffic in Deckhouse Kubernetes Platform (DKP):

- NLB (Network Load Balancer) — operates at the network level, routing traffic based on IP addresses
  and ports without inspecting request contents.
- ALB (Application Load Balancer) — operates at the application level, analyzing HTTP(S) headers, paths, and domains.
  It supports SSL termination and content-based routing.

## Network-level load balancing (NLB)

NLB-based load balancing can be implemented in two ways:

- Using an external load balancer provided by a cloud provider.
- Using the built-in MetalLB balancer, which works in both cloud and bare-metal clusters.

## Application-level load balancing (ALB)

For application-level traffic balancing, DKP provides the following solutions:

- [Ingress NGINX Controller](https://github.com/kubernetes/ingress-nginx) (via the [`ingress-nginx`](/modules/ingress-nginx/) module).
- [Kubernetes Gateway API](https://kubernetes.io/docs/concepts/services-networking/gateway/) ([`alb`](/modules/alb/) module).
- [Istio](https://istio.io/) (via the [`istio`](/modules/istio/) module).

### Difference between the Kubernetes Gateway API and an API gateway {#gateway-api-vs-api-gateway}

Despite the similar names, these are different concepts:

- The Kubernetes Gateway API is a set of Kubernetes resources (a specification) that describe how inbound traffic is routed to services. It is a configuration interface implemented by controllers, and the successor to the Ingress API.
- An API gateway is an architectural component (or product) that aggregates several application APIs behind a single entry point and centralizes cross-cutting functions such as authentication, authorization, and request rate limiting for API consumers.

In other words, the Kubernetes Gateway API describes how to configure traffic routing, while an API gateway is a type of infrastructure that handles that traffic. Some API gateways can be configured through the Kubernetes Gateway API. The `alb` module is an implementation of the Kubernetes Gateway API.

### Role separation in the Gateway API model {#role-separation}

When using the `alb` module, responsibilities are typically split as follows:

- cluster administrator — deploys cluster-scroped gateway infrastructure with ClusterALBInstance;
- namespace administrator — deploy namespaced gateway infrastructure with  ALBInstance and configures how traffic is accepted with ListenerSet;
- application team — publishes applications with HTTPRoute and other route objects.

This model simplifies multitenancy compared with the classic Ingress approach.

### Comparison of the ingress-nginx and alb modules {#comparison-of-the-ingress-nginx-and-alb-modules}

Both modules solve the same task — receiving and routing external traffic to applications — but rely on different standards: `ingress-nginx` uses the Ingress API with annotations, while `alb` uses the Kubernetes Gateway API. The modules can be used in a cluster simultaneously. The table below compares their capabilities in the current versions.

| Capability | `ingress-nginx` | `alb` |
| :--- | :--- | :--- |
| Routing standard | Ingress API with annotations | Kubernetes Gateway API |
| Proxy implementation | nginx | Envoy Proxy |
| Lifecycle stage | General Availability | Preview |
| Development | Maintenance mode: the upstream Ingress NGINX project no longer develops new features, while DKP provides security updates | Actively developed |
| Minimum DKP version | Available in all supported versions | 1.76 |
| DKP editions | All editions | All editions |
| Role separation model | cluster administrator, namespace administrator | cluster administrator, namespace administrator, application team |
| Multiple independent entry points | Multiple Ingress controllers selected via `ingressClass` | Multiple Gateway objects selected via `gatewayName`; cluster-scoped and namespaced gateways |
| HTTP/HTTPS (HTTP/1.1, HTTP/2, HTTP/3) | Yes | Yes |
| WebSocket | Yes | Yes |
| gRPC | Yes | Yes |
| FastCGI | Yes | No |
| TCP | No | Yes (TCPRoute) |
| UDP | No | Yes (UDPRoute) |
| TLS passthrough | Yes | Yes (TLSRoute) |
| Proxy Protocol | Yes | Yes |
| Traffic ingress methods | `LoadBalancer`, `HostNetwork` and `HostPort` inlets | `LoadBalancer` and `HostPort` inlets |
| Automatic TLS certificate issuance (cert-manager) | Yes | Yes |
| HTTPS policy tuning (TLS versions, ciphers, HSTS) | Yes | TLSv1.2/1.3 by default; HSTS via a response-header annotation |
| WAF | ModSecurity at the controller or Ingress level | ModSecurity/Coraza at the route level, OWASP CRS preset |
| External authentication | Yes | Yes |
| IP allowlist | Yes | Yes |
| Basic authentication | Yes | Yes |
| Request rate limiting | Yes | Yes |
| Session affinity | Yes | Yes |
| GeoIP | Geo-based request statistics in metrics | Request enrichment with headers based on MaxMind databases |
| Prometheus metrics and Grafana dashboards | Yes, detailed by namespace, vhost, Ingress resource, and location | Yes: Envoy Proxy metrics and dashboards for requests, routes, and upstreams |
| OpenTelemetry tracing | Yes | Yes |

Further setup:

- [ALB with Ingress NGINX Controller](alb/nginx.html);
- [ALB with Kubernetes Gateway API](alb/alb-gateway-api.html);
- [ALB with Istio](alb/istio.html).
