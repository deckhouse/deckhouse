---
title: "ALB with Istio"
permalink: en/admin/configuration/network/ingress/alb/istio.html
description: "Configure Application Load Balancer with Istio in Deckhouse Kubernetes Platform. Istio Ingress Gateway setup, traffic management, and service mesh integration."
extractedLinksMax: 4
relatedLinks:
  - title: "Publishing applications using Istio"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb/istio.html
  - title: "Incoming traffic balancing"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/
  - title: "istio module documentation"
    url: /modules/istio/
  - title: "istio module Custom Resources"
    url: /modules/istio/cr.html
---

ALB with Istio is implemented via [Istio Ingress Gateway](#istio-ingress-gateway) or [Ingress NGINX](#ingress-nginx).
The [`istio`](/modules/istio/) module is used for this purpose.

Use this option when you need traffic management in a service mesh (for example, canary routing or mTLS). Configuration details are in the ["istio module documentation"](/modules/istio/).

Creating an [IngressIstioController](/modules/istio/cr.html) and preparing infrastructure is a cluster administrator task. Application publishing with Gateway and VirtualService resources, including [canary deployment](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/istio.html#canary-deployment-with-virtualservice), is described in ["Publishing applications using Istio Ingress Gateway resource"](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/istio.html#publishing-applications-using-istio-ingress-gateway-resource).

## Ingress to publish applications

### Istio Ingress Gateway {#istio-ingress-gateway}

To publish the application using the Istio Ingress Gateway, follow these steps:

1. Create an IngressIstioController resource.

   The example below creates a controller with the `HostPort` inlet on frontend nodes:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: IngressIstioController
   metadata:
     name: main
   spec:
     # ingressGatewayClass contains the label selector value used to create the Gateway resource.
     ingressGatewayClass: istio-hp
     inlet: HostPort
     hostPort:
       httpPort: 80
       httpsPort: 443
     nodeSelector:
       node-role.deckhouse.io/frontend: ""
     tolerations:
       - effect: NoExecute
         key: dedicated.deckhouse.io
         operator: Equal
         value: frontend
     resourcesRequests:
       mode: VPA
   ```

1. Create a Secret with the TLS certificate and key for HTTPS. Place it in the `d8-ingress-istio` namespace and give application developers the Secret name and the `ingressGatewayClass` value:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: app-tls-secret
     namespace: d8-ingress-istio # Secret namespace is d8-ingress-istio, not the application namespace.
   type: kubernetes.io/tls
   data:
     tls.crt: |
       <TLS_CRT_DATA>
     tls.key: |
       <TLS_KEY_DATA>
   ```

Supported Secret formats are in the [Istio documentation](https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#key-formats).

Application developers then create the Gateway and VirtualService resources. Examples, including [canary deployment](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/istio.html#canary-deployment-with-virtualservice), are in ["Publishing applications using Istio Ingress Gateway resource"](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/istio.html#publishing-applications-using-istio-ingress-gateway-resource).

### Ingress NGINX {#ingress-nginx}

To publish through Ingress NGINX with an Istio sidecar, enable the `enableIstioSidecar` parameter in the [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) of the [`ingress-nginx`](/modules/ingress-nginx/) module and share the `ingressClass` name with application developers.

Ingress and Service manifests with the required annotations are in ["Publishing applications using Ingress NGINX"](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/istio.html#publishing-applications-using-ingress-nginx).
