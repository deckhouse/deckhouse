---
title: Режим обслуживания модуля или приложения
description: "Режим, в котором Deckhouse Platform перестаёт применять шаблоны и восстанавливать ресурсы модуля или приложения, чтобы их можно было менять вручную. Включается полем spec.maintenance: NoResourceReconciliation и не связан с режимом обслуживания узла."
lang: ru
---

В этом режиме изменения настроек и values не отражаются в ресурсах, удалённые ресурсы не восстанавливаются, хуки продолжают работать. Ресурсы, отрендеренные из шаблонов, получают лейбл `maintenance.deckhouse.io/no-resource-reconciliation`, и [admission-политики](glossary.html#admission-policy) перестают их защищать. У [экземпляра приложения](glossary.html#application-instance) условие `Managed` становится `False`, срабатывает алерт `ApplicationIsInMaintenanceMode`. Чтобы вернуть обычный режим, удалите поле `spec.maintenance`.

Поле задаётся в [ModuleConfig](reference/api/cr.html#moduleconfig) для [модуля](glossary.html#module) и в [Application](reference/api/cr.html#application) для приложения. Режим предназначен для отладки и разработки, а не для production-кластеров. [Режим обслуживания узла](glossary.html#maintenance-mode) — другой механизм.

Подробнее в разделе [«Жизненный цикл и отладка»](architecture/marketplace/lifecycle.html).
