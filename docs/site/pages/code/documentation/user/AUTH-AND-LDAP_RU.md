---
title: "OmniAuth и LDAP"
menuTitle: Настройка OmniAuth и LDAP
force_searchable: true
description: Руководство по настройке OmniAuth и LDAP
permalink: ru/code/documentation/user/oauth-and-ldap.html
lang: ru
weight: 45
---


## Конфигурация OmniAuth

Deckhouse Code поддерживает настройку OmniAuth согласно [официальной документации GitLab](https://docs.gitlab.com/integration/omniauth/). При этом реализованы дополнительные возможности, описанные ниже.

### OpenID Connect (OIDC)

Для интеграции с провайдерами OIDC доступны следующие параметры:

- `allowed_groups`— список групп, пользователям которых разрешён вход. Пользователи вне этих групп будут заблокированы.  
  По умолчанию — `null` (разрешены все группы).

- `admin_groups`— список групп, пользователи которых получают административные права.  
  По умолчанию — `null` (права администратора не выдаются ни одной группе).

- `groups_attribute`— имя атрибута, из которого извлекаются группы пользователя.
  По умолчанию — `'groups'`.

### Пример конфигурации OIDC

Настройка выполняется в секции `spec.appConfig.omniauth.`:

```yaml
providers:
  - name: 'openid_connect'
    allowed_groups:
      - 'gitlab'
    admin_groups:
      - 'admin'
    groups_attribute: 'gitlab_group'
```

## SAML

Для провайдеров SAML доступны аналогичные параметры:

- `allowed_groups` — список групп с разрешённым входом.  
  По умолчанию — `null` (разрешены все группы).

- `admin_groups` — группы с административными правами.  
  По умолчанию — `null` (права администратора не выдаются ни одной группе).

- `groups_attribute` — имя атрибута, содержащего группы.
  По умолчанию — `'Groups'`.

### Пример конфигурации SAML

Настройка выполняется в секции `spec.appConfig.omniauth.`:

```yaml
providers:
  - name: 'saml'
    allowed_groups:
      - 'gitlab'
    admin_groups:
      - 'admin'
    groups_attribute: 'gitlab_group'
```

> Если пользователь входит в `admin_groups`, но не указан в `allowed_groups`, доступ будет запрещён. В этом случае административные права также не будут назначены.

## LDAP Synchronization

Deckhouse Code поддерживает синхронизацию пользователей, групп и прав доступа с LDAP-сервером. Синхронизация выполняется автоматически раз в час, либо с заданной периодичностью.

Вы можете настроить периодичность синхронизации через параметр `cronJobs` в секции `spec.appConfig.`:

```yaml
cron_jobs:
  ldap_sync_worker:
    cron: "0 * * * *"
```

### Ограничения на стороне LDAP-сервера

Во время синхронизации выполняются LDAP-запросы ко всем пользователям и группам, указанным в конфигурации. При необходимости используется постраничная загрузка (pagination).
Если на стороне LDAP установлены ограничения на число возвращаемых объектов, это может привести к ошибкам синхронизации или удалению прав доступа у пользователей.

### Пример конфигурации LDAP-провайдера

Конфигурация размещается в `spec.appConfig.ldap.`:

```yaml
main:
  label: ldap
  host: 127.0.0.1
  port: 3389
  bind_dn: 'uid=viewer,ou=People,dc=example,dc=com'
  base: 'ou=People,dc=example,dc=com'
  uid: 'cn'
  password: 'viewer123'
  sync_name: true
  group_sync: {
    create_groups: true,
    base: 'ou=Groups,dc=example,dc=org',
    filter: '(objectClass=groupOfNames)',
    prefix: {
      attribute: 'businessCategory',
      default: 'default-program',
    },
    top_level_group: "LdapGroups",
    name_mask: "(?<=-)[A-z0-9А-я]*$",
    owner: "root",
    role_mapping: [
      { by_name: '.*-project_manager-.*', gitlab_role: 'maintainer' },
      { by_name: '.*-developer-.*', gitlab_role: 'developer' },
      { by_name: '.*-participant-.*', gitlab_role: 'reporter' }
    ]
  }
```

### Группы и права доступа

LDAP-группы сопоставляются с группами GitLab. При этом можно назначать роли пользователям на основе имени группы.

Обязательные параметры:

- `group_sync.base` — DN, с которого начинается поиск LDAP-групп.

Опциональные параметры:

- `group_sync.create_groups` —  если `true`, группы будут создаваться в Deckhouse Code.
- `group_sync.filter` — LDAP фильтр для поиска групп.
- `group_sync.scope` — область поиска групп (0 — Base, 1 — SingleLevel, 2 — WholeSubtree).
- `group_sync.prefix` — определяет, из какого атрибута брать имя родительской группы. Если атрибут отсутствует — используется значение по умолчанию.
- `group_sync.top_level_group` — имя группы верхнего уровня, в которую будут добавлены все синхронизированные группы.
- `group_sync.name_mask` —  регулярное выражение для извлечения имени группы из атрибута CN.
- `group_sync.owner` — имя пользователя, который будет добавлен как владелец группы (по умолчанию — `root`).

### Секция `role_mapping`

Назначает права пользователям на основе имени группы (`cn`):

- `role_mapping.by_name` — регулярное выражение; если имя группы совпадает, пользователю назначается соответствующая роль.
- `role_mapping.gitlab_role` — название роли в Deckhouse Code (например: `guest`, `reporter`, `developer`, `maintainer`, `owner`).

### Определение членов группы

Deckhouse Code поддерживает следующие атрибуты для определения членов группы (все значения — массив DN):

- `member`;
- `uniquemember`;
- `memberof`;
- `memberuid`;
- `submember`.

### Синхронизация пользователей

Во время синхронизации обновляются имена и email-адреса пользователей, а также статус блокировки.

Опциональные параметры:

`sync_name` — если `true`, имя пользователя будет обновлено по данным LDAP.

#### Устранение проблем с синхронизацией

Если предыдущее задание синхронизации завершилось некорректно, Redis может сохранить блокировку на его выполнение (по умолчанию параметр `concurrency = 1`). Это помешает запуску нового задания.

Чтобы снять блокировку:

1. Подключитесь к Redis, используя базы, указанные в `config/redis.shared_state.yml` and `config/redis.queues.yml`.
1. Удалите ключ `sidekiq:concurrency_limit:throttled_jobs:{ldap/sync_worker}` следующими командами:

   ```console
   keys *ldap*
   del "sidekiq:concurrency_limit:throttled_jobs:{ldap/sync_worker}"
   ```
