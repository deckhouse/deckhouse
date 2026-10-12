---
title: "Cloud provider — GCP: FAQ"
---

## Как поднять кластер

1. Настройте облачное окружение.
2. Включите модуль или передайте флаг `--extra-config-map-data base64_encoding_of_custom_config` [с параметрами модуля](configuration.html) в скрипт установки `install.sh`.
3. Создайте один или несколько custom resource [GCPInstanceClass](cr.html#gcpinstanceclass).
4. Создайте один или несколько custom resource [NodeGroup](/modules/node-manager/cr.html#nodegroup) для управления количеством и процессом заказа машин в облаке.

## Добавление CloudStatic узлов в кластер

К виртуальным машинам, которые вы хотите добавить к кластеру в качестве узлов, добавьте `Network Tag`, аналогичный префиксу кластера.

Префикс кластера можно узнать с помощью команды:

```shell
d8 k get mc global -o jsonpath='{.spec.settings.prefix}{"\n"}'
```

Если значение пустое, возьмите его из устаревшего параметра [`cloud.prefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-cloud-prefix) ресурса ClusterConfiguration (префикс перенесён в ModuleConfig `global`):

```shell
d8 k -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' \
  | base64 -d | grep prefix
```

## Как создать прерываемые узлы в GCP?

Прерываемые виртуальные машины в Google Cloud называются preemptible-инстансами. Чтобы группа узлов заказывала их, установите параметр [`preemptible`](cr.html#gcpinstanceclass-v1-spec-preemptible) ресурса GCPInstanceClass в `true`.

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: worker-preempt
spec:
  machineType: n1-standard-4
  diskSizeGb: 30
  preemptible: true
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-preempt
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: GCPInstanceClass
      name: worker-preempt
    minPerZone: 2
    maxPerZone: 4
    zones:
      - europe-west4-a
```

Вместе с этим параметром Deckhouse Platform (DP) отключает автоматический перезапуск виртуальной машины и задаёт её остановку при обслуживании хоста.

### Ограничения прерываемых узлов в GCP

Учитывайте следующие ограничения:

- Параметр `preemptible` действует только для групп узлов типа CloudEphemeral, потому что GCPInstanceClass применяется только к ним.
- DP не отслеживает уведомления Google Cloud о предстоящем прерывании и не выполняет drain узла заранее. Когда Google Cloud останавливает инстанс, узел переходит в состояние `NotReady`, а группа заказывает новый узел взамен прерванного. До этого момента поды остановленного узла могут отображаться в Kubernetes API в состоянии `Running`, хотя они уже недоступны.
- Размещайте на прерываемых узлах только ту нагрузку, которая переносит внезапную потерю узла.

{% alert level="warning" %}
Изменение параметра [`preemptible`](cr.html#gcpinstanceclass-v1-spec-preemptible) в существующем GCPInstanceClass приводит к пересозданию всех узлов соответствующей группы.
{% endalert %}
