---
title: "Модуль local-path-provisioner"
---

Позволяет пользователям Kubernetes использовать локальное хранилище на узлах.

## Как это работает?

Для каждого CR [LocalPathProvisioner](cr.html) создается соответствующий `StorageClass`.

Допустимая топология для `StorageClass` вычисляется на основе списка `nodeGroup` из CR. Топология используется при шедулинге Pod'ов.

Когда Pod заказывает диск, то:
- создается `HostPath` PV
- `Provisioner` создает на нужном узле локальную папку по пути, состоящем из параметра `path` CR, имени PV и имени PVC.
  
  Пример пути:

  ```shell
  /opt/local-path-provisioner/pvc-d9bd3878-f710-417b-a4b3-38811aa8aac1_d8-monitoring_prometheus-main-db-prometheus-main-0
  ```

## Ограничения

- Ограничение на размер диска не поддерживается для локальных томов.
