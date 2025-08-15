---
title: "Модуль registry: настройка"
description: ""
---

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}

Для настройки параметров работы с container registry используйте секцию [`registry`](../deckhouse/configuration.html#parameters-registry) конфигурации модуля `deckhouse`.

В секции указываются:

- Режим доступа к container registry.
- Параметры для режима доступа `Direct`:
  - Корневой сертификат центра сертификации.
  - Адрес репозитория в container registry.
  - Лицензионный ключ для доступа к container registry.
  - Пароль для аутентификации в container registry.
  - Протокол для подключения к container registry.
  - Имя пользователя для аутентификации в container registry.
