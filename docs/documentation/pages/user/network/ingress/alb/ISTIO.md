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

- ["Using Ingress NGINX"](#publishing-applications-using-ingress-nginx).
- ["Publishing applications using Istio Ingress Gateway resource"](#publishing-applications-using-istio-ingress-gateway-resource).

### Publishing applications using Ingress NGINX {#publishing-applications-using-ingress-nginx}

To publish an application using Ingress NGINX, the DKP administrator must configure the Ingress controller by adding an Istio sidecar to it.

To publish an application, prepare an Ingress resource that references a Service. Set `ingressClassName` to the controller with the Istio sidecar (the administrator provides the value). Required annotations for the Ingress resource:

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
  ingressClassName: nginx # IngressClass name of the controller with the Istio sidecar.
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

To publish an application using the Istio Ingress Gateway resource, create a Gateway. In `spec.selector`, specify the label that references the ingressGatewayClass and the secret name provided by the cluster administrator:

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
        # Secret with the certificate and key created by the administrator in the d8-ingress-istio namespace.
        # Supported Secret formats: https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#key-formats.
        credentialName: app-tls-secret
      hosts:
        - app.example.com
```

Then define routing rules with a VirtualService that links the gateway to the service it serves:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: vs-app
  namespace: app-ns
spec:
  gateways:
    - gateway-app # Name of the Gateway resource created above.
  hosts:
    - app.example.com
  http:
    - route:
        - destination:
            host: app-svc # Name of the service that should receive traffic.
```

### Canary deployment with VirtualService {#canary-deployment-with-virtualservice}

For a general overview of canary in DKP, see ["Canary deployment"](/products/kubernetes-platform/documentation/v1/user/network/canary-deployment.html). The example below uses VirtualService and DestinationRule.

To shift traffic gradually between application versions, use a DestinationRule with subsets and weights in the VirtualService. The example below sends 90% of traffic to the stable version and 10% to canary:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: app-svc
  namespace: app-ns
spec:
  host: app-svc
  subsets:
    - name: stable
      labels:
        version: v1
    - name: canary
      labels:
        version: v2
---
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: vs-app
  namespace: app-ns
spec:
  gateways:
    - gateway-app
  hosts:
    - app.example.com
  http:
    - route:
        - destination:
            host: app-svc
            subset: stable
          weight: 90
        - destination:
            host: app-svc
            subset: canary
          weight: 10
```

Pods of each version must have the `version: v1` and `version: v2` labels that match the DestinationRule subsets. Before changing weights, confirm that both versions are selected by the `app-svc` Service.

After you apply the manifests, send a series of requests to `app.example.com` and confirm that responses roughly match the configured weights (about 9 to 1). Adjust the weights if needed and recheck.
