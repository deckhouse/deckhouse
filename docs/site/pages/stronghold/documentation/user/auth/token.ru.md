---
title: "Токен"
permalink: ru/stronghold/documentation/user/auth/token.html
lang: ru
---

### Метод аутентификации по токену (Token auth)

Метод аутентификации по токену является встроенным и автоматически доступен по адресу `/auth/token`. Он позволяет пользователям проходить аутентификацию с помощью токена, а также создавать новые токены, отзывать секреты по токену и т.д.

Когда любой другой метод аутентификации возвращает идентификатор, ядро Deckhouse Stronghold вызывает метод token для создания нового уникального токена для этого идентификатора.

Хранилище токенов также может быть использовано в обход любого другого метода аутентификации: вы можете создавать токены напрямую, а также выполнять различные другие операции с токенами, такие как обновление и отзыв.

#### Аутентификация

##### Через CLI

В этом примере пользователь выполняет вход в систему, используя токен:

```shell
d8 stronghold login token=<token>
```

В следующем примере пользователь выполняет вход в систему с использованием метода аутентификации `userpass`. Пользователь вводит свои учетные данные в формате `username=значение` и `password=значение`.

```shell
d8 stronghold login -method=userpass \
   username=mitchellh \
   password=foo
```

##### Через API

Токен задается непосредственно в виде заголовка для HTTP API. Заголовок должен иметь вид X-Vault-Token: &lt;token> или Authorization: Bearer &lt;token>.

```shell
curl \
   --request POST \
   --data '{"password": "foo"}' \
   http://127.0.0.1:8200/v1/auth/userpass/login/mitchellh
```

В ответе будет содержаться токен по адресу `auth.client_token`, как представлено ниже в примере:

```json
{
   "lease_id": "",
   "renewable": false,
   "lease_duration": 0,
   "data": null,
   "auth": {
      "client_token": "c4f280f6-fdb2-18eb-89d3-589e2e834cdb",
      "policies": ["admins"],
      "metadata": {
         "username": "mitchellh"
      },
      "lease_duration": 0,
      "renewable": false
   }
}
```
