---
title: "Мультикластер"
permalink: ru/admin/network/multicluster.html
lang: ru
---

## Мультикластер средствами Istio

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#%D0%BC%D1%83%D0%BB%D1%8C%D1%82%D0%B8%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80 -->

### Требования к кластерам

* Домены кластеров в параметре [`clusterDomain`](../../reference/cr/clusterconfiguration/#clusterconfiguration-clusterdomain) ресурса [*ClusterConfiguration*](../../reference/cr/clusterconfiguration/) должны быть одинаковыми для всех членов мультикластера. По умолчанию значение параметра — `cluster.local`.

* Подсети сервисов и подов в параметрах [`serviceSubnetCIDR`](../../reference/cr/clusterconfiguration/#clusterconfiguration-servicesubnetcidr) и [`podSubnetCIDR`](../../reference/cr/clusterconfiguration/#clusterconfiguration-podsubnetcidr) ресурса [*ClusterConfiguration*](../../reference/cr/clusterconfiguration/) должны быть уникальными для каждого кластера.

  > - При анализе HTTP и HTTPS запросов *(в терминологии istio)* идентифицировать их и принять решение о дальнейшей маршрутизации, запрещении или разрешении возможно по заголовкам.
  > - А при анализе TCP запросов *(в терминологии istio)* идентифицировать их и принять решение о дальнейшей маршрутизации, запрещении или разрешении возможно только по IP-адресу назначения и номеру порта.
  >
  > Если IP адреса сервисов или подов пересекутся между кластерами, то под маршрутизирующие, запрещающие или разрешающие правила istio могут попасть запросы других подов иных кластеров.
  > Пересечение подсетей сервисов и подов жестко запрещено в single-network режиме, и не рекомендуется в режиме multiple-networks ([источник — документация Istio](https://istio.io/latest/docs/ops/deployment/deployment-models/#single-network))
  >
  > - В режиме single-network поды разных кластеров могут взаимодействовать друг с другом напрямую.
  > - В режиме multiple-networks поды разных кластеров могут взаимодействовать друг с другом только при использовании istio-gateway.

### Общие принципы

<div data-presentation="../../presentations/istio/multicluster_common_principles_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1WeNrp0Ni2Tz3_Az0f45rkWRUZxZUDx93Om5MB3sEod8/ --->

* Мультикластер требует установления взаимного доверия между кластерами. Соответственно, для построения мультикластера нужно в кластере A сделать кластер Б доверенным и в кластере Б сделать кластер А доверенным. Технически это достигается взаимным обменом корневыми сертификатами.
* Для сбора информации о соседних сервисах Istio подключается напрямую к API-серверу соседнего кластера. Данный модуль Deckhouse берет на себя организацию соответствующего канала связи.

### Включение мультикластера

При включении мультикластера (параметр модуля `istio.multicluster.enabled = true`) происходит следующее:

* В кластер добавляется прокси для публикации доступа к API-серверу посредством стандартного Ingress-ресурса:
  * Доступ через данный публичный адрес ограничен авторизацией на основе Bearer-токенов, подписанных доверенными ключами. Обмен доверенными публичными ключами происходит автоматически средствами Deckhouse при взаимной настройке мультикластера.
  * Непосредственно прокси имеет read-only-доступ к ограниченному набору ресурсов.
* В кластер добавляется сервис, который экспортирует метаданные кластера наружу:
  * Корневой сертификат Istio (доступен без аутентификации).
  * Публичный адрес, через который доступен API-сервер (доступен только для аутентифицированных запросов из соседних кластеров).
  * Список публичных адресов сервиса `ingressgateway` (доступен только для аутентифицированных запросов из соседних кластеров).
  * Публичные ключи сервера для аутентификации запросов к API-серверу и закрытым метаданным (см. выше).

### Управление мультикластером

<div data-presentation="../../presentations/istio/multicluster_istio_multicluster_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1D3nuoC0okJQRCOY4teJ6p598Bd4JwPXZT5cdG0hW8Hc/ --->

Для сборки мультикластера необходимо в каждом кластере создать набор ресурсов `IstioMulticluster`, которые описывают все остальные кластеры.

<!-- перенесено с небольшими изменениями из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/examples.html#%D1%83%D1%81%D1%82%D1%80%D0%BE%D0%B9%D1%81%D1%82%D0%B2%D0%BE-%D0%BC%D1%83%D0%BB%D1%8C%D1%82%D0%B8%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80%D0%B0-%D0%B8%D0%B7-%D0%B4%D0%B2%D1%83%D1%85-%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80%D0%BE%D0%B2-%D1%81-%D0%BF%D0%BE%D0%BC%D0%BE%D1%89%D1%8C%D1%8E-%D1%80%D0%B5%D1%81%D1%83%D1%80%D1%81%D0%B0-istiomulticluster -->

### Пример устройства мультикластера из двух кластеров

> Доступно только в редакции Enterprise Edition.

Для устройства мультикластера средствами Istio используйте ресурс IstioMulticluster.

Cluster A:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: cluster-b
spec:
  metadataEndpoint: https://istio.k8s-b.example.com/metadata/
```

Cluster B:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: cluster-a
spec:
  metadataEndpoint: https://istio.k8s-a.example.com/metadata/
```

## Мультикластер средствами Cilium

Нужен контент
