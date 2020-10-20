---
title: "Сloud provider — VMware vSphere: FAQ"
---

## Как мне поднять кластер?

1. Настройте инфраструктурное окружение в соответствии с [требованиями](configuration.html#требования-к-окружениям) к окружению.
2. [Включите](configuration.html) модуль, или передайте флаг `--extra-config-map-data base64_encoding_of_custom_config` с [параметрами модуля](configuration.html#параметры) в скрипт установки `install.sh`.
3. Создайте один или несколько custom resource [VsphereInstanceClass](cr.html#vsphereinstanceclass).
4. Создайте один или несколько custom resource [NodeManager](/modules/040-node-manager/cr.html#nodegroup) для управления количеством и процессом заказа машин в облаке.

## Как мне поднять гибридный (вручную заведённые ноды) кластер?

1. Удалить flannel из kube-system: `kubectl -n kube-system delete ds flannel-ds`;
2. [Включитть(configuration.html) модуль и прописать ему необходимые для работы параметры.

**Важно!** Cloud-controller-manager синхронизирует состояние между vSphere и Kubernetes, удаляя из Kubernetes те узлы, которых нет в vSphere. В гибридном кластере такое поведение не всегда соответствует потребности, поэтому если узел кубернетес запущен не с параметром `--cloud-provider=external`, то он автоматически игнорируется (Deckhouse прописывает `static://` в ноды в в `.spec.providerID`, а cloud-controller-manager такие узлы игнорирует).



