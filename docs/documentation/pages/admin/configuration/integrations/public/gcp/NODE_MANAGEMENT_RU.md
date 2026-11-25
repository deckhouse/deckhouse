---
title: Управление узлами в Google Cloud Platform
permalink: ru/admin/integrations/public/gcp/node-management.html
lang: ru
---

Количество и параметры процесса заказа машин в облаке настраиваются в кастомном ресурсе [NodeGroup](/modules/node-manager/cr.html#nodegroup), в котором также указывается название используемого для этой [группы узлов](../../../../glossary.html#node-group) [инстанс-класса](../../../../glossary.html#instance-class)
([параметр `cloudInstances.classReference`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference) NodeGroup). Инстанс-класс для cloud-провайдера
GCP — это кастомный ресурс [GCPInstanceClass](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass), в котором указываются конкретные параметры самих машин.

## Включение вложенной виртуализации

Для запуска виртуальных машин (например, KVM) внутри GCP-инстансов необходимо включить вложенную виртуализацию.

{% alert %}
Вложенная виртуализация поддерживается только на определённых типах машин. Список совместимых типов приведён [в документации GCP](https://cloud.google.com/compute/docs/instances/nested-virtualization/overview#supported_machine_types).
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: vm-nodes
spec:
  machineType: n2-standard-8
  nestedVirtualization: true
```

## Добавление CloudStatic узлов в кластер

Для виртуальных машин, которые будут узлами кластера, задайте `Network Tag`, аналогичный префиксу кластера.

Префикс кластера можно узнать с помощью команды:

```shell
d8 k get mc global -o jsonpath='{.spec.settings.prefix}{"\n"}'
```

Если значение пустое, возьмите его из устаревшего параметра [`cloud.prefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-cloud-prefix) модуля `control-plane-manager` (префикс перенесён в ModuleConfig `global`):

```shell
d8 k -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' \
  | base64 -d | grep prefix
```

## Прерываемые узлы

Прерываемые виртуальные машины в Google Cloud называются preemptible-инстансами. Чтобы [группа узлов](../../../../glossary.html#node-group) заказывала их, установите параметр [`preemptible`](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass-v1-spec-preemptible) ресурса GCPInstanceClass в `true`.

Порядок создания таких узлов и особенности их работы описаны в разделе [«Как создать прерываемые узлы в GCP?»](/modules/cloud-provider-gcp/faq.html#как-создать-прерываемые-узлы-в-gcp).

## Замена образа ОС

Образ ОС узлов задаётся параметром [`image`](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass-v1-spec-image) ресурса GCPInstanceClass для узлов типа `CloudEphemeral` и в секции `instanceClass` ресурса GCPClusterConfiguration для узлов типа `CloudPermanent` и master-узлов. Чтобы перевести узлы кластера на новый образ, выполните шаги из раздела [«Как заменить образ ОС в облачном кластере?»](../../../../faq.html#как-заменить-образ-ос-в-облачном-кластере).
