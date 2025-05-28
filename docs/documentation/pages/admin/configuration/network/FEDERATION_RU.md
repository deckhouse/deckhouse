---
title: "Федерация"
permalink: ru/admin/configuration/network/cluster-federation.html
lang: ru
---

## Федерация средствами Istio (Service Mesh)

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#%D1%84%D0%B5%D0%B4%D0%B5%D1%80%D0%B0%D1%86%D0%B8%D1%8F -->

### Требования к кластерам

* У каждого кластера должен быть уникальный домен в параметре [`clusterDomain`](../../reference/cr/clusterconfiguration/#clusterconfiguration-clusterdomain) ресурса [*ClusterConfiguration*](../../reference/cr/clusterconfiguration/). Обратите внимание, что ни один из кластеров не должен иметь домен `cluster.local`, который является значением по умолчанию.

  > `cluster.local` — неизменяемый псевдоним для домена локального кластера.
  > Указание `cluster.local` как principals в AuthorizationPolicy всегда будет указывать на локальный кластер, даже если в mesh существует кластер, у которого [`clusterDomain`](../../reference/cr/clusterconfiguration/#clusterconfiguration-clusterdomain) явно определен как `cluster.local` ([источник — документация Istio](https://istio.io/latest/docs/tasks/security/authorization/authz-td-migration/#best-practices)).

* Подсети сервисов и подов в параметрах [`serviceSubnetCIDR`](../../reference/cr/clusterconfiguration/#clusterconfiguration-servicesubnetcidr) и [`podSubnetCIDR`](../../reference/cr/clusterconfiguration/#clusterconfiguration-podsubnetcidr) ресурса [*ClusterConfiguration*](../../reference/cr/clusterconfiguration/) должны быть уникальными для каждого кластера.

  > - При анализе HTTP и HTTPS запросов *(в терминологии istio)* идентифицировать их и принять решение о дальнейшей маршрутизации, запрещении или разрешении возможно по заголовкам.
  > - А при анализе TCP-запросов *(в терминологии istio)* идентифицировать их и принять решение о дальнейшей маршрутизации, запрещении или разрешении возможно только по IP-адресу назначения и номеру порта.
  >
  > Если IP-адреса сервисов или подов пересекутся между кластерами, то под маршрутизирующие, запрещающие или разрешающие правила istio могут попасть запросы других подов иных кластеров.
  > Пересечение подсетей сервисов и подов жестко запрещено в single-network режиме, и допустимо, но не рекомендуется в режиме multi-networks ([источник — документация Istio](https://istio.io/latest/docs/ops/deployment/deployment-models/#single-network)).
  >
  > - В режиме single-network поды разных кластеров могут взаимодействовать друг с другом напрямую.
  > - В режиме multi-networks поды разных кластеров могут взаимодействовать друг с другом только при использовании istio-gateway.

### Общие принципы федерации

* Федерация требует установления взаимного доверия между кластерами. Соответственно, для установления федерации нужно в кластере A сделать кластер Б доверенным и аналогично в кластере Б сделать кластер А доверенным. Технически это достигается взаимным обменом корневыми сертификатами.
* Для прикладной эксплуатации федерации необходимо также обменяться информацией о публичных сервисах. Чтобы опубликовать сервис bar из кластера Б в кластере А, необходимо в кластере А создать ресурс ServiceEntry, который описывает публичный адрес ingress-gateway кластера Б.

<div data-presentation="../../presentations/istio/federation_common_principles_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1EI2MQMuVCGACnLNBXMGVDNJVhwU3vJYtVcHhrWfjLDc/ --->

### Включение федерации

При включении федерации (параметр модуля `istio.federation.enabled = true`) происходит следующее:

* В кластер добавляется сервис `ingressgateway`, чья задача — проксировать mTLS-трафик извне кластера на прикладные сервисы.
* В кластер добавляется сервис, который экспортирует метаданные кластера наружу:
  * корневой сертификат Istio (доступен без аутентификации);
  * список публичных сервисов в кластере (доступен только для аутентифицированных запросов из соседних кластеров);
  * список публичных адресов сервиса `ingressgateway` (доступен только для аутентифицированных запросов из соседних кластеров).

### Управление федерацией

<div data-presentation="../../presentations/istio/federation_istio_federation_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1MpmtwJwvSL32EdwOUNpJ6GjgWt0gplzjqL8OOprNqvc/ --->

Для построения федерации необходимо сделать следующее:

* В каждом кластере создать набор ресурсов `IstioFederation`, которые описывают все остальные кластеры.
  * После успешного автосогласования между кластерами, в ресурсе `IstioFederation` заполнятся разделы `status.metadataCache.public` и `status.metadataCache.private` служебными данными, необходимыми для работы федерации.
* Каждый ресурс(`service`), который считается публичным в рамках федерации, пометить лейблом `federation.istio.deckhouse.io/public-service: ""`.
  * В кластерах из состава федерации, для каждого `service` создадутся соответствующие `ServiceEntry`, ведущие на `ingressgateway` оригинального кластера.

> Важно чтобы в этих `service`, в разделе `.spec.ports` у каждого порта обязательно было заполнено поле `name`.

<!-- перенесено с небольшими изменениями из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/examples.html#%D1%83%D1%81%D1%82%D1%80%D0%BE%D0%B9%D1%81%D1%82%D0%B2%D0%BE-%D1%84%D0%B5%D0%B4%D0%B5%D1%80%D0%B0%D1%86%D0%B8%D0%B8-%D0%B8%D0%B7-%D0%B4%D0%B2%D1%83%D1%85-%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80%D0%BE%D0%B2-%D1%81-%D0%BF%D0%BE%D0%BC%D0%BE%D1%89%D1%8C%D1%8E-custom-resource-istiofederation -->

### Пример устройства федерации из двух кластеров

> Доступно только в редакции Enterprise Edition.

Для устройства федерации средствами Istio используйте custom resource IstioFederation.

Cluster A:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: cluster-b
spec:
  metadataEndpoint: https://istio.k8s-b.example.com/metadata/
  trustDomain: cluster-b.local
```

Cluster B:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: cluster-a
spec:
  metadataEndpoint: https://istio.k8s-a.example.com/metadata/
  trustDomain: cluster-a.local
```
