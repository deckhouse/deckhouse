---
title: Sysctl parameters managed by the platform
description: "List of sysctl parameters that DKP configures and maintains on cluster nodes."
permalink: en/reference/sysctl.html
lang: en
---

Deckhouse automatically configures and manages a set of the server's kernel parameters using the `sysctl` utility.
The configured parameters improve network throughput, prevent resource depletion, and optimize memory management.

{% alert level="info" %}
If you modify these parameters, Deckhouse will automatically revert them to the values listed below.
{% endalert %}

| Parameter | Value set by Deckhouse | Description |
| -------- | ----------------------- | ----------- |
| `/sys/block/*/queue/nr_requests` | `256` | Number of queued requests for block devices. |
| `/sys/block/*/queue/read_ahead_kb` | `256` | Amount of extra data that kernel reads from the disk to improve future read performance. |
| `/sys/kernel/mm/transparent_hugepage/enabled` | `never` | Disables Transparent HugePage. |
| `/sys/kernel/mm/transparent_hugepage/defrag` | `never` | Disables the Transparent HugePage defragmentation. |
| `/sys/kernel/mm/transparent_hugepage/use_zero_page` | `0` | Disables usage of huge zero pages. |
| `/sys/kernel/mm/transparent_hugepage/khugepaged/defrag` | `0` | Disables `khugepaged` defragmentation. |
| `/proc/sys/net/ipv4/conf/*/rp_filter` | `0` | Disables reverse path filtering for all interfaces. |
| `fs.file-max` | `1000000` | Maximum number of open files. |
| `fs.inotify.max_user_instances` | `5120` | Maximum number of inotify instances. |
| `fs.inotify.max_user_watches` | `524288` | Maximum number of files monitored by a single inotify instance. |
| `fs.may_detach_mounts` | `1` | Allows lazy unmounting of a file system. |
| `kernel.numa_balancing` | `0` | Disables automatic NUMA memory balancing. |
| `kernel.panic` | `10 (0 if fencing is enabled)` | Time in seconds until the node reboots after it encounters the fatal kernel panic error. By default, it's set to `10`. If [`fencing`](/modules/node-manager/cr.html#nodegroup-v1-spec-fencing) mode is enabled for the node, it's set to `0`, preventing the node from rebooting. |
| `kernel.panic_on_oops` | `1` | Allows the system to trigger a kernel panic after an unexpected oops error. Required for kubelet to work correctly. |
| `kernel.pid_max` | `2000000` | Maximum number of process IDs that can be assigned in the system. |
| `net.bridge.bridge-nf-call-arptables` | `1` | Enables traffic filtering through arptables. Required for kube-proxy to work correctly. |
| `net.bridge.bridge-nf-call-ip6tables` | `1` | Enables traffic filtering through ip6tables. Required for kube-proxy to work correctly. |
| `net.bridge.bridge-nf-call-iptables` | `1` | Enables traffic filtering through iptables. Required for kube-proxy to work correctly. |
| `net.core.netdev_max_backlog` | `5000` | Maximum number of packets allowed in the processing queue. |
| `net.core.rmem_max` | `16777216` | Maximum receive buffer size in bytes. |
| `net.core.somaxconn` | `1000` | Maximum number of pending connections. |
| `net.core.wmem_max` | `16777216` | Maximum send buffer size in bytes. |
| `net.ipv4.conf.all.forwarding` | `1` | Enables IPv4 packet forwarding between network interfaces. Equivalent to the `net.ipv4.ip_forward` parameter. |
| `net.ipv4.ip_local_port_range` | `"32768 61000"` | Range of ports available for outgoing TCP and UDP connections. |
| `net.ipv4.neigh.default.gc_thresh1` | `16384` | Lower threshold for the amount of ARP entries after which the system starts cleaning up old entries. |
| `net.ipv4.neigh.default.gc_thresh2` | `28672` | Middle threshold for the amount of ARP entries after which the system starts garbage collection. |
| `net.ipv4.neigh.default.gc_thresh3` | `32768` | Absolute maximum number of ARP entries. |
| `net.ipv4.tcp_max_syn_backlog` | `8096` | Maximum number of queued SYN connections. |
| `net.ipv4.tcp_no_metrics_save` | `1` | Disables saving of TCP metrics of closed connections and reusing them for new connections. |
| `net.ipv4.tcp_rmem` | `"4096 12582912 16777216"` | Receive buffer sizes for incoming TCP packets in bytes: `"<minimum> <default> <maximum>"`. |
| `net.ipv4.tcp_slow_start_after_idle` | `0` | Disables using the congestion window (CWND) and slow start algorithm for TCP connections. |
| `net.ipv4.tcp_tw_reuse` | `1` | Enables reusing the outgoing TCP connections in `TIME-WAIT` state. |
| `net.ipv4.tcp_wmem` | `"4096 12582912 16777216"` | Send buffer sizes for outgoing TCP packets in bytes: `"<minimum> <default> <maximum>"`. |
| `net.netfilter.nf_conntrack_max` | `<no-of-cores * 131072> or 524288` | Maximum number of tracked connections in the conntrack table. Calculated as "number of CPU cores" * 131072, but no lower than `524288`. |
| `net.nf_conntrack_max` | `<no-of-cores * 131072> or 524288` | Maximum number of tracked connections in the conntrack table for older kernels. Calculated as "number of CPU cores" * 131072, but no lower than `524288`. |
| `vm.dirty_background_ratio` | `5` | Percentage of system memory that can be filled with dirty pages before the kernel starts writing them to disk in the background. |
| `vm.dirty_expire_centisecs` | `12000` | Duration (in centiseconds) a dirty page can remain in system memory before it must be written to disk. |
| `vm.dirty_ratio` | `80` | Percentage of system memory that can be filled with dirty pages before all processes must stop and flush data to disk. |
| `vm.min_free_kbytes` | `131072` | Minimum amount of free memory in kilobytes reserved by the kernel for critical operations. |
| `vm.overcommit_memory` | `1` | Enables memory overcommitment. |
| `vm.swappiness` | `0` | Disables swap file usage. |
