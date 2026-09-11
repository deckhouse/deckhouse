---
title: "Размещение виртуальных машин по узлам"
permalink: ru/admin/configuration/virtualization/vm-classes-placement.html
description: "Правила VirtualMachineClass, которые ограничивают выбор узлов для виртуальных машин этого класса."
search: размещение по узлам, nodeSelector, tolerations, класс виртуальной машины
lang: ru
---

Необязательный блок [`.spec.nodeSelector`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-nodeselector) ограничивает набор узлов, на которых работают виртуальные машины этого класса. Узлы отбираются по лейблам:

```yaml
spec:
  nodeSelector:
    matchExpressions:
      - key: node.deckhouse.io/group
        operator: In
        values:
          - green
```

{% alert level="warning" %}
Изменение блока [`.spec.nodeSelector`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-nodeselector) затрагивает все виртуальные машины класса сразу. Те из них, что работают на узлах, переставших подходить под новые условия, придётся переместить:

- в коммерческих редакциях DP переносит такие ВМ на подходящие узлы;
- в DP Open ВМ перезапускаются, а момент перезапуска зависит от параметра [`.spec.disruptions.restartApprovalMode`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-disruptions-restartapprovalmode) виртуальной машины, который по умолчанию равен `Manual` и требует подтверждения владельца проекта.
{% endalert %}

Как выполнить операцию в веб-интерфейсе в [форме создания классов ВМ](vm-classes.html#настройки-virtualmachineclass):

1. Нажмите кнопку «Добавить» в блоке «Условия планирования ВМ на узлах» → «Лейблы и выражения».
1. Задайте «Ключ», «Оператор» и «Значение», они соответствуют параметру [`.spec.nodeSelector`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-nodeselector).
1. Нажмите клавишу «Enter», чтобы подтвердить параметры ключа.
1. Нажмите кнопку «Создать».
