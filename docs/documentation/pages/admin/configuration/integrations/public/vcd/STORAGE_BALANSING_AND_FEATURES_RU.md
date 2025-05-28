---
title: Хранилище и балансировка
permalink: ru/admin/integrations/public/vcd/vcd-storage.html
lang: ru
---

## Хранилище

Deckhouse использует CSI-драйвер VMware Cloud Director для заказа и подключения дисков.

- Диски создаются как **VCD Independent Disks**.
- Для корректной работы требуется, чтобы шаблон виртуальной машины имел включённое guest-свойство `disk.EnableUUID`.
- Поддерживается изменение размера дисков начиная с версии Deckhouse v1.59.1.

Конфигурация хранилища задаётся через ресурс ModuleConfig модуля `cloud-provider-vcd`. Можно исключить ненужные StorageClass по шаблону имени:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vcd
spec:
  version: 1
  enabled: true
  settings:
    storageClass:
      exclude:
        - ".*-hdd"
        - iscsi-fast
```

По умолчанию создаются StorageClass на основе всех доступных в VMware Cloud Director StorageProfile.

## Балансировка нагрузки

Для проброса внешнего трафика используются возможности Edge Gateway:

- Рекомендуется использовать MetalLB в режиме L2 для назначения IP-адресов frontend-узлам.
- Настройка DNAT и firewall выполняется на стороне Edge Gateway (см. раздел по настройке).
- Проброс портов 80/443/22 обязателен для корректной работы приложений, HTTPS и доступа к control plane.

Также можно вручную настроить SNAT или правила firewall для дополнительных сервисов кластера.
