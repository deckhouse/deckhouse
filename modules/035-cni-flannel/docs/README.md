# Модуль cni-flannel

## Содержимое модуля

Модуль включается автоматически для следующих cloud-provider'ов:
- openstack
- vsphere

Для включения в bare metall, необходимо в configmap deckhouse добавить:
```
cniFlannelEnabled: "true"
```

### Параметры

* `flannel`:
    * `podNetworkMode` — режим работы `host-gw` или `vxlan`.
        * Значение по умолчанию `host-gw`.

Пример:
```yaml
flannel: |
  podNetworkMode: vxlan
```
