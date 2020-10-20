---
title: "Сloud provider — GCP: FAQ"
---

## Как мне поднять кластер

1. [Настройте](configuration.html#настройка-окружения) облачное окружение. Возможно, [автоматически](configuration.html#автоматизированная-подготовка-окружения).
2. [Включите](configuration.html) модуль, или передайте флаг `--extra-config-map-data base64_encoding_of_custom_config` с [параметрами модуля](configuration.html#параметры) в скрипт установки `install.sh`.
3. Создайте один или несколько custom resource [GCPInstanceClass](cr.html#gcpinstanceclass).
4. Создайте один или несколько custom resource [NodeGroup](/modules/040-node-manager/cr.html#nodegroup) для управления количеством и процессом заказа машин в облаке.
