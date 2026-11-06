---
title: Orphan-ресурсы приложения
description: Объекты, которые остаются в неймспейсе после удаления Helm-релиза приложения, например объекты, созданные нагрузкой или хуками во время работы. Deckhouse Platform удаляет перечисленные типы вместе с объектом [Application](reference/api/cr.html#application).
lang: ru
---

Пакет перечисляет типы orphan-ресурсов в поле `orphanResources` файла `openapi/values.yaml`. При удалении Application Deckhouse Platform удаляет объекты этих типов в неймспейсе [экземпляра приложения](glossary.html#application-instance) с лейблом `packages.deckhouse.io/instance: <INSTANCE_NAME>` после удаления Helm-релиза и остановки хуков. Объекты, созданные нагрузкой и хуками, не получают лейбл автоматически.

Orphan-ресурсы удаляются только при удалении Application. При [приостановке приложения](glossary.html#application-suspension) и при смене версии пакета они остаются в кластере.

Подробнее в разделе [«Жизненный цикл и отладка»](architecture/marketplace/lifecycle.html).
