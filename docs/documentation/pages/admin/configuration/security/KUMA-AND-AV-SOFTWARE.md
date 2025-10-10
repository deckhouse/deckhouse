---
title: Integration with KUMA and antivirus software
permalink: en/admin/configuration/security/kuma-and-av-software.html
description: "Configure KUMA and antivirus software integration in Deckhouse Kubernetes Platform. Security event forwarding, audit log analysis, and Kaspersky integration setup."
---

Deckhouse Kubernetes Platform (DKP) supports integration with Kaspersky Unified Monitoring and Analysis Platform (KUMA),
a unified monitoring and analysis system by Kaspersky Lab.
As part of the integration, security events and audit logs from the cluster are sent to KUMA for further analysis.

## Sending logs to KUMA

To send logs to KUMA, configure [log collection and delivery in DKP](../logging/delivery.html) using the following resources:

- [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination): Defines log storage parameters.
- [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig): Defines cluster log collection parameters.

{% alert level="info" %}
On the KUMA side, configure the appropriate resources for receiving events.
{% endalert %}

### Example ClusterLogDestination and ClusterLoggingConfig configurations

- Sending logs in JSON format over UDP:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: kuma-udp-json
  spec:
    type: Socket
    socket:
      address: IP_ADDRESS:PORT # Replace as needed.
      mode: UDP
      encoding:
        codec: "JSON"
  ---
  apiVersion: deckhouse.io/v1alpha2
  kind: ClusterLoggingConfig
  metadata:
    name: kubelet-audit-logs
  spec:
    type: File
    file:
      include:
      - /var/log/kube-audit/audit.log
    destinationRefs:
      - kuma-udp-json
  ```

- Sending logs in JSON format over TCP:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: kuma-tcp-json
  spec:
    type: Socket
    socket:
      address: IP_ADDRESS:PORT # Replace as needed.
      mode: TCP
      tcp:
        verifyCertificate: false
        verifyHostname: false
      encoding:
        codec: "JSON"
  ---
  apiVersion: deckhouse.io/v1alpha2
  kind: ClusterLoggingConfig
  metadata:
    name: kubelet-audit-logs
  spec:
    type: File
    file:
      include:
      - /var/log/kube-audit/audit.log
    destinationRefs:
      - kuma-tcp-json
  ```

- Sending logs in CEF format over TCP:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: kuma-tcp-cef
  spec:
    type: Socket
    socket:
      extraLabels:
        cef.name: d8
        cef.severity: "1"
      address: IP_ADDRESS:PORT # Replace as needed.
      mode: TCP
      tcp:
        verifyCertificate: false
        verifyHostname: false
      encoding:
        codec: "CEF"
  ---
  apiVersion: deckhouse.io/v1alpha2
  kind: ClusterLoggingConfig
  metadata:
    name: kubelet-audit-logs
  spec:
    type: File
    file:
      include:
      - /var/log/kube-audit/audit.log
    logFilter:
      - field: userAgent
        operator: Regex
        values: [ "kubelet.*" ]
    destinationRefs:
      - kuma-tcp-cef
  ```

- Sending logs in Syslog format over TCP:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: kuma-tcp-syslog
  spec:
    type: Socket
    socket:
      address: IP_ADDRESS:PORT # Replace as needed.
      mode: TCP
      tcp:
        verifyCertificate: false
        verifyHostname: false
      encoding:
        codec: "Syslog"
  ---
  apiVersion: deckhouse.io/v1alpha2
  kind: ClusterLoggingConfig
  metadata:
    name: kubelet-audit-logs
  spec:
    type: File
    file:
      include:
      - /var/log/kube-audit/audit.log
    logFilter:
      - field: userAgent
        operator: Regex
        values: [ "kubelet.*" ]
    destinationRefs:
      - kuma-tcp-syslog
  ```

- Sending logs to Apache Kafka:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: kuma-kafka
  spec:
    type: Kafka
    kafka:
      bootstrapServers:
        - kafka-address:9092 # Replace as needed.
      topic: k8s-logs
  ---
  apiVersion: deckhouse.io/v1alpha2
  kind: ClusterLoggingConfig
  metadata:
    name: kubelet-audit-logs
  spec:
    destinationRefs:
      - kuma-kafka
    file:
      include:
        - /var/log/kube-audit/audit.log
    logFilter:
      - field: userAgent
        operator: Regex
        values:
          - kubelet.*
    type: File
  ```

## Antivirus scanning exclusions for nodes

If antivirus software is installed on DKP cluster nodes (for example, Kaspersky Endpoint Security for Linux, KESL),
you may need to exclude Deckhouse service directories from scanning to avoid false positives.

List of Deckhouse service directories (also available [CSV format](/products/kubernetes-platform/documentation/v1/deckhouse-directories.csv)):

| Directory | Purpose |
| --------- | ------- |
| `/etc/cni/` (any node) | CNI module configuration files |
| `/etc/kubernetes` (any node) | Static pod manifests, PKI certificate files |
| `/mnt/kubernetes-data` (master node) | Only present in clusters deployed in the cloud when a separate disk is used for the etcd database |
| `/mnt/vector-data` (any node, `log-shipper` module) | Service data for log delivery status |
| `/opt/cni/bin/` (any node) | CNI module executables |
| `/opt/deckhouse/bin/` (any node) | Executables required for Deckhouse operation |
| `/var/lib/bashible` (any node) | Node configuration files |
| `/var/lib/containerd` (any node) | Stores CRI-related data (for example, containerd). Contains container image layers, filesystem snapshots, metadata, logs, and other container information |
| `/var/lib/deckhouse/` (master node) | Deckhouse module files dynamically loaded from the image registry |
| `/var/lib/etcd` (master node) | etcd database |
| `/var/lib/kubelet/` (any node) | kubelet configuration files |
| `/var/lib/upmeter` (master node, `upmeter` module) | [`upmeter`](/modules/upmeter/) module database |
| `/var/log/containers` (any node) | Container logs (when using containerd) |
| `/var/log/pods/` (any node) | Logs of all pod containers running on the node |

### KESL configuration recommendations

To ensure DKP functions correctly with KESL installed, follow these steps:

1. Disable the following KESL tasks:

   - `Firewall Management (ID: 12)`
   - `Web Threat Protection (ID: 14)`
   - `Network Threat Protection (ID: 17)`
   - `Web Control (ID: 26)`

   > The list of tasks may differ in future KESL versions.

1. Make sure node resources meet the requirements of:

   - [DKP](/products/kubernetes-platform/guides/production.html#resource-requirements)
   - [KESL](https://support.kaspersky.com/KES4Linux/12.1.0/en-US/197642.htm)

1. For performance optimization, follow the [official Kaspersky recommendations](https://support.kaspersky.com/KES4Linux/12.1.0/en-US/206054.htm).
