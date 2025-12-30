---
title: "ALB with Istio"
permalink: en/admin/configuration/network/ingress/alb/istio.html
description: "Configure Application Load Balancer with Istio in Deckhouse Kubernetes Platform. Istio Ingress Gateway setup, traffic management, and service mesh integration."
---

ALB with Istio is implemented via Istio Ingress Gateway or Ingress NGINX.
The [`istio`](/modules/istio/) module is used for this purpose.

## Ingress to publish applications

### Istio Ingress Gateway

Example:

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
      key: key: dedicated.deckhouse.io
      operator: Equal
      value: frontend
  resourcesRequests:
    mode: VPA
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: app-tls-secert
  namespace: d8-ingress-istio # Note that the namespace is not app-ns.
type: kubernetes.io/tls
data:
  tls.crt: |
    <tls.crt data>
  tls.key: |
    <tls.key data>
```

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
        # A Secret resource with a certificate and a key, which must be created in the d8-ingress-istio namespace.
        credentialName: app-tls-secrets
      hosts:
        - app.example.com
```

Supported Secret formats can be found at the [official website of the Istio project](https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#key-formats).

```yaml
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
```

### Ingress NGINX

To use Ingress NGINX, you need to:

* Configure the Ingress controller by adding Istio sidecar to it.
  Enable the `enableIstioSidecar` parameter in the [`ingress-nginx`](/modules/ingress-nginx/) module's [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) custom resource.
* Set up an Ingress that refers to the Service. The following annotations are mandatory for Ingress:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"`: Using this annotation,
    the Ingress controller sends requests to the Service ClusterIP (from Service CIDR) instead of sending them directly
    to the application's pods. This is required because the `istio-proxy` sidecar container only catches traffic
    directed to Service CIDR. Any requests out of this range do not go through Istio.
  * `nginx.ingress.kubernetes.io/upstream-vhost: myservice.myns.svc`: Using this annotation,
    the sidecar container can identify the application service that serves requests.

Examples:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: productpage
  namespace: bookinfo
  annotations:
    # Nginx proxies traffic to the ClusterIP instead of pods' own IPs.
    nginx.ingress.kubernetes.io/service-upstream: "true"
    # In Istio, all routing is carried out based on the `Host:` headers.
    # Instead of letting Istio know about the `productpage.example.com` external domain,
    # we use the internal domain of which Istio is aware.
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
