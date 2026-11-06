---
title: Проверка подписи образов
description: Проверка целостности и подлинности образов пользовательских приложений на этапе валидации запросов к API Kubernetes, до создания пода, по подписям Cosign и открытым ключам, перечисленным в [SecurityPolicy](/modules/admission-policy-engine/cr.html#securitypolicy).
lang: ru
---

На этапе сборки образ подписывается закрытым ключом с помощью Cosign. Подпись публикуется в хранилище образов отдельным тегом и не меняет сам образ. В кластере SecurityPolicy с параметром `verifyImageSignatures` перечисляет открытые ключи, а модуль [`admission-policy-engine`](/modules/admission-policy-engine/) с Ratify проверяет подпись каждого образа при создании пода. Это часть [контроля целостности](glossary.html#integrity-control), доступная во всех [редакциях](glossary.html#edition). Проверка на уровне контейнерного рантайма есть только в Certified Core и Certified Pro.

Подробнее в разделе [«Контроль целостности»](architecture/security/integrity-control.html).
