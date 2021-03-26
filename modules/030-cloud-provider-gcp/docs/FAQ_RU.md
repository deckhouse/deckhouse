---
title: "Сloud provider — GCP: FAQ"
---

## Как мне поднять кластер

1. Настройте облачное окружение.
2. Включите модуль, или передайте флаг `--extra-config-map-data base64_encoding_of_custom_config` с [параметрами модуля](configuration.html) в скрипт установки `install.sh`.
3. Создайте один или несколько custom resource [GCPInstanceClass](cr.html#gcpinstanceclass).
4. Создайте один или несколько custom resource [NodeGroup](/modules/040-node-manager/cr.html#nodegroup) для управления количеством и процессом заказа машин в облаке.
