---
title: Обзор
permalink: ru/architecture/kubernetes-and-scheduling/ 
lang: ru
search: control plane, scheduling
---

В данном подразделе описывается архитектура модулей, входящих в подсистему **Kubernetes & Scheduling** DKP.

В подсистему **Kubernetes & Scheduling** входят следущие модули:

* [control-plane-manager](/modules/control-plane-manager/) - основной модуль подсистемы,с его помощью осуществляется [управление компонентами control plane кластера](control-plane-management/),
* [descheduler](/modules/descheduler/) -  анализирует состояние кластера и выполняет вытеснение подов, соответствующих условиям, описанным в активных [стратегиях](/modules/descheduler/#%D1%81%D1%82%D1%80%D0%B0%D1%82%D0%B5%D0%B3%D0%B8%D0%B8).
* [vertical-pod-autoscaler](/modules/vertical-pod-autoscaler/) - автоматически регулирует запросы на ресурсы и лимиты для контейнеров в подах на основе фактического потребления ресурсов. Архитектура **vertical-pod-autoscaler** описана на соответствующей [странице](../vpa.html).

В подразделе описывается также [архитектура control plane Kubernetes-кластера](control-plane/) и важная "деталь" любого Kubernetes-кластера — агент [kubelet](kubelet/).
