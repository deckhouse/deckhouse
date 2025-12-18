Включите модуль виртуализации. В параметре [.spec.settings.virtualMachineCIDRs](/modules/virtualization/configuration.html#parameters-virtualmachinecidrs) модуля укажите подсети, IP-адреса из которых будут назначаться виртуальным машинам:

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
    # Укажите подсети, из которых будут назначаться IP-адреса виртуальным машинам.
    - 10.66.10.0/24
    - 10.66.20.0/24
    - 10.66.30.0/24
  version: 1
EOF
```

{% alert level="warning" %}
Подсети блока `.spec.settings.virtualMachineCIDRs` не должны пересекаться с подсетями узлов кластера, подсетью сервисов или подсетью подов (`podCIDR`).
{% endalert %}
