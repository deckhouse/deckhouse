---
title: "Cloud provider — Yandex Cloud: examples"
---

Below is an example of the Yandex Cloud cloud provider configuration.

## An example of the module configuration

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 1
  enabled: true
  settings:
    additionalExternalNetworkIDs:
    - enp6t4snovl2ko4p15em
```

## An example of the `YandexInstanceClass` custom resource

```yaml
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: test
spec:
  cores: 4
  memory: 8192
```

## LoadBalancer

### Service annotations

Defaults (these settings apply to all Services unless overridden by annotations on a specific Service):

- `YANDEX_CLOUD_DEFAULT_LB_TARGET_GROUP_NETWORK_ID` — required variable. Specifies the NetworkID where the Target Group is created by default.
- `YANDEX_CLOUD_DEFAULT_LB_LISTENER_SUBNET_ID` — sets the default SubnetID for listeners of created NLBs.

> **Warning.** All newly created NLBs are internal by default; this behavior can be overridden with the `yandex.cpi.flant.com/loadbalancer-external` annotation.

The following annotations are supported by Yandex Cloud Controller Manager:

1. `yandex.cpi.flant.com/target-group-network-id` — overrides the default `YANDEX_CLOUD_DEFAULT_LB_TARGET_GROUP_NETWORK_ID` for this Service. Specifies the NetworkID in which the Target Group for the NLB will be created.
1. `yandex.cpi.flant.com/listener-subnet-id` — overrides the default `YANDEX_CLOUD_DEFAULT_LB_LISTENER_SUBNET_ID` for this Service. Sets the SubnetID for the listeners of the created NLB. When this annotation is used, the created NLBs will be internal.
1. `yandex.cpi.flant.com/listener-address-ipv4` — allows you to set a predefined IPv4 address for listeners. Works for both internal and external NLBs.
1. `yandex.cpi.flant.com/loadbalancer-external` — overrides the default behavior (all new NLBs are internal). Enables creation of an external NLB for this Service.
1. `yandex.cpi.flant.com/target-group-name-prefix` — sets the Target Group name prefix in the format `<annotation value><Yandex cluster name><NetworkID>` (for a Service). The same annotation can be set on a node to include the node into a non-default Target Group (Target Groups will be created with names `<annotation value><Yandex cluster name><network id of the instance interfaces>`).

If separate Target Groups are created for the control plane or master nodes, add the label `node.kubernetes.io/exclude-from-external-load-balancers: ""` to the master nodes. This prevents the controller from trying to automatically add master nodes to new Target Groups for load balancers.

If you create your own load balancer for master nodes and want YCC to also be able to attach its own load balancers to the masters, pre-create a Target Group with a name matching the pattern `${CLUSTER-NAME}${VPC.ID}`.

### Target Group healthchecks

Healthcheck parameters (for created NLB Target Groups):

1. `yandex.cpi.flant.com/healthcheck-interval-seconds` — how often to run the check, in seconds (default 2).
1. `yandex.cpi.flant.com/healthcheck-timeout-seconds` — how long to wait for an endpoint response, in seconds. If no response is received within this time, the check is considered failed (default 1).
1. `yandex.cpi.flant.com/healthcheck-unhealthy-threshold` — how many consecutive failed checks are required to mark an endpoint as unhealthy and exclude it from load balancing (default 2).
1. `yandex.cpi.flant.com/healthcheck-healthy-threshold` — how many consecutive successful checks are required to mark an endpoint as healthy and include it back into load balancing (default 2).
