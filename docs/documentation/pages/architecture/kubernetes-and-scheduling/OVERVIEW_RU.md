---
title: Подсистема Kubernetes & Scheduling
permalink: ru/architecture/kubernetes-and-scheduling/ 
lang: ru
search: подсистема Kubernetes, scheduling, control-plane-manager, descheduler, VPA, kubelet
description: Архитектура подсистемы Kubernetes & Scheduling в Deckhouse Kubernetes Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

В данном подразделе описывается архитектура модулей, входящих в подсистему Kubernetes & Scheduling платформы Deckhouse Kubernetes Platform (DKP).

В подсистему Kubernetes & Scheduling входят следующие модули:

* [`control-plane-manager`](/modules/control-plane-manager/) — основной модуль подсистемы, с помощью которого осуществляется [управление компонентами control plane кластера](control-plane-management.html);
* [`descheduler`](/modules/descheduler/) — анализирует состояние кластера и выполняет вытеснение подов в соответствии с [активными стратегиями](/modules/descheduler/#стратегии);
* [`vertical-pod-autoscaler`](/modules/vertical-pod-autoscaler/) — автоматически корректирует запросы и лимиты ресурсов контейнеров в подах на основе фактического потребления. Архитектура модуля описана на [соответствующей странице](vpa.html).

В подразделе также описывается архитектура [control plane](control-plane.html) и [агента kubelet](kubelet.html).
