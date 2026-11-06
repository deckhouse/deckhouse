---
title: Приостановка приложения
description: Состояние установленного приложения, требования пакета которого перестали выполняться, например отключён обязательный модуль. Deckhouse Platform удаляет Helm-релиз, но сохраняет [Application](reference/api/cr.html#application) в состоянии Suspended.
lang: ru
---

Deckhouse Platform проверяет требования пакета постоянно, а не только при создании. Когда они перестают выполняться, выполняются хуки `beforeDeleteHelm`, удаляется Helm-релиз, а с ним ресурсы [экземпляра приложения](glossary.html#application-instance), затем выполняются хуки `afterDeleteHelm`. [Orphan-ресурсы](glossary.html#application-orphan-resources) не удаляются. Когда требования снова выполняются, приложение устанавливается заново с теми же настройками.

Ресурсы удаляются вместе с Helm-релизом, включая PersistentVolumeClaim. Чтобы сохранить данные, добавьте им аннотацию `helm.sh/resource-policy: keep`.

Подробнее в разделе [«Жизненный цикл и отладка»](architecture/marketplace/lifecycle.html).
