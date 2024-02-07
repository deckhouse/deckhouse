---
title: "Cloud provider — GCP: примеры"
---

## Пример custom resource `GCPInstanceClass`

Ниже представлен простой пример конфигурации custom resource `GCPInstanceClass` :

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: test
spec:
  machineType: n1-standard-1
```

## Настройка политик безопасности на узлах

Вариантов, зачем может понадобиться ограничить или, наоборот, расширить входящий или исходящий трафик на виртуальных машинах кластера в GCP, может быть множество. Например:

* Разрешить подключение к узлам кластера с виртуальных машин из другой подсети.
* Разрешить подключение к портам статического узла для работы приложения.
* Ограничить доступ к внешним ресурсам или другим виртуальным машинам в облаке по требованию службы безопасности.

Для всего этого следует применять дополнительные network tags.

## Установка дополнительных network tags на статических и master-узлах

Данный параметр можно задать либо при создании кластера, либо в уже существующем кластере. В обоих случаях дополнительные network tags указываются в `GCPClusterConfiguration`:
- для master-узлов — в секции `masterNodeGroup` в поле `additionalNetworkTags`;
- для статических узлов — в секции `nodeGroups` в конфигурации, описывающей соответствующую nodeGroup, в поле `additionalNetworkTags`.

Поле `additionalNetworkTags` содержит массив строк с именами network tags.

## Установка дополнительных network tags на эфемерных узлах

Необходимо указать параметр `additionalNetworkTags` для всех [`GCPInstanceClass`](cr.html#gcpinstanceclass) в кластере, которым нужны дополнительные network tags.
