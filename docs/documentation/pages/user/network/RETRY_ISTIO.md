---
title: "Configuring request retries with Istio"
description: "Configuring request retries for HTTP requests using Istio in Deckhouse Kubernetes Platform. VirtualService configuration with retry parameters, attempts, perTryTimeout."
permalink: en/user/network/retry_istio.html
---

You can use the [istio](/modules/istio/) module to configure request retries.
Before configuring retries, make sure the module is enabled in the cluster.

To configure request retries, use the [VirtualService](#virtualservice-resource) resource from Istio.

{% alert level="warning" %}
By default, when errors occur, all requests (including POST requests) are retried up to three times.
{% endalert %}

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

## VirtualService resource

If necessary, refer to the [VirtualService documentation](https://istio.io/v1.21/docs/reference/config/networking/virtual-service/).

Using VirtualService is optional. Standard Services will continue to work if their functionality is sufficient.
With this resource, you can configure request routing:

- Arguments for making routing decisions:
  - `host`
  - `uri`
  - `weight`
- Parameters of the final destinations:
  - New `host`
  - New `uri`
  - If the `host` is defined via a [DestinationRule](../network/managing_request_between_service_istio.html#destinationrule-resource), requests can be routed to subsets
  - Timeout and retry settings

{% alert level="warning" %}
For `destination` in Istio to work correctly, it must be specified explicitly.
If you are using an external API, specify it with a [ServiceEntry](/modules/istio/istio-cr.html#serviceentry).
{% endalert %}
