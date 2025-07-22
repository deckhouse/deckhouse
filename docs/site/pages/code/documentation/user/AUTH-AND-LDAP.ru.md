---
title: "OmniAuth & LDAP"
menuTitle: Настройка OmniAuth и LDAP
force_searchable: true
description: руководство по настройке OmniAuth и LDAP
permalink: ru/code/documentation/user/oauth-and-ldap.html
lang: ru
weight: 45
---


## Конфигурация OmniAuth

Настройка в основном основывается на [официальной документации](https://docs.gitlab.com/integration/omniauth/). Однако Deckhouse Code добавляет некоторые расширения, описанные в разделах ниже.

### OpenID Connect (OIDC)

Для интеграции с OIDC были добавлены следующие параметры:

- **`allowed_groups`**: Массив групп, которым разрешён вход. Пользователи, не входящие в эти группы, будут заблокированы и не смогут войти.  
  **По умолчанию:** `null` (разрешены все группы).

- **`admin_groups`**: Массив групп, которым разрешён вход с правами администратора. Пользователи из этих групп получат роль администратора.  
  **По умолчанию:** `null` (админские права не выдаются ни одной группе).

- **`groups_attribute`**: Имя атрибута, используемого для получения групп пользователя.  
  **По умолчанию:** `'groups'`.

### Пример конфигурации OIDC

Секции находится в разделе `spec.appConfig.omniauth.`

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

Для интеграции с SAML были добавлены следующие параметры:

- **`allowed_groups`**: Массив групп, которым разрешён вход. Пользователи, не входящие в эти группы, будут заблокированы и не смогут войти.  
  **По умолчанию:** `null` (разрешены все группы).

- **`admin_groups`**: Массив групп, которым разрешён вход с правами администратора. Пользователи из этих групп получат роль администратора.  
  **По умолчанию:** `null` (админские права не выдаются ни одной группе).

- **`groups_attribute`**: Имя атрибута, используемого для получения групп пользователя.  
  **По умолчанию:** `'Groups'`.

### Пример конфигурации SAML

Секци находится в разделе `spec.appConfig.omniauth.`

```yaml
providers:
  - name: 'saml'
    allowed_groups:
      - 'gitlab'
    admin_groups:
      - 'admin'
    groups_attribute: 'gitlab_group'
```

> **Примечание**: для OIDC и SAML — если пользователь принадлежит к admin_groups, но не указан в allowed_groups, он не сможет войти. В этом случае admin_groups игнорируется и административные права не назначаются.

## LDAP Synchronization

Выполняет синхронизацию пользователей, групп и прав доступа с LDAP-сервером. По умолчанию запускается раз в час.

Вы можете настроить периодичность синхронизации через параметр `cronJobs` в секции `spec.appConfig.`:

```yaml
cron_jobs:
  ldap_sync_worker:
    cron: "0 * * * *"
```

### Ограничения на стороне LDAP-сервера

Во время синхронизации выполняются запросы ко всем пользователям и группам, указанным в конфигурации.
Задача синхронизации автоматически использует пагинацию при необходимости.
Однако если на стороне LDAP-сервера установлено ограничение на максимальное количество возвращаемых записей, это может привести к неожиданной блокировке или удалению доступа у пользователей.

### Пример конфигурации LDAP-провайдера

Раздел конфигурации находится в `spec.appConfig.ldap.`

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

### Синхронизация групп и прав доступа

Deckhouse Code переиспользует базовую ролевую модель Gitlab.

Создаёт группы GitLab и назначает роли пользователям на основе записей, полученных с LDAP-сервера.
Может быть настроена с помощью следующих параметров:

Обязательные параметры:

- **group_sync.base** — базовый DN, с которого начинается поиск..

Опциональные параметры:

- **group_sync.create_groups** —  если `true`, группы будут создаваться в Deckhouse Code.
- **group_sync.filter** — LDAP фильтр для поиска групп.
- **group_sync.scope** — Область поиска групп (0 — Base, 1 — SingleLevel, 2 — WholeSubtree).
- **group_sync.prefix** — Определяет, из какого атрибута брать имя родительской группы. Если атрибут отсутствует — используется значение по умолчанию.
- **group_sync.top_level_group** — Имя верхнеуровневой группы, в которую будут добавлены все синхронизированные группы.
- **group_sync.name_mask** —  Регулярное выражение для извлечения имени группы из атрибута CN.
- **group_sync.owner** — имя пользователя, который будет добавлен как владелец группы (по умолчанию — `root`).

### Секция `role_mapping`

Назначает права пользователям на основе имени группы (`cn`):

- **role_mapping.by_name** — регулярное выражение; если имя группы совпадает, пользователю назначается соответствующая роль.
- **role_mapping.gitlab_role** — название роли в Deckhouse Code (например: `guest`, `reporter`, `developer`, `maintainer`, `owner`).

### Как определяется членство в группе

Участники групп определяются на основе следующих атрибутов группы. Значение каждого атрибута должно быть массивом DN пользователей:

- `member`
- `uniquemember`
- `memberof`
- `memberuid`
- `submember`

### Синхронизация пользователей

Блокирует и разблокирует пользователей, а также обновляет их имя и email на основе данных с LDAP-сервера.

Опциональные параметры:

`sync_name` - если `true`, имя пользователя будет обновлено по данным LDAP.

#### Устранение проблем с синхронизацией

Если предыдущее задание синхронизации не завершилось корректно, Redis может сохранить запись о том, что оно ещё выполняется.
Это предотвратит запуск нового задания, поскольку параметр concurrency установлен в 1.

Чтобы исправить ситуацию, выполните следующие шаги:

1. Подключитесь к Redis, используя базы, указанные в `config/redis.shared_state.yml` and `config/redis.queues.yml`.
2. Удалите следующий ключ:  `sidekiq:concurrency_limit:throttled_jobs:{ldap/sync_worker}` с помощью команд:
- `keys *ldap*` - убеждается в наличии соответствующего ключа
- `del "sidekiq:concurrency_limit:throttled_jobs:{ldap/sync_worker}"` - удаляет ключ из Redis
