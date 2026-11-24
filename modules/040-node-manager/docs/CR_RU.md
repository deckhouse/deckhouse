---
title: "Модуль node-manager: кастомные ресурсы"
---

{% alert level="info" %}
Ресурс [NodeConfig](#nodeconfig) описывает узел [Deckhouse Engine](/products/kubernetes-platform/documentation/v1/reference/deckhouse-engine.html). Он создаётся автоматически во время установки. Сохраняются только изменения в параметрах машины (`spec.network` и `spec.storage`). Версия API `internal.deckhouse.io/v1alpha1` и её форма могут меняться между релизами.
{% endalert %}

<!-- SCHEMA -->
