---
title: "Set up virtualization"
permalink: en/virtualization-platform/documentation/admin/install/steps/virtualization.html
---

## Virtualization setup

After configuring the storage, you need to enable the virtualization module. Enabling and configuring the module is done using the ModuleConfig resource.

In the `spec` parameters, you need to set:

- `enabled: true` — flag to enable the module;
- `settings.virtualMachineCIDRs` — subnets, IP addresses from which virtual machines will be assigned IPs;
- `settings.dvcr.storage.persistentVolumeClaim.size` — size of the disk space for storing virtual machine images.

Example of virtualization module configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  enabled: true
  version: 1
  settings:
    dvcr:
      storage:
        persistentVolumeClaim:
          size: 50G
          storageClassName: sds-replicated-thin-r1
        type: PersistentVolumeClaim
    virtualMachineCIDRs:
      - 10.66.10.0/24
```

Wait until all the pods of the module are in the `Running` status:

```shell
sudo -i d8 k get po -n d8-virtualization
```

Example output:

```console
NAME                                         READY   STATUS    RESTARTS      AGE
cdi-apiserver-858786896d-rsfjw               3/3     Running   0             10m
cdi-deployment-6d9b646b5b-8dgmj              3/3     Running   0             10m
cdi-operator-5fdc989d9f-zmk55                3/3     Running   0             10m
dvcr-74dc9c94b-pczhx                         2/2     Running   0             10m
virt-api-78d49dcbbf-qwggw                    3/3     Running   0             10m
virt-controller-6f8fff445f-w866w             3/3     Running   0             10m
virt-handler-g6l9h                           4/4     Running   0             10m
virt-handler-t5fgb                           4/4     Running   0             10m
virt-handler-ztj77                           4/4     Running   0             10m
virt-operator-58dc5459d5-hpps8               3/3     Running   0             10m
virtualization-api-5d69f55947-k6h9n          1/1     Running   0             10m
virtualization-controller-69647d98c6-9rkht   3/3     Running   0             10m
vm-route-forge-288z7                         1/1     Running   0             10m
vm-route-forge-829wm                         1/1     Running   0             10m
vm-route-forge-nq9xr                         1/1     Running   0             10m
```

How to configure the `virtualization` module in the web interface:

- Go to the "System" tab, then to the "Deckhouse" -> "Modules" section.
- Select the `virtualization` module from the list.
- In the pop-up window, select the "Configuration" tab.
- To display the settings, click the "Advanced settings" switch.
- Configure the settings. The names of the fields on the form correspond to the names of the parameters in YAML.
- To apply the settings, click the "Save" button.

### Parameter description

The following are descriptions of the virtualization module parameters.

#### Parameters for enabling/disabling the module

The module state is controlled through the `.spec.enabled` field. Specify:

- `true`: To enable the module.
- `false`: To disable the module.

#### Configuration version

The `.spec.version` parameter defines the version of the configuration schema. The parameter structure may change between versions. The current values are given in the settings section.

#### Deckhouse Virtualization Container Registry (DVCR)

The `.spec.settings.dvcr.storage` block configures a persistent volume for storing images:

- `.spec.settings.dvcr.storage.persistentVolumeClaim.size`: Volume size (for example, `50G`). To expand the storage, increase the value of the parameter.
- `.spec.settings.dvcr.storage.persistentVolumeClaim.storageClassName`: StorageClass name (for example, `sds-replicated-thin-r1`).

{% alert level="warning" %}
The storage serving this storage class (`.spec.settings.dvcr.storage.persistentVolumeClaim.storageClassName`) must be accessible on the nodes where DVCR is running (system nodes, or worker nodes if there are no system nodes).
{% endalert %}

#### Network settings

The `.spec.settings.virtualMachineCIDRs` block specifies subnets in CIDR format (for example, `10.66.10.0/24`). IP addresses for virtual machines are allocated from these ranges automatically or on request.

Example:

```yaml
spec:
  settings:
    virtualMachineCIDRs:
      - 10.66.10.0/24
      - 10.66.20.0/24
      - 10.77.20.0/16
```

The first and the last subnet address are reserved and not available for use.

{% alert level="warning" %}
The subnets in the `.spec.settings.virtualMachineCIDRs` block must not overlap with cluster node subnets, services subnet, or pods subnet (`podCIDR`).

It is forbidden to delete subnets if addresses from them have already been issued to virtual machines.
{% endalert %}

#### Storage class settings for images

The storage class settings for images are defined in the `.spec.settings.virtualImages` parameter of the module settings.

Example:

```yaml
spec:
  ...
  settings:
    virtualImages:
      allowedStorageClassNames:
      - sc-1
      - sc-2
      defaultStorageClassName: sc-1
```

Where:

- `allowedStorageClassNames` (optional): A list of the allowed StorageClasses for creating a VirtualImage that can be explicitly specified in the resource specification.
- `defaultStorageClassName` (optional): The StorageClass used by default when creating a VirtualImage if the `.spec.persistentVolumeClaim.storageClassName` parameter is not set.

#### Storage class settings for disks

The storage class settings for disks are defined in the `.spec.settings.virtualDisks` parameter of the module settings.

Example:

```yaml
spec:
  ...
  settings:
    virtualDisks:
      allowedStorageClassNames:
      - sc-1
      - sc-2
      defaultStorageClassName: sc-1
```

Where:

- `allowedStorageClassNames` (optional): A list of the allowed StorageClass for creating a VirtualDisk that can be explicitly specified in the resource specification.
- `defaultStorageClassName` (optional): The StorageClass used by default when creating a VirtualDisk if the `.spec.persistentVolumeClaim.storageClassName` parameter is not specified.

##### Security event audit configuration

{% alert level="warning" %}
Not available in Community Edition.
{% endalert %}

{% alert level="warning" %}
To set up auditing, the following modules must be enabled:

- `log-shipper`
- `runtime-audit-engine`
{% endalert %}

To enable security event auditing, set the module’s `.spec.settings.audit.enabled` parameter to `true`:

```yaml
spec:
  enabled: true
  settings:
    audit:
      enabled: true
```
