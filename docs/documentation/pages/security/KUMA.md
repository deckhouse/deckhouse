---
title: KUMA
permalink: en/security/kuma.html
lang: en
---

### KUMA

Kaspersky Unified Monitoring and Analysis Platform (KUMA) integrates Kaspersky Lab products with third-party solutions
into a unified information security system.
It's a key component in implementing a comprehensive protection approach,
securing corporate and industrial environments as well as the IT/OT system interface,
which is the most common target for attackers, against modern cyber threats.

#### Configuration details

{% alert level="warning" %}
To work with KUMA, you **must enable** the [log-shipper](../modules/log-shipper/) module.
{% endalert %}

To send data to [KUMA](https://support.kaspersky.com/help/kuma/1.5/en-US/217694.htm), configure the following resources in DKP:

- [`ClusterLogDestination`](../modules/log-shipper/cr.html#clusterlogdestination)
- [`ClusterLoggingConfig`](../modules/log-shipper/cr.html#clusterloggingconfig)

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
