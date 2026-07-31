---
title: "Федерация"
permalink: ru/admin/configuration/network/alliance/federation.html
lang: ru
---

## Федерация средствами Istio (Service Mesh)

{% alert level="info" %}
Доступно только в DKP Enterprise Edition (EE) и DKP Certified Security Edition Pro (CSE Pro).
{% endalert %}

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#%D1%84%D0%B5%D0%B4%D0%B5%D1%80%D0%B0%D1%86%D0%B8%D1%8F -->

### Требования к кластерам

* Каждый кластер должен иметь уникальное значение параметра [`clusterDomain`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain) в ресурсе ClusterConfiguration. Обратите внимание, что ни один из кластеров не должен иметь домен `cluster.local`, который является значением по умолчанию.

  > Значение `cluster.local` использовать нельзя, так как это зарезервированный псевдоним для домена локального кластера.
  > Если в AuthorizationPolicy указать `cluster.local` как principals, правило будет применяться только к локальному кластеру, даже если в Service mesh существует кластер, у которого [`clusterDomain`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain) явно определен как `cluster.local` (Подробнее — [в документации Istio](https://istio.io/latest/docs/tasks/security/authorization/authz-td-migration/#best-practices)).

* Требования к уникальности подсетей сервисов и подов в параметрах [`serviceSubnetCIDR`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-servicesubnetcidr) и [`podSubnetCIDR`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-podsubnetcidr) ресурса [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) при работе кластеров в федерации отсутствуют.

  > При анализе трафика Istio использует:
  > - для HTTP/HTTPS-запросов — заголовки;
  > - для TCP-запросов — IP-адрес назначения и порт.
  >
  > Istio работает в режиме [multi-network](https://istio.io/latest/docs/ops/deployment/deployment-models/#multiple-networks) — поды разных кластеров взаимодействуют друг с другом только через Istio ingress gateway. Прямое взаимодействие между подами разных кластеров не поддерживается.

### Общие принципы федерации

* Федерация требует взаимного доверия между кластерами. Для этого необходимо обменяться корневыми сертификатами: кластер A должен доверять кластеру B и наоборот.
* Для настройки межкластерного доступа к сервисам необходимо обменяться информацией о публичных сервисах. В ванильном istio это можно сделать с помощью ресурса `ServiceEntry`. `ServiceEntry` описывает публичный адрес `ingressgateway` кластера Б, чтобы сервисы кластера A могли обращаться к сервису `bar` в кластере Б. Данный модуль автоматизирует этот процесс (см. ниже).

<div data-presentation="../../../../presentations/istio/federation_common_principles_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1EI2MQMuVCGACnLNBXMGVDNJVhwU3vJYtVcHhrWfjLDc/ --->

### Включение федерации: создаваемые сервисы

При включении федерации (установка параметра модуля `istio.federation.enabled = true`) в кластер добавляются:

* Сервис `ingressgateway`, который проксирует mTLS-трафик извне к прикладным сервисам.
* Сервис экспорта метаданных:
  * корневой сертификат Istio (доступен без аутентификации);
  * список публичных сервисов в кластере (доступен только для аутентифицированных запросов из соседних кластеров);
  * список публичных адресов сервиса `ingressgateway` (доступен только для аутентифицированных запросов из соседних кластеров).

### Настройка федерации

<div data-presentation="../../../../presentations/istio/federation_istio_federation_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1MpmtwJwvSL32EdwOUNpJ6GjgWt0gplzjqL8OOprNqvc/ --->

Для настройки федерации необходимо:

* В каждом кластере создать набор ресурсов [IstioFederation](/modules/istio/cr.html#istiofederation) для описания других кластеров.
  * После успешного автосогласования между кластерами в ресурсе IstioFederation будут записаны необходимые служебные данные в `status.metadataCache.public` и `status.metadataCache.private`.
* Каждый сервис, который считается публичным в рамках федерации, пометить лейблом `federation.istio.deckhouse.io/public-service: ""`.
  * В остальных кластерах из состава федерации для каждого такого сервиса автоматически создадутся соответствующие `ServiceEntry` и DestinationRule, указывающие на `ingressgateway` исходного кластера.
  * Лейбл должен быть с пустым значением. Другие ресурсы, например Deployment, Pod или VirtualService, для публикации сервиса в федерации помечать не нужно.

{% alert level="warning" %}
Для публикации в федерации не поддерживаются сервисы типа `ExternalName`, сервисы без `.spec.ports` и сервисы с портами без поля `name`.

Имя каждого порта должно начинаться с поддерживаемого Istio-префикса: `http`, `http2`, `https`, `tcp`, `tls`, `grpc` или `grpc-web`. По имени порта модуль определяет протокол в генерируемом `ServiceEntry`. Если префикс не распознан, порт будет обработан как TCP.
{% endalert %}

Пример публичного сервиса:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: reviews
  namespace: bookinfo
  labels:
    federation.istio.deckhouse.io/public-service: ""
spec:
  selector:
    app: reviews
  ports:
  - name: http
    port: 9080
    targetPort: 9080
```

Диагностика публикации сервисов в федерации:

Проверьте, какие сервисы помечены как публичные в локальном кластере:

```shell
d8 k get svc -A -l federation.istio.deckhouse.io/public-service=
```

Проверьте состояние обмена метаданными с удалёнными кластерами:

```shell
d8 k get istiofederation
d8 k get istiofederation <name> -o jsonpath='{.status.conditions}'
```

Проверьте, что удалённый кластер передал список своих публичных сервисов:

```shell
d8 k get istiofederation <name> -o jsonpath='{.status.metadataCache.private.publicServices}'
```

Проверьте, что по полученным метаданным созданы маршруты в локальном кластере:

```shell
d8 k -n d8-istio get serviceentry,destinationrule
```

В `status.conditions` ресурса IstioFederation должны перейти в `True` условия `PublicMetadataExchangeReady`, `PrivateMetadataExchangeReady` и `DataplaneConnectionReady`. Если обмен метаданными не работает, проверьте [алерт `D8IstioFederationMetadataEndpointDoesntWork`](../../../../reference/alerts.html#istio-d8istiofederationmetadataendpointdoesntwork) и доступность параметра `spec.metadataEndpoint` удалённого кластера.

<!-- перенесено с небольшими изменениями из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/examples.html#%D1%83%D1%81%D1%82%D1%80%D0%BE%D0%B9%D1%81%D1%82%D0%B2%D0%BE-%D1%84%D0%B5%D0%B4%D0%B5%D1%80%D0%B0%D1%86%D0%B8%D0%B8-%D0%B8%D0%B7-%D0%B4%D0%B2%D1%83%D1%85-%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80%D0%BE%D0%B2-%D1%81-%D0%BF%D0%BE%D0%BC%D0%BE%D1%89%D1%8C%D1%8E-custom-resource-istiofederation -->

### Пример устройства федерации из двух кластеров

Для настройки федерации средствами Istio используйте кастомный ресурс [IstioFederation](/modules/istio/cr.html#istiofederation).

Кластер A:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: cluster-a
spec:
  metadataEndpoint: https://istio.k8s-a.example.com/metadata/
  trustDomain: cluster-a.local
```

Кластер B:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: cluster-b
spec:
  metadataEndpoint: https://istio.k8s-b.example.com/metadata/
  trustDomain: cluster-b.local
```
