# Модуль cni-flannel

## Содержимое модуля

Модуль включается автоматически для следующих cloud-provider'ов:
- openstack
- vsphere

Для включения в bare-metal, необходимо в configmap deckhouse добавить:
```
cniFlannelEnabled: "true"
```

### Параметры

* `flannel`:
    * `podNetworkMode` — режим работы `host-gw` или `vxlan`.
        * Значение по умолчанию `host-gw`.
        * **Внимание!** Изменять параметр можно только при использовании модуля в bare-metal-кластерах.

Пример:
```yaml
flannel: |
  podNetworkMode: vxlan
```
