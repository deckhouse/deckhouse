---
title: "Publishing applications using Istio"
description: "Publish applications with Istio in Deckhouse Kubernetes Platform. NGINX Ingress with Istio sidecar and Istio Ingress Gateway with Gateway and VirtualService."
permalink: en/user/network/ingress/alb/istio.html
extractedLinksMax: 4
relatedLinks:
  - title: "Utilizing Application Load Balancer (ALB)"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html
  - title: "ALB with Istio"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/istio.html
  - title: "istio module documentation"
    url: /modules/istio/
---

## Publishing applications using Istio

Application publishing with Istio is configured in two layers. The cluster administrator deploys the IngressIstioController (and related infrastructure) as described in ["Istio Ingress Gateway"](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/istio.html#istio-ingress-gateway). Application developers create Gateway and VirtualService resources as shown below.

When deploying an application using Istio, you can choose one of the following options:

- ["Using NGINX Ingress"](#publishing-applications-using-nginx-ingress).
- ["Publishing applications using Istio Ingress Gateway resource"](#publishing-applications-using-istio-ingress-gateway-resource).

### Publishing applications using NGINX Ingress {#publishing-applications-using-nginx-ingress}

To publish an application using NGINX Ingress, the DKP administrator must configure the Ingress controller by adding an Istio sidecar to it.

To publish an application, prepare an Ingress resource that references a Service.
Required annotations for the Ingress resource:

- `nginx.ingress.kubernetes.io/service-upstream: "true"`: With this annotation,
  the Ingress controller will send requests to the Service's ClusterIP (from the Service CIDR range)
  instead of sending them directly to the application's Pods.
  The `istio-proxy` sidecar container intercepts traffic only toward the Service CIDR range.
  All other requests are sent directly.
- `nginx.ingress.kubernetes.io/upstream-vhost: productpage.bookinfo.svc`: With this annotation,
  the sidecar can identify the application service the request is intended for.

Examples:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: productpage
  namespace: bookinfo
  annotations:
    # Enables proxying traffic to the Service's ClusterIP via nginx instead of directly to Pod IPs.
    nginx.ingress.kubernetes.io/service-upstream: "true"
    # In Istio, all routing is based on the `Host:` request header.
    # This avoids the need to inform Istio about the existence of the external domain `productpage.example.com`;
    # the internal domain known to Istio is used instead.
    nginx.ingress.kubernetes.io/upstream-vhost: productpage.bookinfo.svc
spec:
  rules:
    - host: productpage.example.com
      http:
        paths:
        - path: /
          pathType: Prefix
          backend:
            service:
              name: productpage
              port:
                number: 9080
```

```yaml
apiVersion: v1
kind: Service
metadata:
  name: productpage
  namespace: bookinfo
spec:
  ports:
  - name: http
    port: 9080
  selector:
    app: productpage
  type: ClusterIP
```

### Publishing applications using Istio Ingress Gateway resource {#publishing-applications-using-istio-ingress-gateway-resource}

To publish an application using the Istio Ingress Gateway, the DKP administrator must create an IngressIstioController resource.

To publish an application using the Istio Ingress Gateway resource:

1. Create a Gateway resource. In the `spec.selector` field, specify the label referencing the ingressGatewayClass and the secret name provided by the cluster administrator:

   ```yaml
   apiVersion: networking.istio.io/v1beta1
   kind: Gateway
   metadata:
     name: gateway-app
     namespace: app-ns
   spec:
     selector:
       # Label selector for using the Istio Ingress Gateway main-hp.
       istio.deckhouse.io/ingress-gateway-class: istio-hp
     servers:
       - port:
           # Standard template for using the HTTP protocol.
           number: 80
           name: http
           protocol: HTTP
         hosts:
           - app.example.com
       - port:
           # Standard template for using the HTTPS protocol.
           number: 443
           name: https
           protocol: HTTPS
         tls:
           mode: SIMPLE
           # Secret resource with the certificate and key, which must be created in the d8-ingress-istio namespace.
           # Supported Secret formats can be found at https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#key-formats.
           credentialName: app-tls-secret
         hosts:
           - app.example.com
   ```

1. Define routing rules using a VirtualService that links the gateway to the service it serves:

   ```yaml
   apiVersion: networking.istio.io/v1beta1
   kind: VirtualService
   metadata:
     name: vs-app
     namespace: app-ns
   spec:
     gateways:
       - gateway-app # The name of the Gateway resource created in the previous step.
     hosts:
       - app.example.com
     http:
       - route:
           - destination:
               host: app-svc # The name of the service to which traffic should be directed.
```
