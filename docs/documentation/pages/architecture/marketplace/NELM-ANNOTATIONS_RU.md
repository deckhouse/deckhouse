---
title: Аннотации Nelm
permalink: ru/architecture/marketplace/nelm-annotations.html
description: "Аннотации Nelm для деплоя Application: порядок, жизненный цикл, трекинг, логирование, шаблонные функции, стадии деплоя и типичные задачи."
lang: ru
search: nelm annotations, werf.io, deployment annotations, аннотации nelm, аннотации деплоя, порядок ресурсов
---

{% raw %}

Шаблоны Application рендерятся и разворачиваются с помощью **Nelm**. Nelm расширяет стандартное поведение Helm аннотациями, управляющими порядком деплоя, жизненным циклом ресурсов, трекингом готовности и выводом логов. На этой странице описаны все аннотации, доступные в шаблонах Application.

## Стадии деплоя

Nelm обрабатывает деплой в три стадии. Разные группы аннотаций влияют на разные стадии.

### 1. Render

Шаблоны вычисляются с текущими values. Доступ к кластеру не нужен (кроме `lookup`). На этой стадии:

- `werf.io/deploy-on` управляет включением ресурса в результат рендера для текущего типа операции (`install`, `upgrade` и т. д.).
- Выполняются шаблонные функции (`werf_secret_file`, `dump_debug` и др.).
- Расшифровываются `secret-values.yaml` и файлы из `secret/`.

### 2. Plan

Nelm подключается к кластеру, читает текущее состояние ресурсов и запускает **dry-run Server-Side Apply** для вычисления точного diff. Затем строит **DAG операций** — какие ресурсы создать, обновить или удалить и в каком порядке.

На этой стадии:
- Аннотации порядка и зависимостей (`werf.io/weight`, `werf.io/deploy-dependency-*`, `*.external-dependency.werf.io/*`) формируют DAG.
- Аннотации жизненного цикла (`werf.io/ownership`, `werf.io/delete-policy`, `werf.io/delete-propagation`) определяют, какие операции попадут в план.

Dry-run SSA означает, что diff вычисляет API server — defaulting, admission-плагины и мутирующие webhook'и применяются корректно.

### 3. Apply

Выполняется DAG: ресурсы создаются, обновляются или удаляются в порядке зависимостей, с параллелизмом там, где зависимостей нет. Трекинг готовности и стриминг логов выполняются параллельно.

На этой стадии:
- Аннотации трекинга (`werf.io/track-termination-mode`, `werf.io/fail-mode`, `werf.io/failures-allowed-per-replica`, `werf.io/no-activity-timeout`) управляют ожиданием.
- Аннотации логов управляют выводом во время деплоя.

---

## Типичные задачи

### 1. Job, которая должна запуститься перед основным приложением

Классический сценарий: миграция БД перед запуском приложения.

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: {{.Application.Instance.Name}}-db-migrate
  annotations:
    werf.io/delete-policy: before-creation
spec: ...
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.Application.Instance.Name}}-app
  annotations:
    werf.io/deploy-dependency-migrate: state=ready,kind=Job,name=db-migrate
spec: ...
```

`before-creation` гарантирует пересоздание Job при каждом деплое (иначе Kubernetes откажет в обновлении иммутабельных полей Job). Deployment ждёт `ready` — то есть успешного завершения Job.

### 2. Сохранение ресурса при удалении или uninstall

Сценарий: PVC с данными БД, который не должен удаляться при uninstall Application.

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{.Application.Instance.Name}}-postgres-data
  annotations:
    helm.sh/resource-policy: keep
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 50Gi
  storageClassName: gp3
```

`helm.sh/resource-policy: keep` предотвращает удаление PVC при uninstall или удалении из чарта. От `d8 k delete pvc` не защищает — для этого используйте `persistentVolumeReclaimPolicy: Retain` на StorageClass.

### 3. Ресурс, общий между релизами

TLS-секрет, используемый несколькими чартами, не должен исчезать при uninstall любого из них.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: {{.Application.Instance.Name}}-shared-tls
  annotations:
    werf.io/ownership: anyone
type: kubernetes.io/tls
data: ...
```

`anyone` сообщает Nelm, что он не единственный владелец ресурса, поэтому Nelm пропускает удаление при uninstall.

### 4. Зависимость от ресурса, созданного оператором

Деплой только после выпуска сертификата cert-manager:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.Application.Instance.Name}}-app
  annotations:
    cert.external-dependency.werf.io/resource: certificates.v1.cert-manager.io/myapp-tls
spec: ...
```

Nelm дождётся состояния `present` и `ready` у `Certificate`, прежде чем создавать Deployment.

### 5. Некритичный компонент, не роняющий весь деплой

DaemonSet с метриками, чья недоступность не должна блокировать релиз:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: {{.Application.Instance.Name}}-metrics-agent
  annotations:
    werf.io/fail-mode: IgnoreAndContinueDeployProcess
    werf.io/track-termination-mode: NonBlocking
spec: ...
```

Nelm не будет ждать готовности и не упадёт по таймауту.

### 6. Ресурс, рендерящийся только при первой установке

Init Job, нужная только при `install`, не при `upgrade`:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: {{.Application.Instance.Name}}-init-data
  annotations:
    werf.io/deploy-on: install
    werf.io/ownership: anyone   # КРИТИЧНО
spec: ...
```

Без `werf.io/ownership: anyone` при upgrade ресурс рендерится как отсутствующий, и владеющий релиз его удалит. `anyone` предотвращает это.

### 7. Медленно стартующий ресурс с большим образом

StatefulSet с большим Docker-образом (ML-модели, Elasticsearch с предзагруженными индексами):

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: elasticsearch
  annotations:
    werf.io/no-activity-timeout: 20m
    werf.io/failures-allowed-per-replica: "3"
    werf.io/show-service-messages: "true"
spec: ...
```

- `no-activity-timeout: 20m` — 20 минут без событий, прежде чем Nelm считает это таймаутом. Используйте для медленного image pull или долгой инициализации.
- `failures-allowed-per-replica: "3"` — допускает до 3 перезапусков на реплику перед объявлением ошибки. Используйте для нестабильных зависимостей при инициализации.
- `show-service-messages: "true"` — выводить Kubernetes Events в лог деплоя (полезно для диагностики `ImagePullBackOff`, ошибок планирования, OOM).

---

## Порядок и зависимости

### Веса

`werf.io/weight` группирует ресурсы: одинаковые веса деплоятся параллельно; разные — последовательно в порядке возрастания.

```yaml
metadata:
  annotations:
    werf.io/weight: "-10"   # Деплоится раньше ресурсов с весом по умолчанию (0)
```

### Прямые зависимости

`werf.io/deploy-dependency-<id>` ждёт, пока конкретный ресурс в том же релизе достигнет указанного состояния, прежде чем деплоить аннотированный ресурс.

```yaml
metadata:
  annotations:
    werf.io/deploy-dependency-db: state=ready,kind=StatefulSet,name=postgres
    werf.io/deploy-dependency-migrations: state=present,kind=Job,name=db-migrate
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
Аннотация не работает, если ресурс-зависимость находится в другой стадии деплоя (pre/main/post). Порядок между стадиями уже обеспечивает сама последовательность стадий.
{% endalert %}
{% raw %}

### Внешние зависимости

`<id>.external-dependency.werf.io/resource` ждёт ресурс **вне релиза** (созданный оператором или другим релизом):

```yaml
metadata:
  annotations:
    cert.external-dependency.werf.io/resource: certificates.v1.cert-manager.io/myapp-tls
    cert.external-dependency.werf.io/name: myapp-production   # Неймспейс внешнего ресурса
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
    werf.io/delete-dependency-app: state=absent,kind=Deployment,name=app
```

---

## Аннотации жизненного цикла

### `helm.sh/resource-policy`

`keep` — не удалять ресурс при uninstall или удалении из чарта. Ресурс продолжает обновляться при install/upgrade, пока рендерится.

### `werf.io/ownership`

- `release` (по умолчанию для обычных ресурсов) — ресурс удаляется при uninstall и при отсутствии в чарте. Применяются release metadata-аннотации.
- `anyone` (по умолчанию для хуков и CRD из `crds/`) — ресурс не удаляется при uninstall. Release metadata-аннотации не применяются.

Используйте `anyone` для ресурсов, общих между релизами, или для ресурсов, которые должны переживать свой релиз (init Job с `werf.io/deploy-on: install`).

### `werf.io/deploy-on`

Управляет тем, в каких операциях жизненного цикла рендерится ресурс.

```yaml
werf.io/deploy-on: pre-install,upgrade,post-install
```

Допустимые значения: `pre-install`, `install`, `post-install`, `pre-upgrade`, `upgrade`, `post-upgrade`, `pre-rollback`, `rollback`, `post-rollback`, `pre-uninstall`, `uninstall`, `post-uninstall`. По умолчанию: `install,upgrade,rollback`.

{% endraw %}
{% alert level="warning" %}
Если ресурс рендерится для `install`, но не для `upgrade`, и у него `werf.io/ownership: release` — ресурс будет **удалён при upgrade**, так как он отсутствует в upgrade-рендере. Установите `werf.io/ownership: anyone`, чтобы это предотвратить.
{% endalert %}
{% raw %}

### `werf.io/delete-policy`

Управляет тем, когда ресурс удаляется относительно операции apply.

| Значение | Когда |
|---|---|
| `before-creation` | Всегда пересоздавать перед apply |
| `before-creation-if-immutable` | Пересоздавать только при ошибке «поле иммутабельно» (по умолчанию для Job) |
| `succeeded` | Удалить после успешного деплоя |
| `failed` | Удалить при провале readiness-чека |

Значения можно комбинировать через запятую: `before-creation,succeeded`.

### `werf.io/delete-propagation`

Стратегия каскадного удаления в Kubernetes.

| Значение | Поведение |
|---|---|
| `Foreground` (по умолчанию) | Ждать удаления зависимых объектов |
| `Background` | Удалить ресурс сразу; зависимые удаляются асинхронно |
| `Orphan` | Удалить ресурс; зависимые оставить |

---

## Аннотации трекинга

| Аннотация | По умолчанию | Описание |
|---|---|---|
| `werf.io/track-termination-mode` | `WaitUntilResourceReady` | `WaitUntilResourceReady` или `NonBlocking` |
| `werf.io/fail-mode` | `FailWholeDeployProcessImmediately` | `FailWholeDeployProcessImmediately` или `IgnoreAndContinueDeployProcess` |
| `werf.io/failures-allowed-per-replica` | `1` | Число перезапусков на реплику до объявления ошибки |
| `werf.io/no-activity-timeout` | `4m` | Go duration; таймаут при отсутствии событий или изменений статуса |
| `werf.io/show-service-messages` | `false` | Показывать Kubernetes Events в выводе деплоя |

---

## Аннотации логов

| Аннотация | По умолчанию | Описание |
|---|---|---|
| `werf.io/skip-logs` | `false` | Скрыть все логи подов |
| `werf.io/skip-logs-for-containers` | — | Список контейнеров через запятую для скрытия |
| `werf.io/show-logs-only-for-containers` | — | Список контейнеров через запятую; показывать только для них |
| `werf.io/show-logs-only-for-number-of-replicas` | `1` | Показывать логи только для первых N реплик |
| `werf.io/log-regex` | — | RE2-паттерн; показывать только совпадающие строки |
| `werf.io/log-regex-skip` | — | RE2-паттерн; скрывать совпадающие строки |
| `werf.io/log-regex-for-<container>` | — | Per-container RE2-фильтр на показ |
| `werf.io/log-regex-skip-for-<container>` | — | Per-container RE2-фильтр на скрытие |

---

## Шаблонные функции

### `werf_secret_file`

Встроить расшифрованное содержимое файла из директории `secret/`:

```yaml
data:
  config.yaml: {{ werf_secret_file "config.yaml" | b64enc }}
```

Файлы в `secret/` хранятся зашифрованными (AES-128-CBC, ключ из `NELM_SECRET_KEY`). При рендере расшифровываются в памяти. Используйте для сертификатов, приватных ключей и больших конфигов, которые неудобно хранить в `secret-values.yaml`.

### `dump_debug`, `printf_debug`, `include_debug`, `tpl_debug`

Функции отладочного вывода, которые пишут в логи, не влияя на результат рендера. Активируются только при debug log-level.

```yaml
{{ dump_debug $ }}
{{ printf_debug "replicaCount: %d" .Values.replicaCount }}
{{ include_debug "myapp.labels" . | nindent 4 }}
{{ tpl_debug "{{ .Values.template }}" . }}
```

---

## Известные подводные камни

1. **`null`-значения и SSA.** Server-Side Apply часто падает на полях со значением `null`. Если `.Values.foo` равно `nil`, в манифесте появится `foo: null`. Решение: guard-условия `{{ if .Values.foo }}` или использование `default`.

2. **Время выполнения `lookup`.** Если ресурсы кластера изменились между стадиями Plan и Apply, отрендеренный план может быть устаревшим. Избегайте `lookup` для критичной логики — передавайте данные через values.

3. **Недетерминированные функции.** `now`, `randAlphaNum` и итерация по map с непостоянным порядком ключей дают разный результат при каждом вызове. С замораживанием плана это может создавать ложные diff'ы.

4. **`werf.io/deploy-dependency-*` и стадии.** Аннотация не работает через стадии деплоя (pre/main/post). Порядок между стадиями обеспечивается самой последовательностью стадий.

{% endraw %}
