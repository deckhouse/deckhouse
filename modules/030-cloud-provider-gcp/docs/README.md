---
title: "Модуль cloud-provider-gcp"
---

## Содержимое модуля

1. cloud-controller-manager — контроллер для управления ресурсами облака из Kubernetes.
    * Создаёт route'ы для PodNetwork в cloud provider'е.
    * Создаёт LoadBalancer'ы для Service-объектов Kubernetes с типом LoadBalancer.
    * Синхронизирует метаданные GCP Instances и Kubernetes Nodes. Удаляет из Kubernetes ноды, которых более нет в GCP.
2. CSI storage — для заказа дисков в GCP.
3. Включение необходимого CNI ([simple bridge]({{ site.baseurl }}/modules/035-cni-simple-bridge/)).
4. Регистрация в модуле [node-manager]({{ site.baseurl }}/modules/040-node-manager/), чтобы [GCPInstanceClass'ы](#gcpinstanceclass-custom-resource) можно было использовать в [CloudInstanceClass'ах]({{ site.baseurl }}/modules/040-node-manager/#nodegroup-custom-resource).

## Конфигурация

### Параметры

Модуль настраивается автоматически на основании [выбранной схемы размещения](/candi/cloud-providers/gcp/). Предусмотрены только параметры в отдельных [GCPInstanceClass](#gcpinstanceclass-custom-resource).

### GCPInstanceClass custom resource

Ресурс описывает параметры группы GCP Instances, которые будет использовать machine-controller-manager из модуля [node-manager]({{ site.baseurl }}/modules/040-node-manager/). На этот ресурс ссылается ресурс `CloudInstanceClass` из вышеупомянутого модуля.

Все опции идут в `.spec`.

* `machineType` — тип заказываемых instances. **Внимание!** Следует убедиться, что указанный тип есть во всех зонах, указанных в `zones`.
    * GCP [позволяет указывать](https://cloud.google.com/compute/docs/instances/creating-instance-with-custom-machine-type#create) не стандартное количество CPU и RAM, например: `custom-8-40960` или `n2-custom-8-40960`.
* `image` — образ, который поставится в заказанные instance'ы.
    * Формат — строка, полный путь до образа, пример: `projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20200129a`.
    * **Внимание!** Сейчас поддерживается и тестируется только Ubuntu 18.04/Centos 7.
    * Список образов можно найти в [документации](https://cloud.google.com/compute/docs/images#ubuntu).
    * Опциональный параметр.
* `preemptible` — Заказывать ли preemptible instance.
    * Формат — bool.
    * По-умолчанию `false`.
    * Опциональный параметр.
* `diskType` — тип созданного диска.
    * По-умолчанию `pd-standard`.
    * Опциональный параметр.
* `diskSizeGb` — размер root диска.
    * Формат — integer. В ГиБ.
    * По-умолчанию `50` ГиБ.
    * Опциональный параметр.
* `additionalNetworkTags` — список дополнительных тегов. К примеру, теги позволяют применять правила фаервола к инстансам. Подробно про network tags можно прочитать в [официальной документации](https://cloud.google.com/vpc/docs/add-remove-network-tags).
    * Формат — массив строк.
    * Опциональный параметр.
* `additionalLabels` — список дополнительных лейблов. Подробно про labels можно прочитать в [официальной документации](https://cloud.google.com/resource-manager/docs/creating-managing-labels).
    * Формат — `key: value`.
    * Опциональный параметр.

#### Пример GCPInstanceClass

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: GCPInstanceClass
metadata:
  name: test
spec:
  machineType: n1-standard-1
```

### Storage

Storage настраивать не нужно, модуль автоматически создаст 4 StorageClass'а, покрывающие все варианты дисков в GCP: standard или ssd, region-replicated или not-region-replicated.

1. `pd-standard-not-replicated`
2. `pd-standard-replicated`
3. `pd-ssd-not-replicated`
4. `pd-ssd-replicated`
