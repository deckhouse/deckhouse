---
title: Security software settings for working with Deckhouse
permalink: en/security_software_setup.html
---

If Kubernetes cluster nodes are analyzed by security scanners (antivirus tools), you may need to configure them to avoid false positives.

Deckhouse Kubernetes Platform (DKP) uses the following directories when running ([download in CSV](deckhouse-directories.csv)):

{% include security_software_setup.liquid %}

## SIEM

Security Information and Event Management (SIEM) is a class of software solutions
designed to collect and analyze security event information.

### KUMA

Kaspersky Unified Monitoring and Analysis Platform (KUMA) integrates Kaspersky Lab products with third-party solutions
into a unified information security system.
It's a key component in implementing a comprehensive protection approach,
securing corporate and industrial environments as well as the IT/OT system interface,
which is the most common target for attackers, against modern cyber threats.

#### Configuration details

{% alert level="warning" %}
To work with KUMA, you **must enable** the [log-shipper](modules/log-shipper/) module.
{% endalert %}

To send data to [KUMA](https://support.kaspersky.com/help/kuma/1.5/en-US/217694.htm), configure the following resources in DKP:

- [`ClusterLogDestination`](modules/log-shipper/cr.html#clusterlogdestination)
- [`ClusterLoggingConfig`](modules/log-shipper/cr.html#clusterloggingconfig)

{% alert level="info" %}
Make sure to configure the necessary resources in KUMA to enable event collection.
{% endalert %}

The following are configuration examples for sending the audit file `/var/log/kube-audit/audit.log` in various formats.

#### Sending logs in JSON via UDP

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: kuma-udp-json
spec:
  type: Socket
  socket:
    address: IP_ADDRESS:PORT # Replace during the setup
    mode: UDP
    encoding:
      codec: "JSON"
---
apiVersion: deckhouse.io/v1alpha1
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

#### Sending logs in JSON via TCP

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: kuma-tcp-json
spec:
  type: Socket
  socket:
    address: IP_ADDRESS:PORT # Replace during the setup
    mode: TCP
    tcp:
      verifyCertificate: false
      verifyHostname: false
    encoding:
      codec: "JSON"
---
apiVersion: deckhouse.io/v1alpha1
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

#### Sending logs in CEF via TCP

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
    address: IP_ADDRESS:PORT # Replace during the setup
    mode: TCP
    tcp:
      verifyCertificate: false
      verifyHostname: false
    encoding:
      codec: "CEF"
---
apiVersion: deckhouse.io/v1alpha1
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

#### Sending logs in Syslog via TCP

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: kuma-tcp-syslog
spec:
  type: Socket
  socket:
    address: IP_ADDRESS:PORT # Replace during the setup
    mode: TCP
    tcp:
      verifyCertificate: false
      verifyHostname: false
    encoding:
      codec: "Syslog"
---
apiVersion: deckhouse.io/v1alpha1
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

#### Sending logs in Apache Kafka

{% alert level="info" %}
Ensure that Apache Kafka is configured to collect data.
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: kuma-kafka
spec:
  type: Kafka
  kafka:
    bootstrapServers:
      - kafka-address:9092 # Replace with the current value during the setup
    topic: k8s-logs
---
apiVersion: deckhouse.io/v1alpha1
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

## Security Software

### KESL

The following are recommendations for configuring Kaspersky Endpoint Security for Linux (KESL) to ensure that it operates smoothly with Deckhouse Kubernetes Platform (whatever edition you choose).

To ensure compatibility with DKP, the following tasks must be disabled on the KESL side:

- `Firewall_Management (ID: 12)`.
- `Web Threat Protection (ID: 14)`.
- `Network Threat Protection (ID: 17)`.
- `Web Control (ID: 26)`.

{% alert level="info" %}
Note that the task list may be different in future KESL versions.
{% endalert %}

Ensure that your Kubernetes nodes meet the minimum resource requirements specified for [DKP](https://deckhouse.io/products/kubernetes-platform/guides/production.html#resource-requirements) and [KESL](https://support.kaspersky.com/KES4Linux/12.1.0/en-US/197642.htm).

If KESL and DKP are run together, you may be required to do some performance tuning as per [Kaspersky recommendations](https://support.kaspersky.com/KES4Linux/12.1.0/en-US/206054.htm).
