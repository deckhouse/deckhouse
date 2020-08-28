---
title: "Сloud provider — GCP: custom resource"
---

## GCPInstanceClass

Ресурс описывает параметры группы GCP Instances, которые будет использовать machine-controller-manager из модуля [node-manager](/modules/040-node-manager/). На этот ресурс ссылается ресурс `CloudInstanceClass` из вышеупомянутого модуля.

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
    * По умолчанию `false`.
    * Опциональный параметр.
* `diskType` — тип созданного диска.
    * По умолчанию `pd-standard`.
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
