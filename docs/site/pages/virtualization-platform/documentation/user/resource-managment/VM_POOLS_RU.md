---
title: "Пулы виртуальных машин"
permalink: ru/virtualization-platform/documentation/user/resource-management/virtual-machine-pools.html
lang: ru
---

{% alert level="warning" %}
Доступно в редакциях EE и SE+.
{% endalert %}

Ресурс [VirtualMachinePool](/modules/virtualization/cr.html#virtualmachinepool) поддерживает заданное число одинаковых виртуальных машин и позволяет масштабировать их через сабресурс `scale`, HorizontalPodAutoscaler (HPA) или KEDA. Поле `virtualMachineTemplate.spec` совпадает с обычным `VirtualMachineSpec`, поэтому реплика ничем не отличается от вручную созданной виртуальной машины.

Создайте пул с нужным числом реплик и шаблоном. Каждый per-replica диск описывается один раз в `virtualDiskTemplates` (политика reclaim, размер, источник данных), а `blockDeviceRefs` шаблона ссылается на эти диски — по имени, с `kind: VirtualDisk` — задавая порядок устройств (загрузки), ровно как в обычной `VirtualMachine`. Каждая запись `virtualDiskTemplates` должна быть указана в `blockDeviceRefs` ровно один раз (биекция проверяется на admission; имена шаблонов дисков уникальны). Помимо per-replica дисков в `blockDeviceRefs` можно перечислить общие read-only образы (`VirtualImage`/`ClusterVirtualImage`) — например, общий ISO/CD-ROM, подключаемый ко всем репликам, — они не per-replica и записи в `virtualDiskTemplates` не требуют.

```bash
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachinePool
metadata:
  name: runners
  namespace: ci
spec:
  replicas: 3
  scaleDownPolicy: NewestFirst
  virtualMachineTemplate:
    spec:
      runPolicy: AlwaysOn
      virtualMachineClassName: generic
      cpu:
        cores: 2
      memory:
        size: 4Gi
      # Cloud-init: каждая реплика самонастраивается при первом запуске (одинаково для всех).
      provisioning:
        type: UserData
        userData: |
          #cloud-config
          users:
            - name: cloud
              sudo: ALL=(ALL) NOPASSWD:ALL
              ssh_authorized_keys:
                - ssh-ed25519 AAAAC3Nz... user@example
      # Устройства и порядок загрузки (первый = загрузочный). Записи VirtualDisk
      # ссылаются на virtualDiskTemplates по имени (per-replica, резолвит контроллер);
      # VirtualImage/ClusterVirtualImage — общий read-only образ для всех реплик.
      blockDeviceRefs:
        - kind: VirtualDisk
          name: root          # загрузочный диск
        - kind: VirtualDisk
          name: cache
        - kind: ClusterVirtualImage
          name: tools-iso      # общий CD-ROM, подключается ко всем репликам
  # Параметры per-replica дисков (reclaim/размер/источник). Каждый должен быть указан выше.
  virtualDiskTemplates:
    # Записываемый корневой диск: свой на каждую реплику, клонируется из образа, удаляется вместе с репликой.
    - name: root
      reclaim:
        onScaleDown: Delete
      spec:
        persistentVolumeClaim:
          size: 30Gi
        dataSource:
          type: ObjectRef
          objectRef:
            kind: VirtualImage
            name: ubuntu
    # Переиспользуемый кэш: переживает scale-down и переподключается при scale-up.
    - name: cache
      reclaim:
        onScaleDown: Retain
        keep: 5
        ttl: 30m
      spec:
        persistentVolumeClaim:
          size: 50Gi
EOF
```

Реплики именуются `<pool>-<random>`. Диски следуют той же схеме: диск на реплику (`Delete`) называется `<replica>-<template>` (например, `runners-1b2e84-root`), переиспользуемый (`Retain`) получает имя `<pool>-<template>-<random>`. Посмотреть реплики можно через `d8 k get vm -l vmpool.virtualization.deckhouse.io/pool=runners`.

## Подключение общего CD-ROM (или любого общего образа) ко всем репликам

Помимо per-replica дисков, в `blockDeviceRefs` можно сослаться на read-only образы — `ClusterVirtualImage` или `VirtualImage`. Такой образ не per-replica: все реплики подключают один и тот же образ (например, общий ISO с инструментами или драйверами). Образы не перечисляются в `virtualDiskTemplates` (у них нет per-replica состояния) и под биекцию не попадают.

Добавьте образ в `blockDeviceRefs` на нужную позицию в порядке загрузки — для установочного ISO поставьте его перед диском, для CD-ROM с инструментами — после:

```yaml
spec:
  virtualMachineTemplate:
    spec:
      blockDeviceRefs:
        - kind: VirtualDisk           # записываемый корневой диск на реплику, грузится первым
          name: root
        - kind: ClusterVirtualImage   # общий read-only CD-ROM, подключается ко всем репликам
          name: tools-iso
  virtualDiskTemplates:
    - name: root
      spec:
        persistentVolumeClaim:
          size: 30Gi
        dataSource:
          type: ObjectRef
          objectRef:
            kind: ClusterVirtualImage
            name: ubuntu
```

Образ подключается к существующим репликам так же, как любое другое устройство, — изменение `blockDeviceRefs` применяется к живой реплике при её следующем пересоздании (ротация или scale-up).

## Масштабирование

Пул поддерживает стандартный сабресурс `scale`, совместимый с ручным изменением числа реплик и с автоскейлерами.

Чтобы изменить число реплик вручную, выполните:

```bash
d8 k scale virtualmachinepool/runners -n ci --replicas=8
```

Пул публикует `status.selector`, поэтому HPA читает метрики CPU/памяти прямо с реплик без дополнительной обвязки:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: runners
  namespace: ci
spec:
  scaleTargetRef:
    apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: VirtualMachinePool
    name: runners
  minReplicas: 3
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

Помимо CPU/памяти пул работает и с кастомными метриками (`Pods`/`External` через `custom.metrics.k8s.io`/`external.metrics.k8s.io`) и с KEDA, например для масштабирования по длине внешней очереди. При `scaleDownPolicy: Explicit` автоскейлер может только увеличивать число реплик: безадресное сжатие через сабресурс `scale` отклоняется (реплики убираются по имени, см. ниже).

Поле `spec.scaleDownPolicy` определяет, какая реплика удаляется при безадресном сжатии:

- `NewestFirst` — первыми удаляются самые молодые реплики;
- `OldestFirst` — первыми удаляются самые старые реплики;
- `Explicit` — безадресное сжатие запрещено; реплики можно убирать только по имени (см. ниже). Используйте, когда только вызывающая сторона знает, какую реплику можно безопасно убрать (например, простаивающую).

## Удаление конкретных реплик

По умолчанию при сжатии пула контроллер сам выбирает, какую реплику удалить.

Чтобы убрать именно заданные реплики (и сжать пул на это число), используйте сабресурс `scaleDownWith`:

```bash
kubectl create --raw \
  /apis/subresources.virtualization.deckhouse.io/v1alpha2/namespaces/ci/virtualmachinepools/runners/scaledownwith \
  -f - <<'EOF'
{"targets": ["runners-1b2e84", "runners-9c0d11"]}
EOF
```

Обычный `kubectl delete vm` пул не сжимает: контроллер воспринимает это как утрату реплики и создаёт замену.

## Переиспользуемые диски (`reclaim`)

Политика `reclaim` задаёт, что происходит с диском реплики при её удалении из пула.

Параметр `reclaim.onScaleDown` элемента `virtualDiskTemplates` определяет это поведение. `reclaim` необязателен; если не задан, диск считается `Delete`.

- `Delete` (по умолчанию) — диск принадлежит виртуальной машине и удаляется вместе с ней; после реплики ничего не остаётся.
- `Retain` — диск принадлежит пулу, переживает реплику и переподключается к следующей при масштабировании вверх. Подходит для состояния, которое дорого пересоздавать и которое должно пережить пересоздание ВМ, чтобы возврат вверх был «тёплым», а не «холодным».

`keep` и `ttl` настраивают пул свободных `Retain`-дисков (применимы только к `Retain`):

- `keep`: сколько недавно освободившихся дисков всегда держать тёплыми для мгновенного масштабирования вверх; они иммунны к `ttl`.
- `ttl`: сколько свободный диск живёт сверх тёплого буфера до сборки мусором.

Примеры:

```yaml
# Эфемерный диск: удаляется вместе с репликой (Delete по умолчанию).
- name: root
  spec:
    persistentVolumeClaim: { size: 30Gi }
    dataSource: { type: ObjectRef, objectRef: { kind: VirtualImage, name: ubuntu } }

# Переиспользуемый диск: держим 3 тёплых для быстрого scale-up, остальные собираем через 1h простоя.
- name: cache
  reclaim:
    onScaleDown: Retain
    keep: 3
    ttl: 1h
  spec:
    persistentVolumeClaim: { size: 100Gi }

# Переиспользуемый диск без ограничения: переиспользуется всегда, не удаляется автоматически (нет ttl).
- name: data
  reclaim:
    onScaleDown: Retain
  spec:
    persistentVolumeClaim: { size: 20Gi }
```

Неверные комбинации отклоняются при создании/изменении: `keep`/`ttl` допустимы только с `Retain`, а `keep > 0` требует `ttl` (без `ttl` ничего не собирается, и `keep` не имеет эффекта). `Retain`-диск без `ttl` хранит все освободившиеся диски бессрочно; ограничивайте `ttl`, если это не то, что нужно.

## Особенности

Ниже перечислены ограничения и неочевидное поведение пула, о которых стоит помнить при эксплуатации.

- Удаление записи из `virtualDiskTemplates` удаляет её диски. Для `Retain`-дисков это уничтожает переиспользуемые данные, поэтому убирайте шаблон только когда он больше не нужен.
- Пул поддерживает число реплик, а не их здоровье. Существующая, но нездоровая ВМ не пересоздаётся (живучесть чинит рестарт на уровне ВМ), а `Stopped`-реплика сохраняется, а не заменяется; пересоздаётся только полностью удалённая реплика.
- `Retain`-диски общие между репликами. При scale-up новая реплика может получить освободившийся диск другой реплики вместе с его данными; жёсткой привязки между репликой и диском нет.
- Изменение `virtualDiskTemplates[].spec` влияет только на новые диски, кроме `size`, который увеличивает существующие (уменьшать нельзя). `dataSource`, `storageClassName` и прочее к уже созданным дискам не применяются.
- Каждый диск из `virtualDiskTemplates` — per-replica: у каждой реплики своя копия. Общие read-only образы (`VirtualImage`/`ClusterVirtualImage`, например единый ISO/CD-ROM) можно подключить ко всем репликам, перечислив их в `blockDeviceRefs` шаблона; записываемый диск между репликами не разделяется.
- Правка `blockDeviceRefs` шаблона (переупорядочивание, добавление или удаление общего образа) применяется к новым репликам; живые реплики сохраняют текущие устройства до пересоздания (ротация или scale-up), как и другие изменения шаблона, требующие перезапуска.
- Изменения шаблона, требующие перезапуска, применяются только после перезапуска реплики согласно `.spec.disruptions.restartApprovalMode` в шаблоне.
