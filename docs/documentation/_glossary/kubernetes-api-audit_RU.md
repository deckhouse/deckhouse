---
title: Аудит API Kubernetes
description: Журналирование обращений к API-серверу Kubernetes по политике аудита. По умолчанию журнал записывается в файл `/var/log/kube-audit/audit.log` на master-узлах.
lang: ru
---

Аудит API Kubernetes отслеживает обращения к API-серверу и помогает разбирать неожиданное поведение и соблюдать требования безопасности. Deckhouse Platform по умолчанию настраивает базовую политику аудита: она пропускает частые служебные обновления, например Endpoints, Events и выборы лидера, и записывает изменения подов, узлов, объектов ServiceAccount и операции в системных неймспейсах. Политику можно дополнить собственными правилами.

Журнал аудита служит источником для [рантайм-аудита](glossary.html#runtime-audit) и [событий безопасности](glossary.html#security-event).

Подробнее в разделе [«Аудит событий API Kubernetes»](admin/configuration/security/events/kubernetes-api-audit.html).
