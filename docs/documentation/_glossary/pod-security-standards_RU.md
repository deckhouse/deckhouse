---
title: Pod Security Standards
description: Три уровня политик безопасности подов, privileged, baseline и restricted, которые Deckhouse Platform применяет к неймспейсам с помощью модуля [`admission-policy-engine`](/modules/admission-policy-engine/).
lang: ru
---

Pod Security Standards — одна из категорий [admission-политик](glossary.html#admission-policy). Уровень `privileged` не накладывает ограничений, `baseline` предотвращает самые распространённые способы повышения привилегий, `restricted` задаёт самые строгие требования к подам. Deckhouse Platform берёт из Kubernetes только описания политик и применяет их средствами Gatekeeper, а не контроллером Pod Security Admission.

Уровень по умолчанию — `baseline`. Он применяется к [неймспейсам](glossary.html#namespace) без лейбла `security.deckhouse.io/pod-policy`, кроме системных. Лейбл задаёт уровень для одного неймспейса, а настройка модуля [`admission-policy-engine`](/modules/admission-policy-engine/) меняет уровень по умолчанию.

Подробнее в разделе [«Политики безопасности»](admin/configuration/security/policies.html).
