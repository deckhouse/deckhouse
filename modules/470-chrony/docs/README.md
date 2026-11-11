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
d8 k exec -it -n d8-chrony ds/chrony-master -- chronyc -N sources
```

Ouput example:

```text
MS Name/IP address         Stratum Poll Reach LastRx Last sample
===============================================================================
^* pool.ntp.org.                 2  10   377   171   -502us[ -909us] +/- 5388us
^- pool.ntp.org.                 2  10   377   666  -5317us[-5698us] +/-  103ms
^+ pool.ntp.org.                 2  10   377   938   -201us[ -567us] +/- 5346us
^+ pool.ntp.org.                 2  10   377   843   -159us[ -530us] +/-   12ms
```

{% alert level="info" %}  
`^+` - combined NTP server (`chrony` combines information from `combined` servers to reduce inaccuracies);  
`^*` - current NTP server;  
`^-` - non-combinable NTP server.    
Extra flag `-v` adds information about meanings of the columns: `chronyc sources -v`.  
{% endalert %}

`chrony` agents on master nodes and on other nodes have one main difference - on all nodes that are not masters, the list of NTP servers contains not only NTP servers from `module config`, but also the addresses of all master nodes of the cluster.  

Thus, agents on master nodes synchronize time only from the list of hosts specified in `module config` (by default from `pool.ntp.org`). And agents on other nodes synchronize time with the list of NTP servers from `module config` plus with `chrony` agents on master nodes.  

This is done so that in case of unavailability of NTP servers specified in `module config`, the time is synchronized with the master nodes.

## Boot-time synchronization on master nodes

On master nodes, the module also sets the clock once during boot, before `kubelet` starts.
This keeps control plane static Pods from starting with a wrong time taken from the real-time clock (RTC) or the hypervisor.

If most of the NTP servers that respond agree on the time within 1 second, `kubelet` starts right after the clock is set.
If no NTP server responds, or the servers that respond disagree, `kubelet` starts with an unverified clock after up to 30 seconds. On nodes managed by bashible, the limit can be raised to 60 seconds using [`earlyTimeSync.timeoutSeconds`](configuration.html#parameters-earlytimesync-timeoutseconds).
On nodes managed by bashible, the time spent waiting for the node network is not included.
On these nodes, systemd waits for `nodelet.service` to report readiness after the check. The check runs in the main nodelet process.
On Deckhouse Engine nodes, the wait starts when nodelet starts and can include the wait for the node network.
Boot-time synchronization can be turned off with the [`earlyTimeSync.enabled`](configuration.html#parameters-earlytimesync-enabled) parameter.

Boot-time synchronization uses the node network and the node DNS settings.
Kubernetes DNS and NTP servers running inside the cluster are not available at this point.

If a master node with boot-time synchronization configured cannot reach the NTP servers, the check delays `kubelet` up to its limit. The alert fires once monitoring collects the failed check.
This applies to an air-gapped cluster that keeps the default `pool.ntp.org` server.
In that case, set [`ntpServers`](configuration.html#parameters-ntpservers) to NTP servers that the master nodes can reach, or disable boot-time synchronization.

Enabling boot-time synchronization or changing its parameters takes effect on the next boot.
The clock on a running node is not changed immediately, because stepping it back on a live control plane can disrupt authentication and leader election.
Disabling boot-time synchronization with `earlyTimeSync.enabled: false` takes effect as soon as the configuration is applied on the master nodes.

If the clock was not synchronized during boot, the [`NodeTimeNotSynchronizedAtBoot`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#chrony-nodetimenotsynchronizedatboot) alert fires for the master node.
The alert stays active until one of the following happens:

- The node boots again and the synchronization succeeds.
- Boot-time synchronization is disabled. On Deckhouse Engine nodes, the alert then clears on the next boot.
- You clear the alert as described in the alert description.

The alert works only when the [`monitoring-kubernetes`](/modules/monitoring-kubernetes/) module is enabled, because that module collects the result from the node.

### Limitations

Boot-time synchronization has the following limitations:

- On Deckhouse Engine nodes, `earlyTimeSync.timeoutSeconds` does not apply: `kubelet` waits for 30 seconds at most.
  Only the first 16 host names and IPv4 addresses of `ntpServers` are used there.
  When a master node boots into a new OS image, this wait counts toward the 15 minutes the node has to confirm the image.
  If the NTP servers are unreachable, set reachable servers in `ntpServers` or disable boot-time synchronization.
- Values of `ntpServers` other than a host name or an IPv4 address, such as an IPv6 address or a value with a port, are skipped.
- It does not cover the first boot of a new master node, including the cluster bootstrap.
  The configuration reaches the node during that boot, after `kubelet` has already started.
- On nodes managed by bashible, disabling the module leaves the boot-time synchronization files on the nodes.
  Deckhouse Engine nodes stop the synchronization on their own from the next boot.
  Before disabling the module, set `earlyTimeSync.enabled` to `false` and wait until the configuration is applied on the master nodes.
  If the module is already disabled, remove the files with a NodeGroupConfiguration as shown in the [FAQ](faq.html#how-do-i-disable-chrony-and-use-ntp-daemon-on-nodes).
