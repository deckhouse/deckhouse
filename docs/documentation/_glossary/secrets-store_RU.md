---
title: Хранилище секретов
description: Внешнее хранилище, совместимое с API HashiCorp Vault, или встроенный Deckhouse Stronghold, из которого модуль [`secrets-store-integration`](/modules/secrets-store-integration/) доставляет секреты приложениям, не сохраняя их в etcd.
lang: ru
---

Модуль [`secrets-store-integration`](/modules/secrets-store-integration/) доставляет секреты в поды в виде смонтированных файлов или переменных окружения, обновляет смонтированные секреты каждые две минуты при изменении значения в хранилище и внедряет entrypoint для приложений, которые не могут читать секреты из хранилища самостоятельно. К внутреннему экземпляру Deckhouse Stronghold модуль подключается автоматически в режиме `DiscoverLocalStronghold`, с любым Vault-совместимым хранилищем работает в режиме `Manual`.

Подробнее в разделе [«Модуль secrets-store-integration»](architecture/security/secrets-store-integration.html).
