---
title: Security software settings for working with Deckhouse
permalink: en/security_software_setup.html
---

If Kubernetes cluster nodes are analyzed by security scanners (antivirus tools), you may need to configure them to avoid false positives.

Deckhouse uses the following directories when working ([download their list in csv...](deckhouse-directories.csv)):

{% include security_software_setup.liquid %}

## Recommendations for configuring KESL (Kaspersky Endpoint Security for Linux) to work with Deckhouse

To ensure that KESL does not affect Deckhouse's performance, follow these configuration recommendations by running them in the command line directly on the host or through the centralized remote management system of Kaspersky Security Center.

Configuration of KESL is carried out [using tasks](https://support.kaspersky.com/KES4Linux/11.2.0/en-US/199339.htm) that have specific numbers. Below is an overview of the general setup and the configuration of the tasks used when setting up KESL in Deckhouse.

### General Setup

Modify KESL settings related to network packet marking bits as there are overlaps with Deckhouse's own network packet markings. To do so:
1. Stop KESL if it is running, and modofy the following settings:

   * Change the `BypassFwMark` parameter value from `0x400` to `0x700`;
   * Change the `NtpFwMark` parameter value from `0x200` to `0x600`.

1. Restart KESL.

   Below are some example commands you can run to restart KESL:

   ```shell
   systemctl stop kesl
   sed -i "s/NtpFwMark=0x200/NtpFwMark=0x600/" /var/opt/kaspersky/kesl/common/kesl.ini
   sed -i "s/BypassFwMark=0x400/BypassFwMark=0x700/" /var/opt/kaspersky/kesl/common/kesl.ini
   systemctl start kesl
   ```

### Task 1. File Threat Protection

Exclude Deckhouse directories from analysis by running the following commands:

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

{% alert level="info" %}
When adding, a notification may be shown that some directories do not exist. The rule will still be added (this is expected behavior).
{% endalert %}

### Task 2. Scan My Computer

Exclude Deckhouse directories from analysis by running the following commands:

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

{% alert level="info" %}
When adding, a notification may be shown that some directories do not exist. The rule will still be added (this is expected behavior).
{% endalert %}

### Task 3. Selective Scan

Exclude Deckhouse directories from analysis by running the following commands:

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

_Note:_ when adding, a notification may be shown that some directories do not exist. The rule will still be added (this is expected behavior).

### Task 4. Critical Areas Scan

Exclude Deckhouse directories from analysis by running the following commands:

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

{% alert level="info" %}
When adding, a notification may be shown that some directories do not exist. The rule will still be added (this is expected behavior).
{% endalert %}

### Task 11. System Integrity Monitoring

Exclude Deckhouse directories from analysis by running the following commands:

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

{% alert level="info" %}
When adding, a notification may be shown that some directories do not exist. The rule will still be added (this is expected behavior).
{% endalert %}

### Task 12. Firewall Management

{% alert level="danger" %}
The task must be disabled. Do not enable it once disabled. It will render Deckhouse inoperable.
{% endalert %}

This task removes all iptables rules not related to KESL ([link to the KESL documentation](https://support.kaspersky.com/KES4Linux/11.3.0/en-US/234820.htm)).

If the task is enabled, disable it by running the following command:

```shell
kesl-control --stop-task 12
```

### Task 13. Anti-Cryptor

Exclude Deckhouse directories from analysis by running the following commands:

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

{% alert level="info" %}
When adding, a notification may be shown that some directories do not exist. The rule will still be added (this is expected behavior).
{% endalert %}

### Task 14. Web Threat Protection

We recommended disabling the task. If you need to enable the task for some reason, configure it independently to avoid affecting Deckhouse performance.

If the task is enabled and its negative impact on Deckhouse is detected, disable the task by executing the command below:

```shell
kesl-control --stop-task 14
```

### Task 17. Network Threat Protection

We recommended disabling the task. If you need to enable the task for some reason, configure it independently to avoid affecting Deckhouse performance.

If the task is enabled and its negative impact on Deckhouse is detected, disable the task by executing the command below:

```shell
kesl-control --stop-task 17
```

### Task 20. Behavior Detection

With default settings, this task has no negative impact on Deckhouse performance. If you need to enable the task for some reason, configure it independently to avoid affecting Deckhouse performance.

If the task is enabled and its negative impact on Deckhouse is detected, disable the task by executing the command below:

```shell
kesl-control --stop-task 20
```

### Task 21. Application Control

With default settings, this task has no negative impact on Deckhouse performance. If you need to enable the task for some reason, configure it independently to avoid affecting Deckhouse performance.

If the task is enabled and its negative impact on Deckhouse is detected, disable the task by executing the command below:

```shell
kesl-control --stop-task 21
```

### Task 22. Web Control

We recommended disabling the task. If you need to enable the task for some reason, configure it independently to avoid affecting Deckhouse performance.

If the task is enabled and its negative impact on Deckhouse is detected, disable the task by executing the command below:

```shell
kesl-control --stop-task 22
```
