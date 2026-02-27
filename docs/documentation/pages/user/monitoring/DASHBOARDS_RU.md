---
title: "Дашборды мониторинга"
permalink: ru/user/monitoring/dashboards.html
lang: ru
---

В этом разделе вы узнаете о работе с дашбордами для анализа состояния Deckhouse Kubernetes Platform (DKP)
и запущенных в нем приложений.

Дашборды представляют собой наборы графиков и таблиц с данными о работе приложений.
Они содержат информацию о загрузке CPU, потреблении памяти, дисковой и сетевой активности,
а также о состоянии подов, контроллеров, узлов и неймспейсов.

## Виды дашбордов

В DKP доступны предустановленные дашборды, а также пользовательские дашборды, которые можно создавать несколькими способами.

| Вид дашбордов | Описание |
| ------------- | -------- |
| [Предустановленные](#предустановленные-дашборды) | Готовые дашборды, поставляемые вместе с DKP. Предназначены для мониторинга состояния запущенных приложений. |
| [Пользовательские, создаваемые с помощью модуля `observability`](#с-помощью-модуля-observability) | Пользовательские дашборды, создаваемые с помощью ресурса ObservabilityDashboard на уровне неймспейсов и с возможностью разграничения прав доступа.<br><br>Это рекомендуемый способ работы с дашбордами. |
| [Пользовательские, создаваемые с помощью GrafanaDashboardDefinition](#с-помощью-grafanadashboarddefinition) | Пользовательские дашборды, создаваемые с помощью ресурса GrafanaDashboardDefinition на уровне кластера. Требуют расширенных прав доступа и не позволяют управлять разграничением прав доступа.<br><br>Это устаревший способ работы с дашбордами, который перестанет поддерживаться в следующих версиях DKP. |

## Предустановленные дашборды

Пользователям DKP предоставляется доступ к базовому набору дашбордов для наблюдения за состоянием запущенных приложений.
Дашборды доступны в [веб-интерфейсе Deckhouse](/modules/console/) в разделе «Мониторинг» → «Дашборды».

{% alert level="info" %}
Предустановленные дашборды недоступны для редактирования.
{% endalert %}

### Ingress Nginx

Дашборды для мониторинга работы Ingress-контроллера.
Содержат метрики, отражающие состояние виртуальных хостов, статистику по HTTP-ответам,
а также данные о задержках при обработке запросов.

Доступные дашборды:

- **Namespaces** — совокупные метрики Ingress-ресурсов по неймспейсам;
- **Namespace Detail** — детальная информация по Ingress-ресурсам в выбранном неймспейсе;
- **VHosts** — обзор состояния виртуальных хостов;
- **VHost Detail** — детальная информация по выбранному виртуальному хосту.

### Потребление ресурсов (Main)

Набор дашбордов для анализа потребления ресурсов приложениями.
Дашборды предназначены для оценки нагрузки, поиска проблем с ресурсами и анализа состояния рабочих нагрузок.

Доступные дашборды:

- **Namespaces** — сводная информация по всем неймспейсам;
- **Namespace** — основные показатели использования ресурсов в выбранном неймспейсе;
- **Namespace / Controller** — статистика использования ресурсов контроллерами в рамках выбранного неймспейса;
- **Namespace / Controller / Pod** — детальные метрики по отдельным подам.

### Security

Дашборды, содержащие метрики, связанные с безопасностью компонентов кластера.

Доступные дашборды:

- **Admission policy engine** — метрики работы [модуля `admission-policy-engine`](/modules/admission-policy-engine/),
  включая информацию о проверках и применении политик.

## Пользовательские дашборды

Пользователи DKP могут создавать собственные дашборды несколькими способами,
в зависимости от требований к управлению доступом и области видимости дашборда.

### С помощью модуля observability

[Модуль `observability`](/modules/observability/) расширяет функциональность модуля `prometheus` и веб-интерфейса Deckhouse,
предоставляя дополнительные возможности для гибкого управления визуализацией метрик и разграничения доступа к ним.

Модуль добавляет новые типы дашбордов, включая ресурсы, ограниченные неймспейсом.
Это даёт пользователям возможность создавать и управлять собственными дашбордами
без необходимости иметь права на объекты кластерного уровня.
Кроме того, модуль упрощает редактирование дашбордов — настройка выполняется
напрямую в веб-интерфейсе, без необходимости работы с ресурсами вручную.

{% alert level="info" %}
Перед началом работы с этими ресурсами убедитесь, что модуль `observability` включён в кластере.
При необходимости обратитесь к администратору DKP.
{% endalert %}

Для создания дашбордов предусмотрены следующие ресурсы:

- [ObservabilityDashboard](/modules/observability/cr.html#observabilitydashboard) — дашборды в рамках неймспейса.
  Отображаются в веб-интерфейсе Deckhouse в разделе «Мониторинг» → «Проекты».

  Пример:

  ```yaml
  apiVersion: observability.deckhouse.io/v1alpha1
  kind: ObservabilityDashboard
  metadata:
    name: example-dashboard
    namespace: my-namespace
    annotations:
      metadata.deckhouse.io/category: "Apps"
      metadata.deckhouse.io/title: "Example dashboard"
  spec:
    definition: |
      {
        "title": "Example dashboard",
        ...
      }
  ```

- [ClusterObservabilityDashboard](/modules/observability/cr.html#clusterobservabilitydashboard) — дашборды для отображения компонентов кластера.
  Отображаются в веб-интерфейсе Deckhouse в разделе «Мониторинг» → «Система».

  Пример:

  ```yaml
  apiVersion: observability.deckhouse.io/v1alpha1
  kind: ClusterObservabilityDashboard
  metadata:
    name: example-dashboard
    annotations:
      metadata.deckhouse.io/category: "Apps"
      metadata.deckhouse.io/title: "Example dashboard"
  spec:
    definition: |
      {
        "title": "Example dashboard",
        ...
      }
  ```

- [ClusterObservabilityPropagatedDashboard](/modules/observability/cr.html#clusterobservabilitypropagateddashboard) — дашборды, расширяющие список дашбордов из двух предыдущих категорий.
  Такие дашборды автоматически добавляются в веб-интерфейс Deckhouse
  и отображаются в разделах «Мониторинг» → «Система» и «Мониторинг» → «Проекты».
  Они становятся доступны пользователям, обладающим правами на соответствующий неймспейс или системный раздел.

  Пример:

  ```yaml
  apiVersion: observability.deckhouse.io/v1alpha1
  kind: ClusterObservabilityPropagatedDashboard
  metadata:
    name: example-dashboard
    annotations:
      metadata.deckhouse.io/category: "Apps"
      metadata.deckhouse.io/title: "Example dashboard"
  spec:
    definition: |
      {
        "title": "Example dashboard",
        ...
      }
  ```

#### Разграничение прав доступа

Доступ к дашбордам настраивается с помощью механизмов [действующей ролевой модели (RBAC)](../../admin/configuration/access/authorization/rbac-current.html).

В зависимости от типа дашборда (системный или пользовательский) права назначаются на следующие ресурсы:

- `observabilitydashboards.observability.deckhouse.io` — дашборды в рамках неймспейсов;
- `clusterobservabilitydashboards.observability.deckhouse.io` — системные дашборды;
- `clusterobservabilitypropagateddashboards.observability.deckhouse.io` — дашборды, распространяемые на всех пользователей.

Для выполнения операций с дашбордами требуются следующие разрешения:

- чтение — `get`;
- создание и редактирование — `create`, `update`, `patch`, `delete`.

Доступ к метрикам в дашбордах также контролируется с помощью RBAC.
В зависимости от выданных прав фильтрация метрик осуществляется автоматически.

Поддерживаются следующие сценарии доступа:

- Пользователи неймспейсов получают доступ только к метрикам своего неймспейса.
  Проверяется RBAC-доступ к ресурсу `metrics.observability.deckhouse.io`.

- Администраторы DKP получают доступ ко всем системным метрикам:
  - метрики Deckhouse (`d8-*`);
  - метрики Kubernetes (`kube-*`);
  - метрики без лейбла `namespace`.
  
  Используется RBAC-доступ к ресурсу `clustermetrics.observability.deckhouse.io`.

- Метрики из пользовательских неймспейсов также могут быть доступны администраторам
  при наличии соответствующих прав на ресурс `metrics.observability.deckhouse.io`.

Пример настройки ресурсов ClusterRole и RoleBinding для доступа к метрикам и дашбордам на чтение и редактирование:

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

#### Конвертация дашбордов из GrafanaDashboardDefinition

Чтобы перенести дашборды, созданные с помощью устаревшего ресурса GrafanaDashboardDefinition,
в один из форматов модуля `observability`, отредактируйте каждый манифест соответствующего дашборда вручную.
Обратите внимание на важные отличия:

| Формат GrafanaDashboardDefinition | Формат модуля `observability` |
| ------------------ | -------- |
| Папка в Grafana для отображения дашборда задается в поле `spec.folder`. | Папка задается с помощью аннотации `observability.deckhouse.io/category`. |
| Название дашборда задается в поле `title` JSON-манифеста. | Название задаётся с помощью аннотации `observability.deckhouse.io/title`. Если аннотация отсутствует, используется поле `title` из JSON-манифеста. |

Пример конвертации:

- Старый формат:

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

- Новый формат:

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

### С помощью GrafanaDashboardDefinition

{% alert level="info" %}
Это устаревший способ, который не рекомендуется для новых дашбордов.
Поддержка этого способа будет прекращена в следующих версиях DKP.
{% endalert %}

Чтобы добавить дашборд напрямую в Grafana, используйте [ресурс GrafanaDashboardDefinition](/modules/prometheus/cr.html#grafanadashboarddefinition).

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: my-dashboard
spec:
  folder: My folder # Папка, в которой в Grafana будет отображаться ваш дашборд.
  definition: |
    {
      "annotations": {
        "list": [
          {
            "builtIn": 1,
            "datasource": "-- Grafana --",
            "enable": true,
            "hide": true,
            "iconColor": "rgba(0, 211, 255, 1)",
            "limit": 100,
...
```

При использовании этого способа учитывайте следующие ограничения:

- Дашборды, добавленные через GrafanaDashboardDefinition, нельзя изменить через интерфейс Grafana.

- Алерты, настроенные в панели «Dashboard», не работают с шаблонами datasource — такой дашборд считается невалидным и не импортируется.
  Начиная с Grafana 9.0, функционал legacy alerting признан устаревшим и заменён на Grafana Alerting.
  В связи с этим не рекомендуется использовать legacy alerting (оповещения панели мониторинга) в дашбордах.

- Если после применения ресурса дашборд не появляется в Grafana, возможно, в JSON-файле дашборда содержится ошибка.
  Чтобы просмотреть логи компонента, отвечающего за применение дашбордов, используйте следующую команду:
  
  ```shell
  d8 k logs -n d8-monitoring deployments/grafana-v10 dashboard-provisioner
  ```
