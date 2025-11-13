---
title: "Руководство администратора модуля stronghold по API"
LinkTitle: "Руководство администратора по API"
description: "Руководство администратора по работе с API модуля stronghold."
---

## Методы аутентификации

Каждый метод аутентификации имеет свой собственный набор API-путей и методов, которые описаны в разделе. Методы аутентификации могут быть активированы по определенному пути, но для упрощения документация будет предполагать использование путей по умолчанию. Если вы активируете методы аутентификации по другому пути, вам следует скорректировать ваши API-запросы соответственно.

### AppRole

Этот раздел подразумевает, что метод активирован по пути `/auth/approle`.

#### Вывод списка ролей

Этот путь возвращает список существующих AppRoles в методе.

| Метод | Путь |
|-------|------|
| LIST  | /auth/approle/role |

Пример запроса: 

```shell
curl \
  --header "X-Vault-Token: ${VAULT_TOKEN}" \
  --request LIST \
    ${VAULT_ADDR}/v1/auth/approle/role
```

Пример ответа API:

```json
{
  "auth": null,
  "warnings": null,
  "wrap_info": null,
  "data": {
    "keys": ["dev", "prod", "test"]
  },
  "lease_duration": 0,
  "renewable": false,
  "lease_id": ""
}
```

#### Создание или обновление AppRole

Создаёт новую AppRole или обновляет существующую AppRole. Путь поддерживает и создание, и обновление возможностей метода. both create and update capabilities. На роль может быть наложено одно или несколько ограничений. Необходимо, чтобы хотя бы одно из них было включено при создании или обновлении роли.

| Метод | Путь |
|-------|------|
| POST  | /auth/approle/role/:role_name |

Параметры:

- role_name (строка: <required>) - Имя AppRole. Должно быть короче 4096 байт, допустимые символы включают a-Z, 0-9, пробел, тире, подчеркивания и точки.
- bind_secret_id (булевый: true) - Требуется, чтобы secret_id был представлен при входе с использованием этого AppRole
- secret_id_bound_cidrs (массив: []) - Строка, разделенная запятыми, или список блоков CIDR; установленное значение указывает блоки IP-адресов, которые могут выполнять операцию входа.
- secret_id_num_uses (целое число: 0) - Количество раз, которое любой конкретный SecretID может использоваться для получения токена из этого AppRole, после чего SecretID по умолчанию истекает. Установка значения равного нулю позволяет неограниченное использование. Однако этот параметр может быть переопределен полем 'num_uses' запроса при создании SecretID.
- secret_id_ttl (строка: "") - Продолжительность в виде целого числа секунд (3600) или целевого временного интервала (60m), после чего любой SecretID по умолчанию истекает. Установка значения равного нулю позволит SecretID не истекать. Однако этот параметр может быть переопределен полем 'ttl' запроса при создании SecretID.
- local_secret_ids (булевый: false) - Если установлено, секретные ID, созданные с использованием этой роли, будут локальными для кластера. Этот параметр можно установить только при создании роли и изменить позднее невозможно.
- token_ttl (целое число: 0 или строка: "") - Инкрементальный срок действия для созданных токенов. Текущее значение этого параметра будет учтено при продлении.
- token_max_ttl (целое число: 0 или строка: "") - Максимальный срок действия для созданных токенов. Текущее значение этого параметра будет учтено при продлении.
- token_policies (массив: [] или строка с разделением запятыми: "") - Список политик токенов, которые будут добавлены в создаваемые токены. В зависимости от метода аутентификации, этот список может быть дополнен значениями пользователя/группы/другими значениями.
- token_bound_cidrs (массив: [] или строка с разделением запятыми: "") - Список блоков CIDR; установленное значение указывает блоки IP-адресов, которые могут успешно аутентифицироваться, и связывает полученный токен с этими блоками.
- token_explicit_max_ttl (целое число: 0 или строка: "") - Если установлено, будет добавлен явный максимальный срок действия токена. Это жесткое ограничение, даже если token_ttl и token_max_ttl позволили бы продление.
- token_no_default_policy (булевый: false) - Если установлено, политика по умолчанию не будет установлена на создаваемых токенах; в противном случае она будет добавлена к установленным политикам в token_policies.
- token_num_uses (целое число: 0) - Максимальное количество раз, которое может быть использован созданный токен (в пределах его срока действия); 0 означает неограниченное количество раз. Если вам необходимо, чтобы токен имел возможность создавать дочерние токены, установите значение на 0.
- token_period (целое число: 0 или строка: "") - Максимальное допустимое значение периода времени, когда запрашивается периодический токен из этой роли.
- token_type (строка: "") - Тип токена, который должен быть создан. Значение может быть равно service, batch, или default для использования настроенного по умолчанию значения (которое, если не изменено, будет service токенами). Для ролей хранилища токенов есть две дополнительные возможности: default-service и default-batch, которые указывают тип для возврата, если клиент не запросит другой тип при создании. Для случаев аутентификации, основанной на машинном взаимодействии, используйте токены типа batch.


Пример данных: 

```json
{
  "token_type": "batch",
  "token_ttl": "10m",
  "token_max_ttl": "15m",
  "token_policies": ["default"],
  "period": 0,
  "bind_secret_id": true
}
```

Пример запроса:

```shell
curl \
  --header "X-Vault-Token: ${VAULT_TOKEN}" \
  --request POST \
  --data @payload.json \
    ${VAULT_ADDR}/v1/auth/approle/role/application
```

Пример ответа API:

```shell
{
  "auth": null,
  "warnings": null,
  "wrap_info": null,
  "data": {
    "keys": ["dev", "prod", "test"]
  },
  "lease_duration": 0,
  "renewable": false,
  "lease_id": ""
}
```

#### Чтение AppRole

Выводит свойства существующего AppRole.

| Метод   | Путь                            |
| :----- | :------------------------------ |
| `GET`  | `/auth/approle/role/:role_name` |

Параметры:

- `role_name` `(string: <required>)` - Имя AppRole. Должно быть короче 4096 байт.

Пример запроса:

```shell
$ curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    ${VAULT_ADDR}/v1/auth/approle/role/application1
```

Пример ответа API:

```json
{
  "auth": null,
  "warnings": null,
  "wrap_info": null,
  "data": {
    "token_ttl": 1200,
    "token_max_ttl": 1800,
    "secret_id_ttl": 600,
    "secret_id_num_uses": 40,
    "token_policies": ["default"],
    "period": 0,
    "bind_secret_id": true,
    "secret_id_bound_cidrs": []
  },
  "lease_duration": 0,
  "renewable": false,
  "lease_id": ""
}
```

#### Удаление AppRole

Удаляет существующий AppRole из метода.

| Метод   | Путь                            |
| :------- | :------------------------------ |
| `DELETE` | `/auth/approle/role/:role_name` |

Параметры:

- `role_name` `(строка: <required>)` - Имя AppRole. Должно быть короче 4096 байт.

Пример запроса:

```shell
$ curl \
  --header "X-Vault-Token: ${VAULT_TOKEN}" \
  --request DELETE \
  ${VAULT_ADDR}/v1/auth/approle/role/application1
```


#### Чтение RoleID AppRole

Выводит RoleID существующего AppRole.

| Метод   | Путь                                    |
| :----- | :-------------------------------------- |
| `GET`  | `/auth/approle/role/:role_name/role-id` |

Параметры:

- `role_name` `(string: <required>)` - Имя AppRole. Должно быть короче 4096 байт.

Пример запроса:

```shell
$ curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    ${VAULT_ADDR}/v1/auth/approle/role/application1/role-id
```

Пример ответа API:

```json
{
  "auth": null,
  "warnings": null,
  "wrap_info": null,
  "data": {
    "role_id": "e5a7b66e-5d08-da9c-7075-71984634b882"
  },
  "lease_duration": 0,
  "renewable": false,
  "lease_id": ""
}
```

#### Обновление RoleID AppRole

Обновляет RoleID существующей роли AppRole в заданное значение.

| Метод   | Путь                                    |
| :----- | :-------------------------------------- |
| `POST` | `/auth/approle/role/:role_name/role-id` |

Параметры:

- `role_name` `(string: <required>)` - Имя AppRole. Должно быть короче 4096 байт.
- `role_id` `(string: <required>)` - Значение RoleID.

Пример данных: 

```json
{
  "role_id": "custom-role-id"
}
```

Пример запроса:

```shell
$ curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request POST \
    --data @payload.json \
    ${VAULT_ADDR}/v1/auth/approle/role/application1/role-id
```

#### Генерация нового SecretID

Генерирует и выводит новый SecretID для существующего AppRole. Аналогично токенам, ответ также будет содержать значение `secret_id_accessor`, которое можно использовать для чтения свойств секрета без раскрытия самого индентификатора, а также для удаления SecretID из AppRole.

| Метод   | Путь                                      |
| :----- | :---------------------------------------- |
| `POST` | `/auth/approle/role/:role_name/secret-id` |

Параметры:

- `role_name` `(string: <required>)` - Имя AppRole. Должно быть короче 4096 байт.
- `metadata` `(строка: "")` - Метаданные, связанные с SecretID. Это должна быть строка в формате JSON, содержащая метаданные в виде пар ключ-значение. Эти метаданные будут установлены на токены, выданные с использованием этого SecretID, и будут записаны в журнал аудита _в открытом виде_.
- `cidr_list` `(массив: [])` - Строка, разделенная запятыми, или список блоков CIDR, ограничивающих использование SecretID из определенного набора IP-адресов. Если на роли установлено значение `secret_id_bound_cidrs`, то список блоков CIDR, указанный здесь, должен быть подмножеством блоков CIDR, указанных на роли.
- `token_bound_cidrs` `(массив: [])` - Строка, разделенная запятыми, или список блоков CIDR; если установлено, указывает блоки IP-адресов, которые могут использовать аутентификационные токены, созданные с помощью этого SecretID. Переопределяет значение, установленное на роли, но должно быть подмножеством.
- `num_uses` `(целое число: 0)` - Количество раз, которое этот SecretID может быть использован, после чего SecretID истекает. Значение ноль позволит неограниченное использование. Переопределяет параметр secret_id_num_uses роли, когда указан.
  Не может быть больше, чем secret_id_num_uses роли.
- `ttl` `(строка: "")` - Продолжительность в секундах (`3600`) или целое число временных единиц (`60m`), после которой этот SecretID истекает. Значение ноль позволит SecretID не истекать. Переопределяет параметр secret_id_ttl роли, когда указан.
  Не может быть дольше, чем secret_id_ttl роли.

Пример данных: 

```json
{
  "metadata": "{ \"tag1\": \"production\" }",
  "ttl": 600,
  "num_uses": 50
}
```

Пример запроса:

```shell
$ curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request POST \
    --data @payload.json \
    ${VAULT_ADDR}/v1/auth/approle/role/application1/secret-id
```

Пример ответа API:

```json
{
  "auth": null,
  "warnings": null,
  "wrap_info": null,
  "data": {
    "secret_id_accessor": "84896a0c-1347-aa80-a3f6-aca8b7558780",
    "secret_id": "841771dc-11c9-bbc7-bcac-6a3945a69cd9",
    "secret_id_ttl": 600,
    "secret_id_num_uses": 50
  },
  "lease_duration": 0,
  "renewable": false,
  "lease_id": ""
}
```

#### Список идентификаторов доступа SecretID

Выводит идентификаторы доступа всех выданных SecretID для AppRole.
Это включает идентификаторы доступа для "пользовательских" SecretID.

| Метод   | Путь                                     |
| :----- | :---------------------------------------- |
| `LIST` | `/auth/approle/role/:role_name/secret-id` |

Параметры:

- `role_name` `(string: <required>)` - Имя AppRole. Должно быть короче 4096 байт.

Пример запроса:

```shell
$ curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request LIST \
    ${VAULT_ADDR}/v1/auth/approle/role/application1/secret-id
```

Пример ответа API:

```json
{
  "auth": null,
  "warnings": null,
  "wrap_info": null,
  "data": {
    "keys": [
      "ce202d2a-8253-c437-bf9a-aceed4241491",
      "a1c0dee4-b869-e68d-3520-2040c1a0849a",
      "be03b7e2-044c-7244-07e1-47560ca1c787",
      "84896a0c-1347-aa80-a3f6-aca8b7558780",
      "439b1328-6523-15e7-403a-a48038cdc45a"
    ]
  },
  "lease_duration": 0,
  "renewable": false,
  "lease_id": ""
}
```

#### Чтение SecretID AppRole

Выводит свойства SecretID AppRole.

| Метод   | Путь                                            |
| :----- | :----------------------------------------------- |
| `POST` | `/auth/approle/role/:role_name/secret-id/lookup` |

Параметры:

- `role_name` `(string: <required>)` - Имя AppRole. Должно быть короче 4096 байт.
- `secret_id` `(строка: <обязательно>)` - SecretID, привязанный к роли.

Пример данных: 

```json
{
  "secret_id": "84896a0c-1347-aa80-a3f6-aca8b7558780"
}
```

Пример запроса:

```shell
$ curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request POST \
    --data @payload.json \
    ${VAULT_ADDR}/v1/auth/approle/role/application1/secret-id/lookup
```

Пример ответа API:

```json
{
  "request_id": "74752925-f309-6859-3d2d-0fcded95150e",
  "lease_id": "",
  "renewable": false,
  "lease_duration": 0,
  "data": {
    "cidr_list": [],
    "creation_time": "2023-02-10T18:17:27.089757383Z",
    "expiration_time": "0001-01-01T00:00:00Z",
    "last_updated_time": "2023-02-10T18:17:27.089757383Z",
    "metadata": {
      "tag1": "production"
    },
    "secret_id_accessor": "2be760a4-87bb-2fa9-1637-1b7fa9ba2896",
    "secret_id_num_uses": 0,
    "secret_id_ttl": 0,
    "token_bound_cidrs": []
  },
  "wrap_info": null,
  "warnings": null,
  "auth": null
}
```

#### Уничтожение SecretID AppRole

Уничтожает SecretID AppRole.

| Метод   | Путь                                              |
| :----- | :------------------------------------------------ |
| `POST` | `/auth/approle/role/:role_name/secret-id/destroy` |

Параметры:

- `role_name` `(string: <required>)` - Имя AppRole. Должно быть короче 4096 байт.
- `secret_id` `(string: <required>)` - SecretID, привязанный к роли.

Пример данных: 

```json
{
  "secret_id": "84896a0c-1347-aa80-a3f6-aca8b7558780"
}
```

Пример запроса:

```shell
$ curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request POST \
    --data @payload.json \
    ${VAULT_ADDR}/v1/auth/approle/role/application1/secret-id/destroy
```

#### Чтение SecretID AppRole

Выводит свойства SecretID AppRole.

| Метод   | Путь                                                      |
| :----- | :-------------------------------------------------------- |
| `POST` | `/auth/approle/role/:role_name/secret-id-accessor/lookup` |

Параметры:

- `role_name` `(string: <required>)` - Имя AppRole. Должно быть короче 4096 байт.
- `secret_id_accessor` `(строка: <обязательно>)` - SecretID доступа, привязанный к роли.

Пример данных: 

```json
{
  "secret_id_accessor": "84896a0c-1347-aa80-a3f6-aca8b7558780"
}
```

Пример запроса:

```shell
$ curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request POST \
    --data @payload.json \
    ${VAULT_ADDR}/v1/auth/approle/role/application1/secret-id-accessor/lookup
```

Пример ответа API:

```json
{
  "request_id": "72836cd1-139c-fe66-1402-8bb5ca4044b8",
  "lease_id": "",
  "renewable": false,
  "lease_duration": 0,
  "data": {
    "cidr_list": [],
    "creation_time": "2023-02-10T18:17:27.089757383Z",
    "expiration_time": "0001-01-01T00:00:00Z",
    "last_updated_time": "2023-02-10T18:17:27.089757383Z",
    "metadata": {
      "tag1": "production"
    },
    "secret_id_accessor": "2be760a4-87bb-2fa9-1637-1b7fa9ba2896",
    "secret_id_num_uses": 0,
    "secret_id_ttl": 0,
    "token_bound_cidrs": []
  },
  "wrap_info": null,
  "warnings": null,
  "auth": null
}
```

#### Уничтожение SecretID AppRole по идентификатору доступа

Уничтожает SecretID AppRole по его идентификатору доступа.

| Метод   | Путь                                                       |
| :----- | :--------------------------------------------------------- |
| `POST` | `/auth/approle/role/:role_name/secret-id-accessor/destroy` |

Параметры:

- `role_name` `(string: <required>)` - Имя AppRole. Должно быть короче 4096 байт.
- `secret_id_accessor` `(string: <required>)` - Идентификатор доступа SecretID, привязанный к роли.

Пример данных: 

```json
{
  "secret_id_accessor": "84896a0c-1347-aa80-a3f6-aca8b7558780"
}
```

Пример запроса:

```shell
$ curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request POST \
    --data @payload.json \
    ${VAULT_ADDR}/v1/auth/approle/role/application1/secret-id-accessor/destroy
```

#### Создание пользовательского SecretID AppRole

Назначает "пользовательский" SecretID для существующей роли AppRole. Это используется в модели "Push" операции.

| Метод   | Путь                                            |
| :----- | :----------------------------------------------- |
| `POST` | `/auth/approle/role/:role_name/custom-secret-id` |

Параметры:

- `role_name` `(string: <required>)` - Имя AppRole. Должно быть короче 4096 байт.
- `secret_id` `(строка: <обязательно>)` - SecretID, который будет привязан к роли.
- `metadata` `(строка: "")` - Метаданные, связанные с SecretID. Это должна быть строка в формате JSON, содержащая метаданные в виде пар ключ-значение. Эти метаданные будут установлены на токены, выданные с использованием этого SecretID, и будут записаны в журнал аудита _в открытом виде_.
- `cidr_list` `(массив: [])` - Строка, разделенная запятыми, или список блоков CIDR, ограничивающих использование SecretID из определенного набора IP-адресов. Если на роли установлено значение `secret_id_bound_cidrs`, то список блоков CIDR, указанный здесь, должен быть подмножеством блоков CIDR, указанных на роли.
- `token_bound_cidrs` `(массив: [])` - Строка, разделенная запятыми, или список блоков CIDR; если установлено, указывает блоки IP-адресов, которые могут использовать аутентификационные токены, созданные с помощью этого SecretID. Переопределяет значение, установленное на роли, но должно быть подмножеством.
- `num_uses` `(целое число: 0)` - Количество раз, которое этот SecretID может быть использован, после чего SecretID истекает. Значение ноль позволит неограниченное использование. Переопределяет параметр secret_id_num_uses роли, когда указан.
  Не может быть больше, чем secret_id_num_uses роли.
- `ttl` `(строка: "")` - Продолжительность в секундах (`3600`) или целое число временных единиц (`60m`), после которой этот SecretID истекает. Значение ноль позволит SecretID не истекать. Переопределяет параметр secret_id_ttl роли, когда указан.
  Не может быть дольше, чем secret_id_ttl роли.

Пример данных: 

```json
{
  "secret_id": "testsecretid",
  "ttl": 600,
  "num_uses": 50
}
```

Пример запроса:

```shell
$ curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request POST \
    --data @payload.json \
    ${VAULT_ADDR}/v1/auth/approle/role/application1/custom-secret-id
```

Пример ответа API:

```json
{
  "auth": null,
  "warnings": null,
  "wrap_info": null,
  "data": {
    "secret_id": "testsecretid",
    "secret_id_accessor": "84896a0c-1347-aa80-a3f6-aca8b7558780",
    "secret_id_ttl": 600,
    "secret_id_num_uses": 50
  },
  "lease_duration": 0,
  "renewable": false,
  "lease_id": ""
}
```

#### Вход с использованием AppRole

Выдает токен Stronghold на основе представленных учетных данных. `role_id` всегда требуется; если `bind_secret_id` включен (по умолчанию) в AppRole, также требуется `secret_id`. Также оцениваются любые другие привязанные значения аутентификации в AppRole (например, CIDR клиентского IP).

| Метод   | Путь                  |
| :----- | :-------------------- |
| `POST` | `/auth/approle/login` |

Параметры:

- `role_id` `(string: <required>)` - RoleID AppRole.
- `secret_id` `(string: <required>)` - SecretID, принадлежащий AppRole.

Пример данных: 

```json
{
  "role_id": "59d6d1ca-47bb-4e7e-a40b-8be3bc5a0ba8",
  "secret_id": "84896a0c-1347-aa80-a3f6-aca8b7558780"
}
```

Пример запроса:

```shell
$ curl \
    --request POST \
    --data @payload.json \
    ${VAULT_ADDR}/v1/auth/approle/login
```

Пример ответа API:

```json
{
  "auth": {
    "renewable": true,
    "lease_duration": 1200,
    "metadata": null,
    "token_policies": ["default"],
    "accessor": "fd6c9a00-d2dc-3b11-0be5-af7ae0e1d374",
    "client_token": "5b1a0318-679c-9c45-e5c6-d1b9a9035d49"
  },
  "warnings": null,
  "wrap_info": null,
  "data": null,
  "lease_duration": 0,
  "renewable": false,
  "lease_id": ""
}
```

#### Чтение, обновление или удаление свойств AppRole

Обновляет соответствующее свойство в существующем AppRole. Все эти параметры AppRole могут быть обновлены с использованием прямого доступа к пути `/auth/approle/role/:role_name`. Пути для каждого поля предоставляются отдельно, чтобы иметь возможность делегировать конкретные пути с использованием системы ACL Stronghold.

| Method            | Path                                                  |
| :---------------- | :---------------------------------------------------- | --------- |
| `GET/POST/DELETE` | `/auth/approle/role/:role_name/policies`              | `200/204` |
| `GET/POST/DELETE` | `/auth/approle/role/:role_name/secret-id-num-uses`    | `200/204` |
| `GET/POST/DELETE` | `/auth/approle/role/:role_name/secret-id-ttl`         | `200/204` |
| `GET/POST/DELETE` | `/auth/approle/role/:role_name/token-ttl`             | `200/204` |
| `GET/POST/DELETE` | `/auth/approle/role/:role_name/token-max-ttl`         | `200/204` |
| `GET/POST/DELETE` | `/auth/approle/role/:role_name/bind-secret-id`        | `200/204` |
| `GET/POST/DELETE` | `/auth/approle/role/:role_name/secret-id-bound-cidrs` | `200/204` |
| `GET/POST/DELETE` | `/auth/approle/role/:role_name/token-bound-cidrs`     | `200/204` |
| `GET/POST/DELETE` | `/auth/approle/role/:role_name/period`                | `200/204` |

Ссылка на путь `/auth/approle/role/:role_name`.

#### Очистка токенов

Выполняет некоторые задачи по обслуживанию для очистки недействительных записей, которые могут оставаться в хранилище токенов. Обычно запуск этой операции не требуется, если заметки об обновлении или служба поддержки не указывают на это. Это может привести к большому количеству операций ввода-вывода с хранилищем, поэтому следует использовать с осторожностью.

| Метод   | Путь                           |
| :----- | :----------------------------- |
| `POST` | `/auth/approle/tidy/secret-id` |

Пример запроса:

```shell
$ curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request POST \
    ${VAULT_ADDR}/v1/auth/approle/tidy/secret-id
```

Пример ответа API:

```json
{
  "request_id": "b20b56e3-4699-5b19-cc6b-e74f7b787bbf",
  "lease_id": "",
  "renewable": false,
  "lease_duration": 0,
  "data": null,
  "wrap_info": null,
  "warnings": [
    "Tidy operation successfully started. Any information from the operation will be printed to Stronghold's server logs."
  ],
  "auth": null
}
```
