Enable the virtualization module. In the parameter [.spec.settings.virtualMachineCIDRs](/modules/virtualization/configuration.html#parameters-virtualmachinecidrs) of the module, specify the subnets, IP addresses from which virtual machines will be assigned:

```shell
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
    # Specify the subnets from which IP addresses will be assigned to virtual machines.
    - 10.66.10.0/24
    - 10.66.20.0/24
    - 10.66.30.0/24
  version: 1
EOF
```
