---
title: "Хранилище образов виртуальных машин"
permalink: ru/admin/configuration/virtualization/image-storage.html
description: "Внутреннее хранилище образов виртуальных машин (DVCR): размер и класс хранения тома, очистка неактуальных данных по расписанию."
search: DVCR, хранилище образов, размер тома, очистка хранилища, сборка мусора
lang: ru
---

Образы виртуальных машин модуль хранит во внутреннем хранилище образов контейнеров (DVCR), которое размещается на постоянном томе кластера. Оттуда образы попадают на диски виртуальных машин, поэтому от размера тома зависит, сколько образов поместится в кластер.

## Размер и класс хранения

Размер тома и класс хранения задаются в блоке [`.spec.settings.dvcr.storage`](/modules/virtualization/configuration.html#parameters-dvcr-storage). Чтобы расширить хранилище, увеличьте размер тома.

{% alert level="warning" %}
После того как том создан, уменьшить его размер и сменить класс хранения нельзя.
{% endalert %}

## Очистка хранилища образов

Когда образы и диски удаляются из кластера, их данные какое-то время остаются в DVCR. Чтобы хранилище не заполнялось неактуальными данными, модуль запускает сборку мусора по расписанию.
По умолчанию она выполняется ежедневно в 02:00. Задать своё расписание можно параметром [`.spec.settings.dvcr.gc.schedule`](/modules/virtualization/configuration.html#parameters-dvcr-gc-schedule) в [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) `virtualization`:

{% tabs dvcr-gc %}

{% tab "В командной строке" %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  # ...
  settings:
    dvcr:
      gc:
        schedule: "0 20 * * *"
  # ...
```

Пока идёт сборка мусора, хранилище работает в режиме «только чтение», поэтому создание образов и дисков в это время откладывается до её завершения.

Посмотреть, сколько места занято и какие данные будут удалены при следующей сборке, можно командой:

```bash
d8 k -n d8-virtualization exec deploy/dvcr -- dvcr-cleaner gc check
```

Пример вывода:

```console
Found 2 cvi, 5 vi, 1 vd manifests in registry
Found 1 cvi, 5 vi, 11 vd resources in cluster
  Total     Used    Avail     Use%
36.3GiB  13.1GiB  22.4GiB      39%
Images eligible for cleanup:
KIND                   NAMESPACE            NAME
ClusterVirtualImage                         debian-12
VirtualDisk            default              debian-10-root
VirtualImage           default              ubuntu-2404
```
{: .nowrap-default }

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Система», далее в раздел «Deckhouse» → «Модули».
1. Из списка выберите модуль `virtualization`.
1. В открывшемся окне на вкладке «Конфигурация» включите переключатель «Дополнительные настройки».
1. В блоке «Хранилище образов дисков и ISO» в поле «Расписание очистки в формате Cron» задайте расписание.
1. Нажмите кнопку «Сохранить».

{% endtab %}

{% endtabs %}
