---
title: "Модуль observability: FAQ"
description: "FAQ для модуля Observability"
menuTitle: "FAQ"
---

## Конвертация существующих дашбордов из GrafanaDashboardDefinition

Для перехода со старого формата дашбордов (`GrafanaDashboardDefinition`) на новый (`ObservabilityDashboard`, `ClusterObservabilityDashboard`), необходимо вручную адаптировать манифесты. Обратите внимание на следующие отличия:

| Старый формат                                    | Новый формат                                                                                                                                       |     |
| ------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------- | --- |
| `spec.folder`                                    | Поле отсутствует. Папка задаётся с помощью аннотации: `observability.deckhouse.io/category`                                                        |     |
| Название дашборда берется из поля Title дашборда | Название задаётся с помощью аннотации: `observability.deckhouse.io/title`. Если аннотация отсутствует — используется поле `title` из JSON дашборда |     |

### Пример конвертации

Старый формат:

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: example-dashboard
spec:
  folder: "Apps"
  json: '{
    "title": "Example Dashboard",
    ...
  }'
```

Новый формат (`ObservabilityDashboard`):

```yaml
apiVersion: observability.deckhouse.io/v1alpha1
kind: ObservabilityDashboard
metadata:
  name: example-dashboard
  namespace: my-namespace
  annotations:
    metadata.deckhouse.io/category: "Apps"
    metadata.deckhouse.io/title: "Example Dashboard"
spec:
  definition: |
    {
      "title": "Example Dashboard",
      ...
    }
```

Новый формат (`ClusterObservabilityDashboard`):

```yaml
apiVersion: observability.deckhouse.io/v1alpha1
kind: ClusterObservabilityDashboard
metadata:
  name: example-dashboard
  annotations:
    metadata.deckhouse.io/category: "Apps"
    metadata.deckhouse.io/title: "Example Dashboard"
spec:
  definition: |
    {
      "title": "Example Dashboard",
      ...
    }
```

## Как предоставить права на метрики и дашборды в конкретном пространстве имен

Для предоставления доступа к метрикам и дашбордам в конкретном пространстве имён необходимо создать ресурсы `ClusterRole` и `RoleBinding`, которые будут определять права пользователя.
Доступ к метрикам и дашбордам предоставляется отдельно:

- Метрики — проверяется наличие права `get` на ресурс `metrics.observability.deckhouse.io`.
- Дашборды — проверяется наличие прав на ресурс `observabilitydashboards.observability.deckhouse.io`:
  - `get` — просмотр дашбордов;
  - `create` — создание, изменение и удаление дашбордов.

### Пример ClusterRole и RoleBinding для доступа к метрикам и дашбордам только на чтение

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: observability-viewer
rules:
  - apiGroups: ["observability.deckhouse.io"]
    resources: ["metrics", "observabilitydashboards"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: bind-observability-viewer
  namespace: my-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: observability-viewer
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
```

### Пример ClusterRole и RoleBinding для доступа к метрикам и дашбордам на чтение и редактирование

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: observability-editor
rules:
  - apiGroups: ["observability.deckhouse.io"]
    resources: ["metrics", "observabilitydashboards"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["observability.deckhouse.io"]
    resources: ["observabilitydashboards"]
    verbs: ["create", "update", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: bind-observability-editor
  namespace: my-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: observability-editor
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
```

## Как предоставить доступ к системным метрикам и дашбордам

Для предоставления доступа к системным метрикам и дашбордам необходимо создать `ClusterRole` и `ClusterRoleBinding`, которые будут определять права пользователя.
Доступ к метрикам и дашбордам предоставляется отдельно:

- Метрики — проверяется наличие права `get` на ресурс `clustermetrics.observability.deckhouse.io`.
- Дашборды — проверяется наличие прав на ресурс `clusterobservabilitydashboards.observability.deckhouse.io`:
  - `get` — просмотр дашбордов;
  - `create` — создание, изменение и удаление дашбордов.

### Пример ClusterRole и ClusterRoleBinding для просмотра системных метрик и дашбордов

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: observability-cluster-viewer
rules:
  - apiGroups: ["observability.deckhouse.io"]
    resources: ["clustermetrics", "clusterobservabilitydashboards"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: bind-observability-cluster-viewer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: observability-cluster-viewer
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
```

### Пример ClusterRole и ClusterRoleBinding для доступа к метрикам и дашбордам на чтение и редактирование

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: observability-cluster-editor
rules:
  - apiGroups: ["observability.deckhouse.io"]
    resources: ["clustermetrics", "clusterobservabilitydashboards"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["observability.deckhouse.io"]
    resources: ["clusterobservabilitydashboards"]
    verbs: ["create", "update", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: bind-observability-cluster-editor
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: observability-cluster-editor
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
```

## Как предоставить полный доступ ко всем метрикам и дашбордам

Для предоставления полного доступа ко всем метрикам и дашбордам в Deckhouse необходимо создать роль `ClusterRole`, которая будет включать все необходимые права, и используйте `ClusterRoleBinding` для назначения этой роли.

### Пример ClusterRole

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: observability-admin
rules:
  - apiGroups: ["observability.deckhouse.io"]
    resources:
      - metrics
      - clustermetrics
      - observabilitydashboards
      - clusterobservabilitydashboards
      - clusterobservabilitypropagateddashboards
    verbs: ["get", "list", "watch", "create", "update", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: bind-observability-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: observability-admin
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
```

> Можно использовать готовую роль `cluster-admin`, однако её следует применять с осторожностью, так как она предоставляет полный доступ ко всем ресурсам кластера.

## Как выдать доступ при использовании RBAC 2.0

Если включена [экспериментальная ролевая модель](/modules/user-authz/#экспериментальная-ролевая-модель), права назначаются через ресурсы `UserRole` и `ClusterUserRole`.

### Пример доступа к метрикам и дашбордам в конкретном пространстве имён

Для предоставления пользователю доступа к пространству имён `myapp` с возможностью просмотра метрик и дашбордов, можно использовать следующий манифест:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: myapp-developer
  namespace: myapp
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:use:role:user
  apiGroup: rbac.authorization.k8s.io
```

> Данный пример предоставляет права не только для доступа к дашбордам и метрикам. Описание данной роли можно найти в [документации модуля user-authz](/modules/user-authz/#use-роли).

## Как предоставить внешний доступ для чтения метрик

Для предоставления внешнего доступа к метрикам необходимо выполнить следующие шаги:

1. Разрешить внешний доступ к метрикам. Для этого необходимо включить параметр [spec.settings.externalMetricsAccess](/modules/observability/configuration.html#parameters-externalmetricsaccess) в настройках модуля observability.
2. Для авторизации запросов создать сервис аккаунт.

   ```yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: metrics-access
     namespace: my-namespace
   ---
   apiVersion: v1
   kind: Secret
   metadata:
     name: metrics-access
     annotations:
       kubernetes.io/service-account.name: metrics-access
   type: kubernetes.io/service-account-token
   ```

3. Предоставить права на чтение метрик для созданного сервис аккаунта с помощью Role и RoleBinding.

   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: Role
   metadata:
     namespace: my-namespace
     name: metrics-access
   rules:
     - apiGroups: ["observability.deckhouse.io"]
       resources: ["metrics"]
       verbs: ["get", "watch", "list"]
     - apiGroups: [""]
       resources: ["namespaces"]
       verbs: ["get", "watch", "list"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: RoleBinding
   metadata:
     name: metrics-access
     namespace: my-namespace
   subjects:
     - kind: ServiceAccount
       name: metrics-access
       namespace: my-namespace
   roleRef:
     kind: Role
     name: metrics-access
     apiGroup: rbac.authorization.k8s.io
   ```

4. Получить авторизационный токен. При создании `ServiceAccount` был создан `Secret`, содержащий авторизационный токен.
   Токены в секретах содержатся в base64. Этот токен можно использовать для доступа к метрикам.
   Получить токен и раскодировать его можно с помощью команды:

   ```bash
     kubectl -n my-namespace get secret metrics-access -ojsonpath='{ .data.token }' | base64 -d
   ```

   Этот токен потребуется на следующем шаге, для настройки datasource в Grafana.

5. Настройка доступа к метрикам из Grafana
   Во внешней Grafana необходимо добавить датасорс Prometheus со следующими параметрами:

   - `Name` - произвольное имя data source-а.
   - `URL` - ссылка на внешний URL метрик. Для доступа к метрикам, необходимо использовать URL `https://observability.%publicDomainTemplate%/<prefix>/`. Где:
     - для доступа к основному Prometheus используется `<prefix>` равный `/metrics/`
     - для доступа к основному Prometheus используется `<prefix>` равный `/metrics/longterm`.
   - `HTTP Headers`:
     - `Header`: Authorization
     - `Value`: Bearer <TOKEN_VALUE>, где токен это токен полученный из `Secret`-а `metrics-access` на предыдущем шаге.

## Как предоставить внешний доступ для записи метрик

Для предоставления внешнего доступа для записи метрик необходимо выполнить следующие шаги:

1. Разрешить внешний доступ к метрикам, для этого необходимо включить параметр [spec.settings.externalMetricsAccess](/modules/observability/configuration.html#parameters-externalmetricsaccess), в настройках модуля observability.

2. Для авторизации запросов, создать сервис аккаунт:

   ```yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: metrics-access
     namespace: my-namespace
   ---
   apiVersion: v1
   kind: Secret
   metadata:
     name: metrics-access
     annotations:
       kubernetes.io/service-account.name: metrics-access
   type: kubernetes.io/service-account-token
   ```

3. Предоставить права на запись метрик для созданного сервис аккаунта с помощью `Role` и `RoleBinding`. Пример манифеста:

   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: Role
   metadata:
     namespace: my-namespace
     name: metrics-access
   rules:
     - apiGroups: ["observability.deckhouse.io"]
       resources: ["metrics"]
       verbs: ["create"]
     - apiGroups: [""]
       resources: ["namespaces"]
       verbs: ["get", "watch", "list"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: RoleBinding
   metadata:
     name: metrics-access
     namespace: my-namespace
   subjects:
     - kind: ServiceAccount
       name: metrics-access
       namespace: my-namespace
   roleRef:
     kind: Role
     name: metrics-access
     apiGroup: rbac.authorization.k8s.io
   ```

4. Получить авторизационный токен. При создании `ServiceAccount` был создан `Secret`, содержащий авторизационный токен.
   Токены в секретах содержатся в base64. Этот токен можно использовать для доступа к метрикам.
   Получить токен и раскодировать его можно с помощью команды:

   ```bash
   kubectl -n my-namespace get secret metrics-access -ojsonpath='{ .data.token }' | base64 -d
   ```

5. Для записи метрик необходимо отправлять запросы по протоколам Prometheus Remote-Write [V1](https://prometheus.io/docs/specs/prw/remote_write_spec/) или [V2](https://prometheus.io/docs/specs/prw/remote_write_spec_2_0/).
   - `URL`: `https://observability.%publicDomainTemplate%/api/v1/write`. [Подробнее про publicDomainTemplate](/reference/api/global.html#parameters-modules-publicdomaintemplate).
   - `HTTP Headers`:
     - `Header`: Authorization
     - `Value`: Bearer <TOKEN_VALUE>, где токен это токен полученный из `Secret`-а `metrics-access` на предыдущем шаге.

## Как предоставить внешний доступ для чтения метрик кластера

Для предоставления внешнего доступа к метрикам кластера, необходимо выполнить следующие шаги:

1. Разрешить внешний доступ к метрикам. Для этого необходимо включить параметр [spec.settings.externalMetricsAccess](/modules/observability/configuration.html#parameters-externalmetricsaccess) в настройках модуля observability.

2. Для авторизации запросов создать сервис аккаунт.

   ```yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: cluster-metrics-access
   ---
   apiVersion: v1
   kind: Secret
   metadata:
     name: cluster-metrics-access
     annotations:
       kubernetes.io/service-account.name: cluster-metrics-access
   type: kubernetes.io/service-account-token
   ```

3. Предоставить права на чтение метрик для созданного сервис аккаунта с помощью Role и RoleBinding.

   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: observability-cluster-metrics-viewer
   rules:
     - apiGroups: ["observability.deckhouse.io"]
       resources: ["clustermetrics"]
       verbs: ["get", "list", "watch"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: bind-observability-cluster-metrics-viewer
   subjects:
     - kind: ServiceAccount
       name: cluster-metrics-access
       namespace: default
   roleRef:
     kind: ClusterRole
     name: observability-cluster-metrics-viewer
     apiGroup: rbac.authorization.k8s.io
   ```

4. Получить авторизационный токен с помощью команды. При создании ServiceAccount был создан `Secret`, содержащий авторизационный токен.
   Токены в секретах содержатся в base64. Этот токен можно использовать для доступа к метрикам.
   Получить токен и раскодировать его можно с помощью команды:

   ```bash
     kubectl -n my-namespace get secret metrics-access -ojsonpath='{ .data.token }' | base64 -d
   ```

   Этот токен потребуется на следующем шаге, для настройки datasource в Grafana.

5. Настройка доступа к метрикам из Grafana
   Во внешней Grafana необходимо добавить датасорс Prometheus со следующими параметрами:

   - `Name` - произвольное имя data source-а.
   - `URL` - ссылка на внешний URL метрик. Для доступа к метрикам, необходимо использовать URL `https://observability.%publicDomainTemplate%/<prefix>/`. Где:
     - для доступа к основному Prometheus используется `<prefix>` равный `/metrics/`
     - для доступа к основному Prometheus используется `<prefix>` равный `/metrics/longterm`.
   - `HTTP Headers`:
     - `Header`: Authorization
     - `Value`: Bearer <TOKEN_VALUE>, где токен это токен полученный из `Secret`-а `cluster-metrics-access` на предыдущем шаге.
