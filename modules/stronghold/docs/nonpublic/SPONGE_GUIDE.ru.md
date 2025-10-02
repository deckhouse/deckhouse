## Дисклеймер

- Создает и удаляет следующие политики: read, manage, manage-with-totp
- Добавляет в stronghold пользователей (entity) и aliases для них, добавляет им политики.
- NOTE: bob создает Entity с именем, равным UUID в Bush и добавляет на него Alias. Если этот Alias был на другой Entity,
  то Alias ПЕРЕВЕШИВАЕТСЯ на новый.
- NOTE: если у Entity были какие-то политики, то они заменяются теми, что загружены из Bush

## Схема

TODO обновить!!!
[image](images/spongebob.png)

### Настройка политик

Настройка политик производится в автоматическом режиме по проектам, которые периодически запрашиваются из bush.
На каждый полученный проект создается 3 политики:

#### read:

   ```json
   {
  "path": {
    "projects/+/734a77ca-f716-4964-bae5-ef491d960958/*": {
      "capabilities": [
        "read",
        "list"
      ]
    },
    "sponge/projects": {
      "capabilities": [
        "read"
      ]
    },
    "totp/code/734a77ca-f716-4964-bae5-ef491d960958/*": {
      "capabilities": [
        "read"
      ]
    },
    "totp/keys/734a77ca-f716-4964-bae5-ef491d960958": {
      "capabilities": [
        "list"
      ]
    }
  }
}
   ```

#### manage:

   ```json
   {
  "path": {
    "projects/+/734a77ca-f716-4964-bae5-ef491d960958/*": {
      "capabilities": [
        "read",
        "list",
        "create",
        "update",
        "delete"
      ]
    },
    "sponge/projects": {
      "capabilities": [
        "read"
      ]
    },
    "totp/code/734a77ca-f716-4964-bae5-ef491d960958/*": {
      "capabilities": [
        "read"
      ]
    },
    "totp/keys/734a77ca-f716-4964-bae5-ef491d960958": {
      "capabilities": [
        "list"
      ]
    }
  }
}
   ```

#### manage-with-totp:

   ```json
   {
  "path": {
    "projects/+/734a77ca-f716-4964-bae5-ef491d960958/*": {
      "capabilities": [
        "read",
        "list",
        "create",
        "update",
        "delete"
      ]
    },
    "sponge/projects": {
      "capabilities": [
        "read"
      ]
    },
    "totp/code/734a77ca-f716-4964-bae5-ef491d960958/*": {
      "capabilities": [
        "read"
      ]
    },
    "totp/keys/734a77ca-f716-4964-bae5-ef491d960958": {
      "capabilities": [
        "list"
      ]
    },
    "totp/keys/734a77ca-f716-4964-bae5-ef491d960958/*": {
      "capabilities": [
        "read",
        "list",
        "create",
        "update",
        "delete"
      ]
    }
  }
}
   ```

### Настройка пользователей

Настройка пользователей производится в автоматическом режиме по пользователям, которые периодически запрашиваются из
bush.
Каждый пользователь имеет список разрешенных проектов и уровень доступа к ним.
Плагин Sponge производит привязку пользователя, проекта и политики согласно информации, полученной из bush.
Пример связки:

```json
{
  "projects": {
    "some_project_uuid_1": {
      "secret": "manage",
      "totp": "manage"
    },
    "some_project_uuid_2": {
      "secret": "manage",
      "totp": "read"
    },
    "some_project_uuid_3": {
      "secret": "read",
      "totp": "manage"
    },
    "some_project_uuid_4": {
      "secret": "read",
      "totp": "read"
    }
  }
}
```

Для полученных проектов пользователя плагин Sponge привяжет такие политики:

- `"some_project_uuid_1":"manage-with-totp"`,
- `"some_project_uuid_2":"manage"`,
- `"some_project_uuid_3":"manage-with-totp"`,
- `"some_project_uuid_4":"read"`

### Настройка sponge

```shell
vault write sponge/configure \
    ca_bush=xxx \
    ca_stronghold=xxx \
    stronghold_client_cert=xxx \
    stronghold_client_key=xxx \
    stronghold_address=xxx \
    stronghold_token=xxx \
    bush_token=xxx \
    user_url=xxx \
    project_url=xxx \
    oidc_mount_accessor=xxx \
    log_level=info
```

- `ca_bush` - сертификат для подключения к Bush.
- `ca_stronghold` - сертификат для подключения к Stronghold.
- `stronghold_client_cert` - сертификат, чтобы доверять ответам от сервера stronghold в плагине Sponge.
- `stronghold_client_key` - ключ чтобы, доверять ответам от сервера stronghold в плагине Sponge.
- `stronghold_address` - адрес на API кластера stronghold.
- `stronghold_token` - токен для подключения к серверу stronghold, позволяющий создавать policies, aliases и entity из
  плагина Sponge.
- `bush_token` - токен для походов в Bush.
- `user_url` - адрес для получения списка users из Bush. Пример:
  `user_url=https://bush.flant.com/external_api/sponge/users`
- `project_url` - адрес для получения списка projects из Bush. Пример:
  `project_url=https://bush.flant.com/external_api/sponge/projects`
- `oidc_mount_accessor` - id auth для которого будут добавляться aliases (oidc Keycloak)
- `log_level` - уровень логирования для отладки инцидентов.

### Sponge API

1. GET `<mount point name>/projects`
   Возвращает список проектов, которые доступны пользователю.
   Пример Response:

   ```json
   {
      "request_id": "6555940d-38e5-d499-98a7-01b56bb32bfb",
      "lease_id": "",
      "renewable": false,
      "lease_duration": 0,
      "data": {
         "projects": [
            {
               "full_identifier": "stronhold-demo-project",
               "secret_permission": "manage",
               "totp_permission": "read",
               "team_identifier": "stronghold",
               "team_uuid": "99999999-bc89-464d-9a2a-70ecbcdbc5f8",
               "uuid": "77777777-f716-4964-bae5-ef491d960958"
            },
            {
               "full_identifier": "flant",
               "secret_permission": "read",
               "totp_permission": "read",
               "team_identifier": "mike",
               "team_uuid": "4a2d01f2-67c4-45ad-8dde-9212ed4dc82b",
               "uuid": "5f682e20-10e9-4a07-9ae1-3e62bf239552"
            }
         ]
      },
      "wrap_info": null,
      "warnings": null,
      "auth": null
   }
   ```

2. GET `<mount point name>/userinfo`
   Возвращает информацию о текущем пользователе.
   Пример Response Body:

   ```json
   {
      "email": "user.example@flant.com",
      "uuid": "47431f6d-e2b5-4bf5-89f3-c773940fac88"
   }
   ```
3. POST `<mount point name>/configure`

   Настройки модуля записываются и хранятся в storage.
   Пример Request Body:

   ```json
   {
      "bush_token": "token",
      "ca_bush": "-----BEGIN CERTIFICATE-----XXX-----END CERTIFICATE-----\n",
      "ca_stronghold": "-----BEGIN CERTIFICATE-----XXX-----END CERTIFICATE-----\n",
      "log_level": "trace",
      "oidc_mount_accessor": "auth_userpass_e5b59894",
      "user_url": "https://localhost:8080/external_api/sponge/users",
      "project_url": "https://localhost:8080/external_api/sponge/projects",
      "stronghold_address": "https://127.0.0.1:8200",
      "stronghold_client_cert": "-----BEGIN CERTIFICATE-----XXX-----END CERTIFICATE-----\n",
      "stronghold_client_key": "-----BEGIN RSA PRIVATE KEY-----XXX-----END RSA PRIVATE KEY-----\n",
      "stronghold_token": "hvs.v4JQlT6v3Zth7XPopMimPtIG"
   }
   ```
4. GET `<mount point name>/configure`
   Показывает только публичные данные конфигурации плагина Sponge.
   Пример Response Body:

   ```json
   {
      "log_level": "trace",
      "user_url": "https://localhost:8080/external_api/sponge/users",
      "project_url": "https://localhost:8080/external_api/sponge/projects",
      "stronghold_address": "https://127.0.0.1:8200"
   }
   ```

### Stronghold API Extension for KV-V2

1. GET `<mount point name>/bob-find/<secret path>/match=<search substring>&max_matched=10`

   Производит глубокий поиск по совпадению имен ключей и имен путей в пределах переданной связки
   `<mount point name>/<secret path>`.
   Результатом поиска является список путей и имен ключей, которые содержат вхождение строки `<search substring>` из
   переданного параметра `match`.
   Для ограничения количества вхождений можно передать параметр `max_matched`, который ограничит результат поиска.
   По умолчанию параметр `max_matched` равен 50.

   Пример использования:
    - Пример структуры в Stronghold:
       ```json
          {
           "<mount point name>":{
             "project-uuid-1":{
               "path-1": {
                 "sub-path-1": {
                    "sub-path-key-1": "value-1", 
                    "sub-path-key-2": "value-2" 
                 },
                 "sub-path-2": {
                    "sub-path-key-3": "value-1", 
                    "sub-path-key-4": "value-2" 
                 },
                 "sub-path-3": {
                    "sub-path": {
                       "key": "value"
                    } 
                 }
               }
             },
             "project-uuid-2":{
               "path-2": {
                 "sub-path-3": {
                  "sub-path-key-1": "value-1", 
                  "sub-path-key-2": "value-2" 
                 }
               }
             }
           }
          }
       ```
    - Пример запроса, который вернет список путей и имен ключей из `project-uuid-1/path-1/`, которые содержат вхождение
      строки `path-` из переданного параметра `match`:

      `curl -H "X-Vault-Request: true" -H "X-Vault-Token: ${VAULT_TOKEN}" http://127.0.0.1:8200/v1/kv-v2/bob-find/project-uuid-1/path-1\?match\=path-\`
    - Результат запроса:

      ```json
      {
         "request_id": "b4644cee-7fd2-e850-3a9b-55d1abf12035",
         "lease_id": "",
         "renewable": false,
         "lease_duration": 0,
         "data": {
            "matches": [
              {
                "path": "project-uuid-1/path-1/sub-path-1"
              },
              {
                "path": "project-uuid-1/path-1/sub-path-2"
              },
              {
                "path": "project-uuid-1/path-1/sub-path-1",
                "key": "sub-path-key-1"
              },
              {
                "path": "project-uuid-1/path-1/sub-path-1",
                "key": "sub-path-key-2"
              },
              {
                "path": "project-uuid-1/path-1/sub-path-2",
                "key": "sub-path-key-1"
              },
              {
                "path": "project-uuid-1/path-1/sub-path-2",
                "key": "sub-path-key-2"
              },
              {
                "path": "project-uuid-1/path-1/sub-path-3/"
              }
           ]
         },
         "wrap_info": null,
         "warnings": null,
         "auth": null
      }
      ```

2. GET `<mount point name>/bob-keys-list/<secret path>`

   Возвращает список ключей без значений в переданном секрете. Принцип работы аналогичен операции read для data.
    - Пример запроса, который вернет список всех ключей без их значений для `project-uuid-1/path-1/sub-path-1`:

      `curl -H "X-Vault-Request: true" -H "X-Vault-Token: ${VAULT_TOKEN}" http://127.0.0.1:8200/v1/kv-v2/bob-keys-list/project-uuid-1/path-1`

    - Результат запроса:

     ```json
     {
       "request_id": "98dcad3e-7f2c-9f96-ea03-d7cec4183447",
       "lease_id": "",
       "renewable": false,
       "lease_duration": 0,
       "data": {
         "data": [
           "sub-path-key-1",
           "sub-path-key-2"
         ],
         "metadata": {
           "created_time": "2025-03-06T13:46:08.835919Z",
           "custom_metadata": null,
           "deletion_time": "",
           "destroyed": false,
           "version": 1
         }
       },
       "wrap_info": null,
       "warnings": null,
       "auth": null
     }
     ```
3. GET `<mount point name>/bob-get-value/<secret path>?key=<key in secret>`

   Возвращает значение по ключу из секрета. Принцип работы аналогичен операции read для data.
   - Пример запроса, который вернет значение по ключу для `project-uuid-1/path-1/sub-path-1?key=sub-path-key-1`:

     `curl -H "X-Vault-Request: true" -H "X-Vault-Token: ${VAULT_TOKEN}" http://127.0.0.1:8200/v1/kv-v2/bob-get-value/project-uuid-1/path-1/sub-path-1?key=sub-path-key-1`

   - Результат запроса:

     ```json
     {
       "request_id": "98dcad3e-7f2c-9f96-ea03-d7cec4183447",
       "lease_id": "",
       "renewable": false,
       "lease_duration": 0,
       "data": {
         "data": {
           "sub-path-key-1": "value-1"
         },
         "metadata": {
           "created_time": "2025-03-06T13:46:08.835919Z",
           "custom_metadata": null,
           "deletion_time": "",
           "destroyed": false,
           "version": 1
         }
       },
       "wrap_info": null,
       "warnings": null,
       "auth": null
     }
     ```

## Тестовый стенд

Vault https://bob.demo-cluster.ru:8200/

JSON https://bob.demo-cluster.ru/users.json https://bob.demo-cluster.ru/projects.json
