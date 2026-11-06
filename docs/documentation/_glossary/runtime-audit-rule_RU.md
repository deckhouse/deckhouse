---
title: Правило рантайм-аудита
description: Правило рантайм-аудита, которое описывает критерий подозрительного поведения в виде условия в синтаксисе Falco. Deckhouse Platform поставляет встроенные правила и принимает пользовательские.
lang: ru
---

К встроенным относятся правила для аудита Kubernetes, которые выявляют проблемы с безопасностью Deckhouse Platform и самого механизма аудита, и нормативные правила `fstec`, удовлетворяющие требованиям приказа ФСТЭК России № 118 к средствам контейнеризации. Пользовательские правила задают с помощью объекта [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) модуля [`runtime-audit-engine`](/modules/runtime-audit-engine/). [Рантайм-аудит](glossary.html#runtime-audit) сверяет события со всеми правилами и формирует [события безопасности](glossary.html#security-event).

Подробнее в разделе [«Архитектура рантайм-аудита»](architecture/security/runtime-audit.html).
