---
title: "Managing request balancing between service endpoints with Istio"
permalink: en/user/network/managing_request_between_service_istio.html
---

You can use the [istio](/modules/istio/) module to manage request load balancing between service endpoints.
Before configuring load balancing, make sure the module is enabled in the cluster.

The main resource for managing request load balancing is [DestinationRule](#destinationrule-resource) from Istio.
This resource allows you to configure the following parameters:

- TCP limits and timeouts.
- Algorithms for load balancing between endpoints.
- Rules for detecting endpoint issues and removing them from the load balancer.
- Encryption settings.

{% alert level="warning" %}
All configured limits apply separately to each client Pod.
For example, if you set a limit of one TCP connection for a Service and there are three client Pods,
the Service will receive three incoming connections.
{% endalert %}

## DestinationRule resource

For more details on DestinationRule, refer to the [Istio documentation](https://istio.io/v1.19/docs/reference/config/networking/destination-rule/).
Use this resource to:

- Define a traffic load balancing strategy between service endpoints:
  - Load balancing algorithm (`LEAST_CONN`, `ROUND_ROBIN`, etc.).
  - Detection and exclusion of unhealthy endpoints.
  - TCP connection and request limits for endpoints.
  - Sticky Sessions support.
  - Circuit Breaker configuration.
- Define alternative endpoint groups for handling traffic (applicable to Canary Deployments).
  Each group can have its own load balancing strategy.
- Configure TLS for outgoing requests.
