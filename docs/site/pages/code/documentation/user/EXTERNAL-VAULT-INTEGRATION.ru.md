---
title: "Интеграция с внешним Vault"
menuTitle: Интеграция с внешним Vault
force_searchable: true
description: Подключение секретов из внешнего vault в CI
permalink: ru/code/documentation/user/external-vault.html
lang: ru
weight: 50
---

# Интеграция с внешним Vault
Эта функция позволяет настроить интеграцию с Vault-сервером и использовать секреты в CI-пайплайнах.
Для начала работы необходимо сконфигурировать Vault-сервер и подготовить соответствующие роли и политики.

## Настройка VAULT
1) Включение JWT-аутентификации
```bash
vault auth enable jwt

vault write auth/jwt/config \
  oidc_discovery_url="https://code.example.com" \
  bound_issuer="https://code.example.com" \
  default_role="gitlab-role"
```

2) Создание роли
```bash
vault write auth/jwt/role/gitlab-role - <<EOF
{
  "role_type": "jwt",
  "user_claim": "sub",
  "bound_audiences": ["vault"],
  "bound_claims": {
    "project_id": "23"
  },
  "policies": ["gitlab-policy"],
  "ttl": "1h"
}
EOF

```

> ⚠️ Важно: всегда используйте bound_claims, чтобы ограничить доступ к роли. В противном случае любой JWT, выданный инстансом, сможет получить доступ с помощью этой роли. 

3) Настройка политики
```bash
vault policy write gitlab-policy - <<EOF
path "kv/data/code/vault-demo" {
  capabilities = ["read"]
}
EOF
```

## Конфигурация CI

### Переменные окружения
Задайте следующие переменные окружения в CI/CD:

- `VAULT_SERVER_URL` - Обязательно. URL адрес vault серва https://vault.example.com.
- `VAULT_AUTH_ROLE` - Опционально. Роль на vault сервере. Если не указано будет использоваться роль по умолчанию сконфигурированная для используемого метода аутентификации
- `VAULT_AUTH_PATH` - Опционально. путь до метода аутентификации. Значение по умолчанию - jwt
- `VAULT_NAMESPACE` - Опционально. Vault namespace.
### Использование секретов в CI
```yaml
stages:
  - test
vault-login:
  stage: test
  image: ruby:3.2
  id_tokens:
    VAULT_ID_TOKEN:
      aud: vault
  secrets:
    DATABASE_PASSWORD:
      vault: code/vault-demo/DATABASE_PASSWORD@kv
      token: $VAULT_ID_TOKEN
  script: echo $DATABASE_PASSWORD
```

### описание параметров
```yaml
DATABASE_PASSWORD:
  vault: code/vault-demo/DATABASE_PASSWORD@kv
  token: $VAULT_ID_TOKEN
  file: false
```
#### `vault` (Обязательно)
строка вида `code/vault-demo/DATABASE_PASSWORD@kv` где 
- `code/vault-demo/` путь до секрета 
- `DATABASE_PASSWORD` название поля 
- `kv` - точка монтирования secret engine по умолчанию 'secret'

По умолчанию используется engine kv-v2.Если необходимо использовать другой engine, можно указать объект вместо строки:.

```yaml
DATABASE_PASSWORD:
  vault: 
    path: code/vault-demo
    field: DATABASE_PASSWORD
    engine:
      name: 'kv-v1'
      path: 'kv1'
  token: $VAULT_ID_TOKEN
  file: false
```
#### `token` (обязательно)
Обязательный параметр.
JWT токен из секции `id_tokens` который будет использоваться для 
аутентификации в vault.
#### `file` (опционально)
по умолчанию true. 
Определяет будет ли секрет сохранен в виде файла или строки.


## Поля включенные в JWT

Следующие поля включены в JWT токен:

| Поле                    | Когда       | Описание                                          |
|-------------------------|-------------|---------------------------------------------------|
| `jti`                   | всегда          | Уникальный идентификатор токена                   |
| `iss`                   | всегда          | Издатель (URL Code)                               |
| `iat`                   | всегда          | Время выпуска                                     |
| `nbf`                   | всегда          | Не валиден до                                     |
| `exp`                   | всегда          | Время истечения срока действия                    |
| `sub`                   | всегда          | Subject (обычно job ID)                           |
| `namespace_id`          | всегда          | ID группы или пользовательского пространства      |
| `namespace_path`        | всегда          | Путь группы или пользовательского пространства    |
| `project_id`            | всегда          | ID проекта                                        |
| `project_path`          | всегда          | Путь проекта                                      |
| `user_id`               | всегда          | ID пользователя                                   |
| `user_login`            | всегда          | Логин пользователя                                |
| `user_email`            | всегда          | Email пользователя                                |
| `pipeline_id`           | всегда          | ID pipeline                                       |
| `pipeline_source`       | всегда          | Источник pipeline                                 |
| `job_id`                | всегда          | ID CI job                                         |
| `ref`                   | всегда          | Git-реф                                           |
| `ref_type`              | всегда          | Тип рефа (`branch` или `tag`)                     |
| `ref_path`              | всегда          | Полный путь до рефа (например, `refs/heads/main`) |
| `ref_protected`         | всегда          | Признак защищённого рефа                          |
| `environment`           | при наличии | Название окружения                                |
| `groups_direct`         | <200 групп  | Пути до групп, где пользователь состоит напрямую  |
| `environment_protected` | при наличии | Защищено ли окружение                             |
| `deployment_tier`       | при наличии | Тип окружения (production, staging и т.д.)        |
| `environment_action`    | при наличии | Указанное действие над окружением                 |

Полезные ссылки:
- https://docs.gitlab.com/ci/secrets/hashicorp_vault/
