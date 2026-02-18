---
title: Архитектура
permalink: ru/architecture/
lang: ru
search: архитектура Deckhouse, архитектура DKP
description: Обзор архитектуры Deckhouse Kubernetes Platform.
---

В данном разделе документации описана архитектура Deckhouse Kubernetes Platform (DKP).

Раздел состоит из следующих подразделов:

* [Модель C4](c4-model/) — обзор модели С4, используемой для визуализации архитектуры платформы, а также описание архитектуры DKP на уровнях 1 и 2 модели C4.
* [Модули](module-development/) — описание архитектуры модулей DKP.
* [Катастрофоустойчивость](disaster-resilience/) — описание реализованных в DKP подходов к обеспечению катастрофоустойчивости.
* [Обновление](updating.html) — описание механизмов обновления DKP.
* Описание архитектуры компонентов платформы, сгруппированных по следующим подсистемам:
  * [Подсистема Deckhouse](deckhouse/)
  * [Подсистема Kubernetes & Scheduling](kubernetes-and-scheduling/)
  * [Подсистема Cluster & Infrastructure](cluster-and-infrastructure/)
  * [Подсистема IAM](iam/)
  * [Подсистема Security](security/)
  * [Подсистема Network](network/)
  * [Подсистема Observability](observability/)

{% alert level="info" %}
В разделе представлена информация не по всем подсистемам и модулям DKP.
Материалы по остальным компонентам будут добавляться по мере готовности.
{% endalert %}

## Архитектура DKP

DKP — это платформа для управления кластерами Kubernetes в любых инфраструктурах — от изолированных серверных сред до публичных облаков. Платформа включает в себя:

* кластер Kubernetes;
* контроллер Deckhouse и управляемые им модули;
* [Bashible](cluster-and-infrastructure/bashible/) — агент, работающий на узлах кластера в виде службы, который запускает bash-скрипты для управления узлами.

Модули объединены в подсистемы в соответствии с их функциональным назначением. Контроллер Deckhouse тоже реализован в виде модуля и является единственным модулем, без которого не может функционировать платформа.

Архитектура DKP в масштабе подсистем и модулей описана в подразделе [Модель C4](c4-model/).

## Модули

Модуль — это набор ресурсов и приложений, предназначенных для расширения функциональности DKP.

Ключевые модули:

* [`deckhouse`](/modules/deckhouse/) — контроллер Deckhouse;
* [`control-plane-manager`](kubernetes-and-scheduling/control-plane-management/) — управляет компонентами control plane кластера;
* [`node-manager`](cluster-and-infrastructure/node-manager/) — управляет узлами кластера.

{% alert level="info" %}
Модули [`control-plane-manager`](/modules/control-plane-manager/) и [`node-manager`](/modules/node-manager/) отсутствуют при установке платформы в существующий Managed Kubernetes-кластер.
{% endalert %}

В модуль входят:

* Helm-чарты;
* хуки [addon-operator](https://github.com/flant/addon-operator/);
* правила сборки компонентов модуля (компонентов Deckhouse);
* другие файлы.

При работе с модулями DKP использует проект [addon-operator](https://github.com/flant/addon-operator/). Ознакомьтесь с его документацией, чтобы узнать, как DKP работает с [модулями](https://github.com/flant/addon-operator/blob/main/docs/src/MODULES.md), [хуками модулей](https://github.com/flant/addon-operator/blob/main/docs/src/HOOKS.md) и [параметрами модулей](https://github.com/flant/addon-operator/blob/main/docs/src/VALUES.md).

Об архитектуре модуля и разработке собственных модулей читайте в разделе [Модули](module-development/).
