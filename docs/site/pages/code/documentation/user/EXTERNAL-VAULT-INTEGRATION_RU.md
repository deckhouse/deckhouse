---
title: "Интеграция с внешним Vault"
menuTitle: Интеграция с внешним Vault
force_searchable: true
description: Подключение секретов из внешнего vault в CI
permalink: ru/code/documentation/user/external-vault.html
lang: ru
weight: 50
---

Эта функция позволяет настроить интеграцию с Vault-сервером и использовать секреты в CI-пайплайнах. Перед началом работы необходимо настроить Vault-сервер и подготовить соответствующие роли и политики.

## Настройка Vault

1. Включите аутентификацию через JWTT:

   ```bash
   vault auth enable jwt

   vault write auth/jwt/config \
     oidc_discovery_url="https://code.example.com" \
     bound_issuer="https://code.example.com" \
     default_role="gitlab-role"
   ```

1. Создайте роль:

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

   > Всегда используйте `bound_claims`, чтобы ограничить доступ к роли. Без этого любой JWT, выданный платформой, сможет получить доступ с этой ролью.
  
1. Настройте политики:

   ```bash
   vault policy write gitlab-policy - <<EOF
   path "kv/data/code/vault-demo" {
     capabilities = ["read"]
   }
   EOF
   ```

## Конфигурация CI

### Переменные окружения

Для корректной работы с Vault в пайплайне CI/CD необходимо задать следующие переменные окружения:

- `VAULT_SERVER_URL` — **обязательно**. URL-адрес Vault-сервера (например, `https://vault.example.com`).
- `VAULT_AUTH_ROLE` — *опционально*. Название роли в Vault. Если не указано, будет использована роль по умолчанию, заданная в конфигурации метода аутентификации.
- `VAULT_AUTH_PATH` — *опционально*. Путь до метода аутентификации в Vault. По умолчанию используется `jwt`.
- `VAULT_NAMESPACE` — *опционально*. Namespace Vault, если используется многоуровневая иерархия.

### Использование секретов в CI

Для получения секретов из Vault можно использовать следующий шаблон job'а:

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

### Параметры секрета

Пример:

```yaml
DATABASE_PASSWORD:
  vault: code/vault-demo/DATABASE_PASSWORD@kv
  token: $VAULT_ID_TOKEN
  file: false
```

Описание параметров:

1. `vault` (обязательно) — путь к секрету в формате строки `path/to/secret/KEY@ENGINE`, где:
   - `code/vault-demo/` — путь до секрета в Vault;
   - `DATABASE_PASSWORD` — имя поля внутри секрета;
   - `kv` — точка монтирования Secret Engine (по умолчанию — `secret`).

По умолчанию используется `engine kv-v2`. Если необходимо использовать другой engine, можно указать объект вместо строки:

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

1. `token` (обязательно) — JWT-токен из секции `id_tokens`, используемый для аутентификации в Vault.

1. `file` (опционально, по умолчанию `true`) — определяет способ предоставления секрета:
   - `true` — секрет сохраняется во временный файл;
   - `false` — секрет передаётся как строка в переменную окружения.

### Поля, включённые в JWT

Следующие поля автоматически включаются в JWT-токен и могут использоваться Vault для проверки прав доступа:

| Поле                    | Условие появления           | Описание                                                                |
|-------------------------|-----------------------------|-------------------------------------------------------------------------|
| `jti`                   | всегда                      | Уникальный идентификатор токена                                         |
| `iss`                   | всегда                      | Издатель токена (обычно URL Deckhouse Code)                            |
| `iat`                   | всегда                      | Время выпуска токена (Issued At)                                        |
| `nbf`                   | всегда                      | Время, до которого токен считается недействительным                     |
| `exp`                   | всегда                      | Время истечения срока действия токена                                   |
| `sub`                   | всегда                      | Subject токена (обычно ID задания CI)                                   |
| `namespace_id`          | всегда                      | ID пространства (группы или пользователя)                               |
| `namespace_path`        | всегда                      | Путь до пространства (например, `groups/dev`)                           |
| `project_id`            | всегда                      | ID проекта                                                               |
| `project_path`          | всегда                      | Путь до проекта                                                          |
| `user_id`               | всегда                      | ID пользователя                                                          |
| `user_login`            | всегда                      | Логин пользователя                                                       |
| `user_email`            | всегда                      | Email пользователя                                                       |
| `pipeline_id`           | всегда                      | ID CI-пайплайна                                                          |
| `pipeline_source`       | всегда                      | Источник запуска пайплайна (push, schedule, MR и т.д.)                  |
| `job_id`                | всегда                      | ID задания CI                                                            |
| `ref`                   | всегда                      | Ссылка Git (Git reference)                                       |
| `ref_type`              | всегда                      | Тип ссылки Git (Git reference) (`branch` или `tag`)                                           |
| `ref_path`              | всегда                      | Полный путь ссылки Git (Git reference) (например, `refs/heads/main`)                       |
| `ref_protected`         | всегда                      | Признак того, что объект по ссылке Git защищён |
| `environment`           | при наличии                 | Название окружения (если используется)                                  |
| `groups_direct`         | при наличии (<200 групп)    | Пути до групп, в которых состоит пользователь                           |
| `environment_protected` | при наличии                 | Является ли окружение защищённым                                        |
| `deployment_tier`       | при наличии                 | Тип окружения (`production`, `staging` и т.п.)                          |
| `environment_action`    | при наличии                 | Тип действия над окружением (например, `deploy`)                        |
