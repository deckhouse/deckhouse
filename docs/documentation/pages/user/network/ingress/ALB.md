---
title: "Utilizing Application Load Balancer (ALB)"
description: "Configuring Application Load Balancer for HTTP/HTTPS/gRPC traffic in Deckhouse Kubernetes Platform. Using ingress-nginx, alb (Gateway API), and istio for request routing, SSL/TLS termination, and application publishing."
permalink: en/user/network/ingress/alb.html
extractedLinksMax: 4
relatedLinks:
  - title: "Publishing applications using the Kubernetes Gateway API"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html
  - title: "Publishing applications using the Ingress NGINX Controller"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb/nginx.html
  - title: "Publishing applications using Istio"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb/istio.html
  - title: "ALB with Kubernetes Gateway API"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html
  - title: "ALB with Ingress NGINX Controller"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/nginx.html
  - title: "Migrating from ingress-nginx to alb"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/migration.html
  - title: "ALB with Istio"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/istio.html
  - title: "Incoming traffic balancing"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/
  - title: "alb module documentation"
    url: /modules/alb/
  - title: "ingress-nginx module documentation"
    url: /modules/ingress-nginx/
  - title: "istio module documentation"
    url: /modules/istio/
---

Application deployment and application-level traffic balancing in Deckhouse Kubernetes Platform (DKP) can be performed using the following tools:

- [Ingress NGINX Controller](alb/nginx.html) (`ingress-nginx` module).
- [Kubernetes Gateway API](alb/gateway-api.html) (`alb` module).
- [Istio](alb/istio.html) (`istio` module).

## Comparison of ALB options

### Ingress-nginx

ALB based on the Ingress NGINX Controller uses the nginx web server and is implemented by the [`ingress-nginx`](/modules/ingress-nginx/) module.
This option is suitable for:

- Basic traffic routing based on domains or URLs.
- Using SSL/TLS to secure traffic.

### Kubernetes Gateway API

ALB is implemented using the [Kubernetes Gateway API](https://kubernetes.io/docs/concepts/services-networking/gateway/) via the [`alb`](/modules/alb/) module. Gateways run on Envoy Proxy. Reception and routing are described using standard API objects (Gateway, ListenerSet, HTTPRoute, and, if necessary, GRPCRoute, TLSRoute, TCPRoute, UDPRoute, BackendTLSPolicy). The controller deploys the ingress infrastructure and validates the configuration to prevent conflicting handlers.

The Gateway API model separates responsibilities between the cluster administrator (ClusterALBInstance), the namespace administrator (ALBInstance and ListenerSet — hostname, TLS, ports), and application developers (HTTPRoute and other route objects).

Use this option for:

- Publishing applications using the Gateway API model instead of the classic Ingress.
- A cluster-wide entry point or a separate gateway for an application or team within your namespace.
- HTTP/HTTPS, gRPC, TCP, UDP, and TLS termination or passthrough.
- Per-route WAF, adding GeoIP fields to HTTP request headers, OpenTelemetry tracing, or an Istio sidecar on the gateway proxy.
- Route parameters not included in the specification, via [`HTTPRoute` annotations](alb/gateway-api.html#supported-httproute-annotations).

For a comparison with `ingress-nginx` and terminology notes, read ["Comparison of the ingress-nginx and alb modules"](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/#comparison-of-the-ingress-nginx-and-alb-modules).

### Istio

An ALB based on the [`istio`](/modules/istio/) module supports traffic management in a service mesh.
Use an Istio-based ALB for:

- Routing for [canary deployment](../canary-deployment.html) and similar scenarios.
- Traffic distribution between application versions and microservices.
- Mutual TLS (mTLS) for encrypting traffic between Pods.
- Request tracing.

## How to tell what is available in the cluster

Before publishing an application, check which ALB mechanisms are enabled and configured:

1. Verify that the required module is enabled:

   ```shell
   d8 k get moduleconfig ingress-nginx alb istio
   ```

1. For Ingress NGINX — list IngressNginxController resources and note the IngressClass name:

   ```shell
   d8 k get ingressnginxcontrollers
   d8 k get ingressclass
   ```

1. For Gateway API — verify that ClusterALBInstance or ALBInstance exists and is ready, then find the managed Gateway and ListenerSet objects:

   ```shell
   d8 k get clusteralbinstances,albinstances --all-namespaces
   d8 k get gateway,listenerset --all-namespaces
   ```

1. For Istio — check IngressIstioController and the ingress gateway class label provided by the cluster administrator:

   ```shell
   d8 k get ingressistiocontrollers
   ```

Ask the cluster administrator for the IngressClass, Gateway name and namespace, or Istio ingress class to use in application manifests.

## Next steps

- Publish an application with [Kubernetes Gateway API](alb/gateway-api.html#publishing-with-listenerset-and-httproute) (`alb` module).
- Publish an application with [Ingress NGINX Controller](alb/nginx.html) (`ingress-nginx` module).
- Publish an application with [Istio](alb/istio.html) (`istio` module).
- For infrastructure setup, see the administrator guides:
  - [ALB with Kubernetes Gateway API](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#creating-a-gateway-object);
  - [ALB with Ingress NGINX Controller](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/nginx.html#load-balancing-configuration-examples);
  - [ALB with Istio](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/istio.html#istio-ingress-gateway).
- To migrate from `ingress-nginx` to `alb`, see [Migrating from ingress-nginx to alb](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/migration.html).
