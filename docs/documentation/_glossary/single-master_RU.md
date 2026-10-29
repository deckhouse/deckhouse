---
title: Кластер с одним master-узлом
description: Режим работы control plane с одним master-узлом. kube-apiserver использует только локальный экземпляр etcd и слушает только адрес master-узла.
lang: ru
---

Запросы на `localhost` обрабатывает прокси-сервер на узле. При добавлении или удалении master-узлов Deckhouse Platform переводит [control plane](glossary.html#control-plane) между режимами с одним master-узлом и [мультимастерным](glossary.html#multi-master).

Подробнее в разделе [«Масштабирование и изменение master-узлов»](admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html).
