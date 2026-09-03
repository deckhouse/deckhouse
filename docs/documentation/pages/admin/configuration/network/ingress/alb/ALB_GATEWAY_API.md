---
title: "ALB with Kubernetes Gateway API"
permalink: en/admin/configuration/network/ingress/alb/alb-gateway-api.html
description: "Publishing applications using the Kubernetes Gateway API."
extractedLinksMax: 4
relatedLinks:
  - title: "Migrating from ingress-nginx to alb"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/migration.html
  - title: "Publishing applications with Kubernetes Gateway API"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html
  - title: "Incoming traffic balancing"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/
  - title: "alb module documentation"
    url: /modules/alb/
  - title: "alb module configuration"
    url: /modules/alb/configuration.html
  - title: "alb module Custom Resources"
    url: /modules/alb/cr.html
  - title: "alb module FAQ"
    url: /modules/alb/faq.html
  - title: "alb module examples"
    url: /modules/alb/examples.html
  - title: "cert-manager module documentation"
    url: /modules/cert-manager/
---

To implement ALB using the [Kubernetes Gateway API](https://kubernetes.io/docs/concepts/services-networking/gateway/), the [`alb`](/modules/alb/) module is used.

The `alb` module implements an Application Load Balancer (ALB) and allows you to publish applications through Kubernetes Gateway API. It deploys and configures the infrastructure for receiving and routing external requests, and also verifies the user configuration of the Gateway API.

{% alert level="info" %}
ALBs created using the Kubernetes Gateway API can be used in a cluster alongside ALBs created using the Ingress NGINX Controller.
Details are in ["Using with other modules and third-party solutions"](#using-with-other-modules-and-third-party-solutions).
{% endalert %}

## Overview and scheme

The module is built on the Kubernetes Gateway API — an API for inbound traffic routing that extends the Ingress API model. A ClusterALBInstance or ALBInstance creates a managed Gateway. A ListenerSet bound to that Gateway describes handlers for incoming requests. Routes direct traffic to application services.

![Gateway API resource and traffic flow scheme](../../../../../images/network/ingress/alb/gateway-api-scheme.svg)

The module supports:

- A single declarative API for HTTP/HTTPS, gRPC, TCP, UDP, and TLS passthrough.
- A separation of responsibilities between the cluster administrator (ClusterALBInstance), the namespace administrator (ALBInstance and ListenerSet — hostname, TLS, ports), and application developers (routes).
- Request-handling features: per-route WAF, external authentication, IP allowlists, rate limiting, session affinity, GeoIP, BackendTLSPolicy, Proxy Protocol, and HTTP/3.

Kubernetes Gateway API and an API gateway serve different purposes. The Kubernetes Gateway API is a set of Kubernetes resources that describe how inbound traffic is routed to services. An API gateway is an architectural component that aggregates application APIs behind a single entry point. The `alb` module is an implementation of the Kubernetes Gateway API.

For a capability comparison with `ingress-nginx`, open the ["Comparison of the ingress-nginx and alb modules"](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/#comparison-of-the-ingress-nginx-and-alb-modules) section.

### Object roles

Gateway API separates responsibilities between cluster and namespace administrators and application developers:

- Cluster administrator — manages traffic infrastructure through ClusterALBInstance (cluster-wide Gateway).
- Namespace administrator — manages ALBInstance and ListenerSet (hostname, TLS, ports) within a namespace.
- Application developers — define routes (HTTPRoute, GRPCRoute, TLSRoute, TCPRoute, UDPRoute).

### Why ListenerSet

ListenerSet is a Gateway API extension. The ListenerSet object describes system and user traffic handlers: hostname, TLS mode, port, and protocol. Each ListenerSet is linked to a parent Gateway through `spec.parentRef`. Routes are then attached to it.

Each Gateway object creates two default listeners: `d8-http` (port `80`) and `d8-https` (port `443`). They are intended for service tasks such as gateway availability checks or cert-manager HTTP-01 challenges. They are not recommended for publishing applications. Use ListenerSet for that purpose instead.

### ClusterALBInstance and ALBInstance {#clusteralbinstance-and-albinstance}

When creating a Gateway managed object for publishing user applications, the custom resources [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) (a cluster-scoped object) and [ALBInstance](/modules/alb/cr.html#albinstance) (a namespaced resource) are used.

The characteristics of these resources and the differences between them are described in the table:

| | ClusterALBInstance | ALBInstance |
| :--- | :--- | :--- |
| Purpose | Deploy a cluster-wide Gateway object | Deploy a local Gateway object |
| Typical use case | - Common entry point (cluster-wide gateway).<br> - System gateway for publishing web interfaces of DKP service components and other modules (may require ["Steps before enabling"](#steps-to-take-before-enabling-and-configuring-alb-in-a-cluster)).<br> - Platform gateway | Dedicated gateway for an application or team in a dedicated namespace |
| Supported inlet types | [`LoadBalancer`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer), [`HostPort`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-hostport) | [`LoadBalancer`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer) |
| Proxy implementation | Envoy Proxy | Envoy Proxy |
| Deployment type | DaemonSet | Deployment |
| Placement of ListenerSet objects and routes | In any user namespace | In the same namespace as the ALBInstance object (required) |
| Access level | Cluster administrator | Namespace administrator |

Creating a ClusterALBInstance object or an ALBInstance object results in creation of a managed Gateway object in the cluster. At the same time:

- Each Gateway object is served by at least one Envoy Proxy instance.
- Traffic reaches it through a Service of type `LoadBalancer` or directly by using [`HostPort`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-hostport) parameters.
- Several ClusterALBInstance or ALBInstance objects may point to the same Gateway object through the `gatewayName` field. In that case, they describe one shared gateway. The request handling infrastructure may still differ depending on settings. You can think of `gatewayName` as an analog of `ingressClass` for [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) objects. The resulting configuration comes from the instance with the earliest `creationTimestamp` (that is, the one created first); the others report a port conflict in status, specifying the name of the controlling instance.

### Validating configuration

In addition to Gateway API infrastructure configuration, user settings are validated before being applied to prevent conflicts. This allows detecting conflicts between identical traffic handlers in different ListenerSet objects when they point to the same Gateway object.

## Inlets

An inlet defines how external traffic reaches the Envoy Proxy that serves the Gateway object:

- [`LoadBalancer`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer) — traffic is accepted through a Service of type `LoadBalancer` (cloud providers or bare metal with MetalLB). Available for both ClusterALBInstance and ALBInstance.
- [`HostPort`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-hostport) — traffic is accepted on node ports without an external load balancer. Available for ClusterALBInstance only.

The inlet mapping table for migration from `ingress-nginx` is in ["Inlet configuration"](migration.html#inlet-configuration). More examples are in ["Examples for different environments"](#infrastructure-examples).

## How to configure ALB

Publishing an application includes enabling the module, creating a managed Gateway, a ListenerSet, and routes.

### Steps before enabling {#steps-to-take-before-enabling-and-configuring-alb-in-a-cluster}

The `alb` module is in Preview. For the current list of supported DKP versions and other parameters, see the [`alb` module configuration](/modules/alb/configuration.html).

Before enabling and configuring ALB in a DKP cluster, do the following:

- If you need to publish service domains — web interfaces of [DKP service components](/products/kubernetes-platform/documentation/v1/user/web/ui.html) and other modules — set the global parameter [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate). Without this parameter, system HTTPRoute, Gateway, and ListenerSet objects for service domains will not work correctly, and the web interfaces will not be published. If you do not need to publish service domains, you can leave this parameter unset. Details are in ["Publishing service domains"](#publishing-service-domains).
- Check API version compatibility in ["Alongside third-party Gateway API implementations"](#alongside-third-party-gateway-api) if such solutions are already used in the cluster.
- On bare metal, for the [`LoadBalancer`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer) inlet prepare an external load balancer or the [`metallb`](/modules/metallb/) module. The [`HostPort`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-hostport) inlet is available for ClusterALBInstance only and does not require MetalLB.

### Enabling the module and creating a Gateway {#creating-a-gateway-object}

Enable the `alb` module as described in [Enabling the module](/modules/alb/configuration.html#enable).

Create a ClusterALBInstance or ALBInstance resource. This creates and configures the managed Gateway object.

{% alert level="warning" %}
Manual modification of Gateway objects managed by the module is not allowed.
{% endalert %}

{% tabs Gateway resource examples %}
{% tab "ClusterALBInstance" %}

Example of a ClusterALBInstance manifest for creating a cluster-wide gateway:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: public-gw
spec:
  gatewayName: public-gw
  inlet:
    type: LoadBalancer
```

{% endtab %}
{% tab "ALBInstance" %}

Example of a minimal working ALBInstance configuration for publishing a gateway in a namespace:

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
    loadBalancer: {}
```

After the ALBInstance reaches the `Ready` state, create ListenerSet and HTTPRoute objects in the same namespace. Next steps are in ["Publishing an application with ListenerSet and HTTPRoute"](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html#publishing-with-listenerset-and-httproute).

{% endtab %}
{% endtabs %}

### Creating a ListenerSet

The ListenerSet object describes system and user traffic handlers: hostname, TLS mode, port, and protocol. Each ListenerSet is linked to a parent Gateway through [`spec.parentRef`](https://gateway-api.sigs.k8s.io/guides/user-guides/listener-set/). Routes are then attached to it.

Placement of ListenerSet objects depends on the type of Gateway object in use:

- For ClusterALBInstance, ListenerSet objects may be placed in any namespace.
- For ALBInstance, ListenerSet objects must be placed in the same namespace as the parent ALBInstance.

In both cases, place the ListenerSet in the same namespace as the associated HTTPRoute, GRPCRoute, and TLSRoute objects. In that case, you do not need additional setup such as creating a ReferenceGrant.

In ListenerSet, use ports `80` and `443` for HTTP/HTTPS. These are Gateway API listener ports. They are not the same as the [`hostPort.httpPort`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-hostport-httpport) and [`hostPort.httpsPort`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-hostport-httpsport) parameters of the HostPort inlet, which set ports on the node.

TCPRoute and UDPRoute objects that use TCP and UDP ports from [`additionalPorts`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-additionalports) attach directly to the corresponding Gateway listener.

An example of a ListenerSet manifest for managing the reception of incoming HTTP and HTTPS requests through a cluster-wide gateway:

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
      port: 80 # In ListenerSet, use port 80 for HTTP. This is not the HostPort inlet httpPort on the node.
      protocol: HTTP
      hostname: app.example.com
    - name: app-https
      port: 443 # In ListenerSet, use port 443 for HTTPS. This is not the HostPort inlet httpsPort on the node.
      protocol: HTTPS
      hostname: app.example.com
      tls:
        mode: Terminate
        certificateRefs:
          - name: app-tls   # Reference to the secret with the TLS certificate.
            namespace: prod
```

### Creating routes

The following route types are used to route incoming requests:

- HTTPRoute: For routing HTTP/HTTPS/TLS requests. HTTPRoute objects support extended settings through annotations that complement the current Gateway API specification.
- GRPCRoute: For routing gRPC traffic.
- TLSRoute: For TLS passthrough routing.
- TCPRoute: For routing TCP traffic. For TCP ports from [`additionalPorts`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-additionalports), attach TCPRoute directly to the Gateway listener, not to a ListenerSet.
- UDPRoute: For routing UDP traffic. For UDP ports from [`additionalPorts`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-additionalports), attach UDPRoute directly to the Gateway listener, not to a ListenerSet.

{% tabs HTTPRoute examples %}
{% tab "HTTP" %}

Example of creating a route for HTTP traffic:

```yaml
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
      port: 80
  hostnames:
    - app.example.com
  rules:
    - backendRefs:
        - name: app-svc # Reference to the internal load balancer of the application.
          port: 8080
```

{% endtab %}
{% tab "HTTPS" %}

Example of creating a route for HTTPS traffic:

```yaml
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

{% endtab %}
{% endtabs %}

More publishing scenarios are in ["Publishing an application with ListenerSet and HTTPRoute"](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html#publishing-with-listenerset-and-httproute).

## Publishing service domains {#publishing-service-domains}

{% alert level="warning" %}
If you need to publish service domains, set the global parameter [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) — without it, system HTTPRoute/Gateway/ListenerSet objects for service domains will not work correctly, and the web interfaces of DKP service components and other modules will not be published. If you do not need to publish service domains, you can leave this parameter unset.
{% endalert %}

To provide access to the DKP cluster’s service domains, specify a default gateway. Create a ClusterALBInstance with the desired inlet type and [configuration](/modules/alb/cr.html#clusteralbinstance), and set [`spec.defaultDeckhouseGateway: true`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-defaultdeckhousegateway) on it.

Example of a ClusterALBInstance manifest with `spec.defaultDeckhouseGateway: true`:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: public-gw
spec:
  gatewayName: public-gw
  defaultDeckhouseGateway: true
  inlet:
    type: LoadBalancer
```

After applying the changes, check the ClusterALBInstance status:

```bash
d8 k get clusteralbinstances
```

The ClusterALBInstance must reach the `Ready` state and create the managed Gateway. After that, ListenerSet and HTTPRoute objects appear in the corresponding system namespaces.

{% alert level="info" %}
Currently, not all DKP modules are available through the Gateway API. Do not disable the `ingress-nginx` module or delete related objects until the required web interfaces are published through the Gateway API and validated.
{% endalert %}

After you configure the default gateway, run the following command to see which modules have **already** published service HTTPRoute objects through the Gateway API in this cluster. The command shows the actual route inventory, not a full platform capability matrix:

```shell
d8 k get httproutes -A -l heritage=deckhouse -o json \
  | jq -r '
["MODULE","GATEWAY API SUPPORT"],
(.items
  | map(.metadata.labels.module // "UNKNOWN")
  | unique[]
  | ["d8-" + ., "yes"])
| @tsv
' \
  | column -t -s $'\t'
```

### Selecting the default DKP gateway when using multiple ClusterALBInstances

A cluster can have multiple cluster-scoped gateways simultaneously, each marked as the default gateway (with the [`spec.defaultDeckhouseGateway: true`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-defaultdeckhousegateway) flag set for the corresponding ClusterALBInstance). In that case, the default gateway is the Gateway object created by the ClusterALBInstance object with the earliest `creationTimestamp` (that is, the one created first).

If no ClusterALBInstance object is marked as the default gateway, DKP allows the Gateway object created by the `alb` module for the instance named `main` to be used as the default gateway.

### Changing the default DKP gateway

If DKP system domains need to move to another Gateway object, complete these steps:

1. Create a new ClusterALBInstance object that describes the required settings and set [`spec.defaultDeckhouseGateway: true`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-defaultdeckhousegateway) on it.
1. In the current ClusterALBInstance object that provides the default DKP gateway, set `spec.defaultDeckhouseGateway: false`.
1. Check that all system ListenerSet objects now point to the new Gateway object in `spec.parentRef`.

## Examples for different environments {#infrastructure-examples}

This section describes traffic reception in different environments and operations on an already deployed ALB. Full CR field reference: [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) and [ALBInstance](/modules/alb/cr.html#albinstance). Additional examples are also available in the [`alb` module examples](/modules/alb/examples.html).

{% tabs Traffic reception %}
{% tab "Cloud provider" %}

### Cloud provider (LoadBalancer inlet) {#cloud-load-balancer}

Example ClusterALBInstance with the [`LoadBalancer`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer) inlet:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: main
spec:
  gatewayName: public-gw
  inlet:
    type: LoadBalancer
    loadBalancer: {}
```

To configure the cloud load balancer, specify the required Service annotations in [`spec.inlet.loadBalancer.serviceAnnotations`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer-serviceannotations). For example, for a Network Load Balancer in AWS:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: main
spec:
  gatewayName: public-gw
  inlet:
    type: LoadBalancer
    loadBalancer:
      serviceAnnotations:
        service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
```

{% endtab %}
{% tab "Bare metal with MetalLB" %}

### Bare metal with MetalLB {#bare-metal-metallb}

To accept traffic on bare metal using MetalLB, enable the [`metallb`](/modules/metallb/) module and create a MetalLoadBalancerClass object with an address pool. Place MetalLB balancers on the same nodes as the Envoy Proxy pods of the `alb` module (typically frontend nodes labeled `node-role.deckhouse.io/frontend`):

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: MetalLoadBalancerClass
metadata:
  name: alb
spec:
  addressPool:
    - 192.168.2.100-192.168.2.150
  isDefault: false
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  type: L2
```

Then create a ClusterALBInstance object and set [`spec.inlet.loadBalancer.loadBalancerClass`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer-loadbalancerclass):

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: main
spec:
  gatewayName: public-gw
  inlet:
    type: LoadBalancer
    loadBalancer:
      loadBalancerClass: alb
      serviceAnnotations:
        # Number of addresses allocated from the pool declared in MetalLoadBalancerClass.
        network.deckhouse.io/l2-load-balancer-external-ips-count: "1"
```

{% endtab %}
{% tab "Bare metal with HostPort" %}

### Bare metal without an external load balancer (HostPort inlet) {#bare-metal-hostport}

Example ClusterALBInstance with the [`HostPort`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-hostport) inlet:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: main
spec:
  gatewayName: public-gw
  inlet:
    type: HostPort
    hostPort:
      httpPort: 80
      httpsPort: 443
```

To place Envoy Proxy pods only on dedicated nodes, set [`nodeSelector`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-nodeselector) and [`tolerations`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-tolerations) on the ClusterALBInstance.

{% endtab %}
{% tab "External L7 balancer" %}

### Accepting traffic behind an external L7 balancer (Proxy Protocol) {#proxy-protocol}

If the `alb` module runs behind an external L7 balancer (for example, Cloudflare or Qrator), enable [`useProxyProtocol`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-useproxyprotocol) to receive real client addresses. Additionally, use [`spec.originalIPDetection`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-originalipdetection) to restrict the list of subnets allowed to provide headers with the client address.

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: main
spec:
  gatewayName: public-gw
  inlet:
    type: HostPort
    hostPort:
      httpPort: 80
      httpsPort: 443
  useProxyProtocol: true
  originalIPDetection:
    setRealIPFrom:
      - 10.0.0.0/16
```

{% alert level="warning" %}
Proxy Protocol and HTTP/3 cannot be enabled at the same time: if both parameters are set, the `alb` module controller rejects the configuration as conflicting.
{% endalert %}

{% endtab %}
{% endtabs %}

### Changing the inlet while keeping the current Gateway {#change-inlet}

To change the inlet used for an existing Gateway object, complete these steps:

1. Create a new ClusterALBInstance object or ALBInstance object with a different name but the same `spec.gatewayName`, using the required inlet type.
2. Check that the new traffic path works correctly.
3. Delete the obsolete ClusterALBInstance object or ALBInstance object.

Because `gatewayName` does not change, the Gateway object stays the same. In most cases, the ListenerSet object and routes do not need to be rewritten.

### Opening an additional TCP/UDP port on the cluster-wide gateway {#tcp-port}

If a dedicated TCP/UDP port is needed in addition to the default HTTP/HTTPS listeners, add the [`spec.inlet.additionalPorts`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-additionalports) field to the corresponding ClusterALBInstance object, for example:

```yaml
spec:
  gatewayName: public-gw
  inlet:
    type: LoadBalancer
    loadBalancer: {}
    additionalPorts:
      - port: 9000
        protocol: TCP
```

The controller adds a corresponding TCP/UDP traffic handler to the managed Gateway object with a section name ([`sectionName`](https://gateway-api.sigs.k8s.io/reference/spec/)) like `tcp-port-9000`. To attach a TCPRoute to it, specify the Gateway name and the matching `sectionName` in the route:

```yaml
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: TCPRoute
metadata:
  name: app-tcp
  namespace: prod
spec:
  parentRefs:
    - name: public-gw
      namespace: d8-alb
      sectionName: tcp-port-9000
      port: 9000
  rules:
    - backendRefs:
        - name: tcp-svc
          port: 9000
```

{% alert level="info" %}
If a TCPRoute or UDPRoute object is created in a namespace different from the Gateway object namespace, create a [ReferenceGrant](https://gateway-api.sigs.k8s.io/api-types/referencegrant/) object in the Gateway's namespace that allows references from the route's namespace.
{% endalert %}

UDPRoute examples and application publishing steps are in ["Working with GRPCRoute, TLSRoute, TCPRoute, and UDPRoute objects"](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html#grpcroute-tlsroute-tcproute-and-udproute-objects).

### Separating public and administrative zones {#public-and-admin-zones}

You can run separate Gateway objects for public and administrative traffic. This lets you assign each zone its own entry point and access policy. This approach is similar to using a separate Ingress NGINX Controller and IngressClass for the administrative zone.

Create a dedicated Gateway object for each zone and restrict administrative traffic with [`spec.acceptRequestsFrom`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-acceptrequestsfrom). The decision is based on the real connection address, not on request headers.

Example of creating two ClusterALBInstance objects — for the public and administrative zones:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: public
spec:
  gatewayName: public-gw
  inlet:
    type: LoadBalancer
    loadBalancer: {}
---
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: admin
spec:
  gatewayName: admin-gw
  inlet:
    type: LoadBalancer
    loadBalancer: {}
  acceptRequestsFrom:
    - 1.2.3.4/32
    - 10.0.0.0/16
```

Then create separate ListenerSet objects and routes for each gateway. Application publishing examples are in ["Publishing an application with ListenerSet and HTTPRoute"](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html#publishing-with-listenerset-and-httproute).

### Namespaced publishing (ALBInstance) {#namespaced-load-balancer}

Example of a minimal working ALBInstance configuration for publishing a gateway in a namespace:

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
    loadBalancer: {}
```

After the ALBInstance reaches the `Ready` state, create ListenerSet and HTTPRoute objects in the same namespace. Next steps are in ["Publishing an application with ListenerSet and HTTPRoute"](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html#publishing-with-listenerset-and-httproute).

### Issuing TLS certificates with cert-manager {#tls-cert-manager}

The `alb` module uses [`cert-manager`](/modules/cert-manager/) to automatically issue TLS certificates. The `d8-http` and `d8-https` listeners serve HTTP-01 challenges when issuing a certificate. To issue a certificate for an application, create a Certificate object that stores the certificate in a Secret, and reference that Secret from `certificateRefs` in the corresponding ListenerSet.

Minimal Certificate example that creates the `app-tls` Secret for a ListenerSet:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: app-tls
  namespace: prod
spec:
  secretName: app-tls
  issuerRef:
    name: letsencrypt
    kind: ClusterIssuer
  dnsNames:
    - app.example.com
```

Certificates are issued by the [`cert-manager`](/modules/cert-manager/) module. If the `alb` module is used alongside `ingress-nginx`, use a separate ClusterIssuer for each ALB type, as described in ["Alongside ingress-nginx"](#alongside-ingress-nginx).

## Using with other modules and third-party solutions {#using-with-other-modules-and-third-party-solutions}

ALBs implemented using the Kubernetes Gateway API in a DKP cluster can be used in conjunction with ALBs implemented using the Ingress NGINX Controller, as well as with ALBs based on third-party Gateway API solutions. For a step-by-step cutover, see [Migrating from ingress-nginx to alb](migration.html).

### Alongside ingress-nginx {#alongside-ingress-nginx}

ALBs implemented using the Kubernetes Gateway API can run in a cluster alongside ["ALB with Ingress NGINX Controller"](nginx.html). In that case, use a separate ClusterIssuer object for each ALB type so that certificate settings and lifecycles are managed independently. The same external hostname must not be served by both ALBs at once without an explicit split at the DNS or external load balancer layer.

{% alert level="info" %}
For the DKP gateway, a ClusterIssuer object is automatically created by default. This same ClusterIssuer object is used to issue certificates for system domains.
{% endalert %}

### Alongside third-party Gateway API implementations {#alongside-third-party-gateway-api}

Use of third-party Gateway API implementations is supported, provided that the cluster uses the following Gateway API object storage versions compatible with the `alb` module controller:

- BackendTLSPolicy: v1;
- GatewayClass: v1;
- Gateway: v1;
- ListenerSet: v1;
- GRPCRoute: v1;
- HTTPRoute: v1;
- ReferenceGrant: v1beta1;
- TCPRoute: v1alpha2/v1;
- UDPRoute: v1alpha2/v1;
- TLSRoute: v1.

During startup, the `alb` module controller checks the currently stored versions of Gateway API objects. If it detects a mismatch between the installed and required versions, it stops and does not proceed. If a given Gateway API object type is completely absent from the cluster, the controller automatically creates the required CRD version and then continues startup.

To manually verify version compatibility of the installed Gateway API objects in the cluster, use:

```bash
declare -A want=(
    [gatewayclasses.gateway.networking.k8s.io]=v1
    [gateways.gateway.networking.k8s.io]=v1
    [grpcroutes.gateway.networking.k8s.io]=v1
    [httproutes.gateway.networking.k8s.io]=v1
    [listenersets.gateway.networking.k8s.io]=v1
    [referencegrants.gateway.networking.k8s.io]=v1beta1
    [tcproutes.gateway.networking.k8s.io]="v1|v1alpha2"
    [udproutes.gateway.networking.k8s.io]="v1|v1alpha2"
    [tlsroutes.gateway.networking.k8s.io]=v1
    [backendtlspolicies.gateway.networking.k8s.io]=v1
)

for crd in "${!want[@]}"; do
    got="$(
        d8 k get crd "$crd" \
          -o jsonpath='{.spec.versions[?(@.storage==true)].name}' \
          2>/dev/null || true
    )"
    if [[ -n "$got" && "$got" =~ ^(${want[$crd]})$ ]]; then
        echo "$crd OK storage=$got"
    else
        echo "$crd FAILED cluster=${got:-MISSING} expected=${want[$crd]}"
    fi
done | sort
```

Otherwise, the module only configures and manages Gateway objects associated with its designated GatewayClass, which minimizes the risk of conflicts when third-party Gateway API implementations are present.

## Enabling HTTP/3 {#http3}

By default the gateway accepts HTTP/1.1 and HTTP/2. To enable HTTP/3, set [`spec.enableHTTP3: true`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-enablehttp3) on the ClusterALBInstance or ALBInstance:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: public-gw
spec:
  gatewayName: public-gw
  enableHTTP3: true
  inlet:
    type: LoadBalancer
```

{% alert level="warning" %}
You cannot enable `enableHTTP3` and `useProxyProtocol` at the same time.
{% endalert %}

Keep the following in mind:

- HTTP/3 uses QUIC over UDP. Make sure the load balancer and network rules allow UDP on the HTTPS port (usually `443`), not only TCP.
- For the `LoadBalancer` inlet, confirm that the cloud or MetalLB load balancer accepts UDP on the required port.
- Clients must support HTTP/3. From a workstation, verify with:

```shell
curl -vk --http3 https://app.example.com/
```

If `curl` was built without HTTP/3, use a QUIC-capable client or check UDP port reachability and Envoy Proxy logs.

## Using GeoIP and GeoLite2 {#geoip}

The `alb` module supports adding GeoIP fields to HTTP request headers based on [MaxMind GeoIP/GeoLite2](https://dev.maxmind.com/geoip/) databases.

Currently, the following database editions can be used:

- GeoIP2-Anonymous-IP;
- GeoIP2-City;
- GeoIP2-ISP;
- GeoIP2-ASN;
- GeoLite2-ASN;
- GeoLite2-City.

{% alert level="info" %}
The current GeoIP integration supports using up to 4 databases simultaneously.
{% endalert %}

Choose how GeoIP databases are obtained:

{% tabs GeoIP database source %}
{% tab "MaxMind" %}

### Downloading GeoIP Databases from MaxMind {#maxmind}

To use GeoIP and download databases directly from MaxMind servers, first create a secret containing the license key, for example:

```bash
d8 k -n prod create secret generic geoip-license \
  --from-literal=licenseKey='<MAXMIND_LICENSE_KEY>'
```

{% alert level="info" %}
When configuring GeoIP for ClusterALBInstance, the secret can be placed in any namespace, but it is recommended to use `d8-alb`.

For ALBInstance objects, the secret must reside in the same namespace as the ALBInstance object.
{% endalert %}

After creating the secret, reference it in a ClusterALBInstance or ALBInstance object, for example:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ALBInstance
metadata:
  name: main
  namespace: prod
spec:
  envoyLogLevel: Warning
  gatewayName: custom-gateway
  geoIP:
    licenseKeySecretRef:
      name: geoip-license
```

{% endtab %}
{% tab "Local mirror" %}

### Downloading GeoIP Databases from a Local Mirror {#local}

To use GeoIP and download databases from a local mirror, specify the mirror URL, for example:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ALBInstance
metadata:
  name: main
  namespace: prod
spec:
  envoyLogLevel: Warning
  gatewayName: custom-gateway
  geoIP:
    maxmindMirror:
      url: "https://local.geoip:8443"
```

You can also use a URL pointing to a local caching GeoIP server in another namespace, for example:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ALBInstance
metadata:
  name: main
  namespace: prod
spec:
  envoyLogLevel: Warning
  gatewayName: custom-gateway
  geoIP:
    maxmindMirror:
      url: "http://geoproxy-cluster.d8-alb.svc:8080/download"
```

{% endtab %}
{% endtabs %}

### Using GeoIP Headers {#headers}

Once GeoIP is configured in the namespace where ClusterALBInstance or ALBInstance proxies reside, a caching and update server for GeoIP databases is started. Envoy Proxy pods are then restarted sequentially so they can fetch GeoIP databases from the local GeoIP server.

To add GeoIP fields to HTTP request headers, specify the names of the HTTP headers that will contain the corresponding information, for example:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ALBInstance
metadata:
  name: main
  namespace: prod
spec:
  envoyLogLevel: Warning
  gatewayName: custom-gateway
  geoIP:
    headers:
      city: geoip_city
      country: geoip_country
    licenseKeySecretRef:
      name: geoip-license
    maxmindEditionIDs:
      - GeoLite2-City
```

GeoIP databases are updated once per day, both on the caching server and in each individual Envoy Proxy pod using the caching server.

PVC settings for GeoIP components are controlled by the [`storageClass`](/modules/alb/configuration.html#parameters-storageclass) module parameter.

## Configuring OpenTelemetry tracing {#tracing}

The `alb` module supports exporting OpenTelemetry traces from Envoy proxies.

To enable export, set the OpenTelemetry Collector endpoint in `spec.openTelemetry.tracing`:

- `service.name` and `service.namespace` — Name and namespace of the collector Service.
- `port` — Port.
- `protocol` — Protocol (`HTTP` or `gRPC`).
- `path` — Path for OTLP/HTTP.

Alternatively, you can specify a single [`url`](/modules/alb/cr.html#albinstance-v1alpha1-spec-opentelemetry-tracing-url).

When using TLS, explicitly set the [`sni`](/modules/alb/cr.html#albinstance-v1alpha1-spec-opentelemetry-tracing-tls-sni) parameter if the OpenTelemetry Collector is behind a proxy or load balancer that selects upstreams based on Server Name Indication.

Configure TLS in [`spec.openTelemetry.tracing.tls`](/modules/alb/cr.html#albinstance-v1alpha1-spec-opentelemetry-tracing-tls).

### Configuring OpenTelemetry tracing TLS

If OpenTelemetry tracing must send data over TLS, create a Kubernetes Secret with the CA certificate and reference it from [`spec.openTelemetry.tracing.tls.caSecretName`](/modules/alb/cr.html#albinstance-v1alpha1-spec-opentelemetry-tracing-tls-casecretname).

For ClusterALBInstance and the default DKP gateway, place the Secret in the `d8-alb` namespace.
For ALBInstance, place the Secret in the same namespace as the ALBInstance object.
The Secret must contain the `cacert` key.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: otel-tracing-ca
  namespace: d8-alb
type: Opaque
stringData:
  cacert: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
---
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterALBInstance
metadata:
  name: proxy-gw
spec:
  gatewayName: proxy-gw
  openTelemetry:
    tracing:
      service:
        name: otel-collector
        namespace: monitoring
      port: 4318
      protocol: HTTP
      path: /v1/traces
      tls:
        sni: otel-collector.monitoring.svc.cluster.local
        caSecretName: otel-tracing-ca
```

## Diagnostics and verification {#verification-and-common-questions}

Use this section to check that the gateway is ready and to answer common first-setup questions. More scenarios are in the ["alb module FAQ"](/modules/alb/faq.html).

- `Ready` and `status` — After creating a ClusterALBInstance or ALBInstance, wait for the `Ready` state and take the Gateway name and namespace from `status`. Field descriptions are in [ClusterALBInstance status](/modules/alb/cr.html#clusteralbinstance-v1alpha1-status) and [ALBInstance status](/modules/alb/cr.html#albinstance-v1alpha1-status):

  ```bash
  d8 k get clusteralbinstance
  d8 k -n <NAMESPACE> get albinstance
  d8 k -n d8-alb get gateway
  ```

- Load balancer address and DNS — With the LoadBalancer inlet, get the address from the load balancer Service (usually in the `d8-alb` namespace) and point DNS for the ListenerSet hostname to it:

  ```bash
  d8 k -n d8-alb get svc
  ```

- Same hostname for both ALBs — The `alb` and `ingress-nginx` modules can run in the same cluster, but the same external hostname must not be served by both ALBs at once without an explicit split at the DNS or external load balancer layer.

- ListenerSet conflict — If two ListenerSet objects with identical handlers point to the same Gateway, the controller rejects the conflicting configuration. Change the hostname, port, or protocol, or remove the duplicate ListenerSet.

### Viewing Envoy Proxy configuration {#envoy-config}

For troubleshooting, inspect the configuration that the controller and the proxy configurator pushed into the Envoy Proxy instance that serves the Gateway object.

1. Select an Envoy Proxy pod for the required Gateway object:

   ```bash
   d8 k -n d8-alb get pods -l alb.deckhouse.io/gateway=shared-gateway
   ```

1. Get the configuration through the following command (replace `<ENVOY_PROXY_POD_NAME>` with the Envoy Proxy pod name from the previous step):

   ```bash
   d8 k -n d8-alb exec -it <ENVOY_PROXY_POD_NAME> -- \
     pilot-agent request GET /config_dump
   ```

   If only one section of the configuration is needed, the required section may be requested explicitly:

   ```bash
   d8 k -n d8-alb exec -it <ENVOY_PROXY_POD_NAME> -- \
     pilot-agent request GET /config_dump?resource=dynamic_listeners
   d8 k -n d8-alb exec -it <ENVOY_PROXY_POD_NAME> -- \
     pilot-agent request GET /config_dump?resource=dynamic_route_configs
   d8 k -n d8-alb exec -it <ENVOY_PROXY_POD_NAME> -- \
     pilot-agent request GET /config_dump?resource=dynamic_active_clusters
   ```

This makes it easy to check whether the expected traffic handlers, virtual hosts, and upstream clusters appeared after changes to the ListenerSet object or Route object.
