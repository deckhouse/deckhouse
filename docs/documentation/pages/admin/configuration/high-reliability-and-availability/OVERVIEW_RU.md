---
title: Высокая надежность и доступность
permalink: ru/admin/high-reliability-and-availability/overview.html
description: Высокая надежность и доступность
lang: ru
---

Кластер под управлением Deckhouse Kubernetes Platform поддерживает режим высокой надежности и доступности (High Availability или HA).
В этом режиме повышается общая отказоустойчиовсть всей системы и надежность кластера.

При включенном режиме HA критически важные компоненты кластера запускаются с учетом требуемой избыточности для обеспечения непрерывной работы. В случае отказа любого из экземпляров работа компонентов не прерывается.

## Включение режима High Availability

{% alert level="info" %}
Обратите внимание, что если в кластере **более одного master-узла**, режим отказоустойчивости **включается автоматически**. Это правило верно как при развёртывании кластера сразу с тремя master-узлами, так и при увеличении master-узлов с одного до трёх.
{% endalert %}

Чтобы включить режим HA, установите в `ModuleConfig/global` параметр `settings.highAvailability` в значение `true`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 2
  settings: 
    highAvailability: true
...
```

Убедиться, что режим включился, можно, посмотрев, например, количество подов `deckhouse` в пространстве имён `d8-system`. Для этого выполните команду:

```text
$ sudo -i d8 k -n d8-system get po | grep deckhouse
```

Количество подов deckhouse должно быть больше одного:

```text
deckhouse-57695f4d68-8rk6l                           2/2     Running   0             3m49s
deckhouse-5764gfud68-76dsb                           2/2     Running   0             3m49s
deckhouse-fgrhy4536s-fhu6s                           2/2     Running   0             3m49s
```

Также доступно включение режима HA [для конкретных поддерживающих его модулей DKP](./manual.html).


