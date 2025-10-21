---
title: "Провайдер идентификации OIDC"
permalink: ru/stronghold/documentation/user/secrets-engines/identity/oidc-provider.html
description: Установка и настройка Stronghold в качестве провайдера идентификации OpenID Connect (OIDC).
lang: ru
---

Stronghold является провайдером идентификации OpenID Connect ([OIDC](https://openid.net/specs/openid-connect-core-1_0.html))
провайдер идентификации. Это позволяет клиентским приложениям, использующим протокол OIDC, использовать
Stronghold как источник [идентификации](../../concepts/identity.html) и широкий спектр [методов аутентификации](../../concepts/auth.html) при аутентификации конечных пользователей. Клиентские приложения могут настраивать свою логику аутентификации
для взаимодействия с Stronghold. После включения Stronghold будет действовать как мост к другим провайдерам идентификации через
существующие методы аутентификации. Клиентские приложения также могут получать информацию об идентификации
для своих конечных пользователей, используя пользовательскую шаблонизацию идентификационной информации Stronghold.

## Установка

Система провайдеров Stronghold OIDC построена на основе механизма секретов идентификации.
Этот механизм секретов установлен по умолчанию и не может быть отключен или перемещен.

Каждое пространство имен Stronghold по умолчанию имеет OIDC provider и ключ. Эта встроенная конфигурация позволяет клиентским приложениям начать использовать Stronghold в качестве источника идентификации с минимальными настройками.

Следующие шаги показывают минимальную конфигурацию, которая позволяет клиентскому приложению использовать
Stronghold в качестве OIDC-провайдера.

1. Включите метод аутентификации Stronghold:

```text
$ d8 stronghold auth enable userpass
Success! Enabled userpass auth method at: userpass/
```

   В режиме OIDC можно использовать любой метод аутентификации Stronghold. Для простоты включите
   метод аутентификации `userpass`.

1. Создайте пользователя:

```text
$ d8 stronghold write auth/userpass/users/end-user password="securepassword"
Success! Data written to: auth/userpass/users/end-user
```

Этот пользователь аутентифицируется в Stronghold через клиентское приложение, иначе известное как
OIDC [relying party](https://openid.net/specs/openid-connect-core-1_0.html#Terminology).

1. Создайте клиентское приложение:

```text
$ d8 stronghold write identity/oidc/client/my-webapp \
  redirect_uris="https://localhost:9702/auth/oidc-callback" \
  assignments="allow_all"
Success! Data written to: identity/oidc/client/my-webapp
```

   Эта операция создает клиентское приложение, которое может быть использовано для настройки OIDC доверяющей стороны.

   Параметр `assignments` ограничивает сущности и группы Stronghold, которым разрешена
   аутентификация через клиентское приложение. По умолчанию ни одной сущности Stronghold это не разрешено.
   Чтобы разрешить аутентификацию всем сущностям Stronghold, используется встроенное назначение `allow_all`.

1. Считывание учетных данных клиента:

```text
$ d8 stronghold read identity/oidc/client/my-webapp

Key                 Value
---                 -----
access_token_ttl    24h
assignments         [allow_all]
client_id           GSDTnn3KaOrLpNlVGlYLS9TVsZgOTweO
client_secret       hvo_secret_gBKHcTP58C4aq7FqPWsuqKgpiiegd7ahpifGae9WGkHRCwFEJTZA9KGdNVpzE0r8
client_type         confidential
id_token_ttl        24h
key                 default
redirect_uris       [https://localhost:9702/auth/oidc-callback]
```

Параметры `client_id` и `client_secret` - это учетные данные клиентского приложения. Эти
значения обычно требуются при настройке доверяющей стороны OIDC.

1. Прочитайте конфигурацию обнаружения OIDC:

```text
$ curl -s http://127.0.0.1:8200/v1/identity/oidc/provider/default/.well-known/openid-configuration
{
  "issuer": "http://127.0.0.1:8200/v1/identity/oidc/provider/default",
  "jwks_uri": "http://127.0.0.1:8200/v1/identity/oidc/provider/default/.well-known/keys",
  "authorization_endpoint": "http://127.0.0.1:8200/ui/vault/identity/oidc/provider/default/authorize",
  "token_endpoint": "http://127.0.0.1:8200/v1/identity/oidc/provider/default/token",
  "userinfo_endpoint": "http://127.0.0.1:8200/v1/identity/oidc/provider/default/userinfo",
  "request_parameter_supported": false,
  "request_uri_parameter_supported": false,
  "id_token_signing_alg_values_supported": [
    "RS256",
    "RS384",
    "RS512",
    "ES256",
    "ES384",
    "ES512",
    "EdDSA"
  ],
  "response_types_supported": [
    "code"
  ],
  "scopes_supported": [
    "openid"
  ],
  "subject_types_supported": [
    "public"
  ],
  "grant_types_supported": [
    "authorization_code"
  ],
  "token_endpoint_auth_methods_supported": [
    "none",
    "client_secret_basic",
    "client_secret_post"
  ]
}
```

Каждый провайдер Stronghold OIDC публикует [метаданные обнаружения](https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata).
Значение `issuer` обычно требуется при настройке доверяющей стороны OIDC.

## Использование

После настройки метода аутентификации Stronghold и клиентского приложения следующие сведения можно
могут быть использованы для настройки OIDC доверяющей стороны для делегирования аутентификации конечного пользователя Stronghold.

- `client_id` - Идентификатор клиентского приложения
- `client_secret` - Секрет клиентского приложения
- `issuer` - Эмитент OIDC-провайдера Stronghold.

В противном случае подробности использования см. в документации конкретной доверяющей стороны OIDC.

## Поддерживаемые процессы

Функция провайдера Stronghold OIDC в настоящее время поддерживает следующий процесс аутентификации:

- [Authorization Code Flow](https://openid.net/specs/openid-connect-core-1_0.html#CodeFlowAuth).
