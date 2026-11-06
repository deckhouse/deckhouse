---
title: Admission-политика
description: Правило, которое применяется к объектам, например подам и Service, в момент их создания или изменения в кластере на основе их манифестов, но не во время работы.
lang: ru
---

Admission-политики формализуют параметры, которые разрешены или запрещены в манифестах объектов. Deckhouse Platform реализует их с помощью модуля [`admission-policy-engine`](/modules/admission-policy-engine/) и делит на три категории: [Pod Security Standards](glossary.html#pod-security-standards), операционные политики ([OperationPolicy](/modules/admission-policy-engine/cr.html#operationpolicy)), которые проверяют параметры, не связанные с безопасностью, например разрешённые registry образов, и политики безопасности ([SecurityPolicy](/modules/admission-policy-engine/cr.html#securitypolicy)), которые проверяют параметры безопасности, например привилегированный режим или доступ к неймспейсам хоста.

Политики дополняют друг друга: если к [неймспейсу](glossary.html#namespace) применяется несколько политик, объект должен удовлетворять каждой. При изменении политики существующие поды проверяются при следующем перезапуске, а для работающих объектов, нарушающих политику, Deckhouse Platform создаёт [алерт](glossary.html#alert). [SecurityPolicyException](/modules/admission-policy-engine/cr.html#securitypolicyexception) позволяет задать точечные исключения для отдельных подов или контейнеров.

Подробнее в разделе [«Политики безопасности»](admin/configuration/security/policies.html).
