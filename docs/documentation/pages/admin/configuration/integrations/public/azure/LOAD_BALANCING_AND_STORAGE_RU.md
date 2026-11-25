---
title: Хранилище и балансировка нагрузки в Microsoft Azure
permalink: ru/admin/integrations/public/azure/load-balancing-and-storage.html
lang: ru
---

## Хранилище

При работе с Azure Deckhouse Platform автоматически создаёт следующие StorageClass:

| Имя                    | Тип диска        |
| ---------------------- | ---------------- |
| `managed-standard`     | Standard\_LRS    |
| `managed-standard-ssd` | StandardSSD\_LRS |
| `managed-premium`      | Premium\_LRS     |

Дополнительно можно:

- Отключить ненужные классы хранилища через [параметр `exclude`](/modules/cloud-provider-azure/configuration.html#parameters-storageclass-exclude).
- Создать свои StorageClass с нужными параметрами пропускной способности и IOPS.

Пример настройки в ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-azure
spec:
  version: 1
  enabled: true
  settings:
    storageClass:
      provision:
      - name: managed-ultra-ssd
        type: UltraSSD_LRS
        diskIOPSReadWrite: 600
        diskMBpsReadWrite: 150
      exclude:
      - managed-standard.*
      - managed-premium
```

## Балансировка нагрузки

Deckhouse Platform автоматически создает ресурсы LoadBalancer в Azure при использовании Kubernetes-сервисов типа LoadBalancer.

{% alert level="info" %}
Весь исходящий трафик из кластера проходит через балансировщики нагрузки. Если ни один из них не настроен для работы с UDP, весь исходящий UDP-трафик будет блокироваться, что может повлиять на работу NTP (`ntpdate`, `chrony` и др.). Решение: вручную добавить правило на любой UDP-порт в существующий LoadBalancer или создать Kubernetes-сервис типа LoadBalancer с UDP-портом.
{% endalert %}
