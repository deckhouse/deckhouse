---
title: "Cloud provider — Azure: примеры"
---

## Пример custom resource `AzureInstanceClass`

Ниже представлен простой пример custom resource `AzureInstanceClass`:

```yaml
apiVersion: deckhouse.io/v1
kind: AzureInstanceClass
metadata:
  name: example
spec:
  machineSize: Standard_F4
```

## Замена образа ОС

В этом примере узлы группы переводятся на новый образ ОС. Новый образ указан в параметре [`urn`](cr.html#azureinstanceclass-v1-spec-urn) инстанс-класса, на который ссылается группа узлов:

```yaml
apiVersion: deckhouse.io/v1
kind: AzureInstanceClass
metadata:
  name: example
spec:
  machineSize: Standard_F4
  urn: Canonical:ubuntu-24_04-lts:server:24.04.202602040
```

После изменения инстанс-класса Deckhouse Platform пересоздаёт узлы типа `CloudEphemeral` из нового образа. Для узлов типа `CloudPermanent` и master-узлов образ задаётся в секции `instanceClass` ресурса AzureClusterConfiguration. Порядок замены описан в разделе [«Как заменить образ ОС в облачном кластере?»](/products/kubernetes-platform/documentation/v1/faq.html#как-заменить-образ-ос-в-облачном-кластере).
