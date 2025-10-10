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

Default values are configured in the cluster for placing load balancer resources (the network for the Target Group and the subnet for the Listener). These values are set automatically during cluster setup and can be overridden with annotations at the individual Service level.

> The default behavior (external or internal LB) depends on the cluster configuration. To explicitly choose the type, use the `yandex.cpi.flant.com/loadbalancer-external` annotation.

The following annotations are supported by Yandex Cloud Controller Manager:

1. `yandex.cpi.flant.com/target-group-network-id` — specifies the NetworkID in which the Target Group for this Service will be created. Overrides the corresponding default value.
1. `yandex.cpi.flant.com/listener-subnet-id` — sets the SubnetID for the Listeners of the LB created for this Service. Overrides the corresponding default value.
1. `yandex.cpi.flant.com/listener-address-ipv4` — sets a predefined IPv4 address for the Listeners (supported for both internal and external LBs).
1. `yandex.cpi.flant.com/loadbalancer-external` — enables creation of an external LB for this Service (use it when you need to explicitly create an external load balancer). Overrides the default behavior.
1. `yandex.cpi.flant.com/target-group-name-prefix` — sets a Target Group name prefix in the format `<annotation value><Yandex cluster name><NetworkID>` (for a Service). A similar annotation can be set on a node to include the node in a non-standard Target Group (Target Groups will be created with names `<annotation value><Yandex cluster name><network id of the instance’s interfaces>`).

If separate Target Groups are created for the control plane or master nodes, add the label `node.kubernetes.io/exclude-from-external-load-balancers: ""` to the master nodes. This prevents the controller from automatically adding master nodes to new Target Groups for load balancers.

If you create your own load balancer for master nodes and want YCC to also be able to place its load balancers on master nodes, pre-create a Target Group with a name matching the pattern `${CLUSTER-NAME}${VPC.ID}`.

### Target Group health checks

Health check parameters (for LB Target Groups created by the controller):

1. `yandex.cpi.flant.com/healthcheck-interval-seconds` — how often to run the check, in seconds (default: 2).
1. `yandex.cpi.flant.com/healthcheck-timeout-seconds` — how long to wait for an endpoint response, in seconds. If no response is received within this time, the check is considered failed (default: 1).
1. `yandex.cpi.flant.com/healthcheck-unhealthy-threshold` — how many consecutive failed checks are required to mark an endpoint as unhealthy and exclude it from load balancing (default: 2).
1. `yandex.cpi.flant.com/healthcheck-healthy-threshold` — how many consecutive successful checks are required to return an endpoint to healthy status and include it back in load balancing (default: 2).
