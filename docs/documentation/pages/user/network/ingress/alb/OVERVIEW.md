---
title: "Utilizing Application Load Balancer (ALB)"
description: "Configuring Application Load Balancer for HTTP/HTTPS/gRPC traffic in Deckhouse Platform. Using ingress-nginx, alb (Gateway API), and istio for request routing, SSL/TLS termination, and application publishing."
permalink: en/user/network/ingress/alb/
extractedLinksMax: 0
relatedLinks:
  - title: "Publishing applications using the Kubernetes Gateway API"
    url: alb/gateway-api.html
  - title: "Publishing applications using the Ingress NGINX Controller"
    url: alb/nginx.html
  - title: "Publishing applications using Istio"
    url: alb/istio.html
  - title: "Incoming traffic balancing"
    url: ../../../../admin/configuration/network/ingress/
  - title: "alb module documentation"
    url: /modules/alb/
  - title: "ingress-nginx module documentation"
    url: /modules/ingress-nginx/
  - title: "istio module documentation"
    url: /modules/istio/
---

Application deployment and application-level traffic balancing in Deckhouse Platform (DP) can be performed using the following tools:

- [Ingress NGINX Controller](alb/nginx.html) (`ingress-nginx` module).
- [Kubernetes Gateway API](alb/gateway-api.html) (`alb` module).
- [Istio](alb/istio.html) (`istio` module).

## Comparison of ALB options

The following sections describe each ALB option and typical scenarios for using it.

### Ingress NGINX

ALB based on the Ingress NGINX Controller uses the nginx web server and is implemented by the [`ingress-nginx`](/modules/ingress-nginx/) module.
This option is suitable for:

- Basic traffic routing based on domains or URLs.
- Using SSL/TLS to secure traffic.

### Kubernetes Gateway API

ALB is implemented using the [Kubernetes Gateway API](https://kubernetes.io/docs/concepts/services-networking/gateway/) via the [`alb`](/modules/alb/) module. Gateways run on Envoy Proxy. Reception and routing are described using standard API objects (Gateway, ListenerSet, HTTPRoute, and, if necessary, GRPCRoute, TLSRoute, TCPRoute, UDPRoute, BackendTLSPolicy). The controller deploys the ingress infrastructure and validates the configuration to prevent conflicting handlers.

The Gateway API model separates responsibilities between the cluster administrator ([ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance)), the namespace administrator ([ALBInstance](/modules/alb/cr.html#albinstance) and ListenerSet — hostname, TLS, ports), and application developers (HTTPRoute and other route objects).

Use this option for:

- Publishing applications using the Gateway API model instead of the classic Ingress.
- A cluster-wide entry point or a separate gateway for an application or team within your namespace.
- HTTP/HTTPS, gRPC, TCP, UDP, and TLS termination or passthrough.
- Per-route WAF or an Istio sidecar on the gateway proxy.
- GeoIP and OpenTelemetry on the gateway (configured by an administrator — ["Using GeoIP and GeoLite2"](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#geoip) and ["Configuring OpenTelemetry tracing"](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#tracing)).
- Route parameters not included in the specification, via [HTTPRoute annotations](alb/gateway-api.html#supported-httproute-annotations).

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

1. Verify that the required module is enabled — the `STATE` column should show `Enabled`:

   ```shell
   d8 k get moduleconfig ingress-nginx alb istio
   ```

1. For Ingress NGINX — list [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) resources and note the IngressClass name:

   ```shell
   d8 k get ingressnginxcontrollers
   d8 k get ingressclass
   ```

1. For Gateway API — verify that [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) or [ALBInstance](/modules/alb/cr.html#albinstance) exists and is in the `Ready` state, then find the managed Gateway and ListenerSet objects:

   ```shell
   d8 k get clusteralbinstances,albinstances --all-namespaces
   d8 k get gateway,listenerset --all-namespaces
   ```

1. For Istio — check [IngressIstioController](/modules/istio/cr.html) and the ingress gateway class label provided by the cluster administrator:

   ```shell
   d8 k get ingressistiocontrollers
   ```

Ask the cluster administrator for the IngressClass, Gateway name and namespace, or Istio ingress class to use in application manifests.

## Next steps

Once the administrator has configured the infrastructure, you can publish the application:
- using the [Kubernetes Gateway API](alb/gateway-api.html#publishing-with-listenerset-and-httproute) (`alb` module);
- using the [Ingress NGINX Controller](alb/nginx.html) (the `ingress-nginx` module);
- using [Istio](alb/istio.html) (the `istio` module).
