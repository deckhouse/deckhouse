---
title: "Сloud provider — AWS: настройки"
---

## Параметры

Модуль настраивается автоматически исходя из выбранной схемы размещения (custom resource `AWSClusterConfiguration`). В большинстве случаев нет необходимости ручной конфигурации модуля.

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](/modules/040-node-manager/cr.html#nodegroup) модуля node-manager, в котором также указывается название используемого для этой группы узлов instance-класса (параметр `cloudInstances.classReference` NodeGroup).  Instance-класс для cloud-провайдера AWS — это custom resource [`AWSInstanceClass`](cr.html#awsinstanceclass), в котором указываются конкретные параметры самих машин.

## Storage

Модуль автоматически создаёт StorageClasses, которые есть в AWS: `gp2`, `sc1` и `st1`. Позволяет сконфигурировать диски с необходимым IOPS. А также отфильтровать ненужные StorageClass, указанием их в параметре `exclude`.

* `provision` — дополнительные StorageClass с определенным IOPS.
  * Формат — массив объектов.
    * `name` — имя будущего класса.
    * `type` — тип диска, `io1` или `io2`.
    * `iopsPerGB` — I/O операций в секунду на каждый Гб (у дисков `gp2` этот параметр `3`).
      * **Внимание!** Если это значение, умноженное на размер запрошенного диска, будет меньше 100 или больше 64000, создание такого диска завершится ошибкой.
      * Подробное описание типов дисков и их IOPS, вы найдёте в [официальной документации](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html).
  * Опциональный параметр.
* `exclude` — полные имена (или regex выражения имён) StorageClass, которые не будут созданы в кластере.
  * Формат — массив строк.
  * Опциональный параметр.
* `default` — имя StorageClass, который будет использоваться в кластере по умолчанию.
  * Формат — строка.
  * Опциональный параметр.
  * Если параметр не задан, фактическим StorageClass по умолчанию будет либо: 
    * Присутствующий в кластере произвольный StorageClass с default аннотацией.
    * Лексикографически первый StorageClass из создаваемых модулем.

```yaml
cloudProviderAws: |
  storageClass:
    provision:
    - iopsPerGB: 5
      name: iops-foo
      type: io1
    exclude: 
    - sc.*
    - st1
    default: gp2
```
