---
title: Аннотации Nelm
permalink: ru/architecture/marketplace/nelm-annotations.html
description: "Аннотации Nelm для развёртывания Application: порядок, жизненный цикл, отслеживание готовности, логи, шаблонные функции, стадии развёртывания и типичные задачи."
lang: ru
search: nelm annotations, werf.io, deployment annotations, аннотации nelm, аннотации развёртывания, порядок ресурсов
---

{% raw %}

Шаблоны Application рендерятся и разворачиваются с помощью **Nelm**. Nelm расширяет стандартное поведение Helm аннотациями, управляющими порядком развёртывания, жизненным циклом ресурсов, отслеживанием готовности и выводом логов. На этой странице описаны наиболее часто используемые аннотации.

## Стадии развёртывания

Nelm выполняет развёртывание в три стадии. Разные группы аннотаций влияют на разные стадии.

### 1. Render

Шаблоны вычисляются с текущими values. DP выполняет рендеринг шаблонов с доступом к кластеру: `.Capabilities` отражает API-сервер, а `lookup` возвращает объекты из кластера. На этой стадии:

- Выполняются шаблонные функции (`include`, `tpl`, `lookup` и другие).
- Все ресурсы рендерятся и сохраняются в релизе независимо от `werf.io/deploy-on`.

### 2. Plan

Nelm подключается к кластеру, читает текущее состояние ресурсов и запускает **dry-run Server-Side Apply** для вычисления точного diff. Затем строит **DAG операций** — какие ресурсы создать, обновить или удалить и в каком порядке.

На этой стадии:
- `werf.io/deploy-on` определяет, при каких операциях (`install`, `upgrade` и т. д.) и на какой стадии развёртывается ресурс.
- Аннотации порядка и зависимостей (`werf.io/weight`, `werf.io/deploy-dependency-*`, `*.external-dependency.werf.io/*`) формируют DAG.
- Аннотации жизненного цикла (`werf.io/ownership`, `werf.io/delete-policy`, `werf.io/delete-propagation`) определяют, какие операции попадут в план.

Благодаря dry-run SSA diff вычисляет API-сервер с учётом подстановки значений по умолчанию, admission-плагинов и мутирующих вебхуков.

### 3. Apply

Выполняется DAG: ресурсы создаются, обновляются или удаляются в порядке зависимостей, с параллелизмом там, где зависимостей нет. Отслеживание готовности и вывод логов выполняются параллельно.

На этой стадии:
- Аннотации отслеживания (`werf.io/track-termination-mode`, `werf.io/fail-mode`, `werf.io/failures-allowed-per-replica`, `werf.io/no-activity-timeout`) управляют ожиданием.
- Аннотации логов управляют выводом во время развёртывания.

---

## Типичные задачи

### 1. Job, которая должна запуститься перед основным приложением

Классический сценарий: миграция БД перед запуском приложения.

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: d8a-{{ .Application.Instance.Name }}-db-migrate
  annotations:
    werf.io/delete-policy: before-creation
spec: ...
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: d8a-{{ .Application.Instance.Name }}-app
  annotations:
    werf.io/deploy-dependency-migrate: state=ready,kind=Job,name=d8a-{{ .Application.Instance.Name }}-db-migrate
spec: ...
```

`before-creation` заставляет Nelm пересоздавать Job при каждом развёртывании, поэтому миграция выполняется каждый раз. По умолчанию Nelm пересоздаёт Job, только если изменилось неизменяемое поле Job. Deployment ждёт `ready` — то есть успешного завершения Job.

### 2. Сохранение ресурса при uninstall или удалении из чарта

Сценарий: PVC с данными БД, который не должен удаляться при uninstall Application.

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: d8a-{{ .Application.Instance.Name }}-postgres-data
  annotations:
    helm.sh/resource-policy: keep
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 50Gi
  storageClassName: gp3
```

`helm.sh/resource-policy: keep` предотвращает удаление PVC при uninstall или удалении из чарта. От `d8 k delete pvc` аннотация не защищает. Чтобы сохранить данные в этом случае, используйте StorageClass с `reclaimPolicy: Retain`: тогда PersistentVolume переживает PVC.

### 3. Ресурс, общий между релизами

TLS-секрет, используемый несколькими чартами, не должен исчезать при uninstall любого из них.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: d8a-{{ .Application.Instance.Name }}-shared-tls
  annotations:
    werf.io/ownership: anyone
type: kubernetes.io/tls
data: ...
```

`anyone` сообщает Nelm, что он не единственный владелец ресурса, поэтому Nelm пропускает удаление при uninstall.

### 4. Зависимость от ресурса, созданного оператором

Развёртывание только после того, как cert-manager создаст объект Certificate:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: d8a-{{ .Application.Instance.Name }}-app
  annotations:
    cert.external-dependency.werf.io/resource: certificates.v1.cert-manager.io/d8a-{{ .Application.Instance.Name }}-tls
spec: ...
```

Nelm ждёт появления объекта `Certificate`, прежде чем создавать Deployment. Выпуска сертификата Nelm не ждёт.

### 5. Некритичный компонент, сбой которого не прерывает развёртывание

DaemonSet с метриками, чья недоступность не должна блокировать релиз:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: d8a-{{ .Application.Instance.Name }}-metrics-agent
  annotations:
    werf.io/fail-mode: IgnoreAndContinueDeployProcess
    werf.io/track-termination-mode: NonBlocking
spec: ...
```

Nelm не будет ждать готовности, и развёртывание не завершится ошибкой по таймауту.

### 6. Ресурс, рендерящийся только при первой установке

Init Job, нужная только при `install`, не при `upgrade`:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: d8a-{{ .Application.Instance.Name }}-init-data
  annotations:
    werf.io/deploy-on: install
    werf.io/ownership: anyone   # КРИТИЧНО
spec: ...
```

Без `werf.io/ownership: anyone` при upgrade ресурс рендерится как отсутствующий, и владеющий релиз его удалит. `anyone` предотвращает это.

### 7. Медленно стартующий ресурс с большим образом

StatefulSet с большим образом контейнера (ML-модели, Elasticsearch с заранее загруженными индексами):

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: d8a-{{ .Application.Instance.Name }}-elasticsearch
  annotations:
    werf.io/no-activity-timeout: 20m
    werf.io/failures-allowed-per-replica: "3"
    werf.io/show-service-messages: "true"
spec: ...
```

- `no-activity-timeout: 20m` — 20 минут без событий, прежде чем Nelm считает это таймаутом. Используйте, если образ загружается медленно или инициализация занимает много времени.
- `failures-allowed-per-replica: "3"` — допускает до 3 ошибок отслеживания на реплику, например, перезапусков контейнеров или ошибок загрузки образа, прежде чем развёртывание завершится ошибкой. Используйте для нестабильных зависимостей при инициализации.
- `show-service-messages: "true"` — выводить события Kubernetes в лог развёртывания (полезно для диагностики `ImagePullBackOff`, ошибок планирования, OOM).

---

## Порядок и зависимости

### Веса

`werf.io/weight` группирует ресурсы: ресурсы с одинаковым весом развёртываются параллельно, с разным — последовательно в порядке возрастания веса. Для ресурса с аннотацией `werf.io/deploy-dependency-*` вес не учитывается.

```yaml
metadata:
  annotations:
    werf.io/weight: "-10"   # Развёртывается раньше ресурсов с весом по умолчанию (0).
```

### Прямые зависимости

`werf.io/deploy-dependency-<id>` ждёт, пока конкретный ресурс в том же релизе достигнет указанного состояния, прежде чем развёртывать ресурс с этой аннотацией.

```yaml
metadata:
  annotations:
    werf.io/deploy-dependency-db: state=ready,kind=StatefulSet,name=d8a-{{ .Application.Instance.Name }}-postgres
    werf.io/deploy-dependency-migrations: state=present,kind=Job,name=d8a-{{ .Application.Instance.Name }}-db-migrate
```

Состояния зависимости:

- `ready` — ресурс в готовом состоянии (например, у Deployment сошлись `availableReplicas == replicas`).
- `present` — ресурс существует в кластере.

Полный формат:

```text
werf.io/deploy-dependency-<id>: state=ready|present[,name=<name>][,namespace=<namespace>][,kind=<kind>][,group=<group>][,version=<version>]
```

{% endraw %}
{% alert level="warning" %}
Аннотация не работает, если ресурс-зависимость находится в другой стадии развёртывания (pre/main/post). Порядок между стадиями уже обеспечивает сама последовательность стадий.
{% endalert %}
{% raw %}

### Внешние зависимости

`<id>.external-dependency.werf.io/resource` ждёт появления ресурса **вне релиза** (созданного оператором или другим релизом). Готовности ресурса Nelm не ждёт:

```yaml
metadata:
  annotations:
    cert.external-dependency.werf.io/resource: certificates.v1.cert-manager.io/d8a-{{ .Application.Instance.Name }}-tls
    cert.external-dependency.werf.io/namespace: myapp-production   # Неймспейс внешнего ресурса
```

Полный формат:

```text
<id>.external-dependency.werf.io/resource: <kind>[.<version>.<group>]/<name>
```

### Зависимости при удалении

`werf.io/delete-dependency-<id>` — аннотированный ресурс будет удалён только после того, как указанный ресурс станет `absent`:

```yaml
metadata:
  annotations:
    werf.io/delete-dependency-app: state=absent,kind=Deployment,name=d8a-{{ .Application.Instance.Name }}-app
```

---

## Аннотации жизненного цикла

### `helm.sh/resource-policy`

`keep` — не удалять ресурс при uninstall или удалении из чарта. Ресурс продолжает обновляться при install/upgrade, пока рендерится. Такой ресурс не удаляется и вместе с [осиротевшими ресурсами](lifecycle.html#осиротевшие-ресурсы) приложения. Если задана аннотация `werf.io/resource-policy`, `helm.sh/resource-policy` не учитывается.

### `werf.io/resource-policy`

Список политик через запятую:

- `skip-create` — не создавать ресурс, если его нет в кластере;
- `skip-update` — не обновлять ресурс, если он уже существует;
- `skip-recreate` — не пересоздавать ресурс, когда иначе потребовалось бы пересоздание;
- `skip-delete` (псевдоним — `keep`) — не удалять ресурс.

### `werf.io/ownership`

- `release` (по умолчанию для обычных ресурсов) — ресурс удаляется при uninstall и при отсутствии в чарте. Применяются release metadata-аннотации.
- `anyone` (по умолчанию для хуков) — ресурс не удаляется при uninstall. Release metadata-аннотации не применяются.

Используйте `anyone` для ресурсов, общих между релизами, или для ресурсов, которые должны переживать свой релиз (init Job с `werf.io/deploy-on: install`).

Хуки, которые остаются после uninstall, DP удаляет вместе с Application, если их тип перечислен в [`orphanResources`](lifecycle.html#осиротевшие-ресурсы). Ресурс с явно заданной аннотацией `werf.io/ownership: anyone` не удаляется.

### `werf.io/deploy-on`

Управляет тем, в каких операциях жизненного цикла рендерится ресурс.

```yaml
werf.io/deploy-on: pre-install,upgrade,post-install
```

Допустимые значения: `pre-install`, `install`, `post-install`, `pre-upgrade`, `upgrade`, `post-upgrade`, `pre-rollback`, `rollback`, `post-rollback`, `pre-delete`, `delete`, `post-delete`. По умолчанию для обычных ресурсов: `install,upgrade,rollback`. Хуки развёртываются на событиях, перечисленных в `helm.sh/hook`.

{% endraw %}
{% alert level="warning" %}
Если ресурс рендерится для `install`, но не для `upgrade`, и у него `werf.io/ownership: release` — ресурс будет **удалён при upgrade**, так как он отсутствует в upgrade-рендере. Установите `werf.io/ownership: anyone`, чтобы это предотвратить.
{% endalert %}
{% raw %}

### `werf.io/delete-policy`

Управляет тем, когда ресурс удаляется относительно операции apply.

| Значение | Когда |
|---|---|
| `before-creation` | Всегда пересоздавать перед apply (по умолчанию для хуков) |
| `before-creation-if-immutable` | Пересоздавать только при ошибке `field is immutable` (по умолчанию для Job) |
| `succeeded` | Удалить после успешного развёртывания |
| `failed` | Удалить, если ресурс не прошёл проверку готовности |

Значения можно комбинировать через запятую: `before-creation,succeeded`.

### `werf.io/delete-propagation`

Стратегия каскадного удаления в Kubernetes.

| Значение | Поведение |
|---|---|
| `Foreground` (по умолчанию) | Ждать удаления зависимых объектов |
| `Background` | Удалить ресурс сразу; зависимые удаляются асинхронно |
| `Orphan` | Удалить ресурс; зависимые оставить |

---

## Аннотации отслеживания

| Аннотация | По умолчанию | Описание |
|---|---|---|
| `werf.io/track-termination-mode` | `WaitUntilResourceReady` | `WaitUntilResourceReady` или `NonBlocking` |
| `werf.io/fail-mode` | `FailWholeDeployProcessImmediately` | `FailWholeDeployProcessImmediately` или `IgnoreAndContinueDeployProcess` |
| `werf.io/failures-allowed-per-replica` | `1` для Deployment, StatefulSet и DaemonSet, `0` для остальных типов | Число ошибок отслеживания на реплику, например, перезапусков контейнеров или ошибок загрузки образа, допустимое до того, как развёртывание завершится ошибкой. Для Job не учитывается |
| `werf.io/no-activity-timeout` | `4m` | Длительность в формате Go; таймаут при отсутствии событий и изменений статуса |
| `werf.io/show-service-messages` | `false` | Показывать события Kubernetes в выводе развёртывания |

---

## Аннотации логов

{% endraw %}
{% alert level="info" %}
DP не собирает логи подов при развёртывании приложений, поэтому эти аннотации не влияют на развёртывание.
{% endalert %}
{% raw %}

| Аннотация | По умолчанию | Описание |
|---|---|---|
| `werf.io/skip-logs` | `false` | Скрыть все логи подов |
| `werf.io/skip-logs-for-containers` | — | Список контейнеров через запятую для скрытия |
| `werf.io/show-logs-only-for-containers` | — | Список контейнеров через запятую; показывать только для них |
| `werf.io/show-logs-only-for-number-of-replicas` | `1` | Показывать логи только для первых N реплик |
| `werf.io/log-regex` | — | RE2-паттерн; показывать только совпадающие строки |
| `werf.io/log-regex-skip` | — | RE2-паттерн; скрывать совпадающие строки |
| `werf.io/log-regex-for-<container>` | — | RE2-фильтр строк для показа в логах указанного контейнера |
| `werf.io/log-regex-skip-for-<container>` | — | RE2-фильтр строк для скрытия в логах указанного контейнера |

---

## Шаблонные функции

### `werf_secret_file`

Встроить расшифрованное содержимое файла из директории `secret/`:

```yaml
data:
  config.yaml: {{ werf_secret_file "config.yaml" | b64enc }}
```

{% endraw %}
{% alert level="warning" %}
DP не предоставляет ключ для расшифровки секретов Nelm, поэтому в пакетах Application нельзя использовать `werf_secret_file`, каталог `secret/` и `secret-values.yaml`.
{% endalert %}
{% raw %}

### `dump_debug`, `printf_debug`, `include_debug`, `tpl_debug`

`dump_debug` и `printf_debug` ничего не выводят, а `include_debug` и `tpl_debug` выводят то же, что `include` и `tpl`. Функции пишут в лог, только если включено отладочное логирование Nelm. DP его не включает, поэтому в пакетах Application эти функции ничего не пишут в лог.

```yaml
{{ dump_debug $ }}
{{ printf_debug "replicaCount: %d" .Values.replicaCount }}
{{ include_debug "myapp.labels" . | nindent 4 }}
{{ tpl_debug "{{ .Values.template }}" . }}
```

---

## Известные подводные камни

1. **`null`-значения и SSA.** Server-Side Apply часто падает на полях со значением `null`. Если `.Values.foo` равно `nil`, в манифесте появится `foo: null`. Решение: проверяйте значение условием `{{ if .Values.foo }}` или используйте `default`.

2. **Время выполнения `lookup`.** Если ресурсы кластера изменились между стадиями Plan и Apply, отрендеренный план может быть устаревшим. Избегайте `lookup` для критичной логики — передавайте данные через values.

3. **Недетерминированные функции.** `now`, `randAlphaNum`, `uuidv4` и результат `keys` без сортировки дают разный результат при каждом рендеринге. DP применяет шаблоны, когда меняется результат рендеринга манифестов, поэтому такие функции приводят к повторному развёртыванию в каждом цикле согласования.

4. **`werf.io/deploy-dependency-*` и стадии.** Аннотация не действует между стадиями развёртывания (pre/main/post). Порядок между стадиями обеспечивается самой последовательностью стадий.

5. **Ресурсы, которые исчезают после развёртывания.** DP каждые 4–5 минут проверяет, что все обычные ресурсы релиза (не хуки) есть в кластере, и применяет шаблоны заново, если какого-то ресурса нет (см. [«Восстановление ресурсов»](lifecycle.html#восстановление-ресурсов)). Поэтому обычный Job, который удаляется после завершения, например, из-за `werf.io/delete-policy: succeeded` или `ttlSecondsAfterFinished`, запускается снова после каждой проверки, а удалённый ресурс, который рендерится только при установке, вызывает повторное развёртывание после каждой проверки. Оформляйте такие Job как Helm-хуки: хуки DP не проверяет.

{% endraw %}
