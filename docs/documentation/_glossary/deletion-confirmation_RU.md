---
title: Подтверждение удаления
description: Подтверждение, которое пользователь должен дать в веб-интерфейсе перед удалением приложения или отключением модуля. Объявляется в секции disable файла package.yaml или module.yaml вместе с сообщением для пользователя.
lang: ru
---

Для [пакета Marketplace](glossary.html#marketplace-package) секция `disable` задаёт `confirmation: true` и локализованные сообщения, например предупреждение, что удаление приложения удаляет данные в его томах. Deckhouse Platform публикует секцию в поле `status.packageMetadata.disableOptions` версии пакета, а [веб-интерфейс Deckhouse](glossary.html#web-ui) показывает сообщение на языке пользователя. При удалении Application через API Kubernetes подтверждение не проверяется.

Для [модуля](glossary.html#module) так же работают параметры `disable.confirmation` и `disable.message` файла `module.yaml` при отключении модуля.

Подробнее в разделе [«Разработка приложений»](architecture/marketplace/application-development.html).
