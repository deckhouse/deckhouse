---
title: Облачный кластер
description: |
  Кластер с типом [clusterType: Cloud](reference/api/cr.html#clusterconfiguration-clustertype). Deckhouse Platform имеет доступ к API облачной инфраструктуры и сам создаёт виртуальные машины узлов и управляет ими.
lang: ru
---

[Установщик](glossary.html#installer) создаёт все нужные ресурсы, такие как виртуальные машины и сети, по `<PROVIDER>ClusterConfiguration` [облачного провайдера](glossary.html#cloud-provider). Master-узлы облачного кластера — это [узлы CloudPermanent](glossary.html#cloud-permanent-node), рабочие узлы обычно [узлы CloudEphemeral](glossary.html#cloud-ephemeral-node).

Подробнее в разделе [«Установка платформы»](installing/).
