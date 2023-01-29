---
title: "Cloud provider — Yandex Cloud: FAQ"
---

## How do I set up the INTERNAL LoadBalancer?

Attach the following annotation to the service:

```yaml
yandex.cpi.flant.com/listener-subnet-id: SubnetID
```

The annotation links the LoadBalancer with the appropriate Subnet.

## How to reserve a public IP address?

This on is used in `externalIPAddresses` and `natInstanceExternalAddress`.

```shell
$ yc vpc address create --external-ipv4 zone=ru-central1-a
id: e9b4cfmmnc1mhgij75n7
folder_id: b1gog0h9k05lhqe5d88l
created_at: "2020-09-01T09:29:33Z"
external_ipv4_address:
  address: 178.154.226.159
  zone_id: ru-central1-a
  requirements: {}
reserved: true
```

## dhcpOptions-related problems and ways to address them

Using DNS servers that differ from those provided by Yandex Cloud in the DHCP settings is a temporary solution. It will be abandoned after Yandex Cloud will introduce the Managed DNS service. To get around the restrictions described below, we recommend using `stubZones` from the [`kube-dns`](../042-kube-dns/) module.

### Editing parameters

Pay attention to the following nuances:

1. When changing parameters, you need to invoke `netplan apply` or a similar command that forces the update of the DHCP lease.
2. You will need to restart all hostNetwork Pods (especially `kube-dns`) for the new `resolv.conf` settings to take effect.

### Aspects of the use

If the dhcpOptions parameter is set, all DNS are routed to the DNS servers specified. These DNS servers **must** serve DNS requests to the Internet and (if needed) resolve intranet resources.

**Do not use** this option if the recursive DNSs specified cannot resolve the same list of zones that the recursive DNSs in the Yandex Cloud subnet can resolve.
