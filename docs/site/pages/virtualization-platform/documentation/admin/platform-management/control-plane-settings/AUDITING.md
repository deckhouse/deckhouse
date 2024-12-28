---
title: "Audit"
permalink: en/virtualization-platform/documentation/admin/platform-management/control-plane-settings/audit.html
---

## Audit

To diagnose API operations, for example, in case of unexpected behavior of control plane components, Kubernetes provides a logging mode for API operations. This mode can be configured by creating [Audit Policy](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy) rules, and the result of the audit will be a log file `/var/log/kube-audit/audit.log` with all the operations of interest. More details can be found in the [Auditing](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/) section of the Kubernetes documentation.

## Basic audit policies

By default, Deckhouse clusters have basic audit policies:
- logging operations for creating, deleting, and modifying resources;
<!-- TODO what resources are meant here? It would be necessary to clarify. -->
- logging actions performed on behalf of service accounts from system namespaces: `kube-system`, `d8-*`;
- logging actions performed with resources in system namespaces: `kube-system`, `d8-*`.

### Disabling basic policies

You can disable log collection by basic policies by setting the [basicAuditPolicyEnabled](https://deckhouse.io/products/virtualization-platform/reference/mc.html#control-plane-manager-parameters-apiserver-basicauditpolicyenabled) flag to `false`.

An example of enabling auditing in kube-apiserver, but without Deckhouse basic policies:

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

You can use the patch:

```shell
d8 k patch mc control-plane-manager --type=strategic -p '{"settings":{"apiserver":{"auditPolicyEnabled":true, "basicAuditPolicyEnabled": false}}}'
```

## Custom Audit Policies

The control-plane manager module automates the configuration of kube-apiserver to add custom audit policies. To make these additional policies work, you need to make sure that audit is enabled in the `apiserver` parameters section and create a secret with the audit policy:

1. Enable the [auditPolicyEnabled](https://deckhouse.io/products/virtualization-platform/reference/mc.html#control-plane-manager-parameters-apiserver-auditpolicyenabled) parameter in the module settings:

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

   You can enable it by editing the resource or using a patch:

   ```shell
   d8 k patch mc control-plane-manager --type=strategic -p '{"settings":{"apiserver":{"auditPolicyEnabled":true}}}'
   ```

1. Create a Secret `kube-system/audit-policy` with a Base64-encoded policy YAML file:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: audit-policy
     namespace: kube-system
   data:
     audit-policy.yaml: <base64>
   ```

   For example, `audit-policy.yaml` can contain a rule for logging all changes in metadata:

   ```yaml
   apiVersion: audit.k8s.io/v1
   kind: Policy
   rules:
   - level: Metadata
     omitStages:
     - RequestReceived
   ```

   For examples and information on audit policy rules, see:

- [Official Kubernetes documentation](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy).
- [GCE generator script code](https://github.com/kubernetes/kubernetes/blob/0ef45b4fcf7697ea94b96d1a2fe1d9bffb692f3a/cluster/gce/gci/configure-helper.sh#L722-L862).

{% alert level="danger" %}
The current implementation does not validate the contents of additional policies.

If unsupported options are specified in the policy in `audit-policy.yaml` or there is a typo, `apiserver` will not start, which will result in the control plane being unavailable.
{% endalert %}

In this case, to restore, you will need to manually remove the `--audit-log-*` parameters in the `/etc/kubernetes/manifests/kube-apiserver.yaml` manifest and restart `apiserver` with the following command:

```bash
crictl stopp $(crictl pods --name=kube-apiserver -q)
```

After the restart there will be enough time to remove the erroneous secret:

```bash
d8 k -n kube-system delete secret audit-policy
```

## How to work with the audit log?

It is assumed that the master nodes have a log collector *(for example, [log-shipper](../../../../reference/cr/clusterloggingconfig.html), promtail, filebeat)*, which will send records from the file to the centralized storage:

```bash
/var/log/kube-audit/audit.log
```

The log file rotation parameters are preset and cannot be changed:
- Maximum disk space is `1000 MB`.
- Maximum write depth is `7 days`.

Please note that "maximum write depth" does not mean "guaranteed". The intensity of writing to the log depends on the settings of additional policies and the number of requests to **apiserver**, so the actual storage depth can be much less than 7 days, for example, 30 minutes. This should be taken into account when configuring the log collector and when writing audit policies.

## Outputting the audit log to standard output

If the cluster has a log collector configured from pods, you can collect the audit log by outputting it to standard output. To do this, set the value `Stdout` in the [apiserver.auditLog.output](https://deckhouse.io/products/virtualization-platform/reference/mc.html#control-plane-manager-parameters-apiserver-auditlog) parameter in the module settings (<https://deckhouse.io/products/virtualization-platform/reference/mc.html#control-plane-manager-parameters-apiserver-auditlog>).

Example of enabling auditing with output to stdout:

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

You can use the patch:

```shell
d8 k patch mc control-plane-manager --type=strategic -p '{"settings":{"apiserver":{"auditPolicyEnabled":true, "auditLog":{"output":"Stdout"}}}}'
```

After restarting kube-apiserver, you can see audit events in its log:

```shell
d8 k -n kube-system logs $(d8 k -n kube-system get po -l component=kube-apiserver -oname | head -n1)

{"kind":"Event","apiVersion":"audit.k8s.io/v1","level":"Metadata","auditID":"38a26239-7f3e-402f-8c56-2fb57a3fe49d","stage":"ResponseComplete","requestURI": ...
```
