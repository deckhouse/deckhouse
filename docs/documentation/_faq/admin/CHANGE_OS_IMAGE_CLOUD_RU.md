---
title: Как заменить образ ОС в облачном кластере?
subsystems:
  - cluster_infrastructure
lang: ru
---

Замена образа ОС в облачном кластере устроена одинаково для всех провайдеров: нужно изменить ссылку на образ в конфигурации DKP. После этого узлы перезаказываются тем же механизмом, которым они были созданы — Terraform, MCM или CAPI.

Где менять конфигурацию:

* для [CloudEphemeral](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/cloud-node.html#добавление-cloudephemeral-узлов-в-облачном-кластере) — в ресурсе `<PROVIDER>InstanceClass`;
* для [CloudPermanent](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/cloud-node.html#добавление-cloudpermanent-узлов-в-облачном-кластере) и master-узлов — в `<PROVIDER>ClusterConfiguration` в `instanceClass` соответствующей группы узлов.

Если провайдер уже переведён на конфигурацию через [ModuleConfig](/products/kubernetes-platform/documentation/v1/faq.html#как-мигрировать-облачный-провайдер-на-конфигурацию-через-modulec) (например, DVP), параметры ВМ задаются через `<PROVIDER>InstanceClass` и для эфемерных, и для перманентных узлов.

Имя и тип поля с образом зависят от провайдера и инфраструктуры и не унифицированы. Например:

* в VCD это строка `template`;
* в DVP — объект `image` с полями `kind` и `name`.

Смотрите описание нужного поля в документации cloud-провайдера.

## Порядок замены

1. Создайте в облаке **новый** образ или шаблон. Если образы версионируются, включайте версию или дату в имя, например `ubuntu-24-04-20260110` → `ubuntu-24-04-20260204`.
1. Укажите новый образ в конфигурации DKP.

   Пример для VMware Cloud Director (`VCDInstanceClass`):

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: VCDInstanceClass
   metadata:
     name: worker
   spec:
     template: Templates/ubuntu-24-04-20260204
   ```

   Пример для DVP (`DVPInstanceClass`):

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: DVPInstanceClass
   metadata:
     name: worker
   spec:
     rootDisk:
       image:
         kind: ClusterVirtualImage
         name: ubuntu-24-04-20260204
   ```

   Для узлов, которые управляются через `<PROVIDER>ClusterConfiguration`, измените поле образа в `instanceClass` нужной группы. Отредактировать конфигурацию можно командой:

   ```bash
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- \
     deckhouse-controller edit provider-cluster-configuration
   ```

1. Дождитесь перезаказа узлов (Terraform / MCM / CAPI). Если новый образ существует и соответствует требованиям DKP, дополнительных действий не требуется.

Для master-узлов при необходимости используйте отдельные инструкции: [multi-master](/modules/control-plane-manager/faq.html#как-изменить-образ-ос-в-кластере-с-несколькими-master-узлами) и [single-master](/modules/control-plane-manager/faq.html#как-изменить-образ-ос-в-кластере-с-одним-master-узлом).

## На что обратить внимание

{% alert level="warning" %}
Если образ указывается по имени, при замене имя в конфигурации **должно измениться**. Удаление старого образа и создание нового с тем же именем не приводит к перезаказу узлов: DKP реагирует на изменение конфигурации, а не на содержимое ресурса в облаке.
{% endalert %}

* Новый образ должен существовать в облаке к моменту перезаказа узлов.
* Новый образ должен соответствовать требованиям DKP к образам ВМ (в том числе поддержке cloud-init) — так же, как при первичной подготовке образа.
