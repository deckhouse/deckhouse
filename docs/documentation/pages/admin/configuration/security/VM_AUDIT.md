---
title: "Virtualization event audit"
permalink: en/admin/configuration/security/events/virtualization-audit.html
description: "Security event audit for virtualization resources: enabling it, the set of events, and viewing them in the cluster logging system."
search: virtualization audit, security events, virtualization-audit, VM audit
---

The audit records actions on virtual machines (VMs) and on the module itself, so that you can investigate an incident and reconstruct the sequence of events.

{% alert level="warning" %}
Available in the DP EE and Ultimate editions.
{% endalert %}

## Enabling the audit

To enable the security event audit, follow these steps:

1. Enable the [`log-shipper`](/modules/log-shipper/) and [`runtime-audit-engine`](/modules/runtime-audit-engine/) modules.
1. Enable the Kubernetes API audit by setting [`.spec.settings.apiserver.auditPolicyEnabled`](/modules/control-plane-manager/configuration.html#parameters-apiserver-auditpolicyenabled) to `true` in the [`control-plane-manager`](/modules/control-plane-manager/) module.
1. Set [`.spec.settings.audit.enabled`](/modules/virtualization/configuration.html#parameters-audit-enabled) to `true` in the module settings:

   ```yaml
   spec:
     settings:
       audit:
         enabled: true
   ```

Until all three conditions are met, the audit component doesn't start in the cluster. For the other parameters, see the [module settings](/modules/virtualization/configuration.html).

## Event types

The event type is recorded in the `type` field. The audit distinguishes the following types:

- `Access to VM`: A connection to a VM over the console, VNC, or port forwarding. Both the start and the end of the session are recorded.
- `Manage VM`: Creating, updating, or deleting a [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) resource.
- `Control VM`: A change of the VM state, including start, stop, restart, migration, and eviction through the [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) resource, as well as a shutdown or restart from the guest OS and an abnormal termination.
- `Module control`: Creating, updating, disabling, or deleting a [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig).
- `Virtualization control`: Creating or deleting a system component of the module in the `d8-virtualization` namespace.
- `Integrity check`: A mismatch between the VM configuration checksum and the reference one.
- `Forbidden operation`: An attempt to perform a forbidden operation.

Regardless of the type, every event contains the same fields:

- `name`: A description of what happened.
- `datetime`: The time of the event.
- `request_subject`: The user or ServiceAccount that performed the action.
- `operation_result`: The result of the operation.
- `uid`: The identifier of the record in the Kubernetes audit.

Additional fields depend on the event type. For example, VM events contain the `virtual_machine_name` and `virtual_machine_namespace` fields, while forbidden operations report the request source in the `source_ip` field and the denial reason in the `forbid_reason` field.

## Viewing events

Events are collected by the `virtualization-audit` system component in the `d8-virtualization` namespace. To forward them to the cluster logging system, for example to [Loki](/modules/loki/), create a [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: virtualization-audit-logs
spec:
  destinationRefs:
    - d8-loki
  kubernetesPods:
    namespaceSelector:
      matchNames:
        - d8-virtualization
    labelSelector:
      matchLabels:
        app: virtualization-audit
  type: KubernetesPods
```

To view the events in [Grafana](/modules/prometheus/), use a [Loki](/modules/loki/) query:

```logql
{namespace="d8-virtualization", pod=~"virtualization-audit-.*"}
```
