---
title: "The chrony module"
description: "Time synchronization on all cluster nodes."
---

Provides time synchronization on all cluster nodes using the [chrony](https://chrony.tuxfamily.org/) utility.

## How it works

The module starts `chrony` agents on all cluster nodes.

By default, the NTP server `pool.ntp.org` is used. The NTP server can be changed via the [settings](/modules/chrony/configuration.html) module.
To view the NTP servers used, you can use the command:

```bash
d8 k exec -it -n d8-chrony chrony-master-r7v6c -- chronyc -N sources
Defaulted container "chrony" out of: chrony, chrony-exporter, kube-rbac-proxy
MS Name/IP address         Stratum Poll Reach LastRx Last sample
===============================================================================
^* pool.ntp.org.                 2  10   377   171   -502us[ -909us] +/- 5388us
^- pool.ntp.org.                 2  10   377   666  -5317us[-5698us] +/-  103ms
^+ pool.ntp.org.                 2  10   377   938   -201us[ -567us] +/- 5346us
^+ pool.ntp.org.                 2  10   377   843   -159us[ -530us] +/-   12ms
```

`^+` - combined NTP server (`chrony` combines information from `combined` servers to reduce inaccuracies);  
`^*` - current NTP server;  
`^-` - non-combinable NTP server.

`chrony` agents on master nodes and on other nodes have one main difference - on all nodes that are not masters, the list of NTP servers contains not only NTP servers from `module config`, but also the addresses of all master nodes of the cluster.  

Thus, agents on master nodes synchronize time only from the list of hosts specified in `module config` (by default from `pool.ntp.org`). And agents on other nodes synchronize time with the list of NTP servers from `module config` plus with `chrony` agents on master nodes.  

This is done so that in case of unavailability of NTP servers specified in `module config`, the time is synchronized with the master nodes.
