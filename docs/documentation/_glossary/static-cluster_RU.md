---
title: Статический кластер
description: |
  Кластер с типом [clusterType: Static](reference/api/cr.html#clusterconfiguration-clustertype). Его узлы — физические серверы или виртуальные машины, которые Deckhouse Platform не создаёт и не управляет ими через API облака.
lang: ru
---

Администратор готовит серверы, а [установщик](glossary.html#installer) настраивает их, развёртывает Kubernetes и устанавливает Deckhouse Platform. Узлы такого кластера — [статические узлы](glossary.html#static-node). Статический кластер можно расширить узлами облачного провайдера до [гибридного кластера](glossary.html#hybrid-cluster).

Подробнее в разделе [«Установка платформы»](installing/).
