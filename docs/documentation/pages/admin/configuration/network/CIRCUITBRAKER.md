---
title: "Circuit Breaker"
permalink: en/admin/network/circuit-breaker.html
---

In Deckhouse Kubernetes Platform Circuit Breaker is implemented with Istio tools (module [`istio`](../#)) and provides the following features:

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/istio/#what-issues-does-istio-help-to-resolve -->

* Temporarily excluding endpoints from balancing if the error limit is exceeded.
* Setting limits on the number of TCP connections and the number of requests per endpoint.
* Detecting abnormal requests and terminating them with an error code (HTTP request timeout).

## Example of Circuit Breaker configuration

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/istio/examples.html#circuit-breaker -->

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

