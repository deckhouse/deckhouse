---
title: Управление узлами в Amazon Web Services
permalink: ru/admin/integrations/public/amazon/node-management.html
lang: ru
---

## Подключение CloudStatic-узлов

Для подключения созданных вручную EC2-инстансов к кластеру Deckhouse Platform:

1. Добавьте IAM-роль `<prefix>-node`.
1. Добавьте группу безопасности `<prefix>-node`.
1. Укажите теги:

   ```yaml
   "kubernetes.io/cluster/<cluster_uuid>" = "shared"
   "kubernetes.io/cluster/<prefix>" = "shared"
   ```

Узнать `cluster_uuid` можно с помощью команды:

```shell
d8 k -n kube-system get cm d8-cluster-uuid -o json | jq -r '.data."cluster-uuid"'
```

Узнать `prefix` можно с помощью команды:

```shell
d8 k get mc global -o jsonpath='{.spec.settings.prefix}{"\n"}'
```

Если значение пустое, возьмите его из устаревшего параметра [`cloud.prefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-cloud-prefix) ресурса [ClusterConfiguration](../../../../glossary.html#cluster-configuration) (префикс перенесён в ModuleConfig `global`):

```shell
d8 k -n kube-system get secret d8-cluster-configuration -o json | \
  jq -r '.data."cluster-configuration.yaml"' | base64 -d | grep prefix
```

## Прерываемые узлы

Прерываемые виртуальные машины в AWS называются spot-инстансами. Чтобы [группа узлов](../../../../glossary.html#node-group) заказывала их, установите параметр [`spot`](/modules/cloud-provider-aws/cr.html#awsinstanceclass-v1-spec-spot) ресурса AWSInstanceClass в `true`.

При получении [рекомендации о повышенном риске прерывания spot-инстанса](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/rebalance-recommendations.html) (EC2 Instance Rebalance Recommendation) компонент `node-termination-handler` автоматически устанавливает на CloudEphemeral-узел taint `aws-node-termination-handler/rebalance-recommendation` с параметром `effect: NoSchedule`. Такой taint запрещает планировщику размещать на узле новую нагрузку без соответствующего toleration.

Taint `aws-node-termination-handler/spot-itn` обозначает уведомление о предстоящем [прерывании spot-инстанса](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html).

При наличии любого из этих taints модуль [`node-manager`](/modules/node-manager/) выполняет cordon и drain узла, затем удаляет его. Если количество узлов в зоне становится меньше значения параметра [`minPerZone`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-minperzone), группа заказывает новые узлы.

Рекомендация позволяет начать вывод узла из эксплуатации до получения уведомления о прерывании. AWS может отправить оба уведомления одновременно.

Порядок создания таких узлов и ограничения их использования описаны в разделе [«Как создать прерываемые узлы в AWS?»](/modules/cloud-provider-aws/faq.html#как-создать-прерываемые-узлы-в-aws).

## Замена образа ОС

Образ ОС узлов задаётся параметром [`ami`](/modules/cloud-provider-aws/cr.html#awsinstanceclass-v1-spec-ami) ресурса AWSInstanceClass для узлов типа `CloudEphemeral` и в секции `instanceClass` ресурса AWSClusterConfiguration для узлов типа `CloudPermanent` и master-узлов. Чтобы перевести узлы кластера на новый образ, выполните шаги из раздела [«Как заменить образ ОС в облачном кластере?»](../../../../faq.html#как-заменить-образ-ос-в-облачном-кластере).
