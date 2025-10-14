---
title: "Мультикластер"
permalink: ru/admin/configuration/network/alliance/multicluster.html
lang: ru
---

## Мультикластер средствами Istio

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#%D0%BC%D1%83%D0%BB%D1%8C%D1%82%D0%B8%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80 -->

{% alert level="info" %}
Доступно только в DKP Enterprise Edition (EE) и DKP Certified Security Edition Pro (CSE Pro 1.67+).
{% endalert %}

### Требования к кластерам

* У всех кластеров, входящих в мультикластер, значение параметра [`clusterDomain`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain) ресурса ClusterConfiguration должно совпадать. По умолчанию используется значение `cluster.local`.

* Подсети сервисов и подов в параметрах [`serviceSubnetCIDR`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-servicesubnetcidr) и [`podSubnetCIDR`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-podsubnetcidr) ресурса ClusterConfiguration должны быть уникальными для каждого кластера.

  > - При анализе HTTP и HTTPS запросов *(в терминологии Istio)* идентифицировать их и принять решение о дальнейшей маршрутизации, запрещении или разрешении возможно по заголовкам.
  > - А при анализе TCP запросов *(в терминологии Istio)* идентифицировать их и принять решение о дальнейшей маршрутизации, запрещении или разрешении возможно только по IP-адресу назначения и номеру порта.
  >
  > Если IP адреса сервисов или подов пересекутся между кластерами, то под маршрутизирующие, запрещающие или разрешающие правила Istio могут попасть запросы других подов иных кластеров.
  > Пересечение подсетей сервисов и подов жестко запрещено в single-network-режиме, и не рекомендуется в режиме multiple-networks (подробнее — [в документации Istio](https://istio.io/latest/docs/ops/deployment/deployment-models/#single-network))
  >
  > - В режиме single-network поды разных кластеров могут взаимодействовать друг с другом напрямую.
  > - В режиме multiple-networks поды разных кластеров могут взаимодействовать друг с другом только при использовании Istio gateway.

### Общие принципы

<div data-presentation="../../../../presentations/istio/multicluster_common_principles_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1WeNrp0Ni2Tz3_Az0f45rkWRUZxZUDx93Om5MB3sEod8/ --->

* Для работы мультикластера требуется взаимное доверие между кластерами. Это реализуется через обмен корневыми сертификатами между кластерами.
* Для сбора информации о соседних сервисах Istio подключается к API-серверу соседнего кластера. Настройку защищённого канала обеспечивает [модуль `istio`](/modules/istio/) в составе Deckhouse.

### Включение мультикластера

При включении мультикластера (параметр модуля `istio.multicluster.enabled = true`) происходит следующее:

* В кластер добавляется прокси для публикации доступа к API-серверу через стандартный Ingress-ресурс:
  * Доступ защищён авторизацией на основе Bearer-токенов, подписанных доверенными ключами.
  * Обмен доверенными публичными ключами происходит автоматически средствами Deckhouse при взаимной настройке мультикластера.
  * Прокси имеет read-only-доступ к строго определённым ресурсам.
* В кластер добавляется сервис, который экспортирует метаданные кластера наружу:
  * Корневой сертификат Istio (доступен без аутентификации).
  * Публичный адрес, через который доступен API-сервер (доступен только для аутентифицированных запросов из соседних кластеров).
  * Список публичных адресов сервиса `ingressgateway` (доступен только для аутентифицированных запросов из соседних кластеров).
  * Публичные ключи сервера для аутентификации запросов к API-серверу и закрытым метаданным.

### Управление мультикластером

<div data-presentation="../../../../presentations/istio/multicluster_istio_multicluster_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1D3nuoC0okJQRCOY4teJ6p598Bd4JwPXZT5cdG0hW8Hc/ --->

Для сборки мультикластера необходимо в каждом кластере создать набор ресурсов [IstioMulticluster](/modules/istio/cr.html#istiomulticluster), описывающих остальные кластеры.

<!-- перенесено с небольшими изменениями из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/examples.html#%D1%83%D1%81%D1%82%D1%80%D0%BE%D0%B9%D1%81%D1%82%D0%B2%D0%BE-%D0%BC%D1%83%D0%BB%D1%8C%D1%82%D0%B8%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80%D0%B0-%D0%B8%D0%B7-%D0%B4%D0%B2%D1%83%D1%85-%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80%D0%BE%D0%B2-%D1%81-%D0%BF%D0%BE%D0%BC%D0%BE%D1%89%D1%8C%D1%8E-%D1%80%D0%B5%D1%81%D1%83%D1%80%D1%81%D0%B0-istiomulticluster -->

### Пример устройства мультикластера из двух кластеров

Для устройства мультикластера средствами Istio используйте кастомный ресурс [IstioMulticluster](/modules/istio/cr.html#istiomulticluster).

Кластер A:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: cluster-a
spec:
  metadataEndpoint: https://istio.k8s-a.example.com/metadata/
```

Кластер B:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: cluster-b
spec:
  metadataEndpoint: https://istio.k8s-b.example.com/metadata/
```

<!-- ## Мультикластер средствами Cilium

Нужен контент -->
