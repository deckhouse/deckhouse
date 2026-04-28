---
title: "Utilizing Application Load Balancer (ALB)"
description: "Configuring Application Load Balancer for HTTP/HTTPS/gRPC traffic in Deckhouse Kubernetes Platform. Using ingress-nginx and istio for request routing, SSL/TLS termination, and application publishing."
permalink: en/user/network/ingress/alb.html
---

Application deployment and application-level traffic balancing can be performed using the following tools:

- [Ingress NGINX Controller](#publishing-applications-using-the-ingress-nginx-controller) (`ingress-nginx` module).
- [Kubernetes Gateway API](#publishing-applications-using-the-kubernetes-gateway-api) (`alb` module).
- [Istio](#publishing-applications-using-istio) (`istio` module).

## Recommendations for choosing and features of different types of ALBs

### Ingress-nginx

ALB, powered by Ingress NGINX Controller, is based on the nginx web server and is implemented by a [`ingress-nginx`](/modules/ingress-nginx/) module.
This option is suitable for:

- Basic traffic routing based on domains or URLs.
- Using SSL/TLS to secure traffic.

### Kubernetes Gateway API

ALB is implemented using the [Kubernetes Gateway API](https://kubernetes.io/docs/concepts/services-networking/gateway/) via the [`alb`](/modules/alb/) module. Gateways run on Envoy Proxy, and reception and routing are described using standard API objects (Gateway, ListenerSet, HTTPRoute, and, if necessary, GRPCRoute, TLSRoute, TCPRoute, BackendTLSPolicy). The controller deploys the necessary ingress infrastructure and validates the configuration to prevent conflicting handlers.

You should choose this option if you need:

- To publish applications using the Gateway API model instead of the classic Ingress.
- A cluster-wide entry point or a separate gateway for an application or team within your namespace.
- HTTP/HTTPS and gRPC routing, TLS termination or pass-through, as well as TCP after TLS termination at the gateway.
- Route parameters not included in the specification, via [`HTTPRoute` annotations](#supported-httproute-annotations).

### Istio

An ALB based on [`istio`](/modules/istio/) module provides advanced traffic management capabilities.
Consider an istio-based ALB if you need:

- Advanced routing, for example, to implement [canary deployment](../canary-deployment.html).
- Traffic distribution between application versions and microservices.
- mTLS for encrypting traffic between Pods.
- Request tracing.

## Publishing applications using the Ingress NGINX Controller

To publish applications, the cluster administrator must create an Ingress controller. Specify the name of this object in the Ingress resource manifest, which is used to route incoming traffic to your application.

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

## Publishing applications using the Kubernetes Gateway API

Applications can be deployed via a cluster-wide gateway (using a `ClusterALBInstance` object created by the cluster administrator) or via a separate gateway for an application or pod in a dedicated namespace (using a `ClusterALBInstance` object).

### Publishing an application through a ClusterALBInstance object

This scenario assumes that the ClusterALBInstance object has already been created by an administrator and has reached the `Ready` state. The name and namespace of the managed Gateway object should be taken from the [`status`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-status) of the ClusterALBInstance object.

Next, create a `ListenerSet` object that will be bound to the desired gateway (using the `spec.parentRef.name` parameter) and HTTPRoute objects (routes) to route incoming requests to the application. Example:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: app-listeners
  namespace: prod
spec:
  parentRef:
    name: public-gw   # The name of the Gateway object from the ClusterALBInstance status, provided by the administrator.
    namespace: d8-alb
  listeners:
    - name: app-http
      port: 80 # HTTP traffic always uses 80 regardless of ClusterALBInstance settings.
      protocol: HTTP
      hostname: app.example.com
    - name: app-https
      port: 443 # HTTPS traffic always uses 443 regardless of ClusterALBInstance settings.
      protocol: HTTPS
      hostname: app.example.com
      tls:
        mode: Terminate
        certificateRefs:
          - name: app-tls   # Reference to the secret with the TLS certificate.
            namespace: prod
---
# Route for HTTP traffic
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app-http-route
  namespace: prod
spec:
  parentRefs:
    - name: app-listeners # ListenerSet name.
      namespace: prod
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: app-http
      port: 80 # HTTP traffic always uses 80 regardless of ClusterALBInstance settings.
  hostnames:
    - app.example.com
  rules:
    - backendRefs:
        - name: app-svc # Reference to the internal load balancer of the application.
          port: 8080 
---
# Route for HTTP traffic
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app-https-route
  namespace: prod
spec:
  parentRefs:
    - name: app-listeners # ListenerSet name.
      namespace: prod
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: app-https
      port: 443 # HTTPS traffic always uses 443 regardless of ClusterALBInstance settings.
  hostnames:
    - app.example.com
  rules:
    - backendRefs:
        - name: app-svc # Reference to the internal load balancer of the application.
          port: 8080
```

### Publishing an application through a ALBInstance object

In this scenario, the ALBInstance object, the Gateway object, the ListenerSet object, and the HTTPRoute object live in the same namespace.

To publish an application using the ALBInstance object, follow these steps:

1. Create the ALBInstance object taking into account the required [settings](/modules/alb/cr.html#albinstance):

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: ALBInstance
   metadata:
     name: app-gw
     namespace: prod
   spec:
     gatewayName: app-gw
     inlet:
       type: LoadBalancer
   ```

1. After the ALBInstance object reaches the `Ready` state, create the ListenerSet object and the HTTPRoute object:

   ```yaml
   apiVersion: gateway.networking.k8s.io/v1
   kind: ListenerSet
   metadata:
     name: app-listeners
     namespace: prod
   spec:
     parentRef:
       name: app-gw # The name of the Gateway object from the ClusterALBInstance.
       namespace: prod
     listeners:
       - name: app-https
         port: 443
         protocol: HTTPS
         hostname: app.example.com
         tls:
           mode: Terminate
           certificateRefs:
             - name: app-tls   # Reference to the Secret with the TLS certificate.
               namespace: prod
   ---
   apiVersion: gateway.networking.k8s.io/v1
   kind: HTTPRoute
   metadata:
     name: app-route
     namespace: prod
   spec:
     parentRefs:
       - name: app-listeners # ListenerSet name.
         namespace: prod
         kind: ListenerSet
         group: gateway.networking.k8s.io
         sectionName: app-https
         port: 443
     hostnames:
       - app.example.com
     rules:
       - backendRefs:
           - name: app-svc # Reference to the internal load balancer of the application.
             port: 8080
   ```

### Working with GRPCRoute, TLSRoute, and TCPRoute objects

The GRPCRoute object is intended for gRPC traffic. For it, create the ListenerSet object with an HTTPS listener, then add the GRPCRoute object:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: grpc-listeners
  namespace: prod
spec:
  parentRef:
    name: app-gw # The name of the Gateway object from the ClusterALBInstance.
    namespace: prod
  listeners:
    - name: grpc-https
      port: 443
      protocol: HTTPS
      hostname: grpc.example.com
      tls:
        mode: Terminate
        certificateRefs:
          - name: grpc-tls  # Reference to the Secret with the TLS certificate.
            namespace: prod
---
apiVersion: gateway.networking.k8s.io/v1
kind: GRPCRoute
metadata:
  name: grpc-route
  namespace: prod
spec:
  parentRefs:
    - name: grpc-listeners # ListenerSet name.
      namespace: prod
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: grpc-https
      port: 443
  hostnames:
    - grpc.example.com
  rules:
    - backendRefs:
        - name: grpc-svc # Reference to the Secret with the TLS certificate.
          port: 9090
```

For TLS passthrough, when traffic must be decrypted on the application side, either a TLS listener or an HTTPS listener can be used. The example below shows the TLS listener variant:

To accept TCP traffic on an additional port, configure the `additionalPorts` parameter in the ALBInstance:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ALBInstance
metadata:
  name: app-gw
  namespace: prod
spec:
  gatewayName: app-gw
   inlet:
      type: LoadBalancer
      additionalPorts:
      - port: 8443    # An additional TCP port to accept TLS traffic.
        protocol: TCP
```

Next, configure ListenerSet and TLSRoute objects respectively:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: tls-pass-listeners
  namespace: prod
spec:
  parentRef:
    name: app-gw # The name of the Gateway object from the ClusterALBInstance.
    namespace: prod
  listeners:
    - name: tls-pass
      port: 8443           # In this case 8443 port is used for TLS.
      protocol: TLS
      hostname: pass.example.com
      tls:
        mode: Passthrough  # TLS passthrough mode is set explicitly.
---
apiVersion: gateway.networking.k8s.io/v1alpha3
kind: TLSRoute
metadata:
  name: tls-pass-route
  namespace: prod
spec:
  parentRefs:
    - name: tls-pass-listeners # ListenerSet name.
      namespace: prod
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: tls-pass
      port: 8443           # In this case 8443 port is used for TLS.
  hostnames:
    - pass.example.com
  rules:
    - backendRefs:s
        - name: tls-pass-svc  # Reference to the internal load balancer of the application.
          port: 8443
```

The same scenario can also be implemented through an HTTPS listener. This variant is especially useful when the standard handler on port `443` should be used because no extra port needs to be opened for TLS passthrough:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: https-pass-listeners
  namespace: prod
spec:
  parentRef:
    name: app-gw # The name of the Gateway object from the ClusterALBInstance.
    namespace: prod
  listeners:
    - name: https-pass
      port: 443 # In this case 443 (HTTPS) port is reused for TLS.
      protocol: HTTPS
      hostname: pass.example.com
      tls:
        mode: Passthrough  # TLS passthrough mode is set explicitly.
---
apiVersion: gateway.networking.k8s.io/v1alpha3
kind: TLSRoute
metadata:
  name: https-pass-route
  namespace: prod
spec:
  parentRefs:
    - name: https-pass-listeners # ListenerSet name.
      namespace: prod
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: https-pass
      port: 443 # In this case 443 (HTTPS) port is reused for TLS.
  hostnames:
    - pass.example.com
  rules:
    - backendRefs:
        - name: tls-pass-svc # Reference to the internal load balancer of the application.
          port: 8443
```

If TLS must be terminated on the gateway and then the traffic must be passed further as a regular TCP stream, create a ListenerSet object with a TLS listener in `Terminate` mode, then attach a TCPRoute object:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: tls-term-listeners
  namespace: prod
spec:
  parentRef:
    name: app-gw # The name of the Gateway object from the ClusterALBInstance.
    namespace: prod
  listeners:
    - name: tls-term
      port: 443 # In this case 443 (HTTPS) port is reused for TLS.
      protocol: TLS
      hostname: term.example.com
      tls:
        mode: Terminate
        certificateRefs:
          - name: term-tls  # Reference to the Secret with the TLS certificate.
            namespace: prod
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: TCPRoute
metadata:
  name: tls-term-route
  namespace: prod
spec:
  parentRefs:
    - name: tls-term-listeners # ListenerSet name.
      namespace: prod
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: tls-term
      port: 443 # In this case 443 (HTTPS) port is reused for TLS.
  rules:
    - backendRefs:
        - name: tcp-svc # Reference to the internal load balancer of the application.
          port: 8080
```

### Publishing the app via a different gateway

If an application needs to move to another managed Gateway object, change the route attachment in stages:

1. Create a new ClusterALBInstance object or ALBInstance object so that the controller creates a new Gateway object.
1. Create a ListenerSet object with the same hostnames, ports, and TLS settings. The new Gateway object must be specified in `spec.parentRef`.
1. Add one more `parentRefs` entry to the existing HTTPRoute object, pointing to the new ListenerSet object.
1. Verify traffic through the new gateway path.
1. After verification, remove the reference to the obsolete ListenerSet object from `parentRefs` of the HTTPRoute object.

### Linking routes in one namespace to ListenerSet object in another

If an HTTPRoute object is created in one namespace and must be attached to a ListenerSet object in another namespace, add a ReferenceGrant object in the namespace of the target ListenerSet object. The example below shows a shared ListenerSet object in namespace `shared-gw`, an application HTTPRoute object in namespace `prod`, and a ReferenceGrant object that allows this attachment:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: shared-listeners
  namespace: shared-gw
spec:
  parentRef:
    name: public-gw
    namespace: d8-alb
  listeners:
    - name: app-https
      port: 443
      protocol: HTTPS
      hostname: app.example.com
      tls:
        mode: Terminate
        certificateRefs:
          - name: app-tls
            namespace: shared-gw
---
apiVersion: gateway.networking.k8s.io/v1
kind: ReferenceGrant
metadata:
  name: allow-prod-httproute-to-shared-listeners
  namespace: shared-gw
spec:
  from:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      namespace: prod
  to:
    - group: gateway.networking.k8s.io
      kind: ListenerSet
      name: shared-listeners
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app-route
  namespace: prod
spec:
  parentRefs:
    - name: shared-listeners
      namespace: shared-gw
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: app-https
      port: 443
  hostnames:
    - app.example.com
  rules:
    - backendRefs:
        - name: app-svc
          port: 8080
```

### Configuring TLS parameters with BackendTLSPolicy

If traffic from the gateway to the backend must use TLS, create a **BackendTLSPolicy** object in the namespace of the backend **Service** object. The example below shows an HTTPRoute object, a backend **Service** object with a named port, a ConfigMap with a CA bundle, and a **BackendTLSPolicy** object that configures TLS validation for that backend:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app-route
  namespace: prod
spec:
  parentRefs:
    - name: app-listeners
      namespace: prod
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: app-https
      port: 443
  hostnames:
    - app.example.com
  rules:
    - backendRefs:
        - name: app-svc
          port: 8443
---
apiVersion: v1
kind: Service
metadata:
  name: app-svc
  namespace: prod
spec:
  selector:
    app: app
  ports:
    - name: https
      port: 8443
      targetPort: 8443
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-backend-ca
  namespace: prod
data:
  ca.crt: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
---
apiVersion: gateway.networking.k8s.io/v1
kind: BackendTLSPolicy
metadata:
  name: app-svc-tls
  namespace: prod
spec:
  targetRefs:
    - group: ""
      kind: Service
      name: app-svc
      sectionName: https
  validation:
    hostname: app.internal.example.com
    caCertificateRefs:
      - group: ""
        kind: ConfigMap
        name: app-backend-ca
```

### Supported HTTPRoute annotations {#supported-httproute-annotations}

Because the current Gateway API specification does not yet cover all features required for a Deckhouse cluster to operate properly, the module provides a gradually growing set of HTTPRoute object annotations that adds the missing configuration options. The controller reads these keys from `HTTPRoute.metadata.annotations`.

| Annotation | Description |
| :--- | :--- |
| `alb.network.deckhouse.io/tls-disable-protocol` | Disables a TLS protocol version for the handler with the hostname of this route (for example value `http2`). This may be required in rare cases when a shared certificate with several DNS names is used together with request redirection. |
| `alb.network.deckhouse.io/whitelist-source-range` | Expects a comma-separated list of subnets in CIDR format: an IP filter at route level; overrides the global whitelist (for example `10.1.1.10/32, 10.2.2.2/32`). |
| `alb.network.deckhouse.io/response-headers-to-add` | JSON object with additional response headers (for example `{"Strict-Transport-Security": "max-age=31536000; includeSubDomains"}`). |
| `alb.network.deckhouse.io/session-affinity` | JSON for cookie session affinity (`mode`, `path`, `cookieName`, `ttl`, etc.); not every field is required (for example `{"mode": "cookie", "path": "/path", "cookieName": "mycookie", "ttl": 0}`). |
| `alb.network.deckhouse.io/hash-key` | For example `source-ip`: consistent hashing for Service backends of the HTTPRoute object. |
| `alb.network.deckhouse.io/service-upstream` | `"true"`: traffic to the upstream goes through the corresponding **Service** object instead of directly to pods. |
| `alb.network.deckhouse.io/basic-auth-secret` | `namespace/secret` with htpasswd data for HTTP basic auth on this route. |
| `alb.network.deckhouse.io/satisfy` | `all` or `any`: defines whether both checks must be satisfied (whitelist and basic-auth) or only one of them (default `all`). |
| `alb.network.deckhouse.io/auth-url` | Defines the URL of the external authentication service. |
| `alb.network.deckhouse.io/auth-signin` | Defines the redirect URL for authentication when `401` is returned by external authentication. |
| `alb.network.deckhouse.io/auth-response-headers` | Comma-separated list: additional headers from the auth response to pass upstream (on top of the standard allowlist). |
| `alb.network.deckhouse.io/rewrite-target` | Allows rewriting paths for rules with `RegularExpression` type by using regex capture groups (for example `/my-path/\1`). |
| `alb.network.deckhouse.io/buffer-max-request-bytes` | Defines the buffer size that may be used when requests are buffered (by default Envoy Proxy does not buffer requests). |
| `alb.network.deckhouse.io/limit-rps` | RPS limit for a route. |
| `alb.network.deckhouse.io/backend-tls-settings` | For example `{"mode": "SIMPLE", "insecureSkipVerify": true, "clientCertificate": "", "privateKey": "", "caCertificates": ""}`; allows explicit configuration of TLS connection parameters to the upstream. |

## Publishing applications using Istio

When deploying an application using Istio, you can choose one of the following options:

- [Using NGINX Ingress](#publishing-applications-using-nginx-ingress).
- [Use Istio Ingress Gateway](#publishing-applications-using-istio-ingress-gateway-resource).

### Publishing applications using NGINX Ingress

To publish an application using NGINX Ingress, the Deckhouse Kubernetes Platform administrator must configure the Ingress controller by adding an Istio sidecar to it.

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

## Publishing applications using Istio Ingress Gateway resource

To publish an application using the Istio Ingress Gateway, the Deckhouse Kubernetes Platform administrator must create an IngressIstioController resource.

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
   apiVersion: networking.istio.io/v1alpha3
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

## gRPC load balancing

For automatic gRPC service load balancing to work,
assign a name with the prefix or value `grpc` to the port in the corresponding Service object.
