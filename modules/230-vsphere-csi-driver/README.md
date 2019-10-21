Модуль vsphere-csi-driver
=======

Весь функционал модуля перенесён в модуль [cloud-provider-vsphere](modules/030-cloud-provider-vsphere). Этот модуль сохранён для обратной совместимости и корректной работы уже созданных StorageClass'ов.

Миграция в модуль [cloud-provider-vsphere](modules/030-cloud-provider-vsphere)
---------

Перед включением нового модуля, следует добавить в конфигурацию этого модуля флаг `disableCSI: true`, чтобы отключить все CSI компоненты.

Пример:

```yaml
vsphereCsiDriver:
  disableCSI: true
```
