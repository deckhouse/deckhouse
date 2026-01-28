---
title: "ALB"
permalink: en/user/network/ingress/alb.html
---

ALB (Application Load Balancer) is implemented using Ingress resources and Gateways.
ALB can handle the following types of traffic: HTTP, HTTPS, and gRPC.
An administrator-configured Ingress controller is used to publish applications.
In most cases, the [ingress-nginx](/modules/ingress-nginx/) module is used;
for more complex tasks, the [istio](/modules/istio/) module may be used.

## Recommendations for choosing and ALB specifics with ingress-nginx and istio

### Ingress-nginx

The [ingress-nginx](/modules/ingress-nginx/) ALB is based on the nginx web server.
This option is suitable for:

- Basic traffic routing based on domains or URLs.
- Using SSL/TLS to secure traffic.

### Istio

An ALB based on [istio](/modules/istio/) provides advanced traffic management capabilities.
Consider an istio-based ALB if you need:

- Advanced routing, for example, to implement [canary deployment](../canary-deployment.html).
- Traffic distribution between application versions and microservices.
- mTLS for encrypting traffic between Pods.
- Request tracing.

## Example of a basic Ingress resource for publishing an application

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: lab-5-ingress
spec:
  ingressClassName: nginx
  rules:
  - host: <Specified in your log>
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: lab-5-service
            port:
              number: 80
```

## Example of an Ingress NGINX resource

To work with Ingress NGINX, the Deckhouse Kubernetes Platform administrator must configure
the Ingress controller by adding an Istio sidecar to it.
To do this,
set the [`enableIstioSidecar`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) parameter
in the IngressNginxController custom resource of the [ingress-nginx](/modules/ingress-nginx/) module.

To publish an application, prepare an Ingress resource that references a Service.
Required annotations for the Ingress resource:

- `nginx.ingress.kubernetes.io/service-upstream: "true"`: With this annotation,
  the Ingress controller will send requests to the Service's ClusterIP (from the Service CIDR range)
  instead of sending them directly to the application's Pods.
  The `istio-proxy` sidecar container intercepts traffic only toward the Service CIDR range.
  All other requests are sent directly.
- `nginx.ingress.kubernetes.io/upstream-vhost: myservice.myns.svc`: With this annotation,
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

## Example of an Istio Ingress Gateway resource

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
        credentialName: app-tls-secrets
      hosts:
        - app.example.com
```

## gRPC load balancing

For automatic gRPC service load balancing to work,
assign a name with the prefix or value `grpc` to the port in the corresponding Service object.
