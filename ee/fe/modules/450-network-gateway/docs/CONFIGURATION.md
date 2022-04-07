---
title: "The network-gateway module: configuration"
---

This module is **disabled** by default. To enable it, add the following lines to the `deckhouse` ConfigMap:

```yaml
data:
  networkGatewayEnabled: "true"
```

## Parameters

The module has the following parameters in the `deckhouse` ConfigMap:

* `nodeSelector` — selects nodes that will be used to configure iptables rules and to run the DHCP server;
    * Format — a [dictionary](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector) of label-value pairs. The instance's pods inherit this field as-is;
    * A mandatory parameter;
* `tolerations` — tolerations for DHCP pods and iptables managers;
    * Format — a [regular list](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/) of tolerations. The instance's pods inherit this field as-is;
    * An optional parameter;
* `subnet` — the address of a local subnet that our gateway serves;
    * Format — IP/Prefix; example: 192.168.42.0/24;
    * The DHCP options to pass to clients are generated based on this address:
        * Address pool — numbers starting with 50 and up to the last one;
        * Router — the subnet's first address;
    * A mandatory parameter;
* `publicAddress` — replaces the src of the packets outgoing from the LAN;
    * A mandatory parameter;
* `disableDHCP` — a bool parameter; disables the DHCP server;
    * By default, the DHCP server is enabled (`false`); 
* `dns` — optional settings to pass to clients via DHCP:
    * `servers` — an array of DNS servers;
        * Example: ['4.2.2.2', '8.8.8.8'];
    * `search` — an array of search domains;
        * Example: ['office.example.com', 'srv.example.com']
* `storageClass` — the name of the storage class to use for storing the DHCP lease;
    * If omitted, either `global.storageClass` or `global.discovery.defaultStorageClass` is used. If the latter two are not specified, the emptyDir is used for storing the data. Dnsmasq (underlies our DHCP server) has its own mechanisms for protecting against the duplication of IP addresses if the lease database is lost (but it is better not to lose it).
