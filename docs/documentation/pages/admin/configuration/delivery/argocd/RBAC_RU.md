---
title: "Настройка ролевой модели доступа"
permalink: ru/admin/configuration/delivery/argocd/rbac/
description: "Настройка ролевой модели доступа Argo CD в Deckhouse Kubernetes Platform."
lang: ru
relatedLinks:
  - title: "Официальный сайт проекта Argo CD"
    url: "https://argo-cd.readthedocs.io"
  - title: "Официальный сайт проекта Argo CD Operator"
    url: "https://argocd-operator.readthedocs.io"
---

Argo CD использует собственную ролевую модель (Role-based Access Control, RBAC), не основанную на ролевой модели Kubernetes и Deckhouse Kubernetes Platform. Ролевая модель Argo CD позволяет ограничивать доступ к ресурсам и операциям через собственные политики и роли.

Перед настройкой ролевой модели доступа выполните [настройку аутентификации и авторизации](../authentication/). После этого назначайте роли пользователям и группам, а также задавайте разрешения на уровне всего экземпляра Argo CD или отдельных проектов с помощью объекта [AppProject](/modules/operator-argo/cr.html#appproject).

Правила ролевой модели доступа (Role-Based Access Control, RBAC) можно определить в двух местах:

- глобально — в объекте [ArgoCD](/modules/operator-argo/cr.html#argocd);
- на уровне проекта — в ролях объекта [AppProject](/modules/operator-argo/cr.html#appproject).

## Встроенные роли

В Argo CD есть две предопределённые роли с набором политик:

- `role:readonly` — доступ только на чтение;
- `role:admin` — доступ с полномочиями администратора.

{% offtopic title="Полное описание политик предопределенных ролей.." %}

```text
# Политики роли readonly.
p, role:readonly, applications, get, */*, allow
p, role:readonly, applicationsets, get, */*, allow
p, role:readonly, certificates, get, *, allow
p, role:readonly, clusters, get, *, allow
p, role:readonly, repositories, get, *, allow
p, role:readonly, write-repositories, get, *, allow
p, role:readonly, projects, get, *, allow
p, role:readonly, accounts, get, *, allow
p, role:readonly, gpgkeys, get, *, allow
p, role:readonly, logs, get, */*, allow

# Политики роли admin.
p, role:admin, applications, create, */*, allow
p, role:admin, applications, update, */*, allow
p, role:admin, applications, update/*, */*, allow
p, role:admin, applications, delete, */*, allow
p, role:admin, applications, delete/*, */*, allow
p, role:admin, applications, sync, */*, allow
p, role:admin, applications, override, */*, allow
p, role:admin, applications, action/*, */*, allow
p, role:admin, applicationsets, get, */*, allow
p, role:admin, applicationsets, create, */*, allow
p, role:admin, applicationsets, update, */*, allow
p, role:admin, applicationsets, delete, */*, allow
p, role:admin, certificates, create, *, allow
p, role:admin, certificates, update, *, allow
p, role:admin, certificates, delete, *, allow
p, role:admin, clusters, create, *, allow
p, role:admin, clusters, update, *, allow
p, role:admin, clusters, delete, *, allow
p, role:admin, repositories, create, *, allow
p, role:admin, repositories, update, *, allow
p, role:admin, repositories, delete, *, allow
p, role:admin, write-repositories, create, *, allow
p, role:admin, write-repositories, update, *, allow
p, role:admin, write-repositories, delete, *, allow
p, role:admin, projects, create, *, allow
p, role:admin, projects, update, *, allow
p, role:admin, projects, delete, *, allow
p, role:admin, accounts, update, *, allow
p, role:admin, gpgkeys, create, *, allow
p, role:admin, gpgkeys, delete, *, allow
p, role:admin, exec, create, */*, allow

# Роль admin включает все полномочия роли readonly.
g, role:admin, role:readonly

# Локальному пользователю admin назначена роль admin.
g, admin, role:admin
```

{% endofftopic %}

## Политика по умолчанию для аутентифицированных пользователей

После успешной аутентификации пользователь получает роль, указанную в параметре [`spec.rbac.defaultPolicy`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-defaultpolicy) ArgoCD.

{% alert level="warning" %}
Все аутентифицированные пользователи получают как минимум те разрешения, которые заданы в параметре [`spec.rbac.defaultPolicy`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-defaultpolicy). Эти права нельзя отозвать правилом с эффектом `deny`.
{% endalert %}

Рекомендуется создать отдельную роль, например `role:authenticated`, выдать ей минимальный набор разрешений и использовать её в [`spec.rbac.defaultPolicy`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-defaultpolicy).

Пример:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  rbac:
    defaultPolicy: role:authenticated
    policy: |
      p, role:authenticated, applications, get, */*, allow
...
```

## Анонимный доступ

Если включить анонимный доступ к экземпляру Argo CD, пользователи смогут получить права согласно политике, указанной в параметре [`spec.rbac.defaultPolicy`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-defaultpolicy), без аутентификации.

Анонимный доступ включается параметром [`spec.usersAnonymousEnabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-usersanonymousenabled) ArgoCD.

{% alert level="warning" %}
При включении анонимного доступа создайте отдельную роль по умолчанию, например `role:unauthenticated`, и назначьте её в [`spec.rbac.defaultPolicy`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-defaultpolicy).
{% endalert %}

## Структура ролевой модели доступа

Синтаксис ролевой модели доступа в Argo CD основан на модели [Casbin](https://casbin.org/docs/overview), в которой используются два типа записей:

- привязка пользователя или группы к роли;
- назначение разрешений роли, пользователю или группе.

### Привязка к роли

Синтаксис:

```text
g, <user/group>, <role>
```

Где:

- `<user/group>` — локальный пользователь, пользователь SSO или группа;
- `<role>` — внутренняя роль Argo CD.

Пример:

```text
g, my-org:team-beta, role:admin
g, user@example.org, role:readonly
```

### Политика доступа

Синтаксис:

```text
p, <role/user/group>, <resource>, <action>, <object>, <effect>
```

Где:

- `<role/user/group>` — субъект, которому назначается правило;
- `<resource>` — тип ресурса;
- `<action>` — разрешённая операция;
- `<object>` — объект, к которому применяется правило;
- `<effect>` — результат проверки: `allow` или `deny`.

Пример:

```csv
p, role:developer, applications, sync, dev-project/*, allow
p, role:developer, logs, get, dev-project/*, allow
```

{% alert level="info" %}
Чтобы назначать правила группе, сначала привяжите группу к роли с помощью записи `g, <group>, <role>`. После этого назначайте разрешения самой роли.
{% endalert %}

## Ресурсы и поддерживаемые действия

Ниже перечислены основные ресурсы Argo CD и действия, которые можно использовать в политиках RBAC.

| Ресурс | get | create | update | delete | sync | action | override | invoke |
| :-- | :-: | :--: | :--: | :--: | :--: | :--: | :--: | :--: |
| `applications` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ |
| `applicationsets` | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `clusters` | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `projects` | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `repositories` | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `accounts` | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `certificates` | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `gpgkeys` | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `logs` | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `exec` | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `extensions` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |

## Политики, привязанные к приложениям

Некоторые ресурсы привязаны к конкретным приложениям:

- `applications`;
- `applicationsets`;
- `logs`;
- `exec`.

Для таких ресурсов значение `<object>` обычно имеет формат `<APP_PROJECT>/<APP_NAME>`.

Пример:

```csv
p, example-user, applications, get, *, allow
p, example-user, logs, get, example-project/my-app, allow
```

Если в Argo CD включён режим размещения приложений в произвольных неймспейсах, формат `<object>` меняется на `<APP_PROJECT>/<APP_NAMESPACE>/<APP_NAME>`.

Пример:

```csv
p, example-user, applications, get, */app-namespace/*, allow
p, example-user, logs, get, example-project/app-namespace/my-app, allow
```

## Ресурс `applications`

Ресурс `applications` поддерживает как базовые действия, так и более детальные правила.

### Детализированные разрешения для `update` и `delete`

Разрешения `update` и `delete`, выданные на приложение, позволяют изменять или удалять само приложение, но не его вложенные ресурсы.

Чтобы разрешить операцию над ресурсами приложения, используйте формат:

```text
<action>/<group>/<kind>/<namespace>/<name>
```

Например, чтобы разрешить пользователю удалять только Pod в приложении `prod-app`:

```csv
p, example-user, applications, delete/*/Pod/*/*, default/prod-app, allow
```

Чтобы разрешить обновление всех ресурсов приложения, но не самого приложения:

```csv
p, example-user, applications, update/*, default/prod-app, allow
```

Чтобы запретить удаление приложения, но разрешить удаление Pod:

```csv
p, example-user, applications, delete, default/prod-app, deny
p, example-user, applications, delete/*/Pod/*/*, default/prod-app, allow
```

Чтобы разрешить обновление приложения, но запретить обновление любых подресурсов:

```csv
p, example-user, applications, update, default/prod-app, allow
p, example-user, applications, update/*, default/prod-app, deny
```

{% alert level="warning" %}
Argo CD не использует символ `/` как разделитель при сравнении glob-шаблонов. Поэтому рекомендуется всегда указывать полный путь ресурса, то есть использовать шаблоны с четырьмя символами `/`.
{% endalert %}

### Действие `action`

Действие `action` соответствует встроенным или пользовательским действиям над ресурсами приложения.

Формат значения `<action>`:

```text
action/<group>/<kind>/<action-name>
```

Например:

- `action/extensions/DaemonSet/restart` — действие для `DaemonSet`;
- `action//Pod/maintenance-off` — действие для `Pod`, у которого нет API-группы.

Пример политики:

```csv
p, example-user, applications, action//Pod/maintenance-off, default/*, allow
p, example-user, applications, action/extensions/DaemonSet/*, default/*, allow
```

Чтобы разрешить все действия над ресурсами приложения:

```csv
p, example-user, applications, action/*, default/*, allow
```

### Действие `override`

Разрешение `override` позволяет передавать произвольные манифесты или другую ревизию при синхронизации `Application`.

{% alert level="warning" %}
Право `override` позволяет пользователю фактически изменить состав или состояние развёрнутых ресурсов приложения. Выдавайте его только тем пользователям, которым это действительно необходимо.
{% endalert %}

Если в ConfigMap `argocd-cm` включён параметр `application.sync.requireOverridePrivilegeForRevisionSync: 'true'`, передача произвольной ревизии при синхронизации также будет требовать право `override`.

## Другие ресурсы

### Ресурс `applicationsets`

Ресурс `applicationsets` также относится к политикам, привязанным к приложениям. Разрешение `create` для этого ресурса фактически даёт возможность создавать объекты [Application](/modules/operator-argo/cr.html#application) через [ApplicationSet](/modules/operator-argo/cr.html#applicationset).

Пример:

```csv
p, dev-group, applicationsets, *, dev-project/*, allow
```

С таким правилом пользователи из `dev-group` смогут создавать ApplicationSet, который управляет приложениями только в рамках проекта `dev-project`.

### Ресурс `logs`

Разрешение `get` для ресурса `logs` позволяет просматривать логи Pod приложения через веб-интерфейс Argo CD. По смыслу это эквивалентно `d8 k logs`.

### Ресурс `exec`

Разрешение `create` для ресурса `exec` позволяет открывать `exec`-сессию в Pod приложения через веб-интерфейс Argo CD. По смыслу это близко к `d8 k exec`.

### Ресурс `extensions`

Ресурс `extensions` используется для вызова proxy-расширений. Проверка таких правил работает совместно с ресурсом `applications`: пользователь должен иметь право на чтение приложения, из которого выполняется запрос.

Пример:

```csv
p, example-user, applications, get, default/*, allow
p, example-user, extensions, invoke, httpbin, allow
```

### Эффект `deny`

Если политика с эффектом `deny` совпала с запросом, доступ будет запрещён, даже если есть более специфичные правила с эффектом `allow`.

Порядок строк в [`spec.rbac.defaultPolicy`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-defaultpolicy) на результат не влияет.

## Порядок проверки политик и режимы сопоставления

Проверка доступа выполняется в два этапа:

1. Проверяются правила из [`spec.rbac.defaultPolicy`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-defaultpolicy).
1. Если решение не найдено, проверяются правила, относящиеся к пользователю и его группам.

Если доступ явно разрешён или запрещён политикой по умолчанию, дальнейшая проверка не выполняется.

Argo CD поддерживает два режима сопоставления значений, задаваемых в [`spec.rbac.policyMatchMode`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-policymatchmode):

- `glob` — сопоставление по glob-шаблонам;
- `regex` — сопоставление по регулярным выражениям.

### Сопоставление в режиме `glob`

В режиме `glob` токены рассматриваются как цельные строки без специальных разделителей.

Пример политики:

```csv
p, example-user, applications, action/extensions/*, default/*, allow
```

Если пользователь `example-user` вызывает действие `extensions/DaemonSet/test`, будут выполнены такие проверки:

1. `example-user` совпадает с `example-user`.
1. `applications` совпадает с `applications`.
1. `action/extensions/DaemonSet/test` совпадает с `action/extensions/*`.
1. `default/my-app` совпадает с `default/*`.

## Использование пользователей и групп SSO

Параметр `scopes` определяет, какие OIDC scopes Argo CD должен анализировать при проверке RBAC помимо `sub`. Если параметр не задан, по умолчанию используется значение `'[groups]'`.

Пример объекта ArgoCD с настройкой scopes:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  rbac:
    policy: |
      p, my-org:team-alpha, applications, sync, my-project/*, allow
      g, my-org:team-beta, role:admin
      g, user@example.org, role:admin
      g, admin, role:admin
      g, role:admin, role:readonly
    defaultPolicy: role:readonly
    scopes: '[groups, email]'
```

В этом примере:

1. `g, admin, role:admin` явно привязывает встроенного пользователя `admin` к роли `role:admin`.
1. `g, role:admin, role:readonly` задаёт наследование ролей: все, у кого есть `role:admin`, автоматически получают и права `role:readonly`.

Этот подход можно комбинировать с ролями на уровне [AppProject](/modules/operator-argo/cr.html#appproject). Например, можно создать проект `team-beta-project` и назначить права пользователям и группам:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: team-beta-project
  namespace: argocd
spec:
  roles:
    - name: admin
      description: Права администратора для team-beta.
      policies:
        - p, proj:team-beta-project:admin, applications, *, team-beta-project/*, allow
      groups:
        - user@example.org
        - my-org:team-beta
```

## Локальные пользователи

Локальным пользователям можно выдать доступ двумя способами:

- назначить правила напрямую;
- привязать пользователя к роли.

Пример назначения правила напрямую:

```csv
p, my-local-user, applications, sync, my-project/*, allow
```

Пример привязки к роли:

```csv
g, my-local-user, role:admin
```

{% alert level="warning" %}
Если одновременно используются SSO и локальные пользователи, возможна неоднозначность. Например, если локальный пользователь `sally` привязан к `role:admin`, а значение одного из SSO scopes тоже равно `sally`, такой пользователь SSO также может получить права `role:admin`.
{% endalert %}

Чтобы избежать этой ситуации, при одновременном использовании SSO и локальных пользователей рекомендуется назначать локальным пользователям правила напрямую, а не через роли.

Пример:

```csv
p, my-local-user, *, *, *, allow
```

## Тестирование политик RBAC

Чтобы проверить корректность правил RBAC, используйте CLI-утилиту `argocd`. Для проверки того, может ли конкретный пользователь, группа или роль выполнить определённое действие, используйте команду:

```bash
argocd admin settings rbac can
```

Например, выполните команду ниже, чтобы проверить, есть ли у пользователя `admin@deckhouse.io` права на создание `applications` в проекте `default`:

```bash
argocd admin settings rbac can admin@deckhouse.io create application 'default/*' --namespace argocd
```
