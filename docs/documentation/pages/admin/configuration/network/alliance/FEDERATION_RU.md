---
title: "Федерация"
permalink: ru/admin/configuration/network/alliance/federation.html
lang: ru
---

## Федерация средствами Istio (Service Mesh)

{% alert level="info" %}
Доступно только в DKP Enterprise Edition (EE) и DKP Certified Security Edition Pro (CSE Pro 1.67+).
{% endalert %}

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#%D1%84%D0%B5%D0%B4%D0%B5%D1%80%D0%B0%D1%86%D0%B8%D1%8F -->

### Требования к кластерам

* Каждый кластер должен иметь уникальное значение параметра [`clusterDomain`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain) в ресурсе ClusterConfiguration. Обратите внимание, что ни один из кластеров не должен иметь домен `cluster.local`, который является значением по умолчанию.

  > Значение `cluster.local` использовать нельзя — зарезервированный псевдоним для домена локального кластера.
  > Если в AuthorizationPolicy указать `cluster.local` как principals, правило будет применяться только к локальному кластеру, даже если в Service mesh существует кластер, у которого [`clusterDomain`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain) явно определен как `cluster.local` (Подробнее — [в документации Istio](https://istio.io/latest/docs/tasks/security/authorization/authz-td-migration/#best-practices)).

* Подсети сервисов и подов, заданные в параметрах [`serviceSubnetCIDR`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-servicesubnetcidr) и [`podSubnetCIDR`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-podsubnetcidr) ресурса ClusterConfiguration должны различаться между кластерами.

  > При анализе трафика Istio использует:
  > - для HTTP/HTTPS-запросов — заголовки;
  > - для TCP-запросов — IP-адрес назначения и порт.
  >
  > Если IP-адреса пересекаются, Istio может ошибочно применить правила маршрутизации к запросам из других кластеров.
  > В режиме single-network пересечения подсетей строго запрещены. В режиме multi-networks — формально допустимы, но не рекомендуются. Подробнее — [в документации Istio](https://istio.io/latest/docs/ops/deployment/deployment-models/#single-network).
  >
  > - В режиме single-network поды разных кластеров могут взаимодействовать напрямую.
  > - В режиме multi-networks поды разных кластеров взаимодействуют только через istio-gateway.

### Общие принципы федерации

* Федерация требует взаимного доверия между кластерами. Для этого необходимо обменяться корневыми сертификатами: кластер A должен доверять кластеру B и наоборот.
* Для настройки межкластерного доступа к сервисам необходимо обменяться информацией о публичных сервисах. Чтобы опубликовать сервис bar из кластера Б в кластере А, необходимо в кластере А создать ресурс ServiceEntry, который описывает публичный адрес ingress-gateway кластера Б.

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
* Каждый ресурс (Service), который считается публичным в рамках федерации, пометить лейблом `federation.istio.deckhouse.io/public-service: ""`.
  * В остальных кластерах из состава федерации, для каждого ресурса Service создадутся соответствующие ServiceEntry, указывающие на `ingressgateway` исходного кластера.

> **Важно**. Убедитесь, что поле `name` в разделе `.spec.ports` ресурса Service заполнено для каждого порта, иначе могут быть проблемы в работе федерации.

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
