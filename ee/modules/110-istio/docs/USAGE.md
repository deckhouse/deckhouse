---
title: "The istio module: usage"
---

## Resource examples

### IstioFederation
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: example-cluster
spec:
  metadataEndpoint: https://istio.k8s.example.com/metadata/
  trustDomain: example.local
```

### IstioMulticluster
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: example-cluster
spec:
  metadataEndpoint: https://istio.k8s.example.com/metadata/
```

## Enabling load balancing for the `ratings.prod.svc.cluster.local` service

Here is how you can enable smart load balancing for the `myservice` service that was previously load balanced via iptables:

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: myservice-lb
  namespace: prod
spec:
  host: myservice.prod.svc.cluster.local
  trafficPolicy:
    loadBalancer:
      simple: LEAST_CONN
```

## Adding additional secondary subsets with their own rules to the myservice.prod.svc service

[VirtualService](istio-cr.html#virtualservice)  must be enabled to use these subsets:
```yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: myservice-extra-subsets
spec:
  host: myservice.prod.svc.cluster.local
  trafficPolicy: # works if only the regular Service is defined.
    loadBalancer:
      simple: LEAST_CONN
  subsets: # subsets must be declared via VirtualService by specifying them in routes.
  - name: testv1
    labels: # the same as selector for the Service. Pods with these labels will be covered by this subset.
      version: v1
  - name: testv3
    labels:
      version: v3
    trafficPolicy:
      loadBalancer:
        simple: ROUND_ROBIN
```

## Circuit Breaker

The [DestinationRule](istio-cr.html#destinationrule) CR is used to define the circuit breaker service.

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: myservice-circuit-breaker
spec:
  host: myservice.prod.svc.cluster.local # either FQDN or a domain local for the namespace.
  trafficPolicy:
    outlierDetection:
      consecutiveErrors: 7 # only seven consecutive errors are allowed
      interval: 5m # over a period of 5 munites
      baseEjectionTime: 15m # the problem endpoint will be excluded from operation for 15 minutes.
```

## Retry

The [VirtualService](istio-cr.html#virtualservice) CR is used to define the service..

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: productpage-retry
spec:
  hosts:
    - productpage # either FQDN or a domain local for the namespace.
  http:
  - route:
    - destination:
        host: productpage # At least one destination or rewrite is required. In this example, we do not change the destination.
    timeout: 8s
    retries:
      attempts: 3
      perTryTimeout: 3s
```

## Canary

Suppose, we have two Deployments with the different versions of the application in the same namespace. Pods of different application versions have different labels (`version: v1` & `version: v2`) attached.

You need to configure two CRs:
* [DestinationRule](istio-cr.html#destinationrule) that describes how to identify different versions of the application, and
* [VirtualService](istio-cr.html#virtualservice) that describes how to balance traffic between different versions of the application.

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: productpage-canary
spec:
  host: productpage
  subsets: # subsets works only if the VirtualService (that has these subsets specified in routes) is used to connect to the host.
  - name: v1
    labels: # similar as the Service's selectors. Pods with these labels will be covered by this subset.
      version: v1
  - name: v2
    labels:
      version: v2
```
```yaml
apiVersion: networking.istio.io/v1alpha3
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
        subset: v1 # reference to the subset defined in DestinationRule 
      weight: 90 # the percentage of traffic to send to pods labeled version: v1.
    - destination:
        host: productpage
        subset: v2
      weight: 10
```


### Load balancing between services with different versions (Canary Deployment)
```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: reviews-route
spec:
  hosts:
  - reviews.prod.svc.cluster.local
  http:
  - route:
    - destination:
        host: reviews.prod.svc.cluster.local
        subset: testv1 # reference to the subset define in DestinationRule
      weight: 25
  - route:
    - destination:
        host: reviews.prod.svc.cluster.local
        subset: testv3
      weight: 75
```

##### Rerouting the /uploads location to another service:
```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: uploads-route
spec:
  hosts:
  - gallery.prod.svc.cluster.local
  http:
  - match:
    - uri:
        prefix: "/uploads" # if the client wants to go to gallery.prod.svc.cluster.local/uploads/a.jpg
    rewrite:
      uri: "/data" # then replace the uri with /data/a.jpg
    route:
    - destination:
        host: share.prod.svc.cluster.local # and forward it to share.prod.svc.cluster.local/data/a.jpg
  - route:
    - destination:
        host: gallery.prod.svc.cluster.local # all other requests remain untouched 
```

## Ingress

To use Ingress, you need to:
* Configure the Ingress controller by adding Istio sidecar to it. In our case, you need to enable the `enableIstioSidecar` parameter in the [ingress-nginx](../../modules/402-ingress-nginx/) module's IngressNginxController CR.
* Set up an Ingress that refers to the Service. The following annotations are mandatory for Ingress:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — using this annotation, the ingress-controller sends requests to a single ClusterIP (from Service CIDR) while envoy load balances them. Ingress-controller's sidecar is only catching traffic directed to Service CIDR.
  * `nginx.ingress.kubernetes.io/upstream-vhost: myservice.myns.svc` — using this annotation, the sidecar can identify the application service that serves requests.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: productpage
  namespace: bookinfo
  annotations:
    nginx.ingress.kubernetes.io/service-upstream: "true" # nginx proxies traffic to the ClusterIP instead of pods' own IPs.
    nginx.ingress.kubernetes.io/upstream-vhost: productpage.bookinfo.svc # In Istio, all routing is carried out based on the `Host:` headers. Instead of letting Istio know about the `productpage.example.com` external domain, we use the internal domain of which Istio is aware.
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

**Caution!** The following algorithm for deciding the fate of a request becomes active after AuthorizationPolicy is created for the application:
* The request is denied if it falls under the DENY policy;
* The request is allowed if there are no ALLOW policies for the application;
* The request is allowed if it falls under the ALLOW policy.
* All other requests are denied.

In other words, if you explicitly deny something, then only this restrictive rule will work. If you explicitly allow something, only explicitly authorized requests will be allowed (however, restrictions will stay in force and have precedence).

**Caution!** The policies based on high-level parameters like namespace or principal require enabling Istio for all involved applications. Also, there must be organized Mutual TLS between applications, by default it is, due module configuration parameter `tlsMode: MutualPermissive`.

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
    action: ALLOW # the default value, can be skipped
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
    action: ALLOW # the default value, can be skipped
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

### Deny all for the foo NS

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
   - source: # Logical conjunction is used for the rules below
       namespaces: ["baz"]
       principals: ["foo.local/*", "bar.local/*"]
```

### Allow from any cluster (via mtls)

**Caution!** The denying rules (if they exist) have priority over any other rules. See the [algorithm](#decision-making-algorithm).

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
       principals: ["*"] # to force the MTLS usage
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
