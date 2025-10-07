---
title: Cluster SLA monitoring
permalink: en/admin/configuration/monitoring/sla.html
description: "Configure SLA monitoring in Deckhouse Kubernetes Platform. Cluster availability tracking, SLA compliance monitoring, and uptime statistics collection for platform components."
---

DKP can collect statistics about the availability of cluster components and DKP components themselves. This data allows evaluating SLA compliance and provides availability information in the web interface.

Additionally, using the [UpmeterRemoteWrite](/modules/upmeter/cr.html#upmeterremotewrite) custom resource, you can export availability metrics via the Prometheus Remote Write protocol.

To start collecting availability metrics and activate the [interface](#interface), enable the [`upmeter`](/modules/upmeter/) module in the [Deckhouse web interface](/modules/console/) or using the following command:

```shell
d8 platform module enable upmeter
```

## Module configuration

The [`upmeter`](/modules/upmeter/) module is configured using the `upmeter` ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: upmeter
spec:
  version: 3
  enabled: true
  settings:
```

A complete list of all settings is available in the [module documentation](/modules/upmeter/configuration.html).

## Interface

DKP provides two web interfaces for availability assessment:
- Status page.

  You can get the page address in the web interface on the main page in the "Tools" section (Status page tile), or by running the command:
  
  ```shell
  d8 k -n d8-upmeter get ing status -o jsonpath='{.spec.rules[*].host}'
  ``` 

  Example of the status page web interface:
  
  ![Example of the status page web interface](../../../images/upmeter/status.png)

- Component availability page.

  You can get the page address in the web interface on the main page in the "Tools" section (Component availability tile), or by running the command:
  
  ```shell
  d8 k -n d8-upmeter get ing upmeter -o jsonpath='{.spec.rules[*].host}'
  ``` 

  Example of the component availability page:
  
  ![Example of upmeter metrics graphs in Grafana](../../../images/upmeter/image1.png)

## Status metrics export

Example of [UpmeterRemoteWrite](/modules/upmeter/cr.html#upmeterremotewrite) configuration for exporting status metrics via the [Prometheus Remote Write](https://docs.sysdig.com/en/docs/installation/prometheus-remote-write/) protocol:

```yaml
apiVersion: deckhouse.io/v1
kind: UpmeterRemoteWrite
metadata:
  labels:
    heritage: upmeter
    module: upmeter
  name: victoriametrics
spec:
  additionalLabels:
    cluster: cluster-name
    some: fun
  config:
    url: https://upmeter-victoriametrics.whatever/api/v1/write
    basicAuth:
      password: "Cdp#Cd.OxfZsx4*89SZ"
      username: upmeter
  intervalSeconds: 300
```

## Authentication

By default, the [`user-authn`](/modules/user-authn/) module is used for authentication. You can also configure authentication via `externalAuthentication` (see below).
If these options are disabled, the module will enable basic authentication with a generated password.

You can view the generated password with the command:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values upmeter -o json | jq '.upmeter.internal.auth.webui.password'
```

To generate a new password, you need to delete the Secret:

```shell
d8 k -n d8-upmeter delete secret/basic-auth-webui
```

You can view the generated password for the status page with the command:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values upmeter -o json | jq '.upmeter.internal.auth.status.password'
```

To generate a new password for the status page, you need to delete the secret:

```shell
d8 k -n d8-upmeter delete secret/basic-auth-status
```

> **Attention!** The `auth.status.password` and `auth.webui.password` parameters are no longer supported.

## FAQ

### Why are some upmeter pods periodically deleted or cannot be scheduled?

The module implements availability tests and health checks for various Kubernetes controllers. Tests are performed by creating and deleting temporary pods.

`upmeter-probe-scheduler` objects are responsible for checking scheduler health. As part of the test, a pod is created and scheduled to a node. Then this pod is deleted.

`upmeter-probe-controller-manager` objects are responsible for testing `kube-controller-manager` health.

As part of the test, a StatefulSet is created and it is verified that this object spawned a pod (since actual pod scheduling is not required and is checked in another test, a pod is created that is guaranteed to not be schedulable, i.e., remains in `Pending` state). Then the StatefulSet is deleted and it is verified that the pod it spawned was also deleted.

`smoke-mini` objects implement network connectivity testing between nodes.
For testing, five StatefulSets with one replica are deployed. As part of the test, connectivity is checked both between `smoke-mini` pods and network connectivity with `upmeter-agent` pods running on master nodes.  
Once a minute, one of the `smoke-mini` pods is moved to another node.
