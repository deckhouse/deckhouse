---
title: "Setup virtualization"
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
sudo -i d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  enabled: true
  settings:
    dvcr:
      storage:
        persistentVolumeClaim:
          size: 50G
        type: PersistentVolumeClaim
    virtualMachineCIDRs:
    - 10.66.10.0/24
    - 10.66.20.0/24
    - 10.66.30.0/24
  version: 1
EOF
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
