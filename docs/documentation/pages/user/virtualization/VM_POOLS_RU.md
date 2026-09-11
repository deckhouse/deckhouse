---
title: "Пулы виртуальных машин"
permalink: ru/user/virtualization/vm-pools.html
description: "Пулы виртуальных машин: создание одинаковых реплик, масштабирование пула, удаление конкретных реплик и переиспользуемые диски."
search: пул ВМ, VirtualMachinePool, реплики, масштабирование пула, reclaim
lang: ru
---

{% alert level="warning" %}
Доступно в коммерческих редакциях DP.
{% endalert %}

Ресурс [VirtualMachinePool](/modules/virtualization/cr.html#virtualmachinepool) поддерживает заданное число одинаковых виртуальных машин и позволяет масштабировать их через субресурс `scale`, HorizontalPodAutoscaler (HPA) или KEDA. Поле `virtualMachineTemplate.spec` совпадает с обычным `VirtualMachineSpec`, поэтому реплика ничем не отличается от вручную созданной виртуальной машины.

{% alert level="warning" %}
Тип ОС `Legacy` в пуле не поддерживается, потому что реплики различают начальной инициализацией, которой у этих операционных систем нет, поэтому каждая реплика оказалась бы побайтовой копией одного диска — для гостевых ОС семейства Windows это ещё и одинаковый SID в сети. Шаблон пула с `osType: Legacy` отклоняется. Создавайте такие виртуальные машины по отдельности.
{% endalert %}

Ниже показано, как создать пул виртуальных машин:

{% tabs pool-create %}

{% tab "В командной строке" %}

Создайте пул с нужным числом реплик и шаблоном виртуальной машины. Диски пула описываются в двух блоках:

- `virtualDiskTemplates` описывает каждый диск реплики один раз, задавая политику `reclaim`, размер и источник данных;
- `blockDeviceRefs` шаблона ссылается на эти диски по имени с `kind: VirtualDisk` и задаёт порядок устройств, то есть порядок загрузки, ровно как в обычной [VirtualMachine](/modules/virtualization/cr.html#virtualmachine).

Каждая запись `virtualDiskTemplates` должна встречаться в `blockDeviceRefs` ровно один раз, иначе DP отклонит пул. Имена шаблонов дисков при этом уникальны.

Помимо дисков реплики в `blockDeviceRefs` можно перечислить общие образы [VirtualImage](/modules/virtualization/cr.html#virtualimage) и [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage), например единый ISO или CD-ROM для всех реплик. Такие образы подключаются только на чтение, они одни на весь пул, и записи в `virtualDiskTemplates` им не нужны.

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
                - <SSH_PUBLIC_KEY>
      # Устройства и порядок загрузки (первый = загрузочный). Записи VirtualDisk
      # ссылаются на virtualDiskTemplates по имени, у каждой реплики свой диск;
      # VirtualImage/ClusterVirtualImage — общий образ только для чтения для всех реплик.
      blockDeviceRefs:
        - kind: VirtualDisk
          name: root          # загрузочный диск
        - kind: VirtualDisk
          name: cache
        - kind: ClusterVirtualImage
          name: tools-iso      # общий CD-ROM, подключается ко всем репликам
  # Параметры дисков реплики (reclaim, размер, источник). Каждый должен быть указан выше.
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
    # Переиспользуемый кеш, переживает уменьшение пула и переподключается при росте.
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

Реплики именуются `<POOL>-<RANDOM>`. Диски следуют той же схеме, и диск на реплику (`Delete`) называется `<REPLICA>-<TEMPLATE>` (например, `runners-1b2e84-root`), переиспользуемый (`Retain`) получает имя `<POOL>-<TEMPLATE>-<RANDOM>`. Посмотреть реплики можно через `d8 k get vm -l vmpool.virtualization.deckhouse.io/pool=runners`.

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Пулы ВМ».
1. Нажмите кнопку «Создать».
1. В открывшемся окне «Создать ресурс» в поле «Имя» введите имя пула.
1. На вкладке «Конфигурация» задайте число реплик в поле «Replicas» и политику удаления реплик в поле «Scale Down Policy».
1. В блоке «Virtual Disk Templates» опишите диски реплик, в блоке «Virtual Machine Template» — шаблон виртуальной машины.
1. Нажмите кнопку «Применить».

> Форма пула построена по спецификации ресурса [VirtualMachinePool](/modules/virtualization/cr.html#virtualmachinepool), поэтому названия полей совпадают с параметрами ресурса. Готовую спецификацию можно вставить на вкладке «YAML».

{% endtab %}

{% endtabs %}

## Подключение общего CD-ROM (или любого общего образа) ко всем репликам

Помимо дисков реплики, в `blockDeviceRefs` можно сослаться на образы только для чтения, [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) или [VirtualImage](/modules/virtualization/cr.html#virtualimage). Такой образ общий, и все реплики подключают один и тот же файл, например ISO с инструментами или драйверами. В `virtualDiskTemplates` образы не перечисляются, потому что своей копии у реплики для них нет, и во взаимно однозначное соответствие с шаблонами они не входят.

Добавьте образ в `blockDeviceRefs` на нужную позицию в порядке загрузки. Установочный ISO поставьте перед диском, а CD-ROM с инструментами после него:

```yaml
spec:
  virtualMachineTemplate:
    spec:
      blockDeviceRefs:
        - kind: VirtualDisk           # Записываемый корневой диск на реплику, грузится первым.
          name: root
        - kind: ClusterVirtualImage   # Общий CD-ROM только для чтения, подключается ко всем репликам.
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

Образ подключается к существующим репликам так же, как любое другое устройство. Изменение `blockDeviceRefs` применяется к работающей реплике при её следующем пересоздании, будь то обновление реплик или рост пула.

## Масштабирование пула

Число реплик в пуле меняется вручную или автоматически, средствами автомасштабирования.

{% tabs pool-scale %}

{% tab "В командной строке" %}

Пул поддерживает стандартный субресурс `scale`, совместимый с ручным изменением числа реплик и со средствами автомасштабирования.

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

Помимо процессора и памяти пул работает и с кастомными метриками (`Pods`/`External` через `custom.metrics.k8s.io`/`external.metrics.k8s.io`) и с KEDA, например для масштабирования по длине внешней очереди. При `scaleDownPolicy: Explicit` автомасштабирование может только увеличивать число реплик, а безадресное сжатие через субресурс `scale` отклоняется, а реплики убираются по имени.

Поле `spec.scaleDownPolicy` определяет, какая реплика удаляется при безадресном сжатии:

- `NewestFirst` — первыми удаляются самые молодые реплики;
- `OldestFirst` — первыми удаляются самые старые реплики;
- `Explicit` — безадресное сжатие запрещено; реплики можно убирать только по имени. Используйте, когда только вызывающая сторона знает, какую реплику можно безопасно убрать (например, простаивающую).

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Пулы ВМ».
1. Из списка выберите нужный пул и нажмите на его имя.
1. На вкладке «Конфигурация» задайте новое значение в поле «Replicas».
1. Нажмите кнопку «Применить».
1. Ход масштабирования отображается в списке пулов в колонках «Статус» и «Готово».

{% endtab %}

{% endtabs %}

## Удаление конкретных реплик

По умолчанию при сжатии пула контроллер сам выбирает, какую реплику удалить.

Чтобы убрать именно заданные реплики (и сжать пул на это число), используйте субресурс `scaleDownWith`:

```bash
d8 k create --raw \
  /apis/subresources.virtualization.deckhouse.io/v1alpha2/namespaces/ci/virtualmachinepools/runners/scaledownwith \
  -f - <<'EOF'
{"targets": ["runners-1b2e84", "runners-9c0d11"]}
EOF
```

Обычный `d8 k delete vm` пул не сжимает, потому что контроллер воспринимает это как утрату реплики и создаёт замену.

## Переиспользуемые диски (reclaim)

Политика `reclaim` задаёт, что происходит с диском реплики при её удалении из пула.

Параметр `reclaim.onScaleDown` элемента `virtualDiskTemplates` определяет это поведение. `reclaim` необязателен; если не задан, диск считается `Delete`.

- `Delete` (по умолчанию) — диск принадлежит виртуальной машине и удаляется вместе с ней; после реплики ничего не остаётся.
- `Retain` — диск принадлежит пулу, переживает реплику и переподключается к следующей при масштабировании вверх. Подходит для состояния, которое дорого пересоздавать и которое должно пережить пересоздание ВМ, чтобы возврат вверх был «тёплым», а не «холодным».

`keep` и `ttl` настраивают пул свободных `Retain`-дисков (применимы только к `Retain`):

- `keep` — сколько недавно освободившихся дисков всегда держать тёплыми для мгновенного масштабирования вверх. На них не действует `ttl`.
- `ttl` — сколько свободный диск живёт сверх тёплого буфера до сборки мусором.

Примеры:

```yaml
# Эфемерный диск: удаляется вместе с репликой (Delete по умолчанию).
- name: root
  spec:
    persistentVolumeClaim: { size: 30Gi }
    dataSource: { type: ObjectRef, objectRef: { kind: VirtualImage, name: ubuntu } }

# Переиспользуемый диск. Три остаются наготове для быстрого роста пула, остальные освобождаются через 1h простоя.
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

Неверные комбинации отклоняются при создании и изменении. Параметры `keep` и `ttl` допустимы только с `Retain`, а `keep > 0` требует `ttl`, потому что без `ttl` ничего не собирается и `keep` ни на что не влияет. `Retain`-диск без `ttl` хранит все освободившиеся диски бессрочно; ограничивайте `ttl`, если это не то, что нужно.

## Ограничения и особенности пула

Ниже перечислены ограничения и неочевидное поведение пула, о которых стоит помнить при эксплуатации.

- Удаление записи из `virtualDiskTemplates` удаляет её диски. Для `Retain`-дисков это уничтожает переиспользуемые данные, поэтому убирайте шаблон только когда он больше не нужен.
- Пул поддерживает число реплик, а не их работоспособность. Существующая, но неисправная машина не пересоздаётся, её восстанавливает перезапуск на уровне самой машины. Остановленная реплика сохраняется, а не заменяется, и пересоздаётся только полностью удалённая.
- Диски с политикой `Retain` общие между репликами. При росте пула новая реплика может получить освободившийся диск другой реплики вместе с его данными, жёсткой привязки между репликой и диском нет.
- Изменение `virtualDiskTemplates[].spec` влияет только на новые диски, кроме `size`, который увеличивает существующие (уменьшать нельзя). `dataSource`, `storageClassName` и прочее к уже созданным дискам не применяются.
- У каждой реплики своя копия каждого диска из `virtualDiskTemplates`. Общий образ только для чтения, [VirtualImage](/modules/virtualization/cr.html#virtualimage) или [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage), например единый ISO, можно подключить ко всем репликам, перечислив его в `blockDeviceRefs` шаблона, а записываемый диск между репликами не разделяется.
- Правка `blockDeviceRefs` в шаблоне, будь то перестановка, добавление или удаление общего образа, применяется к новым репликам. Работающие реплики сохраняют текущие устройства до пересоздания, как и при других изменениях шаблона, требующих перезапуска.
- Изменения шаблона, требующие перезапуска, применяются только после перезапуска реплики согласно [`.spec.disruptions.restartApprovalMode`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-disruptions-restartapprovalmode) в шаблоне.
