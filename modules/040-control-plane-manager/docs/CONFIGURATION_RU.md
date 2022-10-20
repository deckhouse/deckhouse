---
title: "Управление control plane: настройки"
---

Управление компонентами control plane кластера осуществляется с помощью модуля `control-plane-manager`, а параметры кластера, влияющие на управление control plane, берутся из данных первичной конфигурации кластера (параметр `cluster-configuration.yaml` Secret'а `d8-cluster-configuration` в пространстве имен `kube-system`), которое создается при установке.

{% include module-bundle.liquid %}

## Параметры

<!-- SCHEMA -->
