---
title: Обзор
permalink: ru/architecture/
lang: ru
search: architecture overview, архитектура Deckhouse
---

В данном разделе документации описана архитектура Deckhouse Kubernetes Platform (DKP).

Раздел состоит из следующих подразделов:

* [**Модель C4**](c4-model/) — обзор модели С4, используемой для визуализации архитектуры платформы, а также описание архитектуры DKP на уровнях 1 и 2 модели C4.
* [**Модули**](module-development/) - описание архитектуры модулей DKP.
* [**Катастрофоустойчивость**](disaster-resilience/) - описание реализованных в DKP подходов к обеспечению катастрофоустойчивости.

Далее описывается архитектура компонентов платформы, сгруппированных по следующим подсистемам:

* [**Подсистема Deckhouse**](deckhouse/)
* [**Подсистема Kubernetes & Scheduling**](kubernetes-and-scheduling/)
* [**Подсистема Cluster & Infrastructure**](cluster-and-infrastructure/)
* [**Подсистема IAM**](iam/)
* [**Подсистема Security**](security/)
* [**Подсистема Network**](network/)
* [**Подсистема Observability**](observability/)

{% alert level="info" %}
Раздел Архитектура в данный момент содержит информацию не по всем подсистемам и модулям DKP. Отсутствующая информация будет добавляться в раздел по мере готовности.
{% endalert %}

## Архитектура DKP

DKP —  это платформа для управления кластерами Kubernetes в любых инфраструктурах — от изолированных серверных сред до публичных облаков. Платформа включает в себя:

* Собственно, сам "ванильный" кластер Kubernetes.
* Контроллер Deckhouse и управляемые им модули.
* [Bashible](cluster-and-infrastructure/bashible/) - агент, работающий как служба на узлах кластера, запускающий bash-скрипты для управления узлами.

 Модули объединены в подсистемы по выполняемому ими функционалу. Контроллер Deckhouse тоже является модулем, и это единственный модуль, без которого не может функционировать платформа. Архитектура DKP в масштабе подсистем и модулей описана в подразделе [**Модель C4**](c4-model/).

## Модули

Модуль — набор ресурсов и приложений, предназначенных для расширения функциональности Deckhouse Kubernetes Platform.

Ключевые модули:

* **deckhouse** - это, собственно, сам контроллер Deckhouse.  
* [control-plane-manager](kubernetes-and-scheduling/control-plane-management/) - управляет компонентами control plane кластера.
* [node-manager](cluster-and-infrastructure/node-manager/) - управляет узлами кластера.

{% alert level="info" %}
Модули [control-plane-manager](/modules/control-plane-manager/) и [node-manager](/modules/node-manager/) отсутствуют при установке платформы в существующий Managed Kubernetes-кластер.
{% endalert %}

В модуль входят:

* Helm-чарты
* хуки [Addon-operator'а](https://github.com/flant/addon-operator/)
* правила сборки компонентов модуля (компонентов Deckhouse)

  и другие файлы.

При работе с модулями Deckhouse использует проект [addon-operator](https://github.com/flant/addon-operator/). Ознакомьтесь с его документацией, если хотите понять, как Deckhouse работает с [модулями](https://github.com/flant/addon-operator/blob/main/docs/src/MODULES.md), [хуками модулей](https://github.com/flant/addon-operator/blob/main/docs/src/HOOKS.md) и [параметрами модулей](https://github.com/flant/addon-operator/blob/main/docs/src/VALUES.md).

Об архитектуре модуля и разработке собственных модулей читайте в разделе [Модули](module-development/) документации.
