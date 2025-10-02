---
title: "Locality failover with Istio"
permalink: en/user/network/locality_failover_istio.html
---

In Deckhouse Kubernetes Platform,
you can implement the Locality failover mechanism using the [`istio`](/modules/istio/) module.
Before configuring the mechanism, make sure the module is enabled in the cluster.

The Locality failover mechanism manages traffic routing
and directs it to a priority failover in case certain service instances become unavailable.

{% alert level="info" %}
If necessary, refer to the [Locality failover documentation](https://istio.io/latest/docs/tasks/traffic-management/locality-load-balancing/failover/).
{% endalert %}

With Istio, you can configure priority-based geographic failover between endpoints.
Zones are determined based on node labels in the following hierarchy:

- `topology.istio.io/subzone`
- `topology.kubernetes.io/zone`
- `topology.kubernetes.io/region`

This is useful for inter-cluster failover when used together with multicluster configurations.

{% alert level="warning" %}
To enable Locality Failover,
use the [DestinationRule](../network/managing_request_between_service_istio.html#destinationrule-resource) resource,
which must also include the `outlierDetection` parameter.
{% endalert %}

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
        enabled: true # Enable LF.
    outlierDetection: # Required.
      consecutive5xxErrors: 1
      interval: 1s
      baseEjectionTime: 1m
```
