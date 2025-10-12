---
title: "Cluster SLA monitoring"
permalink: en/virtualization-platform/documentation/admin/platform-management/monitoring/sla.html
---

DVP can collect statistics about the availability of cluster components and Deckhouse components themselves. This data allows you to evaluate the degree of SLA compliance and obtain availability information in the web interface.

In addition, using the custom resource [UpmeterRemoteWrite](/modules/upmeter/cr.html#upmeterremotewrite), you can export availability metrics via the Prometheus Remote Write protocol.

To start collecting availability metrics and activate the [interface](#interface), enable the `upmeter` module [in the Deckhouse web interface](/modules/console/stable/) or using the following command:

```shell
d8 platform module enable upmeter
```

## Module Configuration

The `upmeter` module is configured using the `upmeter` ModuleConfig:

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

The complete list of settings is available [in the module documentation](/modules/upmeter/configuration.html).

## Interface

DVP provides two web interfaces for availability assessment:

1. Status page.

   You can get the page address in the web interface on the main page in the "Tools" section (the "Status page" tile), or by running the command:
   
   ```shell
   d8 k -n d8-upmeter get ing status -o jsonpath='{.spec.rules[*].host}'
   ``` 

   Example of the status page web interface:
   
   ![Example of the status page web interface](/images/upmeter/status.png)

1. Component availability page.

   You can get the page address in the web interface on the main page in the "Tools" section (the "Component availability" tile), or by running the command:
   
   ```shell
   d8 k -n d8-upmeter get ing upmeter -o jsonpath='{.spec.rules[*].host}'
   ``` 

   Example of the component availability page:
   
   ![Example of upmeter metrics charts in Grafana](/images/upmeter/image1.png)

## Status Metrics Export
 
Example configuration of UpmeterRemoteWrite for exporting status metrics via the [Prometheus Remote Write](https://docs.sysdig.com/en/docs/installation/prometheus-remote-write/) protocol:

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

The `upmeter-probe-scheduler` objects are responsible for checking the scheduler health. As part of the test, a pod is created and scheduled to a node. Then this pod is deleted.

The `upmeter-probe-controller-manager` objects are responsible for testing the `kube-controller-manager` health.  
As part of the test, a StatefulSet is created and it is checked that this object spawned a pod (since actual pod scheduling is not required and is checked in another test, a pod is created that is guaranteed to not be schedulable, i.e., remains in the `Pending` state). Then the StatefulSet is deleted and a check is performed to ensure that the pod it spawned is also deleted.

The `smoke-mini` objects implement network connectivity testing between nodes.
To check, five StatefulSets with one replica are deployed. As part of the test, connectivity between `smoke-mini` pods is checked, as well as network connectivity with `upmeter-agent` pods running on master nodes.  
Once a minute, one of the `smoke-mini` pods is moved to another node.
