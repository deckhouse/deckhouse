---
title: "Canary deployment"
permalink: en/admin/network/canary-deployment.html
---

In Deckhouse Kubernetes Platform Canary deployment is implemented by Istio tools (module [`istio`](../#)).

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/istio/examples.html#canary-->

## Examples of Canary deployment settings

> Istio is only responsible for flexible request routing that relies on special request headers (such as cookies) or simply randomness. The CI/CD system is responsible for customizing this routing and "switching" between canary versions.

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
