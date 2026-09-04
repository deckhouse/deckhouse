---
title: "Publishing applications using the Kubernetes Gateway API"
description: "Publish applications with Kubernetes Gateway API in Deckhouse Kubernetes Platform. ListenerSet, HTTPRoute, GRPCRoute, TLSRoute, TCPRoute, BackendTLSPolicy, HTTPRoute annotations, and WAF."
permalink: en/user/network/ingress/alb/gateway-api.html
extractedLinksMax: 4
relatedLinks:
  - title: "Utilizing Application Load Balancer (ALB)"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html
  - title: "ALB with Kubernetes Gateway API"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html
  - title: "Migrating from ingress-nginx to alb"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/migration.html
  - title: "alb module documentation"
    url: /modules/alb/
  - title: "alb module Custom Resources"
    url: /modules/alb/cr.html
  - title: "alb module configuration"
    url: /modules/alb/configuration.html
  - title: "alb module FAQ"
    url: /modules/alb/faq.html
  - title: "alb module examples"
    url: /modules/alb/examples.html
---

## Publishing applications using the Kubernetes Gateway API

Applications can be published through a cluster-wide gateway (a ClusterALBInstance created by the cluster administrator) or through a dedicated gateway in the application namespace (an ALBInstance).

Creating the managed Gateway (ClusterALBInstance or ALBInstance, inlets, enabling the module) is an administrator task. Infrastructure setup is described in ["Enabling the module and creating a Gateway"](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#creating-a-gateway-object).

This scenario assumes that the ClusterALBInstance or ALBInstance object has already been created and has reached the `Ready` state. Ask the administrator for the name and namespace of the managed Gateway, or read the Gateway name from the instance's [`status.gateway`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-status) field:

```shell
d8 k get clusteralbinstance <CLUSTER_ALB_INSTANCE_NAME> \
  -o jsonpath='{.status.gateway}{"\n"}'
d8 k -n <NAMESPACE> get albinstance <ALB_INSTANCE_NAME> \
  -o jsonpath='{.status.gateway}{"\n"}'
```

For ClusterALBInstance, the managed Gateway usually lives in the `d8-alb` namespace. For ALBInstance, it lives in the same namespace as the ALBInstance object. Status field descriptions are in [`status`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-status).

The namespace administrator creates a ListenerSet attached to that Gateway ([`spec.parentRef`](https://gateway-api.sigs.k8s.io/guides/user-guides/listener-set/)). Application developers create HTTPRoute objects that attach to the ListenerSet.

The `d8-http` and `d8-https` listeners serve system tasks such as gateway availability checks or cert-manager HTTP-01 challenges. Do not attach application routes to them — use a ListenerSet instead.

### Publishing an application with ListenerSet and HTTPRoute {#publishing-with-listenerset-and-httproute}

Example of a ListenerSet and HTTPRoute for publishing an application through a managed Gateway:

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
      port: 80 # ListenerSet HTTP listeners use port 80. This is not the HostPort inlet httpPort on the node.
      protocol: HTTP
      hostname: app.example.com
    - name: app-https
      port: 443 # ListenerSet HTTPS listeners use port 443. This is not the HostPort inlet httpsPort on the node.
      protocol: HTTPS
      hostname: app.example.com
      tls:
        mode: Terminate
        certificateRefs:
          - name: app-tls   # Secret with the TLS certificate (issue with cert-manager or provide manually).
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
      port: 80 # ListenerSet HTTP listeners use port 80. This is not the HostPort inlet httpPort on the node.
  hostnames:
    - app.example.com
  rules:
    - backendRefs:
        - name: app-svc # Reference to the internal load balancer of the application.
          port: 8080 
---
# Route for HTTPS traffic
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
      port: 443 # ListenerSet HTTPS listeners use port 443. This is not the HostPort inlet httpsPort on the node.
  hostnames:
    - app.example.com
  rules:
    - backendRefs:
        - name: app-svc # Reference to the internal load balancer of the application.
          port: 8080
```

### Checking publication {#checking-publication}

After you apply the ListenerSet and HTTPRoute, check the object status:

```shell
d8 k -n <NAMESPACE> get listenerset
d8 k -n <NAMESPACE> describe listenerset <LISTENERSET_NAME>
d8 k -n <NAMESPACE> get httproute
d8 k -n <NAMESPACE> describe httproute <HTTPROUTE_NAME>
```

Confirm the following:

- ListenerSet listeners are `Programmed` or `Accepted`.
- The HTTPRoute `parentRefs` point to the intended ListenerSet and the `Accepted` conditions are true.
- The entry-point address and DNS for the hostname match what the administrator provided (for the `LoadBalancer` inlet, the address usually comes from a Service in the `d8-alb` namespace).

Check application reachability from a client. Substitute the entry-point address and hostname (a successful response is `200` or another status code expected from the application):

```shell
curl -vk \
  --resolve app.example.com:443:<ENTRY_POINT_ADDRESS> \
  https://app.example.com/
```

If the route is not accepted, check the Gateway name from the instance `status`, the ListenerSet namespace, and the port and `sectionName` in `parentRefs`. Hostname and port conflicts with other ListenerSet objects show up as a condition on the conflicting listener's status, visible in the `describe listenerset` output from the previous step.

### Working with GRPCRoute, TLSRoute, TCPRoute, and UDPRoute objects {#grpcroute-tlsroute-tcproute-and-udproute-objects}

Besides HTTPRoute, you can use GRPCRoute (gRPC traffic), TLSRoute (TLS passthrough), and TCPRoute/UDPRoute (arbitrary TCP and UDP traffic) to publish applications.

#### GRPCRoute for gRPC traffic

The GRPCRoute object is intended for gRPC traffic. For it, create the ListenerSet object with an HTTPS listener, then add the GRPCRoute object:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: grpc-listeners
  namespace: prod
spec:
  parentRef:
    name: app-gw # The name of the Gateway object from the ALBInstance status.
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
        - name: grpc-svc # Name of the gRPC backend Service.
          port: 9090
```

#### TLS passthrough with TLSRoute

For TLS passthrough, when traffic must be decrypted on the application side, either a TLS listener or an HTTPS listener can be used.

Additional ports are set in [`spec.inlet.additionalPorts`](/modules/alb/cr.html#albinstance-v1alpha1-spec-inlet-additionalports) on the object that owns the gateway:

- For a cluster-wide Gateway — on ClusterALBInstance (configured by the cluster administrator).
- For a namespaced gateway — on ALBInstance (configured by the namespace administrator, or by you if you can edit the object).

For a ClusterALBInstance example, see ["Opening an additional TCP/UDP port on the cluster-wide gateway"](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#tcp-port). The example below uses ALBInstance.

{% tabs TLS passthrough %}
{% tab "TLS listener" %}

Because the TLS listener in this example uses an additional port, ask an administrator to add `additionalPorts` to the object that owns the gateway (ClusterALBInstance or ALBInstance), or add it yourself if you have the required permissions:

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

Next, configure ListenerSet and TLSRoute objects:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: tls-pass-listeners
  namespace: prod
spec:
  parentRef:
    name: app-gw # The name of the Gateway object from the ALBInstance status.
    namespace: prod
  listeners:
    - name: tls-pass
      port: 8443           # In this case 8443 port is used for TLS.
      protocol: TLS
      hostname: pass.example.com
      tls:
        mode: Passthrough  # TLS passthrough mode is set explicitly.
---
apiVersion: gateway.networking.k8s.io/v1
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
    - backendRefs:
        - name: tls-pass-svc  # Reference to the internal load balancer of the application.
          port: 8443
```

{% endtab %}
{% tab "HTTPS listener" %}

The HTTPS listener variant is useful when the standard handler on port `443` should be used: no extra port needs to be opened for TLS passthrough.

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: https-pass-listeners
  namespace: prod
spec:
  parentRef:
    name: app-gw # The name of the Gateway object from the ALBInstance status.
    namespace: prod
  listeners:
    - name: https-pass
      port: 443 # In this case 443 (HTTPS) port is reused for TLS.
      protocol: HTTPS
      hostname: pass.example.com
      tls:
        mode: Passthrough  # TLS passthrough mode is set explicitly.
---
apiVersion: gateway.networking.k8s.io/v1
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

{% endtab %}
{% endtabs %}

#### TLS termination at the gateway with TCPRoute

If TLS must be terminated on the gateway and then forwarded to the backend as a TCP stream (for example, when the application accepts decrypted TCP rather than HTTPS), create a ListenerSet object with a TLS listener in `Terminate` mode, then attach a TCPRoute object:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: tls-term-listeners
  namespace: prod
spec:
  parentRef:
    name: app-gw # The name of the Gateway object from the ALBInstance status.
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

#### Additional TCP and UDP ports

For TCP and UDP ports from [`additionalPorts`](/modules/alb/cr.html#albinstance-v1alpha1-spec-inlet-additionalports), attach the route directly to the managed Gateway listener instead of creating a separate ListenerSet. Otherwise the controller rejects the configuration because of overlapping handlers.

Set the additional ports on the object that owns the gateway: ClusterALBInstance for a cluster-wide Gateway, or ALBInstance for a namespaced gateway. For ClusterALBInstance, use the example in ["Opening an additional TCP/UDP port on the cluster-wide gateway"](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#tcp-port).

{% tabs TCP and UDP %}
{% tab "TCP" %}

To publish a TCP service, ask an administrator to expose an additional TCP port on the object that owns the gateway (ClusterALBInstance or ALBInstance), or expose it yourself if you have the required permissions:

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
      - port: 9000
        protocol: TCP
```

The `alb` module controller creates the TCP listener on the managed Gateway automatically from `spec.inlet.additionalPorts`. Attach the TCPRoute to that listener:

```yaml
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: TCPRoute
metadata:
  name: tcp-route
  namespace: prod
spec:
  parentRefs:
    - name: app-gw # The name of the Gateway object from the ALBInstance status.
      namespace: prod
      kind: Gateway
      group: gateway.networking.k8s.io
      sectionName: tcp-port-9000
      port: 9000
  rules:
    - backendRefs:
        - name: tcp-svc # Reference to the internal load balancer of the application.
          port: 9000
```

{% endtab %}
{% tab "UDP" %}

To publish a UDP service, ask an administrator to expose an additional UDP port on the object that owns the gateway (ClusterALBInstance or ALBInstance), or expose it yourself if you have the required permissions:

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
      - port: 5353
        protocol: UDP
```

The `alb` module controller creates the UDP listener on the managed Gateway automatically from `spec.inlet.additionalPorts`. Attach the UDPRoute to that listener:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: UDPRoute
metadata:
  name: udp-route
  namespace: prod
spec:
  parentRefs:
    - name: app-gw # The name of the Gateway object from the ALBInstance status.
      namespace: prod
      kind: Gateway
      group: gateway.networking.k8s.io
      sectionName: udp-port-5353
      port: 5353
  rules:
    - backendRefs:
        - name: udp-svc # Reference to the internal load balancer of the application.
          port: 5353
```

{% endtab %}
{% endtabs %}

### Publishing the app via a different gateway

If an application needs to move to another managed Gateway object, change the route attachment in stages:

1. Obtain a new managed Gateway from the cluster administrator (a new ClusterALBInstance or ALBInstance), so that the controller creates a new Gateway object. Creating managed Gateways is described in ["Enabling the module and creating a Gateway"](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#creating-a-gateway-object).
1. Create a ListenerSet object with the same hostnames, ports, and TLS settings. The new Gateway object must be specified in [`spec.parentRef`](https://gateway-api.sigs.k8s.io/guides/user-guides/listener-set/).
1. Add one more `parentRefs` entry to the existing HTTPRoute object, pointing to the new ListenerSet object.
1. Verify traffic through the new gateway path.
1. After verification, remove the reference to the obsolete ListenerSet object from `parentRefs` of the HTTPRoute object.

### Linking routes in one namespace to a ListenerSet in another

By default, the Gateway API does not allow routes to reference objects in other namespaces — this must be allowed explicitly. If an HTTPRoute object is created in one namespace and must be attached to a ListenerSet in another namespace, add a ReferenceGrant in the namespace of the target ListenerSet.

The example below shows a shared ListenerSet in namespace `shared-gw`, an application HTTPRoute in namespace `prod`, and a ReferenceGrant that allows this attachment:

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

If traffic from the gateway to the backend must use TLS, create a BackendTLSPolicy in the namespace of the backend Service. The example below shows an HTTPRoute, a backend Service with a named port, a ConfigMap with a CA bundle, and a BackendTLSPolicy that configures TLS validation for that backend:

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

GeoIP and OpenTelemetry tracing are configured by the administrator on ClusterALBInstance or ALBInstance, following ["Using GeoIP and GeoLite2"](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#geoip) and ["Configuring OpenTelemetry tracing"](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#tracing).

### Supported HTTPRoute annotations {#supported-httproute-annotations}

Because the current Gateway API specification does not yet cover all features required for a DKP cluster, the module provides HTTPRoute annotations for the missing options. The controller reads these keys from `HTTPRoute.metadata.annotations`.

| Annotation | Description |
| :--- | :--- |
| `alb.network.deckhouse.io/tls-disable-protocol` | Disables the listener protocol for the route with the specified hostname (for example, value `http2`). This may be required in rare cases when a shared certificate with several DNS names is used together with request redirection |
| `alb.network.deckhouse.io/whitelist-source-range` | Expects a comma-separated list of subnets in CIDR format: an IP filter at route level; overrides the global whitelist (for example, `10.1.1.10/32, 10.2.2.2/32`) |
| `alb.network.deckhouse.io/response-headers-to-add` | JSON object with additional response headers (for example, `{"Strict-Transport-Security": "max-age=31536000; includeSubDomains"}`) |
| `alb.network.deckhouse.io/session-affinity` | JSON for session affinity with cookie mode (`mode`, `path`, `cookieName`, `ttl`, etc.); not every field is required (for example, `{"mode": "cookie", "path": "/path", "cookieName": "mycookie", "ttl": 0}`) |
| `alb.network.deckhouse.io/hash-key` | Consistent hashing for Service backends of the HTTPRoute object (for example, `source-ip`) |
| `alb.network.deckhouse.io/service-upstream` | `"true"`: traffic to the upstream goes through the corresponding Service object instead of directly to pods |
| `alb.network.deckhouse.io/basic-auth-secret` | `namespace/secret` with htpasswd data for HTTP basic auth on this route |
| `alb.network.deckhouse.io/satisfy` | `all` or `any`: defines whether both checks must be satisfied (whitelist and basic-auth) or only one of them (default `all`) |
| `alb.network.deckhouse.io/auth-url` | Defines the URL of the external authentication service |
| `alb.network.deckhouse.io/auth-signin` | Defines the redirect URL for authentication when `401` is returned by external authentication |
| `alb.network.deckhouse.io/auth-response-headers` | Comma-separated list: additional headers from the auth response to pass upstream (on top of the standard allowlist) |
| `alb.network.deckhouse.io/mod-security` | JSON configuration for the per-route ModSecurity/Coraza WAF |
| `alb.network.deckhouse.io/rewrite-target` | Allows rewriting paths for rules with `RegularExpression` type by using regex capture groups (for example, `/my-path/\1`) |
| `alb.network.deckhouse.io/buffer-max-request-bytes` | Defines the buffer size that may be used when requests are buffered; the value is in bytes (integer). By default Envoy Proxy does not buffer requests |
| `alb.network.deckhouse.io/limit-rps` | RPS limit for a route |
| `alb.network.deckhouse.io/backend-tls-settings` | For example, `{"mode": "SIMPLE", "insecureSkipVerify": true, "clientCertificate": "", "privateKey": "", "caCertificates": "", "sni": "example.com", "secret": "<NAMESPACE>/<SECRET_NAME>"}`; allows explicit configuration of TLS connection parameters to the upstream. `<NAMESPACE>` — Secret namespace; `<SECRET_NAME>` — Secret name |
| `alb.network.deckhouse.io/idle-timeout` | Sets the per-route Envoy `idle_timeout`, in seconds. Similar to `ingress-nginx` `proxy-read-timeout`/`proxy-send-timeout`; this is an inactivity timeout, not a total request timeout |
| `alb.network.deckhouse.io/proxy-buffer-size` | Sets the maximum size of response headers when configured on an upstream cluster; if exceeded, Envoy returns `503`. Similar to `nginx.ingress.kubernetes.io/proxy-buffer-size` |

### Publishing an application when the Istio sidecar is enabled {#publishing-with-istio-sidecar}

When the Istio sidecar is enabled for the gateway proxy through the [`istioSidecar`](/modules/alb/cr.html#albinstance-v1alpha1-spec-istiosidecar) parameter of ALBInstance or [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-istiosidecar), traffic to the backend must reach the sidecar through the Service and carry the Service FQDN in the `Host` header.

Configure the HTTPRoute as follows:

- Add the `alb.network.deckhouse.io/service-upstream: "true"` annotation so that traffic goes through the Service object instead of directly to pods. This is the equivalent of the `ingress-nginx` `nginx.ingress.kubernetes.io/service-upstream: "true"` annotation.
- Add a `URLRewrite` filter that sets `hostname` to the FQDN of the backend Service object. This replaces the `ingress-nginx` `nginx.ingress.kubernetes.io/upstream-vhost` annotation.

Example of an HTTPRoute configured for a gateway with an Istio sidecar:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: myservice
  namespace: myns
  annotations:
    alb.network.deckhouse.io/service-upstream: "true" # Traffic goes through the Service object so the Istio sidecar can process it.
spec:
  parentRefs:
    - name: app-listeners # ListenerSet name.
      namespace: myns
      kind: ListenerSet
      group: gateway.networking.k8s.io
      sectionName: app-https
      port: 443
  hostnames:
    - myservice.example.com
  rules:
    - filters:
        - type: URLRewrite
          urlRewrite:
            hostname: myservice.myns.svc # FQDN of the backend Service object, so the sidecar identifies the destination.
      backendRefs:
        - name: myservice
          port: 80
```

### WAF on HTTPRoute {#waf-on-httproute}

The `alb.network.deckhouse.io/mod-security` annotation enables the ModSecurity/Coraza WAF for a specific HTTPRoute. The WAF is configured per route and does not affect other routes unless the same annotation is added there.

Supported annotation fields:

| Field | Description |
| :--- | :--- |
| `mode` | WAF engine mode: `on`, `off`, or any other value for `DetectionOnly` |
| `preset` | Optional ruleset. Currently only `owasp-crs` is supported. If the field is omitted, no ruleset is loaded |
| `paranoiaLevel` | Optional CRS paranoia level from `1` to `4`. Applied only when `preset` is `owasp-crs` |
| `configRef.namespace` | Optional namespace of the ConfigMap with custom rules. Defaults to the namespace of the HTTPRoute |
| `configRef.name` | Name of the ConfigMap with custom rules |
| `configRef.key` | Optional key in the ConfigMap. If omitted, all keys are read in sorted order |
| `directives` | Optional inline list of ModSecurity/Coraza directives appended after the ruleset and ConfigMap rules |

Directive order:

1. Base directives shipped with the module (`@coraza.conf`, `SecRuleEngine`, `SecResponseBodyAccess Off`).
1. Rules from the ruleset in `preset`.
1. Rules from `configRef`.
1. Directives from the annotation's `directives` field.

Directives from the annotation are applied last, so they can override the ruleset or ConfigMap rules.

The examples below show the `alb.network.deckhouse.io/mod-security` annotation on an HTTPRoute.

{% tabs WAF examples %}
{% tab "mode: on" %}

Minimal configuration: WAF enabled without a ruleset or custom directives.

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app
  namespace: prod
  annotations:
    alb.network.deckhouse.io/mod-security: |
      {
        "mode": "on"
      }
spec:
  hostnames:
    - app.example.com
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: ListenerSet
      name: app-listeners
      namespace: prod
      sectionName: app-https
      port: 443
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: app-svc
          port: 8080
```

{% endtab %}
{% tab "OWASP CRS" %}

WAF with the OWASP CRS ruleset and a `paranoiaLevel` value:

```yaml
metadata:
  annotations:
    alb.network.deckhouse.io/mod-security: |
      {
        "mode": "on",
        "preset": "owasp-crs",
        "paranoiaLevel": 1
      }
```

{% endtab %}
{% tab "ConfigMap" %}

WAF with OWASP CRS, custom rules from a ConfigMap, and extra directives in the annotation:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: waf-rules
  namespace: prod
data:
  rules.conf: |
    SecRule ARGS:test "@streq block" \
      "id:1000001,phase:2,deny,status:403,msg:'test waf block'"
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app
  namespace: prod
  annotations:
    alb.network.deckhouse.io/mod-security: |
      {
        "mode": "on",
        "preset": "owasp-crs",
        "paranoiaLevel": 1,
        "configRef": {
          "name": "waf-rules",
          "key": "rules.conf"
        },
        "directives": [
          "SecResponseBodyAccess Off"
        ]
      }
```

{% endtab %}
{% endtabs %}

Rule syntax reference:

- [Coraza syntax and `SecRule` format](https://www.coraza.io/docs/seclang/syntax/)
- [ModSecurity variables reference](https://github.com/SpiderLabs/ModSecurity/wiki/Reference-Manual-%28v2.x%29-Variables)
- [ModSecurity operators reference](https://github.com/SpiderLabs/ModSecurity/wiki/Reference-Manual-%28v2.x%29-Operators)
- [ModSecurity `SecRuleEngine` and related directives](https://github.com/owasp-modsecurity/ModSecurity/wiki/Reference-Manual-%28v2.x%29)

Current WAF notes and limitations:

- Only the `owasp-crs` ruleset is supported.
- `paranoiaLevel` is ignored when `preset` is omitted or differs from `owasp-crs`.
- Valid `paranoiaLevel` values are `1`–`4`. In practice it is recommended to start with `1`.
- The WAF currently inspects only incoming requests to the application and can block such requests when rules match. Responses sent back to the client are not inspected.
- Rules from ConfigMap values may be multiline: lines ending with `\` are joined automatically.

