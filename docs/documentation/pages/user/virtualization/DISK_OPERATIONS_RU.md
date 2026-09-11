---
title: "Изменение размера и миграция дисков виртуальных машин"
permalink: ru/user/virtualization/disk-operations.html
description: "Изменение размера диска виртуальной машины и перенос диска в другое хранилище сменой класса хранения."
search: изменение размера диска, миграция диска, storageClassName, расширение диска
lang: ru
---

Уже созданный диск можно увеличить или перенести в другое хранилище, не удаляя его и не пересоздавая машину.

## Изменение размера диска

Диск можно увеличить, даже если он подключён к работающей виртуальной машине. Уменьшить диск нельзя.

{% tabs vd-resize %}

{% tab "В командной строке" %}

1. Посмотрите текущий размер диска:

   ```bash
   d8 k get vd linux-vm-root
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME            PHASE   CAPACITY   VIRTUALMACHINE   AGE
   linux-vm-root   Ready   10Gi       linux-vm         10m
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Задайте новый размер в параметре [`.spec.persistentVolumeClaim.size`](/modules/virtualization/cr.html#virtualdisk-v1alpha2-spec-persistentvolumeclaim-size):

   ```bash
   d8 k patch vd linux-vm-root --type merge -p '{"spec":{"persistentVolumeClaim":{"size":"11Gi"}}}'

   # Того же результата можно добиться, отредактировав ресурс.
   d8 k edit vd linux-vm-root
   ```

1. Убедитесь, что размер изменился:

   ```bash
   d8 k get vd linux-vm-root
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME            PHASE   CAPACITY   VIRTUALMACHINE   AGE
   linux-vm-root   Ready   11Gi       linux-vm         12m
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

{% endtab %}

{% tab "В веб-интерфейсе" %}

Размер можно изменить со страницы виртуальной машины:

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Виртуальные машины».
1. Из списка выберите ВМ, к которой подключён диск, и нажмите на её имя.
1. На вкладке «Конфигурация» в разделе «Диски» нажмите на символ карандаша рядом с размером диска.
1. В открывшемся окне укажите больший размер.
1. Нажмите кнопку «Применить».

Либо со страницы самого диска:

1. Перейдите в раздел «Виртуализация» → «Диски».
1. Выберите нужный диск и нажмите на его имя.
1. На вкладке «Конфигурация» в поле «Размер» укажите больший размер.
1. Нажмите появившуюся кнопку «Сохранить».
1. Проверьте статус диска на его странице.

{% endtab %}

{% endtabs %}

## Миграция дисков на другие хранилища

В коммерческих редакциях Deckhouse Platform (DP) диск можно перенести в другое хранилище, изменив его класс хранения. Перенос работает и для дисков, заданных в спецификации ВМ, и для подключённых отдельным ресурсом.

{% alert level="warning" %}
Виртуальная машина должна находиться в фазе `Running`, а исходное и целевое хранилища должны быть одного типа. Перенести диск с тома файловой системы на блочное устройство или наоборот нельзя.
{% endalert %}

Чтобы перенести диск, укажите новый класс хранения в параметре [`.spec.persistentVolumeClaim.storageClassName`](/modules/virtualization/cr.html#virtualdisk-v1alpha2-spec-persistentvolumeclaim-storageclassname):

```bash
d8 k patch vd disk --type=merge --patch '{"spec":{"persistentVolumeClaim":{"storageClassName":"new-storage-class-name"}}}'

# Того же результата можно добиться, отредактировав ресурс.
d8 k edit vd disk
```

После этого запускается живая миграция ВМ, в ходе которой диск переезжает в новое хранилище.

Если перенести нужно несколько дисков одной машины, меняйте класс хранения последовательно, по одному диску за раз:

```bash
d8 k patch vd disk1 --type=merge --patch '{"spec":{"persistentVolumeClaim":{"storageClassName":"new-storage-class-name"}}}'
d8 k patch vd disk2 --type=merge --patch '{"spec":{"persistentVolumeClaim":{"storageClassName":"new-storage-class-name"}}}'
```

Неудачную миграцию DP повторяет с растущей задержкой. Первая попытка идёт сразу, следующие через 5 и 10 секунд, дальше задержка удваивается и с седьмой попытки остаётся равной 300 секундам. Чтобы отменить миграцию, верните в спецификации прежний класс хранения.
