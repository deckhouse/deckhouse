---
title: Конфигурация кластера (ClusterConfiguration)
description: Структура с общими параметрами кластера, такими как версия Kubernetes, сетевые настройки, параметры CRI и тип кластера.
lang: ru
---

Конфигурация кластера задаётся при установке и редактируется в работающем кластере командой `d8 system edit cluster-configuration`. После сохранения изменений Deckhouse Platform приводит состояние кластера в соответствие с новой конфигурацией. Часть параметров [защищена](glossary.html#protected-parameters) и по умолчанию не может быть изменена в работающем кластере.

Параметр `clusterType` определяет, является кластер [статическим](glossary.html#static-cluster) или [облачным](glossary.html#cloud-cluster).

Подробнее в разделе [«Конфигурация Deckhouse Platform»](admin/configuration/).
