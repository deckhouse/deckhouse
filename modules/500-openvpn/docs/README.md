---
title: "The openvpn module"
webIfaces:
- name: openvpn-admin
---

The openvpn module allows peers to authenticate each other using certificates and provides a simple web interface.

Using the web interface, you can:
- issue certificates;
- revoke certificates;
- cancel certificate revocation;
- get a ready-to-use custom configuration file.

The web interface is integrated with the [user-authn](../150-user-authn/) module, allowing you to manage user access to this web interface.

## Exposing the VPN service

Generally, one or more external IP addresses are used for a connection. The following connection methods are supported:
- External IP-based (`ExternalIP`) — when there are nodes with public IPs.
- LoadBalancer-based (`LoadBalancer`) — AWS, Google Cloud and other LBs are supported.
- `Direct` — for non-standard cases; this method allows you to manually configure the way traffic is routed from the cluster entry to the OpenVPN Pod.

## Available cluster resources after connecting to the VPN

The following parameters are pushed to the user's computer after connecting to the VPN:
- the `kube-dns` address is added to the client's DNS servers to allow direct access to Kubernetes services via FQDN;
- a route to the local network;
- a route to the cluster service network;
- a route to the Pod network.

## User's traffic audit

The module can log user's activity via VPN in JSON format. Traffic is grouped
by `src_ip, dst_ip, src_port, dst_port, ip_proto` fields. Container logs can be collected and sent to storage for further audit
using the [log-shipper](../460-log-shipper/) module.
