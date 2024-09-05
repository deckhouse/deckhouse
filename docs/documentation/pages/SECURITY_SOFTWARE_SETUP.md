---
title: Security software settings for working with Deckhouse
permalink: en/security_software_setup.html
---

If security scanners (antivirus tools) scan nodes of the Kubernetes cluster, then it may be necessary to configure them to exclude false positives.

Deckhouse uses the following directories when working ([download in csv...](deckhouse-directories.csv)):

{% include security_software_setup.liquid %}

## Recommendations for configuring KESL (Kaspersky Endpoint Security for Linux) to work with Deckhouse

To ensure that KESL does not affect Deckhouse's performance, you need to follow these configuration recommendations via the command line directly on the host or through the centralized remote management system of Kaspersky Security Center.

### General Setup

Modify KESL settings related to network packet marking bits as there are overlaps with the network packet marking by Deckhouse.

First, stop KESL if it is running, and change the settings:

* Change the `BypassFwMark` parameter value from `0x400` to `0x700`;
* Change the `NtpFwMark` parameter value from `0x200` to `0x600`.

Restart KESL.

Example commands:

```shell
systemctl stop kesl
sed -i "s/NtpFwMark=0x200/NtpFwMark=0x600/" /var/opt/kaspersky/kesl/common/kesl.ini
sed -i "s/BypassFwMark=0x400/BypassFwMark=0x700/" /var/opt/kaspersky/kesl/common/kesl.ini
systemctl start kesl
```

### Task 1 "File Threat Protection"

Add Deckhouse directories to exclusions by executing the following commands:

```shell
kesl-control --set-settings 1 --add-exclusion /etc/cni
kesl-control --set-settings 1 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 1 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 1 --add-exclusion /mnt/vector-data
kesl-control --set-settings 1 --add-exclusion /opt/cni/bin
kesl-control --set-settings 1 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 1 --add-exclusion /var/lib/bashable
kesl-control --set-settings 1 --add-exclusion /var/lib/containerd
kesl-control --set-settings 1 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 1 --add-exclusion /var/lib/etcd
kesl-control --set-settings 1 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 1 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 1 --add-exclusion /var/log/containers
kesl-control --set-settings 1 --add-exclusion /var/log/pods
```

Note: You might receive a notification that some directories does not exist, but the rule will be added - this is normal.

### Task 2 "Scan My Computer"

Add Deckhouse directories to exclusions by executing the following commands:

```shell
kesl-control --set-settings 2 --add-exclusion /etc/cni
kesl-control --set-settings 2 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 2 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 2 --add-exclusion /mnt/vector-data
kesl-control --set-settings 2 --add-exclusion /opt/cni/bin
kesl-control --set-settings 2 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 2 --add-exclusion /var/lib/bashable
kesl-control --set-settings 2 --add-exclusion /var/lib/containerd
kesl-control --set-settings 2 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 2 --add-exclusion /var/lib/etcd
kesl-control --set-settings 2 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 2 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 2 --add-exclusion /var/log/containers
kesl-control --set-settings 2 --add-exclusion /var/log/pods
```

Note: You might receive a notification that some directories does not exist, but the rule will be added - this is normal.

### Task 3 "Selective Scan"

Add Deckhouse directories to exclusions by executing the following commands:

```shell
kesl-control --set-settings 3 --add-exclusion /etc/cni
kesl-control --set-settings 3 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 3 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 3 --add-exclusion /mnt/vector-data
kesl-control --set-settings 3 --add-exclusion /opt/cni/bin
kesl-control --set-settings 3 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 3 --add-exclusion /var/lib/bashable
kesl-control --set-settings 3 --add-exclusion /var/lib/containerd
kesl-control --set-settings 3 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 3 --add-exclusion /var/lib/etcd
kesl-control --set-settings 3 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 3 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 3 --add-exclusion /var/log/containers
kesl-control --set-settings 3 --add-exclusion /var/log/pods
```

Note: You might receive a notification that some directories does not exist, but the rule will be added - this is normal.

### Task 4 "Critical Areas Scan”

Add Deckhouse directories to exclusions by executing the following commands:

```shell
kesl-control --set-settings 4 --add-exclusion /etc/cni
kesl-control --set-settings 4 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 4 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 4 --add-exclusion /mnt/vector-data
kesl-control --set-settings 4 --add-exclusion /opt/cni/bin
kesl-control --set-settings 4 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 4 --add-exclusion /var/lib/bashable
kesl-control --set-settings 4 --add-exclusion /var/lib/containerd
kesl-control --set-settings 4 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 4 --add-exclusion /var/lib/etcd
kesl-control --set-settings 4 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 4 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 4 --add-exclusion /var/log/containers
kesl-control --set-settings 4 --add-exclusion /var/log/pods
```

Note: You might receive a notification that some directories does not exist, but the rule will be added - this is normal.

### Task 11 "System Integrity Monitoring”

Add Deckhouse directories to exclusions by executing the following commands:

```shell
kesl-control --set-settings 11 --add-exclusion /etc/cni
kesl-control --set-settings 11 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 11 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 11 --add-exclusion /mnt/vector-data
kesl-control --set-settings 11 --add-exclusion /opt/cni/bin
kesl-control --set-settings 11 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 11 --add-exclusion /var/lib/bashable
kesl-control --set-settings 11 --add-exclusion /var/lib/containerd
kesl-control --set-settings 11 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 11 --add-exclusion /var/lib/etcd
kesl-control --set-settings 11 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 11 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 11 --add-exclusion /var/log/containers
kesl-control --set-settings 11 --add-exclusion /var/log/pods
```

Note: You might receive a notification that some directories does not exist, but the rule will be added - this is normal.

### Task 12 "Firewall Management”

The task **MUST BE DISABLED AND NOT ENABLED**!!! Because this will cause Deckhouse to stop functioning. This task removes all iptables rules not related to KESL ([link to the KESL documentation](https://support.kaspersky.com/KES4Linux/11.3.0/ru-RU/234820.htm)).

If the task is enabled, disable it by executing the command:

```shell
kesl-control --stop-task 12
```

### Task 13 "Anti-Cryptor”

Add Deckhouse directories to exclusions by executing the following commands:

```shell
kesl-control --set-settings 13 --add-exclusion /etc/cni
kesl-control --set-settings 13 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 13 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 13 --add-exclusion /mnt/vector-data
kesl-control --set-settings 13 --add-exclusion /opt/cni/bin
kesl-control --set-settings 13 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 13 --add-exclusion /var/lib/bashable
kesl-control --set-settings 13 --add-exclusion /var/lib/containerd
kesl-control --set-settings 13 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 13 --add-exclusion /var/lib/etcd
kesl-control --set-settings 13 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 13 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 13 --add-exclusion /var/log/containers
kesl-control --set-settings 13 --add-exclusion /var/log/pods
```

Note: You might receive a notification that some directories does not exist, but the rule will be added - this is normal.

### Task 14 "Web Threat Protection”

Recommended to disable, but if there is a need to enable the task, configure it independently to avoid affecting Deckhouse performance.

If the task is enabled and negatively impacts Deckhouse, disable it by executing the command:

```shell
kesl-control --stop-task 14
```

### Task 17 "Network Threat Protection”

Recommended to disable, but if there is a need to enable the task, configure it independently to avoid affecting Deckhouse performance.

If the task is enabled and negatively impacts Deckhouse, disable it by executing the command:

```shell
kesl-control --stop-task 17
```

### Task 20 "Behavior Detection”

With default settings, it does not negatively impact Deckhouse performance. If there is a need to enable the task, configure it independently to avoid affecting Deckhouse performance.

If the task is enabled and negatively impacts Deckhouse, disable it by executing the command:

```shell
kesl-control --stop-task 20
```

### Task 21 "Application Control”

With default settings, it does not negatively impact Deckhouse performance. If there is a need to enable the task, configure it independently to avoid affecting Deckhouse performance.

If the task is enabled and negatively impacts Deckhouse, disable it by executing the command:

```shell
kesl-control --stop-task 21
```

### Task 22 "Web Control”

Recommended to disable, but if there is a need to enable the task, configure it independently to avoid affecting Deckhouse performance.

If the task is enabled and negatively impacts Deckhouse, disable it by executing the command:

```shell
kesl-control --stop-task 22
```
