---
title: Инфраструктурный узел
description: Собирательное название master-, frontend-, системных узлов и узлов мониторинга кластера в отличие от worker-узлов, на которых работают пользовательские приложения.
lang: ru
---

На инфраструктурных узлах работают control plane кластера и компоненты Deckhouse Platform. Рекомендуемые минимальные ресурсы зависят от роли: [master-узлам](glossary.html#master-node) и [узлам мониторинга](glossary.html#monitoring-node) нужны быстрые диски, [frontend-узлам](glossary.html#frontend-node) требуется меньше всего ресурсов, а размер [системных узлов](glossary.html#system-node) зависит от того, есть ли в кластере выделенные узлы мониторинга.

Подробнее в разделе [«Подготовка к production»](guides/production.html).
