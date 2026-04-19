---
title: "Cloud provider — GCP: FAQ"
---

## Как поднять кластер

1. Настройте облачное окружение.
2. Включите модуль или передайте флаг `--extra-config-map-data base64_encoding_of_custom_config` [с параметрами модуля](configuration.html) в скрипт установки `install.sh`.
3. Создайте один или несколько custom resource [GCPInstanceClass](cr.html#gcpinstanceclass).
4. Создайте один или несколько custom resource [NodeGroup](../../modules/node-manager/cr.html#nodegroup) для управления количеством и процессом заказа машин в облаке.

## Добавление CloudStatic узлов в кластер

К виртуальным машинам, которые вы хотите добавить к кластеру в качестве узлов, добавьте `Network Tag`, аналогичный префиксу кластера.

Префикс кластера можно узнать, воспользовавшись следующей командой:

```shell
d8 k -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' \
  | base64 -d | grep prefix
```
