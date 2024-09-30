---
title: "Cloud provider — Dynamix: настройки провайдера"
description: Настройки облачного провайдера Deckhouse для Dynamix.
---

{% alert level="info" %}
Если control plane кластера размещен на виртуальных машинах или bare-metal-серверах, cloud-провайдер использует настройки модуля `cloud-provider-dynamix` в конфигурации Deckhouse. Если control plane кластера размещен в облаке, cloud-провайдер использует структуру [DynamixClusterConfiguration](#dynamixclusterconfiguration) для настройки.
{% endalert %}

<!-- SCHEMA -->
