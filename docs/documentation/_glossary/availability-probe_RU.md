---
title: Проба доступности
description: Проверка, которую модуль upmeter регулярно выполняет для расчёта доступности компонентов Deckhouse Platform. Пробы объединены в группы, группы и отдельные пробы можно отключать.
lang: ru
---

Компонент upmeter-agent работает на [master-узлах](glossary.html#master-node) и выполняет группы проверок: control-plane (доступность API-сервера и работа контроллеров), deckhouse (состояние кластера и deckhouse-контроллер), extensions, load-balancing, monitoring-and-autoscaling, nginx (у каждого [Ingress-контроллера](glossary.html#ingress-controller) есть под в состоянии `Ready`), nodegroups и synthetic (сетевое взаимодействие между узлами). Например, группа control-plane создаёт неймспейс, под и StatefulSet и проверяет, что Kubernetes их обрабатывает.

Результаты проб контроллер upmeter хранит без учёта интервалов простоя из объекта [Downtime](/modules/upmeter/cr.html#downtime), показывает на странице статуса и дашборде доступности и использует для [мониторинга SLA](glossary.html#sla-monitoring). Пробы и группы проб отключаются параметром модуля `disabledProbes`.

Подробнее в разделе [«Модуль upmeter»](architecture/observability/upmeter.html).
