---
title: "The openvpn module"
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
