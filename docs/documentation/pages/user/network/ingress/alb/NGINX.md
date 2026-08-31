---
title: "Publishing applications using the Ingress NGINX Controller"
description: "Publish applications with Ingress NGINX Controller in Deckhouse Kubernetes Platform. Ingress resource examples and gRPC load balancing."
permalink: en/user/network/ingress/alb/nginx.html
extractedLinksMax: 4
relatedLinks:
  - title: "Utilizing Application Load Balancer (ALB)"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html
  - title: "ALB with Ingress NGINX Controller"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/nginx.html
  - title: "ingress-nginx module documentation"
    url: /modules/ingress-nginx/
  - title: "ingress-nginx module Custom Resources"
    url: /modules/ingress-nginx/cr.html
  - title: "ingress-nginx module examples"
    url: /modules/ingress-nginx/examples.html
---

## Publishing applications using the Ingress NGINX Controller

To publish applications, the cluster administrator must create an Ingress controller. The setup procedure is described in ["Load balancing configuration examples"](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/nginx.html#load-balancing-configuration-examples). Ask the administrator for the controller `ingressClass` name and specify it in the Ingress resource that routes traffic to your application.

Example of a basic Ingress resource for publishing an application.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
spec:
  ingressClassName: nginx # The name of the Ingress controller provided by the cluster administrator.
  rules:
  - host: application.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: productpage
            port:
              number: 80
```

## gRPC load balancing

This section applies to gRPC publishing through the Ingress NGINX Controller (`ingress-nginx`). For Gateway API, use [GRPCRoute](gateway-api.html#grpcroute-tlsroute-tcproute-and-udproute-objects).

For automatic gRPC service load balancing behind Ingress NGINX to work,
assign a name with the prefix or value `grpc` to the port in the corresponding Service object.
