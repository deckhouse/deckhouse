---
title: "The openvpn module"
description: "Access to Deckhouse Kubernetes Platform cluster resources via OpenVPN with certificate-based authentication."
webIfaces:
- name: openvpn-admin
---

The openvpn module allows peers to authenticate each other using certificates and provides a simple web interface.

Using the web interface, you can:

- Issue certificates.
- Revoke certificates.
- Cancel certificate revocation.
- Get a ready-to-use custom configuration file.

Integrating with the [user-authn](/modules/user-authn/) module lets you manage user access to the web interface.

## Exposing the VPN service

1. Select one or more external IP addresses for connection.
1. Use one of the connection methods:
   - By external IP address (`ExternalIP`) - if there are nodes with public IP addresses.
   - Using `LoadBalancer` - for all cloud providers and their placement schemes that support LoadBalancer ordering.
   - `Direct` - configure the traffic path manually: from the entry point to the cluster to the pod with OpenVPN.

## Available cluster resources after connecting to the VPN

The following parameters are pushed to the user's computer after connecting to the VPN:

- The `kube-dns` address is added to the client's DNS servers to allow direct access to Kubernetes services via FQDN.
- A route to the local network.
- A route to the cluster service network.
- A route to the Pod network.

## User's traffic audit

The module can log user's activity via VPN in JSON format.
Traffic is grouped by `src_ip`, `dst_ip`, `src_port`, `dst_port`, `ip_proto` fields.
Using the [log-shipper](/modules/log-shipper/) module, container logs can be collected and stored for later auditing.
