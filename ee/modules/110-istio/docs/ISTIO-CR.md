---
title: "The istio module: Custom Resources (by istio.io)"
---

## Routing

### DestinationRule

[Reference](https://istio.io/latest/docs/reference/config/networking/destination-rule/).

Allows you to:
* Define a strategy for balancing traffic between service endpoints:
  * The balancing algorithm to use (LEAST_CONN, ROUND_ROBIN, ...);
  * Symptoms that an endpoint is unhealthy and rules for removing it from load balancing;
  * Limits for TCP connections and endpoint requests;
  * Sticky Sessions;
  * Circuit Breaker;
* Define alternative endpoint groups for traffic processing (suitable for Canary Deployments). Also, you can configure balancing strategies for each group;
* Configuring tls for outgoing requests.

### VirtualService

[Reference](https://istio.io/latest/docs/reference/config/networking/virtual-service/).

Using VirtualService is optional; regular services fit just fine if their capabilities match your requirements.

Allows you to configure request routing:
* Arguments on which routing decisions are based on:
  * Host;
  * uri;
  * Weight;
* Parameters of the resulting directions:
  * The new host;
  * The new uri;
  * If the host is defined using [DestinationRule](#destinationrule), then requests can be sent to subsets;
  * Timeout and retry settings.

> **Caution!** Istio must be aware of the `destination`; if you use an external API, register it via [ServiceEntry](#serviceentry).

### ServiceEntry

[Reference](https://istio.io/latest/docs/reference/config/networking/service-entry/).

It is similar to Endpoints + Service in vanilla Kubernetes. Informs Istio about the existence of an external service and lets you redefine its address.

## Authentication

Answers the question "Who has made the request?" Not to be confused with authorization - the latter determines what the authenticated subject can or cannot do.

There are two authentication methods:
* mTLS;
* JWT tokens;

### PeerAuthentication

[Reference](https://istio.io/latest/docs/reference/config/security/peer_authentication/).

Allows you to define the mTLS strategy for an individual NS. Defines how traffic will be tunneled (or not) to the sidecar. Each mTLS request can automatically identify the source and allows you to use it in the authorization rules.

### RequestAuthentication

[Reference](https://istio.io/latest/docs/reference/config/security/request_authentication/).

Allows you to configure JWT authentication for requests.

## Authorization

**Caution!** Authorization without the use of mTLS or JWT authentication will not work fully. In this case, you will be able to use only basic arguments, such as source.ip and request.headers, for defining policies.

### AuthorizationPolicy

[Reference](https://istio.io/latest/docs/reference/config/security/authorization-policy/).

Enables and defines access control to the workload. The AuthorizationPolicy CR supports both ALLOW and DENY rules. The following decision-making algorithm is used if at least one policy is defined for a workload:

* Reject the request if there is a DENY policy for it;
* Allow the request if there is no ALLOW policy for it;
* Allow the request if there is ALLOW policy for it;
* Deny the request.

The following arguments are used in the decision-making algorithm:
* source:
  * namespace
  * principal (i.e., the ID the user has received after authentication)
  * ip
* destination:
  * method (GET, POST, ...)
  * Host
  * port
  * uri

### Sidecar

[Reference](https://istio.io/latest/docs/reference/config/networking/sidecar/)

This resource limits the number of services for which information is transmitted to the istio-proxy sidecar.
