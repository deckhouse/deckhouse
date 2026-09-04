---
title: "Migrating from ingress-nginx to alb"
permalink: en/admin/configuration/network/ingress/alb/migration.html
description: "Migrate from the ingress-nginx module to the alb module in Deckhouse Kubernetes Platform: Gateway API cutover, traffic switching, and rollback."
extractedLinksMax: 4
relatedLinks:
  - title: "ALB with Kubernetes Gateway API"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html
  - title: "ALB with Ingress NGINX Controller"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/nginx.html
  - title: "Incoming traffic balancing overview"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/
  - title: "The alb module documentation"
    url: /modules/alb/
---

This guide describes migration from the `ingress-nginx` module to the `alb` module. Within this migration, application publishing transitions from the Ingress API to the Gateway API.

The guide covers model differences, preparing `alb` module infrastructure with ClusterALBInstance or ALBInstance and a managed Gateway, migrating applications to the Gateway API, migrating Deckhouse Kubernetes Platform (DKP) system interfaces, switching traffic to the `alb` module, and rolling back if needed.

Procedure:

1. [Preparing alb module infrastructure](#step-1-preparing-alb-infrastructure).
1. [Migrating DKP interfaces](#step-2-migrating-dkp-interfaces) — only if system interfaces currently use Ingress and must move to the Gateway API.
1. [Migrating application publishing](#step-3-migrating-application-publishing).
1. [Switching traffic to the alb module](#step-4-switching-traffic-to-alb).
1. [Cleanup](#step-5-cleanup).

## Reasons to move to the Gateway API {#gateway-api-advantages}

Main reasons to move from the Ingress API to the Gateway API:

- Active maintenance of the upstream Ingress NGINX project used by DKP has ended. Further upstream development of features, fixes, and integrations is no longer expected. For new application publishing scenarios, use the Gateway API.
- Unlike the Ingress API, the Gateway API describes routes with protocol-specific resources, configures traffic entry points explicitly, controls route attachment, and manages cross-namespace access through dedicated resources. Complex traffic configurations can be defined with API resources instead of relying mainly on controller-specific annotations.
- The Gateway API separates responsibilities by role. Cluster and network administrators manage traffic infrastructure and Gateway objects through [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) or [ALBInstance](/modules/alb/cr.html#albinstance-v1alpha1-spec). Namespace administrators configure traffic reception through ListenerSet objects (hostname, TLS, ports). Application developers define routing with HTTPRoute and other route resources. This separation supports delegated configuration, validation, and gradual migration.

## Model comparison {#model-comparison}

This section compares how administrators define traffic infrastructure, how DKP provisions it, and how application routing configuration is translated into data-plane configuration.

### Ingress API and ingress-nginx

The diagram below shows the resource model and traffic flow through the `ingress-nginx` module.

![ingress-nginx resource and traffic flow scheme](../../../../../images/network/ingress/alb/ingress-nginx-scheme.svg)

HTTP and HTTPS traffic processing through the `ingress-nginx` module is configured as follows:

1. A cluster administrator creates a cluster-scoped IngressNginxController object.
1. The IngressNginxController specifies the name of the IngressClass it uses. If the name is omitted, `nginx` is used.
1. DKP reconciles the object and provisions the required infrastructure, including the IngressClass. Multiple IngressNginxController objects can use the same IngressClass.
1. By default, DKP resources are published through the IngressClass named `nginx`. A different class can be selected in the DKP global configuration.
1. Network administrators or application development teams create Ingress objects that select the required IngressClass explicitly or implicitly.
1. The resulting nginx configuration combines infrastructure settings from IngressNginxController with the Ingress objects selected by the IngressClass.

### Gateway API and alb

The diagram below shows the resource model and traffic flow through the `alb` module.

![Gateway API resource and traffic flow scheme](../../../../../images/network/ingress/alb/gateway-api-scheme.svg)

HTTP, HTTPS, gRPC, TLS, TCP, and UDP traffic processing through the `alb` module is configured as follows:

1. A cluster administrator creates a cluster-scoped ClusterALBInstance object.
1. The ClusterALBInstance specifies infrastructure parameters and the mandatory `gatewayName`, which identifies the managed Gateway.
1. The `alb` module controller reconciles the object and provisions the managed Gateway and the required traffic-processing infrastructure. Multiple ClusterALBInstance objects can use the same `gatewayName` and therefore the same Gateway.
1. If a default DKP Gateway is configured, DKP modules create their ListenerSet, HTTPRoute, and other Gateway API resources for that Gateway.
1. Network administrators or application development teams create Gateway API resources and attach them to the managed Gateway.
1. The resulting Envoy Proxy configuration combines infrastructure settings from ClusterALBInstance with the configuration represented by the Gateway API resources attached to the Gateway.

### Key architectural differences

The table highlights the following architectural differences:

| Parameter | Ingress API and ingress-nginx | Gateway API and alb |
| --- | --- | --- |
| Resource relationships | Infrastructure and routing rules are associated indirectly through an IngressClass | Resources form an explicit graph using `parentRefs` and other typed references, with the Gateway as the root |
| Separation of responsibilities | IngressNginxController defines infrastructure, while Ingress combines application routing with controller-specific configuration | Cluster and network administrators manage instances and gateways, namespace administrators manage ListenerSet objects, and application developers manage HTTPRoute and other route resources |
| Configuration model | An Ingress combines most HTTP/HTTPS routing configuration in one object and controller-specific annotations | Listeners, routes, backends, and policies are represented by separate, composable objects |
| Shared configuration and entry points | Multiple IngressNginxController objects that use the same IngressClass consume the same set of Ingress rules while providing separate entry points | Multiple ClusterALBInstance objects with the same `gatewayName` provide separate entry points backed by the same Gateway configuration |
| Cross-namespace references | An Ingress, its backend Service, and its TLS Secret generally reside in the same namespace | Controlled cross-namespace references are supported through ReferenceGrant |
| Protocol support | Ingress primarily models HTTP and HTTPS traffic | Dedicated route types model HTTP, gRPC, TCP, TLS, and UDP traffic |
| Extensibility | Additional behavior is commonly configured through implementation-specific annotations | More behavior is expressed through structured, validated API resources and policies |
| Lifecycle and ownership | Infrastructure and routing configuration have limited delegation and attachment controls | Gateway infrastructure can remain stable while application development teams independently create, update, and remove routes |
| Route attachment | Selecting an IngressClass provides a broad association between an Ingress and controllers | Gateway objects and listeners explicitly control which routes may attach to them |

{% alert level="warning" %}
Multiple ClusterALBInstance objects can refer to the same Gateway, but Gateway-level settings such as `additionalPorts` can conflict. Use compatible Gateway-level settings on every instance associated with the same Gateway.
{% endalert %}

Consistent with the Gateway API conflict-resolution model, the `alb` module builds the final configuration from the oldest ClusterALBInstance. Conflicting settings from newer instances are ignored and reported in their status.

## Step 1. Preparing alb module infrastructure {#step-1-preparing-alb-infrastructure}

On this step, choose the `alb` module instance type (ClusterALBInstance or ALBInstance), configure the inlet, and prepare TLS certificates for the Gateway API entry point.

### Choosing ALBInstance or ClusterALBInstance

When choosing the instance type, keep the following in mind:

- Use [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) for a shared or platform-level Gateway, for publishing DKP system interfaces, or when the `HostPort` inlet is required.
- To publish DKP system interfaces, follow [Publishing service domains](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#publishing-service-domains).
- Use [ALBInstance](/modules/alb/cr.html#albinstance-v1alpha1-spec) for a Gateway dedicated to an application or team and managed within its namespace. ALBInstance supports the `LoadBalancer` and `ClusterIP` inlets; for migration from `ingress-nginx`, `LoadBalancer` is the relevant one.
- A detailed comparison is in [ClusterALBInstance and ALBInstance](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#clusteralbinstance-and-albinstance).

### Inlet configuration {#inlet-configuration}

In IngressNginxController, the inlet type combines the method of accepting traffic with optional behaviors such as Proxy Protocol and SSL passthrough. The `alb` module configures these concerns separately:

- ClusterALBInstance supports the `LoadBalancer` and `HostPort` inlet types.
- ALBInstance supports the `LoadBalancer` and `ClusterIP` inlet types.
- Proxy Protocol is enabled by the [`spec.useProxyProtocol`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-useproxyprotocol) parameter. It can be enabled or disabled on an existing instance without restarting the Envoy Proxy Pod objects or recreating the instance.
- TLS passthrough is configured with a TLS listener and a TLSRoute, rather than with a dedicated inlet type.

Use the following mapping when selecting an inlet for the `alb` module:

| IngressNginxController inlet | alb module configuration | Migration notes |
| --- | --- | --- |
| `LoadBalancer` | ClusterALBInstance or ALBInstance with `spec.inlet.type: LoadBalancer` | The controller provisions a Service of type `LoadBalancer` |
| `LoadBalancerWithProxyProtocol` | `LoadBalancer` inlet with `spec.useProxyProtocol: true` | Configure the external load balancer to send Proxy Protocol. Proxy Protocol and HTTP/3 cannot be enabled simultaneously: the `alb` module controller rejects such a configuration as conflicting |
| `LoadBalancerWithSSLPassthrough` | `LoadBalancer` inlet with a TLS listener and TLSRoute | TLS passthrough is part of the Gateway API routing configuration and is not an inlet variant |
| `HostPort` | ClusterALBInstance with `spec.inlet.type: HostPort` | `HostPort` is not supported by ALBInstance |
| `HostPortWithProxyProtocol` | ClusterALBInstance with the `HostPort` inlet and `spec.useProxyProtocol: true` | Proxy Protocol and HTTP/3 cannot be enabled simultaneously |
| `HostPortWithSSLPassthrough` | ClusterALBInstance with the `HostPort` inlet, a TLS listener, and TLSRoute | TLS passthrough is configured independently of the inlet |
| `HostWithFailover` | No direct equivalent | Use a ClusterALBInstance with the `LoadBalancer` inlet backed by MetalLB. Follow ["Example for bare metal with the MetalLB load balancer"](/modules/alb/examples.html#bare-metal-metallb) and validate load-balancer failover before switching traffic |

{% alert level="warning" %}
During migration, the `ingress-nginx` module with a `HostPort` or `HostWithFailover` inlet and the `alb` module with a `HostPort` inlet cannot use the same host ports on the same nodes. Select separate node sets or a different set of host ports for one of the controllers.
{% endalert %}

Map related inlet parameters as follows:

| IngressNginxController | ClusterALBInstance or ALBInstance |
| --- | --- |
| `spec.loadBalancer.annotations` | `spec.inlet.loadBalancer.serviceAnnotations` |
| `spec.loadBalancer.loadBalancerClass` | `spec.inlet.loadBalancer.loadBalancerClass` |
| `spec.loadBalancer.httpPort`, `httpsPort` | `spec.inlet.loadBalancer.httpPort`, `httpsPort` |
| `spec.loadBalancer.sourceRanges` | `spec.inlet.loadBalancer.loadBalancerSourceRanges` |
| `spec.hostPort.httpPort`, `httpsPort` | `spec.inlet.hostPort.httpPort`, `httpsPort` |
| `spec.acceptRequestsFrom` | `spec.acceptRequestsFrom` |
| `spec.*.behindL7Proxy`, `realIPHeader`, `acceptClientIPHeadersFrom` | `spec.originalIPDetection.realIPHeader`, `setRealIPFrom` |

When migrating these parameters, consider the following:

- Copy only service annotations supported by the target load balancer implementation.
- Set [`spec.inlet.loadBalancer.loadBalancerClass`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer-loadbalancerclass) at creation — the parameter is immutable afterward.
- Keep the default HTTP and HTTPS ports as `80` and `443`, or set a port to `0` in the `alb` module to disable the corresponding default listener.
- Configure HostPort parameters for ClusterALBInstance only. Specify at least one port.
- Preserve the required source CIDR restrictions in [`spec.acceptRequestsFrom`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-acceptrequestsfrom).
- Configure trusted proxy CIDRs explicitly. Do not trust client-IP headers from arbitrary sources.
- Verify `loadBalancerSourceRanges` behavior with the target load balancer implementation. The value is passed to the `LoadBalancer` Service. Cloud providers may not support or may ignore this parameter.

The inlet type is immutable for both ClusterALBInstance and ALBInstance. To change the inlet, create a new instance with the same `gatewayName`, validate traffic, switch to it, and delete the old instance.

### TLS and certificates

{% alert level="warning" %}
When `ingress-nginx` and `alb` are used simultaneously and certificates are issued by Issuer or ClusterIssuer resources with HTTP-01 solvers, use separate Certificate resources and Secret objects for the Ingress API and Gateway API paths. Otherwise certificate issuance or renewal may conflict.
{% endalert %}

DKP provisions a Gateway-specific ClusterIssuer with a Let's Encrypt HTTP-01 solver for the [default DKP Gateway](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#publishing-service-domains). Separate Certificate and Secret objects are required only for Issuer and ClusterIssuer resources with HTTP-01 solvers. This does not apply to resources configured exclusively with DNS-01 solvers.

Instructions for configuring an HTTP-01 issuer with the Gateway API solver are in ["Adding a custom HTTP-01 ClusterIssuer or Issuer for ALB"](/modules/alb/faq.html#custom-http01-clusterissuer-alb).

To finish preparing the infrastructure, do the following:

1. Create a ClusterALBInstance or ALBInstance with the selected inlet and the parameters from the tables above.
1. Wait until the instance and the managed Gateway are ready.
1. Prepare separate Certificate resources and Secret objects for the Gateway API path when Issuer or ClusterIssuer resources with HTTP-01 solvers are used at the same time.

## Step 2. Migrating DKP interfaces {#step-2-migrating-dkp-interfaces}

If DKP system interfaces are published through Ingress and must move to the Gateway API, follow [Publishing service domains](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#publishing-service-domains). Not every DKP module publishes service HTTPRoute objects through the Gateway API yet. After you configure the default gateway, the `jq` command in that section shows the actual inventory of routes already published in the cluster, not a full platform capability matrix.

If you do not need to migrate DKP interfaces, continue with step 3.

## Step 3. Migrating application publishing {#step-3-migrating-application-publishing}

The list of Gateway API resources supported by the `alb` module and ownership details for managed Gateway objects are in ["Supported Gateway API resources"](/modules/alb/).

### Converting Ingress to Gateway API

To convert Ingress resources to Gateway API resources, do the following:

1. Confirm that the target ALBInstance or ClusterALBInstance was created in [step 1](#step-1-preparing-alb-infrastructure) and is ready.
1. Obtain the managed Gateway namespace and name from the instance status.
1. Convert each Ingress and its related configuration into the appropriate ListenerSet, route, and policy resources for that Gateway. The built-in conversion tool produces a draft, but not every `ingress-nginx` feature has a direct Gateway API equivalent.
1. Review and adjust the generated manifests before applying them.
1. Apply the resources:

   ```shell
   d8 k apply -f gateway-api.yaml
   ```
1. Verify Gateway, ListenerSet, and route statuses before switching traffic.

#### Using the built-in ingress2gateway tool

The gateway controller provides an HTTP endpoint that accepts a single object, a Kubernetes List object, or a YAML payload containing multiple documents. The converter reads the following input resources:

- Ingress — the resources selected for conversion.
- Service — used to resolve named backend ports and service metadata.
- DexAuthenticator — used to resolve `spec.applicationDomain` when an `ingress-nginx` external-auth configuration contains nginx variables such as `$host`.

Pass all related resources as input so the converter can preserve the supported parts of their configuration. The endpoint is disabled by default and listens only inside gateway controller Pod objects.

1. Temporarily enable [`migrations.ingress2Gateway.enabled`](/modules/alb/configuration.html#parameters-migrations-ingress2gateway-enabled) in the `alb` module configuration and wait for the gateway controller to restart:

   ```shell
   d8 k patch moduleconfig alb --type merge \
     --patch '{"spec":{"settings":{"migrations":{"ingress2Gateway":{"enabled":true}}}}}'
   d8 k -n d8-alb rollout status deployment/gateway-controller
   ```

1. Forward the endpoint from a gateway controller Pod:

   ```shell
   d8 k -n d8-alb port-forward deployment/gateway-controller 8082:8082
   ```

1. In another terminal, export all recognized resource types and send the resulting Kubernetes List directly to the converter:

   ```shell
   converter_url='http://127.0.0.1:8082/ingress2gateway'
   converter_url="${converter_url}?gateway=<GATEWAY_NAMESPACE>/<GATEWAY_NAME>"
   converter_url="${converter_url}&scope=<SCOPE>&ingress-class=<INGRESS_CLASS>"

   d8 k get ingress,service,dexauthenticator \
     --all-namespaces --output yaml | \
     curl --fail-with-body --silent --show-error --request POST \
       --header 'Content-Type: application/yaml' \
       --data-binary @- \
       --output gateway-api.yaml \
       "$converter_url"
   ```

   In the URL, query parameter names are lowercase: `gateway`, `scope`, and `ingress-class`. In the example, their values are placeholders in angle brackets and uppercase:

   - `<GATEWAY_NAMESPACE>` — namespace of the managed Gateway from the instance status in step 1;
   - `<GATEWAY_NAME>` — name of the managed Gateway from the same status.

     The `<GATEWAY_NAMESPACE>/<GATEWAY_NAME>` pair is the value of the `gateway` parameter. Use the default value `d8-alb/public-gw` only if it matches your Gateway.

   - `<SCOPE>` — `cluster` for the cluster-scoped model, or `namespaced` for a Gateway in an application namespace (as with ALBInstance). If the parameter is omitted, `cluster` is used;
   - `<INGRESS_CLASS>` — IngressClass of the source Ingress resources. If the parameter is omitted, `nginx` is used.

   {% alert level="info" %}
   The request body is limited to 8 MiB. For large clusters, export only the namespaces and related resources being migrated.
   {% endalert %}

1. Review `gateway-api.yaml` and the conversion diagnostics included as YAML comments, then apply the resources.
1. After completing the conversion, disable the endpoint:

   ```shell
   d8 k patch moduleconfig alb --type merge \
     --patch '{"spec":{"settings":{"migrations":{"ingress2Gateway":{"enabled":false}}}}}'
   ```

#### Extending Gateway API with annotations

The Gateway API specification does not cover every implementation-specific traffic-management feature required by DKP. The `alb` module therefore uses HTTPRoute annotations for options that are not yet represented by standard Gateway API fields.

As the corresponding features become available in the Gateway API, the `alb` module will gradually replace annotation-based configuration with native Gateway API resources and fields.

During migration, use standard Gateway API fields where possible and replace `ingress-nginx` annotations only with supported `alb` annotations — including after you apply `gateway-api.yaml`. The current list is in ["Supported HTTPRoute annotations"](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html#supported-httproute-annotations).

The `ingress2gateway` utility produces a draft of Gateway API resources and **does not** migrate every `ingress-nginx` annotation automatically. After conversion, compare the Ingress with the table below and apply the required settings manually.

| Ingress annotation (`ingress-nginx`) | Equivalent in the `alb` module |
| --- | --- |
| `nginx.ingress.kubernetes.io/whitelist-source-range` | HTTPRoute annotation `alb.network.deckhouse.io/whitelist-source-range` |
| `nginx.ingress.kubernetes.io/service-upstream` | HTTPRoute annotation `alb.network.deckhouse.io/service-upstream` |
| `nginx.ingress.kubernetes.io/upstream-vhost` | HTTPRoute `URLRewrite` filter with `hostname` (see [publishing with an Istio sidecar](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html#publishing-with-istio-sidecar)) |
| `nginx.ingress.kubernetes.io/auth-url` / `auth-signin` | HTTPRoute annotations `alb.network.deckhouse.io/auth-url` and `alb.network.deckhouse.io/auth-signin` |
| `nginx.ingress.kubernetes.io/auth-type: basic` and a Secret | HTTPRoute annotation `alb.network.deckhouse.io/basic-auth-secret` |
| `nginx.ingress.kubernetes.io/limit-rps` | HTTPRoute annotation `alb.network.deckhouse.io/limit-rps` |
| `nginx.ingress.kubernetes.io/proxy-body-size` | HTTPRoute annotation `alb.network.deckhouse.io/buffer-max-request-bytes` (value in bytes) |
| `nginx.ingress.kubernetes.io/proxy-buffer-size` | HTTPRoute annotation `alb.network.deckhouse.io/proxy-buffer-size` |
| `nginx.ingress.kubernetes.io/proxy-read-timeout` / `proxy-send-timeout` | HTTPRoute annotation `alb.network.deckhouse.io/idle-timeout` (idle timeout, not total request duration) |
| `nginx.ingress.kubernetes.io/affinity` / cookie | HTTPRoute annotation `alb.network.deckhouse.io/session-affinity` |
| `nginx.ingress.kubernetes.io/rewrite-target` | HTTPRoute annotation `alb.network.deckhouse.io/rewrite-target` or standard Gateway API filters |
| `nginx.ingress.kubernetes.io/configuration-snippet` and other nginx snippets | No direct equivalent; redesign the configuration for Gateway API and `alb` annotations |

If an Ingress annotation is missing from the table and has no Gateway API field, behavior after migration may differ or disappear. Validate the application on the `alb` module path before switching production traffic.

## Step 4. Switching traffic to the alb module {#step-4-switching-traffic-to-alb}

Run `ingress-nginx` and `alb` simultaneously until the `alb` module path has been validated and the rollback window has closed.

Migrate individual domains or namespaces when they can use separate DNS records. Otherwise, switch the shared external entry point by changing DNS records or the backend pool of an external load balancer.

On this step:

- Test the `alb` module without changing DNS with `migrationGateway` when you need validation without a DNS change (`ingress-nginx` version `1.1.0` and later).
- Enable `http01CertificateSolverBridging` when Gateway API path certificates use HTTP-01 while public DNS still points to Ingress.
- Choose the switching method for your current entry point in the subsections below: automatically provisioned load balancer, manually managed load balancer, or HostPort/HostWithFailover.
- Before the final production cutover, complete ["Validating the cutover"](#validating-the-cutover).

### Testing the alb module via the Ingress NGINX Controller

The [`spec.migrationGateway`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-migrationgateway) parameter of IngressNginxController enables source-IP-based testing without changing DNS.

Requests from `sourceCIDRs` continue to enter through the existing Ingress NGINX Controller entry point but are proxied to the internal Service of the target `alb` module instance. Other requests continue to use the original Ingress backends.

{% alert level="info" %}
The parameter is available in the `ingress-nginx` module version `1.1.0` and later.
{% endalert %}

Despite its use in this guide for migration to the `alb` module, [`migrationGateway`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-migrationgateway) is not specific to the `alb` module.

Its `serviceRef` can point to the entry-point Service of any Gateway API implementation that accepts ordinary HTTP and HTTPS traffic on the configured ports.

Locate the `alb` module configuration Service:

```shell
d8 k get service --all-namespaces \
  --selector alb.deckhouse.io/configuration-service
```

Example output for a ClusterALBInstance named `main` — a `ClusterIP` Service in the `d8-alb` namespace:

<!-- markdownlint-disable MD031 -->
```console
NAMESPACE   NAME   TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)         AGE
d8-alb      main   ClusterIP   10.222.1.10    <none>        80/TCP,443/TCP  10d
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

For an ALBInstance, the Service is in the ALBInstance's namespace, named `d8-alb-<ALB_INSTANCE_NAME>`.

Configure the source IngressNginxController, using narrow tester CIDRs initially:

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: <CONTROLLER_NAME>
spec:
  # Spec fields unrelated to migrationGateway (inlet, ingressClass, and so on) are omitted.
  migrationGateway:
    sourceCIDRs:
      - <SOURCE_CIDR>
    serviceRef:
      namespace: <SERVICE_NAMESPACE>
      name: <SERVICE_NAME>
      ports:
        http: <HTTP_PORT>
        https: <HTTPS_PORT>
```

where:

- `<CONTROLLER_NAME>` — name of the source IngressNginxController;
- `<SOURCE_CIDR>` — CIDR of clients whose requests are proxied to the `alb` module during testing;
- `<SERVICE_NAMESPACE>` — namespace of the `alb` module configuration Service;
- `<SERVICE_NAME>` — name of the `alb` module configuration Service;
- `<HTTP_PORT>` — HTTP port on the target Service;
- `<HTTPS_PORT>` — HTTPS port on the target Service.

For matching clients, `migrationGateway` forwards requests to the target Service ports and bypasses the backends and location-level behavior from the original Ingress resources. HTTP requests go to the configured HTTP port. For HTTPS, nginx terminates the incoming TLS connection and establishes a new TLS connection to the configured HTTPS port, using the original hostname in the Host header and for SNI. HTTP, HTTPS, and HTTP-based protocol upgrades such as WebSocket are supported. gRPC and non-HTTP protocols are not.

The setting applies to every Ingress served by the controller. Before expanding `sourceCIDRs`, verify every hostname reachable from those addresses and confirm that authentication, redirects, headers, GeoIP behavior, WebSocket connections, and other required policies exist on the `alb` module path.

The `migrationGateway` parameter works by forwarding an already-accepted, and if needed re-encrypted, HTTP or HTTPS request to the target Service, so it is not supported where nginx does not perform this processing:

- with the `HostPortWithSSLPassthrough` and `LoadBalancerWithSSLPassthrough` inlets — with TLS passthrough, nginx does not decrypt traffic and cannot determine which request to forward;
- with the `HostWithFailover` inlet — this inlet always uses Proxy Protocol, and `migrationGateway` does not support sending Proxy Protocol to the target Service;
- with `enableIstioSidecar` — traffic is handled by the Istio sidecar rather than directly by nginx.

The target Service must accept ordinary HTTP and HTTPS without Proxy Protocol — `migrationGateway` does not send it.

If the `alb` module path needs Proxy Protocol in production, disable it for `migrationGateway` testing (`spec.useProxyProtocol: false`). Re-enable it before shifting production traffic and validate the entry point through the load balancer that sends Proxy Protocol.

When a trusted L7 proxy is used in front of the Ingress NGINX Controller, verify real-IP processing before relying on `sourceCIDRs`.

To immediately return selected clients to the original Ingress backends, remove `spec.migrationGateway`:

```shell
d8 k patch ingressnginxcontroller <CONTROLLER_NAME> --type json \
  --patch '[{"op":"remove","path":"/spec/migrationGateway"}]'
```

where `<CONTROLLER_NAME>` is the name of the source IngressNginxController.

### Preserving HTTP-01 validation

The [`migrationGateway`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-migrationgateway) parameter handles selected application requests.

The [`migrations.http01CertificateSolverBridging`](/modules/alb/configuration.html#parameters-migrations-http01certificatesolverbridging) parameter keeps cert-manager Gateway API HTTP-01 challenges reachable through the Ingress NGINX Controller while public DNS still points to it.

When Issuer or ClusterIssuer resources with HTTP-01 solvers are used, do the following:

1. Enable bridging before requesting certificates for the Gateway API path:

   ```shell
   d8 k patch moduleconfig alb --type merge \
     --patch '{"spec":{"settings":{"migrations":{"http01CertificateSolverBridging":{"enabled":true,"ingressClassName":"<INGRESS_CLASS>"}}}}}'
   d8 k -n d8-alb rollout status deployment/gateway-controller
   ```

   where `<INGRESS_CLASS>` is the IngressClass that currently receives public traffic on port `80`.

1. Keep the `ingress-nginx` module enabled. The `alb` module controller creates temporary Ingress resources for cert-manager solver HTTPRoute resources, allowing challenges to pass through the Ingress NGINX Controller to the solver Service.

1. Issue certificates for the Gateway API path before the traffic cutover. Test the prepared TLS configuration through `migrationGateway` or by connecting directly to the Gateway API entry-point address with a hostname override, for example using the curl `--resolve` option.

1. After DNS or the external load balancer sends public traffic to the Gateway API entry point, verify HTTP-01 issuance through the Gateway API path and disable bridging:

   ```shell
   d8 k patch moduleconfig alb --type merge \
     --patch '{"spec":{"settings":{"migrations":{"http01CertificateSolverBridging":{"enabled":false}}}}}'
   ```

### Choosing the switching method

Choose the switching method based on how external traffic is accepted today: through an automatically provisioned load balancer, a manually managed load balancer, or direct HostPort/HostWithFailover access.

{% tabs Switching method %}
{% tab "DNS / provisioned LB" %}

#### Automatically provisioned load balancer

To switch traffic through DNS, do the following:

1. Wait for the `alb` module load balancer and its health checks to become ready.
1. Lower the DNS TTL in advance.
1. Change each application DNS record from the Ingress NGINX Controller address to the Gateway API entry-point address.

When the Ingress NGINX Controller uses a `LoadBalancer` Service provisioned by the Kubernetes cloud provider, the source and target controllers normally have separate load balancer IP addresses or hostnames.

Use weighted DNS records for a gradual switch only if the DNS provider supports them.

A provider-managed load balancer does not normally allow node-by-node migration between two independently managed Service objects.

If MetalLB or another implementation uses a fixed address, the same address cannot be assigned to both Service objects simultaneously. Transfer it only during a coordinated cutover.

{% endtab %}
{% tab "Manually managed LB" %}

#### Manually managed load balancer

To switch traffic through an external load balancer, do the following:

1. Keep the public DNS record unchanged.
1. Add `alb` module nodes and their configured ports as healthy backends in the external load balancer pool.
1. Shift traffic from Ingress NGINX Controller backends gradually.
1. Remove the old backends only after validation.

This supports node-by-node migration and weighted traffic distribution when the external load balancer provides those features.

When configuring backends, keep the following in mind:

- Configure health checks against `/healthz` on the `alb` module HTTP port.
- Preserve the original protocol and client-address handling: align Proxy Protocol settings for traffic and health checks, or configure trusted forwarded headers for an L7 load balancer.
- If both controllers run on the same nodes, use different host ports. Alternatively, select disjoint node sets.

{% endtab %}
{% tab "HostPort or HostWithFailover" %}

#### Direct HostPort or HostWithFailover access

To switch traffic with direct HostPort or HostWithFailover access, do the following:

1. Deploy the `alb` module on a separate node set or use non-conflicting host ports.
1. For a node-by-node switch, add `alb` module node addresses to the DNS pool and remove Ingress NGINX Controller node addresses after each node passes validation.
1. Account for DNS TTLs and client-side caching: when records point to node addresses, DNS pools do not provide deterministic weighting or connection draining.

DNS cannot distinguish controllers that use different ports on the same node address. If clients must continue using ports `80` and `443`, use separate nodes or introduce a load balancer or NAT rule.

`HostWithFailover` has no direct equivalent in the `alb` module. When equivalent failover behavior is required, use the `LoadBalancer` inlet with MetalLB.

{% endtab %}
{% endtabs %}

### Validating the cutover {#validating-the-cutover}

{% alert level="warning" %}
Traffic processing can differ between the Ingress NGINX Controller and the `alb` module, including generated and trusted headers, protocol handling, and supported features. Thoroughly test applications through the `alb` module path and, where necessary, adjust them to the traffic-processing behavior of the `alb` module before switching production traffic.
{% endalert %}

Before shifting production traffic:

- Confirm that the step 3 resources are applied and that the target ALBInstance or ClusterALBInstance reports `ready: true` and `synced: true`. Verify that `conflictPorts`, `conflictBackendTLS`, and `conflictFrontendTLS` are all `false`; if any is `true`, the matching `<CONFLICT>Owner` field names the controlling instance.
- Check the `Accepted` and `Programmed` conditions on the Gateway and ListenerSet status, and `Accepted` on the route status (`d8 k -n <NAMESPACE> get gateway,listenerset,httproute <NAME> -o yaml` — the `status.conditions` section). `True` means the controller accepted the resource and applied its configuration.
- Test every hostname and protocol through the Gateway API entry-point address or with `migrationGateway`, including TLS certificates, redirects, authentication, long-lived connections, and application-specific policies.
- Verify preservation of the client address, Proxy Protocol or forwarded-header processing, source restrictions, and load balancer health checks.
- If HTTP-01 bridging is still required, keep `http01CertificateSolverBridging` enabled until DNS or the load balancer is switched.
- Confirm that metrics, logs, alerts, and dashboards identify traffic and errors on both controllers.
- Keep Ingress resources, the Ingress NGINX Controller, its external entry point, and valid certificates available throughout the rollback window.
- Choose the switching method above and define rollback criteria.

### Rolling back

Define rollback criteria before the cutover. If validation fails:

1. For `migrationGateway` testing, remove `spec.migrationGateway`.
1. For a DNS switch, restore the Ingress NGINX Controller address and wait for the previous DNS TTL to expire.
1. For a manually managed load balancer, restore Ingress NGINX Controller nodes and ports in its backend pool.
1. If public traffic again enters through the Ingress NGINX Controller and the Issuer or ClusterIssuer resources for Gateway API certificates use HTTP-01 solvers, re-enable `http01CertificateSolverBridging`.

Do not delete the old Ingress resources, Ingress NGINX Controller, load balancer, or DNS values until rollback has been tested and the agreed stabilization period has elapsed.

## Step 5. Cleanup {#step-5-cleanup}

After the rollback window closes, do the following:

1. Remove `migrationGateway` if it is still set.
1. Disable `http01CertificateSolverBridging` and `ingress2Gateway` if they are still enabled.
1. Delete Ingress resources and Ingress NGINX Controllers that no longer serve applications or DKP system interfaces.
1. Remove unused load balancers, certificates, Secret objects, and DNS records.
1. Restore normal DNS TTLs after confirming that no clients use the old entry point.

{% alert level="info" %}
Some DKP interfaces may still be published through the Ingress API. Do not disable the `ingress-nginx` module or delete related objects until the required interfaces are published through the Gateway API and validated. Follow ["Publishing service domains"](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#publishing-service-domains).
{% endalert %}
