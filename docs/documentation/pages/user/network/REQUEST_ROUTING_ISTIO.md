---
title: "Request routing with Istio"
permalink: en/user/network/request_routing_istio.html
---

You can use the [`istio`](/modules/istio/) module in Deckhouse Kubernetes Platform to route HTTP and TCP requests.

The main resource for managing routing is [VirtualService](#virtualservice-resource) from Istio, which allows you to configure routing for HTTP or TCP requests.

## VirtualService resource

For more details on VirtualService, refer to the [Istio documentation](https://istio.io/v1.19/docs/reference/config/networking/virtual-service/).

Using VirtualService is optional. Standard Services will continue to work if their functionality is sufficient. With this resource, you can configure request routing:

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
For `destination` in Istio to work correctly, it must be specified explicitly. If you are using an external API, specify it with a [ServiceEntry](/modules/istio/istio-cr.html#serviceentry).
{% endalert %}
