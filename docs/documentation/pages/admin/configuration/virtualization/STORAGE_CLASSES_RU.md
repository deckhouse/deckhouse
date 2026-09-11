---
title: "Хранилища для дисков и образов виртуальных машин"
permalink: ru/admin/configuration/virtualization/storage-classes.html
description: "Ограничение списка классов хранения, доступных для дисков и образов виртуальных машин, и выбор класса по умолчанию."
search: классы хранения, StorageClass, allowedStorageClassSelector, диски ВМ
lang: ru
---

Класс хранения для образа или диска выбирает владелец проекта. Вы можете ограничить этот выбор и задать класс, который применяется по умолчанию. За образы отвечает блок [`.spec.settings.virtualImages`](/modules/virtualization/configuration.html#parameters-virtualimages), за диски — блок [`.spec.settings.virtualDisks`](/modules/virtualization/configuration.html#parameters-virtualdisks).

Пример:

```yaml
spec:
  settings:
    virtualImages:
      allowedStorageClassSelector:
        matchNames:
          - sc-1
          - sc-2
      defaultStorageClassName: sc-1
    virtualDisks:
      allowedStorageClassSelector:
        matchNames:
          - sc-3
      defaultStorageClassName: sc-3
```

Оба блока устроены одинаково и оба необязательны. Параметр `allowedStorageClassSelector.matchNames` перечисляет классы, которые разрешено выбирать в спецификации [VirtualImage](/modules/virtualization/cr.html#virtualimage) и [VirtualDisk](/modules/virtualization/cr.html#virtualdisk), а `defaultStorageClassName` задаёт класс для тех ресурсов, где параметр [`.spec.persistentVolumeClaim.storageClassName`](/modules/virtualization/cr.html#virtualdisk-v1alpha2-spec-persistentvolumeclaim-storageclassname) не задан.
