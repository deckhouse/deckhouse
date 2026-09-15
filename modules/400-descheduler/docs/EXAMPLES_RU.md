---
title: "Модуль descheduler: примеры"
---

## Настройка интервала перераспределения подов

Чтобы настроить, как часто модуль `descheduler` будет запускать цикл перераспределения подов, используйте параметр [`deschedulingInterval`](configuration.html#parameters-deschedulinginterval).

Например, чтобы запускать `descheduler` каждые 5 минут, укажите `deschedulingInterval: Frequent`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: descheduler
spec:
  enabled: true
  settings:
    deschedulingInterval: Frequent
```

Поддерживаются следующие значения:

- `Frequent` — запуск каждые 5 минут. Подходит для кластеров, где важно быстрее перераспределять поды;
- `Moderate` — запуск каждые 15 минут (по умолчанию);
- `Rare` — запуск каждые 30 минут. Подходит для кластеров, где важно минимизировать количество перераспределений подов.

## Пример стратегии LowNodeUtilization

```yaml
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: low-node-utilization
spec:
  strategies:
    lowNodeUtilization:
      enabled: true
      thresholds:
        cpu: 20
      targetThresholds:
        cpu: 50
```

## Пример стратегии HighNodeUtilization

```yaml
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: high-node-utilization
spec:
  strategies:
    highNodeUtilization:
      enabled: true
      thresholds:
        cpu: 50
        memory: 50
```

## Защита подов по StorageClass

Параметр [`protectedStorageClasses`](cr.html#descheduler-v1alpha2-spec-protectedstorageclasses) запрещает вытеснять поды, использующие PersistentVolumeClaim из указанных StorageClass. Это полезно для хранилищ, привязанных к узлу (например, `local-path`), где вытеснение пода либо ломает локальность данных, либо оставляет под без возможности запуска.

```yaml
---
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: protect-local-storage
spec:
  protectedStorageClasses:
    - local-path
  strategies:
    lowNodeUtilization:
      enabled: true
      thresholds:
        cpu: 20
      targetThresholds:
        cpu: 50
```

С такой конфигурацией descheduler продолжит перераспределять остальные поды, но под с PersistentVolumeClaim из StorageClass `local-path` вытеснен не будет.

Обратите внимание:

- Если параметр не указан или список пуст, поведение не меняется: поды с PersistentVolumeClaim вытесняются как обычно.
- Под также считается защищённым, если его PersistentVolumeClaim не удалось определить, то есть PVC отсутствует или у него пустой `storageClassName`. Такие поды не вытесняются, пока параметр задан.
