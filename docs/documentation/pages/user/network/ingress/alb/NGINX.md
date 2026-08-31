---
title: "Publishing applications using the Ingress NGINX Controller"
description: "Publish applications with Ingress NGINX Controller in Deckhouse Kubernetes Platform. Ingress examples, HTTPS, gRPC, and verification."
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

{% tabs Ingress examples %}
{% tab "HTTP" %}

Example of a basic Ingress for publishing an application over HTTP:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  namespace: prod
spec:
  ingressClassName: nginx # IngressClass name provided by the cluster administrator.
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

{% endtab %}
{% tab "HTTPS" %}

For HTTPS, reference a Secret with the certificate in the `tls` section. The Secret must exist in the same namespace as the Ingress. The certificate can be issued by cert-manager or created by an administrator.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  namespace: prod
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - application.example.com
      secretName: application-tls # Secret with tls.crt and tls.key in the prod namespace.
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

{% endtab %}
{% endtabs %}

Additional routing options are set with Ingress annotations. The supported annotation list is in the [`ingress-nginx` module documentation](/modules/ingress-nginx/). Common examples:

- `nginx.ingress.kubernetes.io/rewrite-target` — path rewrite.
- `nginx.ingress.kubernetes.io/whitelist-source-range` — allowed CIDR list.
- `nginx.ingress.kubernetes.io/backend-protocol: "GRPC"` — backend protocol for gRPC when the Service port name does not indicate gRPC.

### Verification

After you apply the Ingress, check the object and the controller address:

```shell
d8 k -n <NAMESPACE> get ingress
d8 k -n <NAMESPACE> describe ingress <INGRESS_NAME>
```

Confirm that the Ingress status includes an address and that DNS for `host` points to the controller entry point provided by the administrator.

Smoke test from a workstation (replace with the entry point address):

```shell
curl -vk \
  --resolve app.example.com:443:<ENTRY_POINT_ADDRESS> \
  https://app.example.com/
```

## gRPC load balancing

This section applies to gRPC publishing through the Ingress NGINX Controller (`ingress-nginx`). For Gateway API, use [GRPCRoute](gateway-api.html#grpcroute-tlsroute-tcproute-and-udproute-objects).

For automatic gRPC service load balancing behind Ingress NGINX, assign a name with the prefix or value `grpc` to the port in the corresponding Service object. If the port name is different, add the `nginx.ingress.kubernetes.io/backend-protocol: "GRPC"` annotation to the Ingress.
