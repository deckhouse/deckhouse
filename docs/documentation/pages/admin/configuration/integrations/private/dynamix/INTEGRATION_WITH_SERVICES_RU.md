---
title: Интеграция с облаком Dynamix
permalink: ru/admin/integrations/private/dynamix/services.html
lang: ru
---

Deckhouse Kubernetes Platform интегрируется с облачной платформой Dynamix и использует [ресурсы DynamixInstanceClass](/modules/cloud-provider-dynamix/cr.html#dynamixinstanceclass) для описания характеристик виртуальных машин, разворачиваемых в кластере.

## Основные возможности

- Заказ и удаление виртуальных машин через API Dynamix;
- Настройка параметров виртуальных машин, включая количество CPU, объём памяти, размер корневого диска;
- Указание шаблона ОС (имя образа) и хранилища;
- Подключение к внешним сетям;
- Использование нескольких групп узлов с индивидуальными параметрами.

Пример описания DynamixInstanceClass:

```yaml
apiVersion: deckhouse.io/v1
kind: DynamixInstanceClass
metadata:
  name: frontend
spec:
  numCPUs: 4
  memory: 8192
  rootDiskSizeGb: 40
  imageName: alt-p10-cloud-x86_64.img
  storageEndpoint: SharedTatlin_G1_SEP
  pool: pool_a
  externalNetwork: extnet_vlan_1700
```

 На него ссылается [параметр cloudInstances.classReference](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference) NodeGroup.

## Рекомендации

- Размещайте образы ОС в разделе «Образы» → «Шаблонные образы» на портале Dynamix.
- Используйте имена образов, точно соответствующие значениям imageName в DynamixInstanceClass.
- Убедитесь, что выбранное хранилище и пул доступны для всех узлов, размещаемых в кластере.
- Проверьте, что виртуальные машины имеют доступ в интернет и DNS-серверы.

Интеграция с облаком обеспечивает автоматическое масштабирование, настройку и управление узлами в соответствии с параметрами, заданными в DynamixInstanceClass и конфигурации кластера.
