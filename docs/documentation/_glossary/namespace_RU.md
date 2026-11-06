---
title: Неймспейс
description: Механизм логической группировки объектов в Kubernetes.
lang: ru
---

Примеры объектов Kubernetes, которые можно сгруппировать с помощью неймспейса: Pod, Deployment, Service, Ingress, ConfigMap.

Неймспейсы часто применяются для изоляции, но без дополнительной настройки они её не обеспечивают, в отличие от [проектов](glossary.html#project) в Deckhouse Platform.

Неймспейсы делятся на [системные](glossary.html#system-namespace) и [пользовательские](glossary.html#user-namespace).
