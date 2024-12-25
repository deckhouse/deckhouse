# Patches

## 1-readd-tuntap-to-default-device-rules.patch

Re-add tun/tap to default device rules.

Since runc v1.2.0 was released, a number of users complained that the removal
of tun/tap device access from the default device ruleset is causing a
regression in their workloads. In our case, we received an error when starting openvpn:
```
2024/12/23 12:28:27 open /dev/net/tun: operation not permitted 
```
Needs to be removed after upgrading to runc 1.2.4:
https://github.com/opencontainers/runc/pull/4556


