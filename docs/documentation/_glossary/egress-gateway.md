---
title: Egress gateway
description: Centralized management and control of outgoing traffic from the cluster, including encryption, routing and monitoring.
lang: en
---

The egress gateway is provided by the [`cni-cilium`](/modules/cni-cilium/) module and is configured with two custom resources. [EgressGateway](/modules/cni-cilium/cr.html#egressgateway) describes a group of nodes, one of which is selected as the active gateway while the others stay in standby mode. [EgressGatewayPolicy](/modules/cni-cilium/cr.html#egressgatewaypolicy) describes which traffic of cluster pods is routed through that gateway.

Two modes are available: the basic mode with pre-configured IP addresses on the egress nodes and the Virtual IP mode, in which additional IP addresses are assigned to nodes dynamically. For details, see [Egress Gateway configuration](admin/configuration/network/egress/gateway.html).
