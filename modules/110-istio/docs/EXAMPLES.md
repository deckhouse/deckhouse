---
title: "The istio module: examples"
---

## Circuit Breaker

The `outlierDetection` settings in the [DestinationRule](istio-cr.html#destinationrule) custom resource help to determine whether some endpoints do not behave as expected. Refer to the [Envoy documentation](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/outlier) for more details on the Outlier Detection algorithm.

Example:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: reviews-cb-policy
spec:
  host: reviews.prod.svc.cluster.local
  trafficPolicy:
    connectionPool:
      tcp:
        maxConnections: 100 # The maximum number of connections to the host (cumulative for all endpoints)
      http:
        maxRequestsPerConnection: 10 # The connection will be re-established after every 10 requests
    outlierDetection:
      consecutive5xxErrors: 7 # Seven consecutive errors are allowed (including 5XX, TCP and HTTP timeouts)
      interval: 5m            # over 5 minutes.
      baseEjectionTime: 15m   # Upon reaching the error limit, the endpoint will be excluded from balancing for 15 minutes.
```

Additionally, the [VirtualService](istio-cr.html#virtualservice) resource is used to configure the HTTP timeouts. These timeouts are also taken into account when calculating error statistics for endpoints.

Example:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: my-productpage-rule
  namespace: myns
spec:
  hosts:
  - productpage
  http:
  - timeout: 5s
    route:
    - destination:
        host: productpage
```

## gRPC balancing

**Caution!** Assign a name with the `grpc` prefix or value to the port in the corresponding Service to make gRPC service balancing start automatically.

## Locality Failover

> Read [the main documentation](https://istio.io/latest/docs/tasks/traffic-management/locality-load-balancing/failover/) if you need.

Istio allows you to configure a priority-based locality (geographic location) failover between endpoints. Istio uses node labels with the appropriate hierarchy to define the zone:

* `topology.istio.io/subzone`
* `topology.kubernetes.io/zone`
* `topology.kubernetes.io/region`

This comes in handy for inter-cluster failover when used together with a [multicluster](#setting-up-multicluster-for-two-clusters-using-the-istiomulticluster-cr).

> **Caution!** The Locality Failover can be enabled using the DestinationRule CR. Note that you also have to configure the outlierDetection.

Example:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: helloworld
spec:
  host: helloworld
  trafficPolicy:
    loadBalancer:
      localityLbSetting:
        enabled: true # LF is enabled
    outlierDetection: # outlierDetection must be enabled
      consecutive5xxErrors: 1
      interval: 1s
      baseEjectionTime: 1m
```

## Retry

You can use the [VirtualService](istio-cr.html#virtualservice) resource to configure Retry for requests.

> **Caution!** All requests (including POST ones) are retried three times by default.

Example:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: ratings-route
spec:
  hosts:
  - ratings.prod.svc.cluster.local
  http:
  - route:
    - destination:
        host: ratings.prod.svc.cluster.local
    retries:
      attempts: 3
      perTryTimeout: 2s
      retryOn: gateway-error,connect-failure,refused-stream
```

## Canary

> **Caution!** Istio is only responsible for flexible request routing that relies on special request headers (such as cookies) or simply randomness. The CI/CD system is responsible for customizing this routing and "switching" between canary versions.

The idea is that two Deployments with different versions of the application are deployed in the same namespace. The Pods of different versions have different labels (`version: v1` and `version: v2`).

You have to configure two custom resources:
* A [DestinationRule](istio-cr.html#destinationrule) – defines how to identify different versions of your application (subsets);
* A [VirtualService](istio-cr.html#virtualservice) – defines how to balance traffic between different versions of your application.

Example:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: productpage-canary
spec:
  host: productpage
  # subsets are only available when accessing the host via the VirtualService from a Pod managed by Istio.
  # These subsets must be defined in the routes.
  subsets:
  - name: v1
    labels:
      version: v1
  - name: v2
    labels:
      version: v2
```

### Cookie-based routing

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: productpage-canary
spec:
  hosts:
  - productpage
  http:
  - match:
    - headers:
       cookie:
         regex: "^(.*;?)?(canary=yes)(;.*)?"
    route:
    - destination:
        host: productpage
        subset: v2 # The reference to the subset from the DestinationRule.
  - route:
    - destination:
        host: productpage
        subset: v1
```

### Probability-based routing

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: productpage-canary
spec:
  hosts:
  - productpage
  http:
  - route:
    - destination:
        host: productpage
        subset: v1 # The reference to the subset from the DestinationRule.
      weight: 90 # Percentage of traffic that the Pods with the version: v1 label will be getting.
  - route:
    - destination:
        host: productpage
        subset: v2
      weight: 10
```

## Ingress to publish applications

### Istio Ingress Gateway

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressIstioController
metadata:
 name: main
spec:
  # ingressGatewayClass contains the label selector value used to create the Gateway resource
  ingressGatewayClass: istio-hp
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  nodeSelector:
    node-role/frontend: ''
  tolerations:
    - effect: NoExecute
      key: dedicated
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
  namespace: d8-ingress-istio # note the namespace isn't app-ns
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
    # label selector for using the Istio Ingress Gateway main-hp
    istio.deckhouse.io/ingress-gateway-class: istio-hp
  servers:
    - port:
        # standard template for using the HTTP protocol
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - app.example.com
    - port:
        # standard template for using the HTTPS protocol
        number: 443
        name: https
        protocol: HTTPS
      tls:
        mode: SIMPLE
        # a secret with a certificate and a key, which must be created in the d8-ingress-istio namespace
        # supported secret formats can be found at https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#key-formats
        credentialName: app-tls-secrets
      hosts:
        - app.example.com
```

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

### NGINX Ingress

To use Ingress, you need to:
* Configure the Ingress controller by adding Istio sidecar to it. In our case, you need to enable the `enableIstioSidecar` parameter in the [ingress-nginx](../../modules/402-ingress-nginx/) module's [IngressNginxController](../../modules/402-ingress-nginx/cr.html#ingressnginxcontroller) custom resource.
* Set up an Ingress that refers to the Service. The following annotations are mandatory for Ingress:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — using this annotation, the Ingress controller sends requests to a single ClusterIP (from Service CIDR) while envoy load balances them. Ingress controller's sidecar is only catching traffic directed to Service CIDR.
  * `nginx.ingress.kubernetes.io/upstream-vhost: myservice.myns.svc` — using this annotation, the sidecar container can identify the application service that serves requests.

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

## Authorization configuration examples

### Decision-making algorithm

The following algorithm for deciding the fate of a request becomes active after `AuthorizationPolicy` is created for the application:
* The request is denied if it falls under the DENY policy;
* The request is allowed if there are no ALLOW policies for the application;
* The request is allowed if it falls under the ALLOW policy.
* All other requests are denied.

In other words, if you explicitly deny something, then only this restrictive rule will work. If you explicitly allow something, only explicitly authorized requests will be allowed (however, restrictions will stay in force and have precedence).

> **Caution!** The policies based on high-level parameters like namespace or principal require enabling Istio for all involved applications. Also, there must be organized Mutual TLS between applications.

Examples:
* Let's deny POST requests for the myapp application. Since a policy is defined, only POST requests to the application are denied (as per the algorithm above).

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: deny-post-requests
    namespace: foo
  spec:
    selector:
      matchLabels:
        app: myapp
    action: DENY
    rules:
    - to:
      - operation:
          methods: ["POST"]
  ```

* Below, the ALLOW policy is defined for the application. It only allows requests from the `bar` namespace (other requests are denied).

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: deny-all
    namespace: foo
  spec:
    selector:
      matchLabels:
        app: myapp
    action: ALLOW # The default value, can be skipped.
    rules:
    - from:
      - source:
          namespaces: ["bar"]
  ```

* Below, the ALLOW policy is defined for the application. Note that it does not have any rules, so not a single request matches it (still, the policy exists). Thus, our decision-making algorithm suggests that if something is allowed, then everything else is denied. In this case, "everything else" includes all the requests.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: deny-all
    namespace: foo
  spec:
    selector:
      matchLabels:
        app: myapp
    action: ALLOW # The default value, can be skipped.
    rules: []
  ```

* Below, the (default) ALLOW policy is defined for the application. Note that it has an empty rule. Any request matches this rule, so it is naturally approved.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: allow-all
    namespace: foo
  spec:
    selector:
      matchLabels:
        app: myapp
    rules:
    - {}
  ```

### Deny all for the foo namespace

There are two ways you can do that:

* Explicitly. Here, the DENY policy is created. It has a single `{}` rule that covers all the requests:

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: deny-all
    namespace: foo
  spec:
    action: DENY
    rules:
    - {}
  ```

* Implicitly. Here, the (default) ALLOW policy is created that does not have any rules. Thus, no requests will match it, and the policy will deny all of them.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: deny-all
    namespace: foo
  spec: {}
  ```

### Deny requests from the foo NS only

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: deny-from-ns-foo
 namespace: myns
spec:
 action: DENY
 rules:
 - from:
   - source:
       namespaces: ["foo"]
```

### Allow requests for the foo NS only

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-intra-namespace-only
 namespace: foo
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       namespaces: ["foo"]
```

### Allow requests from anywhere in the cluster

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-my-cluster
 namespace: myns
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       principals: ["mycluster.local/*"]
```

### Allow any requests for foo or bar clusters

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-foo-or-bar-clusters-to-ns-baz
 namespace: baz
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       principals: ["foo.local/*", "bar.local/*"]
```

### Allow any requests from foo or bar clusters where the namespace is baz

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-foo-or-bar-clusters-to-ns-baz
 namespace: baz
spec:
 action: ALLOW
 rules:
 - from:
   - source: # Logical conjunction is used for the rules below.
       namespaces: ["baz"]
       principals: ["foo.local/*", "bar.local/*"]
```

### Allow from any cluster (via mTLS)

> **Caution!** The denying rules (if they exist) have priority over any other rules. See the [algorithm](#decision-making-algorithm).

Example:

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-any-cluster-with-mtls
 namespace: myns
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       principals: ["*"] # To force the mTLS usage.
```

### Allow all requests from anywhere (including no mTLS - plain text traffic)

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-any
 namespace: myns
spec:
 action: ALLOW
 rules: [{}]
```

## Setting up federation for two clusters using the IstioFederation CR

> Available in Enterprise Edition only.

Cluster A:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: cluster-b
spec:
  metadataEndpoint: https://istio.k8s-b.example.com/metadata/
  trustDomain: cluster-b.local
```

Cluster B:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: cluster-a
spec:
  metadataEndpoint: https://istio.k8s-a.example.com/metadata/
  trustDomain: cluster-a.local
```

## Setting up multicluster for two clusters using the IstioMulticluster CR

> Available in Enterprise Edition only.

Cluster A:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: cluster-b
spec:
  metadataEndpoint: https://istio.k8s-b.example.com/metadata/
```

Cluster B:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: cluster-a
spec:
  metadataEndpoint: https://istio.k8s-a.example.com/metadata/
```

## Control the data-plane behavior

### Prevent istio-proxy from terminating before the main application's connections are closed

By default, during termination, all containers in a Pod, including istio-proxy one, receive SIGTERM signal simultaneously. But some applications need time to properly handle the termination and sometimes they need to do some network requests. It isn't possible when the istio-proxy stops before the application do. The solution is to add a preStop hook which evaluates the application's activity via discovering application's network sockets and let the sidecar stop when they aren't in the network namespace.

The annotation below adds the preStop hook to istio-proxy container in application's Pod:

```yaml
annotations:
  inject.istio.io/templates: "sidecar,d8-hold-istio-proxy-termination-until-application-stops"
```

## `CNIPlugin` Application traffic redirection mode restrictions

Unlike the `InitContainer` mode, the redirection setting is done at the moment of Pod creating, not at the moment of triggering the `istio-init` init-container. This means that application init-containers will not be able to interact with other services because all traffic will be redirected to the `istio-proxy` sidecar, which is not yet running. Workarounds:

* Run the application init container from the user with uid `1337`. Requests from this user are not intercepted under Istio control.
* Exclude an service IP address or port from Istio control using the `traffic.sidecar.istio.io/excludeOutboundIPRanges` or `traffic.sidecar.istio.io/excludeOutboundPorts` annotations.

## Upgrading Istio

### Upgrading Istio control-plane

* Deckhouse allows you to install different control-plane versions simultaneously:
  * A single global version to handle namespaces or Pods with indifferent version (namespace label `istio-injection: enabled`). It is configured by the [globalVersion](configuration.html#parameters-globalversion) parameter.
  * The other ones are additional, they handle namespaces or Pods with explicitly configured versions (`istio.io/rev: v1x19` label for namespace or Pod). They are configured by the [additionalVersions](configuration.html#parameters-additionalversions) parameter.
* Istio declares backward compatibility between data-plane and control-plane in the range of two minor versions:
![Istio data-plane and control-plane compatibility](https://istio.io/latest/blog/2021/extended-support/extended_support.png)
* Upgrade algorithm (i.e. to `1.19`):
  * Configure additional version in the [additionalVersions](configuration.html#parameters-additionalversions) parameter (`additionalVersions: ["1.19"]`).
  * Wait for the corresponding pod `istiod-v1x19-xxx-yyy` to appear in `d8-istio` namespace.
  * For every application Namespase with istio enabled:
    * Change `istio-injection: enabled` label to `istio.io/rev: v1x19`.
    * Recreate the Pods in namespace (one at a time), simultaneously monitoring the application's workability.
  * Reconfigure `globalVersion` to `1.19` and remove the `additionalVersions` configuration.
  * Make sure, the old `istiod` Pod has gone.
  * Change application namespace labels to `istio-injection: enabled`.

To find all Pods with old Istio revision, execute the following command:

```shell
kubectl get pods -A -o json | jq --arg revision "v1x16" \
  '.items[] | select(.metadata.annotations."sidecar.istio.io/status" // "{}" | fromjson |
   .revision == $revision) | .metadata.namespace + "/" + .metadata.name'
```

### Auto upgrading istio data-plane

> Available in Enterprise Edition only.

To automate istio-sidecar upgrading, set a label `istio.deckhouse.io/auto-upgrade="true"` on the application `Namespace` or on the individual resources — `Deployment`, `DaemonSet` or `StatefulSet`.
