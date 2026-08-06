---
title: "ALB with Kubernetes Gateway API"
permalink: en/admin/configuration/network/ingress/alb/alb-gateway-api.html
description: "Publishing applications using the Kubernetes Gateway API."
---

To implement ALB using the [Kubernetes Gateway API](https://kubernetes.io/docs/concepts/services-networking/gateway/), the [`alb`](/modules/alb/) module is used.

The `alb` module implements an Application Load Balancer (ALB) and allows you to publish applications through Kubernetes Gateway API. It deploys and configures the infrastructure for receiving and routing external requests, and also verifies the user configuration of the Gateway API.

The module is built on the Kubernetes Gateway API — the modern standard for managing inbound traffic that succeeds the Ingress API. It provides:

- a single declarative API for HTTP/HTTPS, gRPC, TCP, UDP, and TLS passthrough;
- a separation of responsibilities between the cluster administrator (ClusterALBInstance), the namespace administrator (ALBInstance/ListenerSet), and the application team (routes);
- extended request-handling capabilities: per-route WAF, external authentication, IP allowlists, rate limiting, session affinity, GeoIP, BackendTLSPolicy, Proxy Protocol, and HTTP/3.

Despite the similar names, the Kubernetes Gateway API and an API gateway are different concepts. The Kubernetes Gateway API is a set of Kubernetes resources that describe how inbound traffic is routed to services. An API gateway is an architectural component that aggregates application APIs behind a single entry point. The `alb` module is an implementation of the Kubernetes Gateway API.

For a capability comparison with `ingress-nginx`, see [Incoming traffic balancing](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/#comparison-of-the-ingress-nginx-and-alb-modules).

{% alert level="info" %}
ALBs created using the Kubernetes Gateway API can be used in a cluster alongside ALBs created using the Ingress NGINX Controller.
For more information, see the section [Using with other modules and third-party solutions](#using-with-other-modules-and-third-party-solutions).
{% endalert %}

## Validating Gateway API configuration

In addition to Gateway API infrastructure configuration, the `alb` module validates user settings to prevent conflicting configurations from being applied. For example, the module checks for conflicts between identical traffic handlers in different ListenerSet objects when they point to the same Gateway object.

## Steps to take before enabling and configuring ALB in a cluster

Before enabling and configuring ALB in a Deckhouse Kubernetes Platform (DKP) cluster:

- If you need to [publish service domains](#publishing-service-domains) ([web interfaces for DKP service components](/products/kubernetes-platform/documentation/v1/user/web/ui.html) and other modules), ensure that the global parameter [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) is specified. If the `publicDomainTemplate` parameter is not specified, HTTPRoute/Gateway/ListenerSet system objects will be created incorrectly, and the web interfaces of DKP service components and other modules will not be published.
- If third-party Gateway API solutions are used in the cluster, [check compatibility](#using-third-party-gateway-api-solutions) between the API versions used for third-party Gateway API objects and the versions required by the `alb` module controller.

## Using with other modules and third-party solutions

ALBs implemented using the Kubernetes Gateway API in a DKP cluster can be used in conjunction with ALBs implemented using the Ingress NGINX Controller, as well as with ALBs based on third-party Gateway API solutions.

### Using with ALB based on the Ingress NGINX Controller

ALBs implemented using the Kubernetes Gateway API can run in a cluster alongside [ALBs based on the Ingress NGINX Controller](nginx.html). In that case, use a separate ClusterIssuer object for each ALB type so that certificate settings and lifecycles are managed independently.

{% alert level="info" %}
For the DKP gateway, a ClusterIssuer object is automatically created by default. This same ClusterIssuer object is used to issue certificates for system domains.
{% endalert %}

### Using third-party Gateway API solutions

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
        d8 k get crd "$crd" -o jsonpath='{.spec.versions[?(@.storage==true)].name}' 2>/dev/null || true
    )"
    if [[ -n "$got" && "$got" =~ ^(${want[$crd]})$ ]]; then
        echo "$crd OK storage=$got"
    else
        echo "$crd FAILED cluster=${got:-MISSING} expected=${want[$crd]}"
    fi
done | sort
```

Otherwise, the module only configures and manages Gateway objects associated with its designated GatewayClass, which minimizes the risk of conflicts when third-party Gateway API implementations are present.

## Publishing an application

The process of publishing an application includes the following steps:

1. [Create a Gateway object](#creating-a-gateway-object) using the [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) resource (cluster-scoped) or the [ALBInstance](/modules/alb/cr.html#albinstance) resource (namespaced).
1. [Create a ListenerSet object](#creating-listenerset-objects-to-manage-the-handling-of-incoming-requests) (manages the reception of incoming requests), which is bound to the Gateway object created in the previous step.
1. [Create objects (routes)](#creating-routes-and-configuring-routing) to route incoming requests to the application. HTTPRoute, GRPCRoute, and TLSRoute objects are attached to the ListenerSet. TCPRoute and UDPRoute for plain TCP/UDP ports from [`additionalPorts`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-additionalports) are attached directly to the Gateway listener.

### Creating a Gateway object

When creating a Gateway managed object for publishing user applications, the custom resources [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) (a cluster-scoped object) and [ALBInstance](/modules/alb/cr.html#albinstance) (a namespaced resource) are used.

The characteristics of these resources and the differences between them are described in the table:

| | **ClusterALBInstance** | **ALBInstance** |
| :--- | :--- | :--- |
| Purpose | Deploy a cluster-wide Gateway object | Deploy a local Gateway object |
| Typical use case | - Common entry point (cluster-wide gateway).<br> - System gateway for publishing web interfaces of DKP service components and other modules (may require [cluster preparation](#steps-to-take-before-enabling-and-configuring-alb-in-a-cluster)).<br> - Platform gateway | Dedicated gateway for an application or team in a dedicated namespace |
| Supported inlet types | `LoadBalancer`, `HostPort` | `LoadBalancer` |
| Proxy implementation | Envoy Proxy | Envoy Proxy |
| Deployment type | DaemonSet | Deployment |
| Placement of ListenerSet objects and routes | In any user namespace | In the same namespace as the ALBInstance object |
| Access level | Cluster administrator | Namespace administrator |

Creating a ClusterALBInstance object or an ALBInstance object results in creation of a managed Gateway object in the cluster. At the same time:

- Each Gateway object is served by at least one Envoy Proxy instance.
- Traffic reaches it through a Service object of type `LoadBalancer` or directly by using `HostPort` parameters.
- Each Gateway object creates two default listeners: `d8-http` (port `80`) and `d8-https` (port `443`). They are intended for service tasks such as gateway availability checks or cert-manager HTTP-01 challenges. They are not recommended for publishing applications; use ListenerSet for that purpose instead.
- Several ClusterALBInstance or ALBInstance objects may point to the same Gateway object through the `gatewayName` field. In that case, they describe one shared gateway, but the request handling infrastructure may differ depending on settings. You can think of `gatewayName` as an analog of `ingressClass` for [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) objects: it determines which route information is included in the configuration of a particular ALB instance.

{% alert level="warning" %}
Manual modification of Gateway objects managed by the module is not allowed.
{% endalert %}

Example of a ClusterALBInstance resource manifest for creating a cluster-wide gateway:

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

An example of an ALBInstance resource manifest for creating a separate gateway for an application or a team in a dedicated namespace is provided in [Publishing an application through an ALBInstance object](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html#publishing-an-application-through-a-albinstance-object).

### Creating ListenerSet objects to manage the handling of incoming requests

The ListenerSet object describes system and user traffic handlers that define hostname, TLS mode, port, and protocol. Each ListenerSet object is linked to a specific parent Gateway object through the `spec.parentRef` field, and routes are then attached to it.

Placement of ListenerSet objects depends on the type of Gateway object in use:

- for ClusterALBInstance, ListenerSet objects may be placed in any namespace;
- for ALBInstance, ListenerSet objects are recommended to be placed in the same namespace.

In both cases, it is recommended to place the ListenerSet object in the same namespace as the HTTPRoute, GRPCRoute, and TLSRoute objects attached to it. This improves configuration readability and helps avoid additional setup such as ReferenceGrant objects. TCPRoute and UDPRoute for plain TCP/UDP ports from `additionalPorts` are attached directly to the Gateway listener.

An example of a ListenerSet resource manifest for managing the reception of incoming HTTP and HTTPS requests through a cluster-wide gateway:

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
```

### Creating routes and configuring routing

The following route types are used to route incoming requests:

- HTTPRoute: For routing HTTP/HTTPS/TLS requests. HTTPRoute objects support extended settings through annotations that complement the current Gateway API specification.
- GRPCRoute: For routing gRPC traffic.
- TLSRoute: For TLS passthrough routing.
- TCPRoute: For routing TCP traffic. For plain TCP ports from `additionalPorts`, attach TCPRoute directly to the Gateway listener, not to a ListenerSet.
- UDPRoute: For routing UDP traffic. For plain UDP ports from `additionalPorts`, attach UDPRoute directly to the Gateway listener, not to a ListenerSet.

Example of a route for HTTP traffic:

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

Example of a route for HTTPS traffic:

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

## Publishing service domains

{% alert level="warning" %}
If you need to publish the service domains, ensure that the global parameter [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) is specified. If it is not specified, system HTTPRoute/Gateway/ListenerSet objects will be created incorrectly, and the web interfaces of DKP service components and other modules will not be published.
{% endalert %}

To provide access to the DKP cluster’s service domains, specify a default gateway. To do this, follow these steps:

1. Create a cluster-scoped ClusterALBInstance object with the desired inlet type and [configuration](/modules/alb/cr.html#clusteralbinstance). Set the [`spec.defaultDeckhouseGateway: true`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-defaultdeckhousegateway) parameter for this ClusterALBInstance.

   Example of a manifest for a cluster-scoped ClusterALBInstance object with a parameter `spec.defaultDeckhouseGateway: true`:

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

1. After applying the changes, check the status of the ClusterALBInstance object:

   ```bash
   d8 k get clusteralbinstances
   ```

   The ClusterALBInstance object must expose the managed Gateway object, and the instance itself must move to a ready state. After that, system ListenerSet objects and HTTPRoute objects must appear in the corresponding system namespaces of the cluster.

### Algorithm for selecting the default DKP gateway when using multiple ClusterALBInstances

A cluster can have multiple cluster-scoped gateways simultaneously, each marked as the default gateway (with the [`spec.defaultDeckhouseGateway: true`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-defaultdeckhousegateway) flag set for the corresponding ClusterALBInstance). In that case, the default gateway is the Gateway object created by the oldest ClusterALBInstance object according to `creationTimestamp`. If no ClusterALBInstance object is marked as the default gateway, DKP allows the Gateway object created by the `alb` module for the instance named `main` to be used as the default gateway.

### Changing the default DKP gateway

If DKP system domains need to move to another Gateway object, complete these steps:

1. Create a new ClusterALBInstance object that describes the required settings and set [`spec.defaultDeckhouseGateway: true`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-defaultdeckhousegateway) on it.
1. In the current ClusterALBInstance object that provides the default DKP gateway, set `spec.defaultDeckhouseGateway: false`.
1. Check that all system ListenerSet objects now point to the new Gateway object in `spec.parentRef`.

## Changing the inlet while keeping the current Gateway {#change-inlet}

To change the inlet used for an existing Gateway object, complete these steps:

1. Create a new ClusterALBInstance object or ALBInstance object with a different name but the same `spec.gatewayName`, using the required inlet type.
2. Check that the new traffic path works correctly.
3. Delete the obsolete ClusterALBInstance object or ALBInstance object.

Because `gatewayName` does not change, the Gateway object stays the same. In most cases, the ListenerSet object and routes do not need to be rewritten.

## Opening an additional TCP/UDP port on the cluster-wide gateway {#tcp-port}

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

The controller adds a corresponding TCP/UDP traffic handler to the managed Gateway object with a section name (`sectionName`) like `tcp-port-9000`. Then a TCPRoute object may be created that points directly to that Gateway object and that `sectionName`:

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
If a TCPRoute or UDPRoute object is created in a namespace different from the Gateway object namespace, a corresponding ReferenceGrant object must be created.
{% endalert %}

[UDPRoute examples and application publishing steps](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html#grpcroute-tlsroute-tcproute-and-udproute-objects) are in the user documentation.

## Port conflicts when using multiple ClusterALBInstance/ALBInstance objects for a single Gateway {#conflicts}

If the same Gateway object is shared by several ClusterALBInstance or ALBInstance objects, the resulting listener set that actually reaches the Gateway object comes from the oldest ClusterALBInstance or ALBInstance object. The others report a port conflict in status, specifying the name of the controlling instance.

## Infrastructure configuration examples {#infrastructure-examples}

The module supports two inlet types:

- `LoadBalancer` — traffic is accepted through a Service object of the `LoadBalancer` type (cloud providers or bare metal with MetalLB). Available for both ClusterALBInstance and ALBInstance.
- `HostPort` — traffic is accepted on node ports without an external load balancer. Available for ClusterALBInstance only.

Full CR field reference: [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) and [ALBInstance](/modules/alb/cr.html#albinstance). Additional examples are also available in the [`alb` module examples](/modules/alb/examples.html).

### Cloud provider (LoadBalancer inlet) {#cloud-load-balancer}

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

### Bare metal with MetalLB {#bare-metal-metallb}

1. Enable the [`metallb`](/modules/metallb/) module.
1. Create a MetalLoadBalancerClass object with an address pool. Place MetalLB balancers on the same nodes as the Envoy Proxy pods of the `alb` module (typically frontend nodes labeled `node-role.deckhouse.io/frontend`):

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

1. Create a ClusterALBInstance object and set [`spec.inlet.loadBalancer.loadBalancerClass`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer-loadbalancerclass):

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

### Bare metal without an external load balancer (HostPort inlet) {#bare-metal-hostport}

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
Proxy Protocol and HTTP/3 cannot be enabled at the same time.
{% endalert %}

### Separating public and administrative zones {#public-and-admin-zones}

Create a dedicated Gateway object for each zone and restrict administrative traffic with [`spec.acceptRequestsFrom`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-acceptrequestsfrom). The decision is based on the real connection address, not on request headers.

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

Then create separate ListenerSet objects and routes for each gateway. Application publishing examples: [Utilizing Application Load Balancer (ALB)](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html).

### Namespaced publishing (ALBInstance) {#namespaced-load-balancer}

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

After the ALBInstance reaches the `Ready` state, create ListenerSet and HTTPRoute objects in the same namespace. Next steps are in [Publishing an application through an ALBInstance object](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html#publishing-an-application-through-a-albinstance-object).

## FAQ and additional materials {#faq}

Answers to common questions about the `alb` module are in the [`alb` module FAQ](/modules/alb/faq.html):

- [Does the module support receiving TCP traffic?](/modules/alb/faq.html#does-the-module-support-receiving-tcp-traffic) — also [Opening an additional TCP/UDP port](#tcp-port) and [route publishing examples](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html#grpcroute-tlsroute-tcproute-and-udproute-objects);
- [Does the module support receiving UDP traffic?](/modules/alb/faq.html#does-the-module-support-receiving-udp-traffic);
- [How do I configure an external load balancer to check if the ClusterALBInstance/ALBInstance is available?](/modules/alb/faq.html) (`/healthz` on port `80`).

The full set of inlet and zone examples is in the [`alb` module examples](/modules/alb/examples.html). Parameter reference: [`alb` module configuration](/modules/alb/configuration.html) and [`alb` Custom Resources](/modules/alb/cr.html).

## Viewing Envoy Proxy Configuration {#envoy-config}

For troubleshooting, it is useful to inspect the configuration that the controller and the proxy configurator pushed into the Envoy Proxy instance that serves the Gateway object.

To do this, follow these steps:

1. Select an Envoy Proxy pod for the required Gateway object:

   ```bash
   d8 k -n d8-alb get pods -l alb.deckhouse.io/gateway=shared-gateway
   ```

1. Get the configuration through the following command:

   ```bash
   d8 k -n d8-alb exec -it <envoy-proxy-pod-name> pilot-agent request GET /config_dump
   ```

   If only one section of the configuration is needed, the required section may be requested explicitly:

   ```bash
   d8 k -n d8-alb exec -it <envoy-proxy-pod-name> pilot-agent request GET /config_dump?resource=dynamic_listeners
   d8 k -n d8-alb exec -it <envoy-proxy-pod-name> pilot-agent request GET /config_dump?resource=dynamic_route_configs
   d8 k -n d8-alb exec -it <envoy-proxy-pod-name> pilot-agent request GET /config_dump?resource=dynamic_active_clusters
   ```

This makes it easy to check whether the expected traffic handlers, virtual hosts, and upstream clusters appeared after changes to the ListenerSet object or Route object.
