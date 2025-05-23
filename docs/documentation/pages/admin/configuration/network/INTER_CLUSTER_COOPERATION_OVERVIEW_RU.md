---
title: "Межкластерное взаимодействие (альянс)"
permalink: ru/admin/network/inter-cluster-cooperation-overview.html
lang: ru
---

<!-- перенесено с небольшими изменениями из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#%D1%84%D0%B5%D0%B4%D0%B5%D1%80%D0%B0%D1%86%D0%B8%D1%8F-%D0%B8-%D0%BC%D1%83%D0%BB%D1%8C%D1%82%D0%B8%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80 -->

> Доступно только в редакции Enterprise Edition.

В Deckhouse Kubernetes Platform поддерживаются две схемы межкластерного взаимодействия:

* [федерация](../network/cluster-federation.html);
* [мультикластер](../network/multicluster.html).

Принципиальные отличия схем:

* Федерация объединяет суверенные кластеры:
  * у каждого кластера собственное пространство имен (для namespace, Service и пр.);
  * доступ к отдельным сервисам между кластерами явно обозначен.
* Мультикластер объединяет созависимые кластеры:
  * пространство имен у кластеров общее — каждый сервис доступен для соседних кластеров так, словно он работает на локальном кластере (если это не запрещают правила авторизации).

Обе схемы могут быть реализованы средствами Istio (модуль [istio](../../reference/mc/istio/)) или Cilium (модуль [cni-cilium](../../reference/mc/cni-cilium/)).
