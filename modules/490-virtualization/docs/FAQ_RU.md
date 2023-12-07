---
title: "Модуль virtualization: FAQ"
---

## Как применить изменения к виртуальной машине?

На данный момент изменения в шаблоне виртуальной машины не применяются к запущенным инстансам автоматически.
Чтобы применить изменения, выполните удаление запущенного инстанса виртуальной машины:

```bash
kubectl delete virtualmachineinstance <vmName>
```

Вновь созданный инстанс виртуальной машины будет включать все последние изменения из ресурса [VirtualMachine](cr.html#virtualmachine).

## Как сохранить образ в registry?

Чтобы сохранить образ в registry, вам необходимо собрать docker image с одной директорией `/disk`, в которую следует положить образ с произвольным именем.
Образ может быть в формате как `qcow2`, так и `raw`.

Пример `Dockerfile`:

```Dockerfile
FROM scratch
ADD https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img /disk/jammy-server-cloudimg-amd64.img
```

## Как выключить модуль виртуализации?

Прежде чем выключить модуль виртуализации, все виртуальные машины и диски должны быть предварительно удалены.

Для удаления модуля, воспользуйтесь следующим [скриптом](https://github.com/deckhouse/deckhouse/blob/main/tools/virtualization/remove-module.sh).
