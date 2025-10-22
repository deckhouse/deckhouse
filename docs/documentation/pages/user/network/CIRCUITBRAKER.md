---
title: "Circuit Breaker"
permalink: en/user/network/circuit-breaker.html
---

In Deckhouse Kubernetes Platform, the Circuit Breaker mechanism is implemented
using Istio (the [istio](/modules/istio/) module) and provides the following capabilities:

- Temporarily exclude an endpoint from load balancing if the error limit is exceeded.
- Configure limits on the number of TCP connections and the number of requests to a single endpoint.
- Detect stuck requests and terminate them with an error code (HTTP request timeout).

## Example Circuit Breaker configuration

To detect problematic endpoints, use the `outlierDetection` settings
in the [DestinationRule](../network/managing_request_between_service_istio.html#destinationrule-resource) custom resource.
The Outlier Detection algorithm is described in more detail in the [Envoy documentation](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/outlier).

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
        maxConnections: 100 # Maximum number of connections to the host, total across all endpoints.
      http:
        maxRequestsPerConnection: 10 # The connection will be recreated after every 10 requests.
    outlierDetection:
      consecutive5xxErrors: 7 # Allows up to 7 errors (including `5xx`, TCP timeouts, and HTTP timeouts)
      interval: 5m            # within 5 minutes,
      baseEjectionTime: 15m   # after which the endpoint will be removed from load balancing for 15 minutes.
```

You can also use the [VirtualService](../network/retry_istio.html#virtualservice-resource) resource to configure HTTP timeouts.
These timeouts are also taken into account when calculating endpoint error statistics.

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
