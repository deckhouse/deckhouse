---
title: Kubernetes API event audit
permalink: en/virtualization-platform/documentation/admin/platform-management/security/events/kubernetes-api-audit.html
---

The [Kubernetes auditing](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/) feature allows you to track requests
to the API server and analyze events occurring in the cluster.
Auditing can be useful for troubleshooting unexpected behavior and for meeting security requirements.

Kubernetes supports audit configuration via the [Audit policy](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy) mechanism,
which allows you to define logging rules for target operations.
By default, audit results are written to the `/var/log/kube-audit/audit.log` file.

## Built-in audit policies

Deckhouse Virtualization Platform (DVP) configures a default audit policy that logs the following:

General events:

- Creation and deletion of cluster nodes (`Node` objects);
- Create, update, and delete operations on resources in all system namespaces (`kube-system`, `d8-*`);
- Requests from service accounts in system namespaces (`kube-system`, `d8-*`);
- Bulk `LIST` requests to all resources (used to diagnose high API server resource consumption);
- Operations related to `virtualization` module resources;
- Actions in the `d8-virtualization` namespace;
- Operations on `ModuleConfig` objects;
- Requests from unauthenticated users (only `Metadata` level events are recorded).

Security-sensitive events:

- Creation, modification, and deletion of `Pod` objects;
- Use of container image references without a `@sha256` digest;
- Creation and deletion of `ServiceAccount` objects, including in system namespaces;
- Creation, modification, and deletion of `Role` and `ClusterRole` objects;
- Creation, modification, and deletion of `ClusterRoleBinding` objects;
- Use of `attach` and `exec` commands against Pods, as well as adding ephemeral containers (get and patch operations on `pods/exec`, `pods/attach`, and `pods/ephemeralcontainers`).

Events explicitly excluded from audit logging (because the corresponding objects change very frequently):

- Operations on `Endpoints`, `EndpointSlice`, and `Event` objects;
- Operations on `Lease` objects (raft leader election in platform components);
- Actions on `ConfigMap` objects used for raft leader election (`cert-manager-cainjector-leader-election`, `cert-manager-controller`, `ingress-nginx`, and similar);
- Operations on `VerticalPodAutoscalerCheckpoints` objects;
- `PATCH` operations on `VerticalPodAutoscaler` objects performed by the `d8-vertical-pod-autoscaler-recommender` service account;
- Actions on `UpmeterHookProbes` objects;
- Any operations in the `d8-upmeter` namespace.

These policies are enabled by default.
To disable them, set the [`basicAuditPolicyEnabled`](/modules/control-plane-manager/configuration.html#parameters-apiserver-basicauditpolicyenabled) parameter to `false`.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    apiserver:
      auditPolicyEnabled: true
      basicAuditPolicyEnabled: false
```

## Configuring a custom audit policy

To create an advanced Kubernetes API audit policy, follow these steps:

1. Enable the [`auditPolicyEnabled`](/modules/control-plane-manager/configuration.html#parameters-apiserver-auditpolicyenabled) parameter
   in the `control-plane-manager` module configuration:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     version: 1
     settings:
       apiserver:
         auditPolicyEnabled: true
   ```

1. Create the `kube-system/audit-policy` Secret containing the policy YAML file encoded in Base64:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: audit-policy
     namespace: kube-system
   data:
     audit-policy.yaml: <Base64>
   ```

   Example `audit-policy.yaml` content with a minimal working configuration:

   ```yaml
   apiVersion: audit.k8s.io/v1
   kind: Policy
   rules:
   - level: Metadata
     omitStages:
     - RequestReceived
   ```

   For more information on possible contents of `audit-policy.yaml`, refer to the following sources:

   - [Kubernetes official documentation](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy)
   - [GCE helper script code snippet](https://github.com/kubernetes/kubernetes/blob/0ef45b4fcf7697ea94b96d1a2fe1d9bffb692f3a/cluster/gce/gci/configure-helper.sh#L722-L862)

## Working with the audit log file

On DVP master nodes, it is assumed that a log collection tool (`log-shipper`, `promtail`, or `filebeat`) is installed
to monitor the `/var/log/kube-audit/audit.log` file.

The log rotation settings for this file are predefined and cannot be changed:

- Maximum file size: 1000 MB.
- Maximum retention period: 30 days.

Depending on the policy configuration and the volume of requests to the API server,
the number of log entries can be very large.
In such cases, the retention period may be reduced to less than 30 minutes.

{% alert level="warning" %}
Unsupported options or typos in the configuration file may cause the API server to fail to start.
{% endalert %}

If the API server fails to start, take the following steps:

1. Manually remove the `--audit-log-*` parameters from the `/etc/kubernetes/manifests/kube-apiserver.yaml` manifest.
1. Restart the API server with the following command:

   ```shell
   docker stop $(docker ps | grep kube-apiserver- | awk '{print $1}')
   
   # Alternative option (depending on the CRI in use).
   crictl stopp $(crictl pods --name=kube-apiserver -q)
   ```

1. After restarting, fix the Secret or delete it with the following command:

   ```shell
   d8 k -n kube-system delete secret audit-policy
   ```

## Redirecting the audit log file to stdout

By default, the audit log is saved to the `/var/log/kube-audit/audit.log` file on master nodes.
If necessary, you can redirect its output to the `kube-apiserver` process stdout instead of a file
by setting the [`apiserver.auditLog.output`](/modules/control-plane-manager/configuration.html#parameters-apiserver-auditlog-output) parameter in the `control-plane-manager` module to `Stdout`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    apiserver:
      auditPolicyEnabled: true
      auditLog:
        output: Stdout
```

In this case, the log will be available in the `kube-apiserver` container stdout.

Then, using the [built-in DVP logging mechanism](/products/virtualization-platform/documentation/admin/platform-management/logging/delivery.html),
you can configure log collection and forwarding to your own security system.
