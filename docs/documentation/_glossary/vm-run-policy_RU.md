---
title: Политика запуска ВМ
description: Политика, которая определяет, как Deckhouse Platform поддерживает состояние виртуальной машины. Задаётся параметром runPolicy машины.
lang: ru
---

Допустимые значения: `AlwaysOnUnlessStoppedManually` (по умолчанию, машина всегда работает, остановить её можно только вручную), `AlwaysOn` (машина запускается снова даже после выключения из гостевой ОС), `Manual` (состоянием управляет пользователь) и `AlwaysOff` (машина всегда выключена, запустить её нельзя).

Состоянием [виртуальной машины](glossary.html#virtual-machine) управляют через [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) или командами `d8 v start`, `d8 v stop`, `d8 v restart` и `d8 v evict`, которые создают такую же операцию.

Подробнее в разделе [«Подключение к виртуальной машине»](user/virtualization/vm-access.html).
