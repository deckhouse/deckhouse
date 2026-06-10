# Модель угроз и поверхности атаки модуля `302-vertical-pod-autoscaler`

Документ сформирован по Методике моделирования угроз и поверхности атаки на основании локально предоставленных исходных данных.

---

## 1. Определение целей и критичных функций модуля

| Параметр | Описание |
| -------- | -------- |
| **Наименование модуля** | `302-vertical-pod-autoscaler`, Deckhouse-модуль `vertical-pod-autoscaler` (VPA). Подсистема `infrastructure`, стадия `General Availability` |
| **Назначение** | Автоматический расчёт и установка параметров `resource requests` подов на основе фактического потребления CPU и памяти. Модуль либо выдаёт рекомендации по ресурсам (режим `Off`), либо автоматически корректирует резервирование CPU/памяти контейнеров (режимы `Auto`/`Recreate`/`Initial`/`InPlaceOrRecreate`). Основан на upstream-проекте `kubernetes/autoscaler` (Vertical Pod Autoscaler) |
| **Режим эксплуатации** | Кластерный/облачный. Модуль функционирует внутри Kubernetes-кластера (DKP) как набор управляющих компонентов уровня control plane. Внешние сетевые интерфейсы (публичные порты, формы аутентификации, входящий пользовательский трафик) отсутствуют — взаимодействие осуществляется только через Kubernetes API, admission-механизм и внутренние сервисы мониторинга |
| **Среда исполнения** | DKP/Kubernetes, **namespace `kube-system`** (не выделенный `d8-*` namespace). Контейнерные образы Deckhouse на базе `common/distroless`, запуск от непривилегированного пользователя `nobody` (UID 64535). Зависимости: модули `prometheus` и `prometheus-metrics-adapter` (объявлены в `module.yaml` как требования), Kubernetes API, Metrics API, Prometheus (`aggregating-proxy` в `d8-monitoring`) |
| **Основные функции** | (1) `Recommender` — сбор истории потребления из Prometheus (`--storage=prometheus`, штатный источник; права чтения Metrics API `metrics.k8s.io` предоставлены SA, но флагами запуска не задействованы), расчёт рекомендаций CPU/памяти, запись их в `status` VPA и в чекпойнты; (2) `Updater` — выявление подов с некорректными ресурсами и их вытеснение (Eviction API) либо in-place-изменение (`pods/resize`) для пересоздания контроллером с новыми requests; (3) `Admission-controller` — мутирующий admission-webhook, проставляющий рассчитанные `resource requests` на вновь создаваемые поды; (4) хранение и восстановление состояния через `VerticalPodAutoscalerCheckpoint`; (5) кастомная функция scoped-рекомендаций для DaemonSet по метке узла (локальный патч 004) |
| **Критичные функции** | Корректность расчёта рекомендаций (целостность данных, на которых принимаются решения о ресурсах); корректность мутации подов admission-webhook'ом; контролируемое вытеснение/ресайз подов updater'ом (затрагивает доступность чужих рабочих нагрузок кластера); защита и ротация TLS-материала webhook'а (`vpa-tls-certs`, включая закрытый ключ CA); разграничение доступа ServiceAccount'ов компонентов (широкие кластерные права); целостность цепочки поставки образов (upstream autoscaler + локальные патчи) |
| **Критичные последствия** | Массовое вытеснение/ресайз подов в кластере (нарушение доступности рабочих нагрузок); установка некорректных (заниженных/завышенных) `resource requests`, ведущая к OOM-kill, недоступности или к переводу подов в `Pending` из-за нехватки ресурсов; подмена/искажение рекомендаций через компрометацию источника метрик; компрометация TLS/CA webhook'а и потенциальная подмена ответов admission-webhook'а; эскалация привилегий через широкие RBAC-права компонентов (особенно updater: `pods/eviction`, `pods/resize`); внедрение скомпрометированного кода через цепочку поставки |
| **Объекты защиты** | Ресурсы `VerticalPodAutoscaler` (spec.targetRef, updatePolicy, resourcePolicy, кастомный `spec.scope`) и их `status` с рекомендациями; ресурсы `VerticalPodAutoscalerCheckpoint` (внутреннее состояние гистограмм потребления); Secret `vpa-tls-certs` (caCert/caKey/serverCert/serverKey); `MutatingWebhookConfiguration vpa-webhook-config` и его `caBundle`; ServiceAccount-токены и RBAC-права компонентов; конфигурация компонентов (флаги запуска, ConfigMap CA для kube-rbac-proxy); диагностические метрики; контейнерные образы и сборочные артефакты |
| **Категории субъектов** | Внешний сетевой нарушитель (0) — **прямой сетевой доступ к модулю отсутствует**; пользователь Kubernetes с правом `create` подов в своём namespace (1); пользователь Kubernetes с правом `create/update VerticalPodAutoscaler` в namespace (1); пользователь/администратор с правами на `VerticalPodAutoscalerCheckpoint` (`d8:user-authz:...:user` — чтение, `admin`/`cluster-admin` — запись) (1–2); оператор/администратор DKP, управляющий `ModuleConfig` модуля (2); скомпрометированный под/рабочая нагрузка в кластере с доступом к своему SA-токену (1); внутренний нарушитель, скомпрометировавший компонент модуля или узел `system`/`master` (2–3); скомпрометированная система мониторинга или MITM на внутреннем пути к `aggregating-proxy` (1–2); внешний поставщик/сборочная среда (upstream `kubernetes/autoscaler`, GOPROXY, registry) — ограниченно доверенная внешняя система поставки (2–3); компоненты Deckhouse/Kubernetes API — доверенные компоненты |
| **Реализованные меры защиты, выявленные по исходным данным** | Запуск всех компонентов от непривилегированного пользователя (`helm_lib_module_pod_security_context_run_as_user_nobody`); контейнерный security context `pss_restricted_flexible` (PSS Restricted); базовый образ `common/distroless`; бинарники `chmod 0700`, владелец 64535; раздельные ServiceAccount'ы на компонент; разграничение доступа к метрикам через `kube-rbac-proxy` (порт 4204, авторизация по `SubjectAccessReview` на subresource `deployments/prometheus-metrics`); TLS для admission-webhook'а с автоматической ротацией сертификата (Go-хук: триггеры `OnBeforeHelm`, ежедневный cron `15 10 * * *`, watch Secret `vpa-tls-certs`; перегенерация за 7 дней до истечения); монтирование TLS-секрета только для чтения; `failurePolicy: Ignore` у webhook'а (fail-open — отказ webhook'а не блокирует создание подов в кластере); `objectSelector`, исключающий самопроверку admission-controller; сборка из закреплённого тега upstream + локальные патчи; приоритетные классы для компонентов; `PodDisruptionBudget` (защитный только у admission-controller через `helm_lib_is_ha_to_value`; у recommender/updater `minAvailable: 0` — без защиты); учёт PDB рабочих нагрузок при вытеснении (Eviction API); гард-рейлы вытеснения updater (`--eviction-rate-limit=1`, `--eviction-tolerance=0.1`, `--min-replicas=2`). **Важно:** `automountServiceAccountToken: false`, заданный на объектах SA, переопределён значением `true` в спецификациях подов всех трёх Deployment'ов — токены SA фактически монтируются в поды (recommender использует токен для bearer-аутентификации к Prometheus), поэтому это не является мерой подавления SA-токена |

## 2. Архитектурная модель модуля

**Компоненты модуля и границы доверия:**

| Компонент | Тип | Назначение | Уровень доверия | Граница доверия |
| --------- | --- | ---------- | --------------- | --------------- |
| **Helm/Deckhouse templates модуля** | Внутренний компонент | Формирование в `kube-system`: Deployment'ов 3 компонентов, `Service vpa-webhook`, `MutatingWebhookConfiguration`, Secret `vpa-tls-certs`, RBAC, ServiceAccount, PDB, PodMonitor, VPA-объектов самоуправления | Доверенный субъект при условии контроля релизного артефакта | Да, между релизным артефактом и Kubernetes API |
| **vpa-admission-controller (Deployment)** | Внутренний компонент | Мутирующий admission-webhook: проставляет рассчитанные `resource requests` на новые поды; обслуживает порт `controller`/8000 (через `Service vpa-webhook:443`); метрики на `127.0.0.1:8944`. Запуск с `--register-webhook=false` | Ограниченно доверенный субъект: обрабатывает недоверенные `AdmissionReview` (CREATE pods всех namespace, CREATE/UPDATE VPA) | Да |
| **vpa-recommender (Deployment, replicas:1)** | Внутренний компонент | Расчёт рекомендаций CPU/памяти на основе истории Prometheus (`--storage=prometheus`, `aggregating-proxy.d8-monitoring`; права на Metrics API `metrics.k8s.io` есть, но штатно не используются); запись `status` VPA и чекпойнтов; метрики на `127.0.0.1:8942` | Ограниченно доверенный субъект: принимает данные метрик из внешнего по отношению к модулю источника | Да |
| **vpa-updater (Deployment, replicas:1)** | Внутренний компонент с расширенными правами | Выявление подов с устаревшими ресурсами и их вытеснение (`pods/eviction`) либо in-place-ресайз (`pods/resize`, patch `pods`) кластерно; метрики на `127.0.0.1:8943` | Ограниченно доверенный субъект с высокой потенциальной мощностью воздействия (затрагивает доступность чужих рабочих нагрузок) | Да |
| **kube-rbac-proxy (sidecar ×3)** | Внутренний компонент (функция безопасности) | Ограничение доступа к метрикам компонентов через `SubjectAccessReview` (subresource `deployments/prometheus-metrics`); порт `4204/HTTPS` | Доверенный субъект | Да, между Prometheus и upstream-метриками компонента |
| **Cert-ordering hook (`hooks/order_certificate.go`)** | Внутренний компонент (функция безопасности) | Генерация self-signed CA и серверного сертификата webhook'а, запись в Secret `vpa-tls-certs` и в `caBundle` webhook'а; ротация (cron `15 10 * * *`, перегенерация за 7 дней до истечения) | Доверенный субъект (исполняется в контексте addon-operator Deckhouse) | Да, между значениями модуля и Kubernetes API/Secret |
| **Secret `vpa-tls-certs`** | Внутренний объект данных | Хранение `caCert.pem`, **`caKey.pem` (закрытый ключ CA)**, `serverCert.pem`, `serverKey.pem` | Доверенный объект защиты | Нет внутри namespace, Да при доступе извне SA admission-controller |
| **MutatingWebhookConfiguration `vpa-webhook-config`** | Внутренний объект (функция безопасности) | Перехват CREATE pods (scope `*`, все namespace) и CREATE/UPDATE VPA; `failurePolicy: Ignore`; `caBundle` для проверки сервера webhook'а | Доверенный объект защиты | Да, между Kubernetes API admission и admission-controller |
| **CRD `VerticalPodAutoscaler` / `VerticalPodAutoscalerCheckpoint`** | Внутренние объекты данных | VPA — конфигурация автоскейлинга и `status` с рекомендациями; Checkpoint — внутреннее состояние гистограмм для восстановления recommender | VPA — недоверенный ввод (создаётся пользователями); Checkpoint — ограниченно доверенный | Да |
| **RBAC: ClusterRole/Role + ServiceAccount компонентов** | Внутренние объекты (функция безопасности) | Разграничение прав admission/recommender/updater (в т.ч. `controllers-reader` c wildcard `*/scale`, `actor`, `evictioner`, `in-place`, `checkpoint-actor`, leader-locking leases) | Доверенный объект защиты | Да |
| **Kubernetes API** | Внешняя система | Хранилище и источник pods, nodes, controllers (`*/scale`), VPA, Checkpoint, LimitRange, ConfigMap, Lease, Event; admission и RBAC | Ограниченно доверенная внешняя система | Да |
| **Prometheus / `aggregating-proxy` (`d8-monitoring`)** | Внешняя система (модуль `prometheus`) | Источник истории потребления (`kube_pod_labels`, cAdvisor); запрос по HTTPS с `--prometheus-insecure=true` и bearer-токеном SA | Ограниченно доверенная внешняя система; TLS-верификация отключена | Да |
| **Metrics API (`metrics.k8s.io`, модуль `prometheus-metrics-adapter`)** | Внешняя система | Источник текущих метрик потребления подов | Ограниченно доверенная внешняя система | Да |
| **Узлы (`master`/`system`)** | Внешняя среда исполнения | Размещение подов компонентов; метки узлов используются в scoped-рекомендациях (патч 004) | Ограниченно доверенная среда | Да |
| **Container Registry и source repositories** | Внешние/сборочные системы | Источник OCI-образов (`deckhouse-registry`), исходного кода upstream `kubernetes/autoscaler` (`SOURCE_REPO`), Go-зависимостей (`GOPROXY`) | Ограниченно доверенная система поставки | Да |
| **Prometheus/Grafana (потребители метрик)** | Внешняя система мониторинга | Сбор и отображение метрик компонентов через `kube-rbac-proxy` и PodMonitor | Ограниченно доверенная система | Да |

**Основные интерфейсы и потоки данных:**

| Источник | Получатель | Протокол/формат | Назначение | Доверенность данных |
| -------- | ---------- | --------------- | ---------- | ------------------- |
| Пользователь Kubernetes / контроллер | Kubernetes API admission → `Service vpa-webhook:443` → admission-controller:8000 | HTTPS `AdmissionReview v1` (TLS, `caBundle`) | Мутация `resource requests` при CREATE pod; валидация/мутация при CREATE/UPDATE VPA | **Недоверенные данные** (объект пода/VPA от любого субъекта с правом create) |
| admission-controller | Kubernetes API (ответ admission) | JSON Patch | Изменение spec пода (requests) | Данные, созданные модулем |
| Пользователь Kubernetes | Kubernetes API | YAML/JSON `VerticalPodAutoscaler` | Создание/изменение объекта VPA (targetRef, updatePolicy, resourcePolicy, scope) | **Недоверенные данные** |
| Kubernetes API | recommender / updater / admission-controller | Kubernetes watch/list/get | Получение pods, nodes, controllers (`*/scale`), VPA, LimitRange, ConfigMap, Lease | Ограниченно доверенные данные |
| `aggregating-proxy` (Prometheus) | recommender | HTTPS, `--prometheus-insecure=true`, bearer-токен SA | История потребления (`kube_pod_labels{job=kube-state-metrics}`, cAdvisor `kubelet`) | Ограниченно доверенные данные; верификация TLS отключена |
| Metrics API | recommender | HTTPS (metrics.k8s.io) | Текущее потребление подов | Ограниченно доверенные данные |
| recommender | Kubernetes API | patch `verticalpodautoscalers/status`; `verticalpodautoscalercheckpoints` get/list/watch/create/patch/delete (без verb `update`) | Публикация рекомендаций и чекпойнтов | Данные, созданные модулем (производные от метрик) |
| updater | Kubernetes API | create `pods/eviction`; patch `pods/resize`, `pods` | Вытеснение/ресайз подов рабочих нагрузок | Привилегированное действие модуля |
| recommender/updater | Kubernetes API | get `nodes` (метки), get/list controllers | Источник меток узлов для scoped-рекомендаций; идентификация управляющих контроллеров | Ограниченно доверенные данные |
| recommender/admission/updater (метрики) | kube-rbac-proxy:4204 → Prometheus | HTTPS scrape, авторизация `SubjectAccessReview` | Экспорт метрик компонентов | Ограниченно доверенные данные |
| Cert-ordering hook | Kubernetes API (Secret/values) | Генерация/запись CA+cert, обновление `caBundle` | Управление TLS-материалом webhook'а | Доверенное действие модуля |
| Deckhouse build/update flow | Registry/Kubernetes | OCI images, Helm templates, клон upstream + патчи | Сборка и обновление модуля | Ограниченно доверенные данные поставки |

**Используемые сторонние компоненты и зависимости (с фиксацией версий):**

| Компонент | Версия / источник | Назначение | Замечания безопасности |
| --------- | ----------------- | ---------- | ---------------------- |
| `kubernetes/autoscaler` Vertical Pod Autoscaler | `1.6.0` (`oss.yaml` id `autoscaler`; тег клонирования `vertical-pod-autoscaler-1.6.0`, `images/vertical-pod-autoscaler/werf.inc.yaml`) | Бинарники admission-controller, recommender, updater | Сборка из upstream с локальными патчами (`002`–`004`); точное соответствие тега и фактической upstream-версии требует уточнения |
| Локальные патчи `002`–`004` | `images/vertical-pod-autoscaler/patches/` | `002` — поддержка OpenKruise DaemonSet (`apps.kruise.io/v1alpha1`); `003` — патч checkpoint-агрегации, назначение которого по собственному `patches/README.md` модуля не определено и не действует при `storage=prometheus` (фактически используемом режиме), но патч всё равно применяется при сборке; `004` — scoped-рекомендации DaemonSet по метке узла (`spec.scope`, `status.groups`) | Патчи изменяют логику recommender/admission и схему CRD; подлежат экспертному ревью (SAST) как недоверенный по происхождению код, влияющий на критичную функцию. Патч `003` — приоритетный кандидат на ревью (неопределённое назначение при сохранённом применении) |
| `kube-rbac-proxy` | Общий образ Deckhouse (`helm_lib_module_common_image ... kubeRbacProxy`) | Авторизация доступа к метрикам | Версия задаётся централизованно; требует уточнения |
| Go toolchain | `${GOLANG_VERSION}` (образ `builder/golang-alpine`), `CGO_ENABLED=0` | Сборка бинарников | Версия Go и базового образа управляются централизованно; требует уточнения |
| Базовый runtime-образ | `common/distroless` (final-образы) | Минимальный runtime без shell/пакетного менеджера | Снижает поверхность атаки контейнера; конкретный дайджест требует уточнения |
| Go-зависимости (`go mod vendor`) | Из `GOPROXY` (секрет сборки) | Транзитивные зависимости VPA | Полный SBOM релизных образов в каталоге модуля не обнаружен; оценка known-CVE требует уточнения |
| Модули-зависимости `prometheus`, `prometheus-metrics-adapter` | Объявлены в `module.yaml` (`>= 0.0.0`) | Источники метрик потребления | Целостность рекомендаций зависит от целостности этих модулей |

**Вывод по границам модуля:** модуль не имеет внешних (публичных) сетевых интерфейсов и не принимает входящий пользовательский трафик из сети. Все границы доверия проходят: (а) между недоверенными объектами Kubernetes (поды/VPA пользователей) и admission-webhook'ом; (б) между внешними источниками метрик и recommender; (в) между ServiceAccount'ами компонентов с широкими правами и Kubernetes API; (г) на цепочке поставки образов. Это определяет смещение модели угроз в сторону внутреннего нарушителя и цепочки поставки.

## 3. Анализ поверхности атаки модуля

| Элемент | Компонент | Версия | Функция безопасности | Тип интерфейса | Уровень доступа | Характер взаимодействия | Недоверенные данные |
| ------- | --------- | ------ | -------------------- | -------------- | --------------- | ----------------------- | ------------------- |
| **Admission-webhook endpoint** (`Service vpa-webhook:443/TCP` → admission-controller `controller:8000/TCP`) | vpa-admission-controller | VPA `1.6.0` + патчи | TLS + `caBundle`; `failurePolicy: Ignore`; `objectSelector` (исключение self); `sideEffects: None`; `timeoutSeconds: 30` | Программный API (Kubernetes admission) | Ограниченный: достижим только через Kubernetes API admission | Внутренний (через Kubernetes API) | `AdmissionReview` с объектом Pod (любой namespace, scope `*`) и объектом VPA (CREATE/UPDATE) |
| **`VerticalPodAutoscaler` CR** (`spec.targetRef`, `updatePolicy.updateMode`, `resourcePolicy`, `spec.scope`) | recommender / updater / admission-controller + Kubernetes API | `autoscaling.k8s.io/v1` | RBAC namespace + admission-валидация (CREATE/UPDATE через webhook) | Программный API | Ограниченный RBAC namespace | Внутренний/внешний через Kubernetes API | `targetRef` (любой контроллер/`*/scale`), `updateMode`, min/maxAllowed, `scope` (ключ метки узла) |
| **`VerticalPodAutoscalerCheckpoint` CR** (`status` гистограммы) | recommender (`checkpoint-actor`) + пользователи (`user-authz`) | `autoscaling.k8s.io/v1` | RBAC (`user` — чтение; `admin`/`cluster-admin` — запись) | Программный API | Ограниченный RBAC; запись доступна `Admin`/`ClusterAdmin` | Внутренний | Состояние гистограмм CPU/памяти, используемое для восстановления recommender |
| **Scoped DaemonSet рекомендации** (`spec.scope` + метки узлов) | recommender (локальный патч `004`) | патч `004` | — | Программный API/внутренняя логика | Ограниченный | Внутренний | Значения меток узлов как ключ группировки рекомендаций |
| **Канал recommender → Prometheus** (`https://aggregating-proxy.d8-monitoring.svc.<clusterDomain>`) | vpa-recommender | VPA `1.6.0` | Bearer-токен SA; **`--prometheus-insecure=true` (TLS-верификация отключена)** | Программный HTTPS | Ограниченный (внутрикластерный) | Внешний (по отношению к модулю) | История потребления, `kube_pod_labels`, cAdvisor-метрики |
| **Канал recommender → Metrics API** (`metrics.k8s.io`) | vpa-recommender | VPA `1.6.0` | RBAC `metrics.k8s.io/pods get,list` | Программный API | Ограниченный | Внешний (по отношению к модулю) | Текущие метрики потребления подов |
| **Действия updater: вытеснение/ресайз** (`pods/eviction` create; `pods/resize`, `pods` patch — кластерно) | vpa-updater | VPA `1.6.0` | Учёт PDB при Eviction API; `--eviction-tolerance=0.1`, `--eviction-rate-limit=1`; `--in-place-skip-disruption-budget` (PDB игнорируется для in-place) | Программный API (исходящее привилегированное действие) | Привилегированный сервисный доступ (кластерно) | Внутренний | Решение о вытеснении/ресайзе, производное от рекомендаций (которые производны от недоверенных метрик) |
| **Действия admission-controller: мутация подов** (применение рекомендации + LimitRange) | vpa-admission-controller | VPA `1.6.0` + патч `004` | — | Программный API | Ограниченный | Внутренний | Сгенерированный JSON-patch к спецификации пода |
| **Метрики компонентов** (`127.0.0.1:8942/8943/8944/metrics` → `kube-rbac-proxy:4204/TCP HTTPS`) | kube-rbac-proxy (×3) | общий образ Deckhouse | RBAC-авторизация (`SubjectAccessReview`, subresource `deployments/prometheus-metrics`); `livez`. **Scrape со стороны PodMonitor выполняется с `insecureSkipVerify: true`** (верификация серверного сертификата отключена — второй незащищённый TLS-канал, симметричный recommender→Prometheus) | Программный HTTPS | Ограниченный RBAC (`prometheus`, `d8-monitoring:scraper`) | Внутренний | Метрики с метками namespace/pod/container (топология) |
| **Secret `vpa-tls-certs`** (`caCert.pem`, **`caKey.pem`**, `serverCert.pem`, `serverKey.pem`) | vpa-admission-controller / cert-hook | DKP-модульный | Монтирование `readOnly`; раздельный SA | Секрет/файловый том | Привилегированный | Внутренний | TLS-сертификат, серверный ключ и **закрытый ключ self-signed CA** |
| **`MutatingWebhookConfiguration` (управление)** | ServiceAccount admission-controller | DKP-модульный | RBAC `mutatingwebhookconfigurations: create,delete,get,list` | Kubernetes API | Привилегированный сервисный доступ | Внутренний | Конфигурация webhook'а (несмотря на `--register-webhook=false`, право create/delete предоставлено) |
| **RBAC ServiceAccount'ов компонентов** (`controllers-reader` c wildcard `*/scale` — все 3 SA; `actor` и `vpa-status-reader` — recommender/updater; `evictioner`, `in-place` — updater; `checkpoint-actor` — recommender; чтение узлов — через `actor` и ClusterRole admission-controller, не через `controllers-reader`) | SA admission/recommender/updater | DKP-модульный | Разграничение прав (ClusterRole/Role) | Kubernetes API | Привилегированный сервисный доступ (кластерно) | Внутренний | Кластерные права чтения всех `*/scale`, контроллеров, узлов; вытеснение/ресайз подов; patch VPA/status |
| **Cert-ordering hook** (`hooks/order_certificate.go`) | addon-operator Deckhouse | модульный (Go) | Генерация/ротация CA+cert; триггеры: `OnBeforeHelm` (bootstrap), cron `15 10 * * *`, watch Secret `vpa-tls-certs`; перегенерация за 7 дней до истечения | Локальный процесс/Kubernetes API | Привилегированный (контекст Deckhouse) | Внутренний | Чтение/запись Secret `vpa-tls-certs`, обновление `caBundle` |
| **Leader-election Leases** (`coordination.k8s.io`) | recommender/updater/admission | DKP-модульный | Для recommender/updater доступ ограничен `resourceNames` (vpa-recommender/vpa-updater); **admission-controller имеет неограниченный кластерный доступ к leases** (create/update/get/list/watch, без `resourceNames`); updater дополнительно имеет `vpa-status-reader` (кластерно get/list/watch leases) | Kubernetes API | Ограниченный/привилегированный сервисный доступ | Внутренний | Объекты Lease (управление лидерством) |
| **Build/update flow** (`SOURCE_REPO`, `GOPROXY`, patches, `werf.inc.yaml`, registry) | Build/update flow (werf) | werf-pipeline | — (вне runtime) | Supply chain | Привилегированный сборочный | Внешний/сборочный | Исходный код upstream `kubernetes/autoscaler`, Go-модули, локальные патчи, OCI-образы |
| **`ModuleConfig` модуля** (`nodeSelector`, `tolerations`) | openapi/config-values | DKP-модульный | RBAC `manage` (`edit`/`view` moduleconfig) | Конфигурационный | Привилегированный (администратор) | Внутренний | Параметры размещения подов (низкий риск) |
| **Runtime-окружение контейнеров** (distroless, `nobody`, PSS Restricted) | все компоненты | `common/distroless` | `run_as_user_nobody`, `pss_restricted_flexible`, `chmod 0700` | Система контейнеризации | — | Внутренний | — (реализует функцию безопасности, снижает последствия RCE/escape) |
| **Liveness/Readiness основных контейнеров** | admission-controller / recommender / updater | — | **Отсутствуют** (исключены в `.dmtlint.yaml`); пробы `/livez` определены только у sidecar `kube-rbac-proxy` | Внутренний механизм самовосстановления | — | Внутренний | — (зависший основной процесс не перезапускается автоматически; влияет на доступность — см. AS-03/TM-03) |

**Требует уточнения:**

| Наблюдение | Значение для дальнейшего анализа                                                                                                                                                                                                                                                                                             |
| ---------- |------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Recommender обращается к `aggregating-proxy` с `--prometheus-insecure=true` (TLS-верификация отключена, подтверждено в рантайме), bearer-токен SA в запросе. | При MITM на внутрикластерном пути или подмене сервиса возможна подмена истории метрик → искажение рекомендаций. **Проверено:** в кластере отсутствуют `NetworkPolicy` и Cilium-политики (`cnp`/`ccnp`) в `kube-system` и `d8-monitoring`; модуль их не поставляет — канал не изолирован на сетевом уровне.                   |
| `controllers-reader` содержит wildcard `*/scale` (get/watch/list по всем apiGroups); в `.dmtlint.yaml` для него явно настроено исключение `wildcards`. | Широкая кластерная видимость subresource `scale` всех ресурсов; повышает последствия компрометации SA recommender/updater/admission.                                                                                                                                                                                         |
| Updater имеет кластерные права `pods/eviction` create и `pods/resize`+`pods` patch; флаг `--in-place-skip-disruption-budget`. | Потенциальное массовое вытеснение/ресайз чужих подов; для in-place PDB не учитывается. **Проверено:** 66 VPA-объектов, из них 50 в `InPlaceOrRecreate` — updater активно вытесняет/ресайзит. RBAC updater подтверждён живьём (`pods/eviction=create`, `pods/resize`,`pods`=`patch`). |
| Закрытый ключ CA (`caKey.pem`) хранится в Secret `vpa-tls-certs` (и в helm-values `internal`). | Чтение Secret/values позволяет выпустить валидный для webhook'а сертификат; область влияния ограничена данным self-signed CA webhook'а.                                                                                                                                                                                      |
| `MutatingWebhookConfiguration` перехватывает CREATE pods во всех namespace (`scope: *`); `failurePolicy: Ignore`. | Ошибка/отказ/RCE в admission-controller затрагивает поток создания подов всего кластера до срабатывания fail-open; одновременно fail-open означает «тихий» пропуск мутации при недоступности.                                                                                                                                |
| В каталоге модуля отсутствуют `Dockerfile`/`docker-compose.yml`, SBOM и VEX. | Сборка описана через `werf.inc.yaml`; оценка supply-chain и known-CVE требует получения полного SBOM/VEX релизных образов.                                                                                                                                                                                                   |
| Модуль размещён в `kube-system`, а не в выделенном `d8-*` namespace. | Соседство с высокодоверенными компонентами control plane повышает последствия компрометации и затрудняет сетевую/RBAC-изоляцию.                                                                                                                                                                                              |

## 4. Идентификация угроз

Идентификация выполнена по модели STRIDE с привязкой к перечню БДУ ФСТЭК России (файл `Угрозы.csv`, идентификаторы УБИ.1–УБИ.11). Свойства безопасности: К — конфиденциальность, Ц — целостность, Д — доступность.

| Компонент | Элемент поверхности атаки | STRIDE | Идентификатор БДУ/перечня | Название угрозы | Источник угрозы | Потенциал | Нарушаемые свойства (К/Ц/Д) |
| --------- | ------------------------- | ------ | ----------------- | --------------- | --------------- | --------- | --------------------------- |
| **Admission-controller** | Admission-webhook endpoint | Denial of Service | УБИ.6 | Угроза отказа в обслуживании | Внутренний нарушитель (пользователь с правом create pod/VPA) | Низкий/Средний | Д |
| **Admission-controller** | Admission-webhook endpoint (`failurePolicy: Ignore`) | Tampering | УБИ.3 | Угроза несанкционированной модификации (искажения) | Внутренний нарушитель | Средний | Ц |
| **Admission-controller** | Мутация пода (JSON Patch к requests) | Tampering | УБИ.3 | Угроза несанкционированной модификации (искажения) | Внутренний нарушитель (через подменённую рекомендацию/VPA) | Средний | Ц, Д |
| **Admission-controller** | Чтение pods/configmaps/nodes/limitranges (RBAC) | Information Disclosure | УБИ.1 | Угроза утечки информации | Внутренний нарушитель | Средний | К |
| **Admission-controller** | RBAC `mutatingwebhookconfigurations create/delete` | Elevation of Privilege | УБИ.2 | Угроза несанкционированного доступа | Внутренний нарушитель (при компрометации компонента) | Высокий | Ц, Д |
| **Admission-controller / TLS** | Secret `vpa-tls-certs` (`caKey.pem`) | Spoofing | УБИ.4 | Угроза несанкционированной подмены | Внутренний нарушитель с доступом к Secret | Средний/Высокий | Ц |
| **VPA CR** | `spec` (`updateMode`, `resourcePolicy`, `targetRef`) | Tampering / Misuse | УБИ.3 | Угроза несанкционированной модификации (искажения) | Пользователь Kubernetes namespace | Низкий/Средний | Ц, Д |
| **VPA CR** | Завышенные/заниженные рекомендации → Pending/OOM | Denial of Service | УБИ.6 | Угроза отказа в обслуживании | Пользователь Kubernetes namespace | Низкий/Средний | Д |
| **VPA CR** | `spec.scope` + ресурсоёмкая scoped-агрегация (патч 004) | Denial of Service | УБИ.7 | Угроза ненадлежащего (нецелевого) использования | Пользователь Kubernetes namespace | Низкий/Средний | Д |
| **Checkpoint CR** | Запись `verticalpodautoscalercheckpoints` (`admin`/`cluster-admin`) | Tampering | УБИ.3 | Угроза несанкционированной модификации (искажения) | Внутренний нарушитель (пользователь с правами записи checkpoint) | Средний | Ц, Д |
| **Checkpoint CR** | Удаление checkpoints (`delete`/`deletecollection`) | — (Deletion) | УБИ.5 | Угроза удаления информационных ресурсов | Внутренний нарушитель | Низкий/Средний | Ц, Д |
| **Checkpoint CR** | Чтение checkpoints (`user-authz:user`) | Information Disclosure | УБИ.1 | Угроза утечки информации | Внутренний нарушитель | Низкий | К |
| **Recommender** | Канал к `aggregating-proxy` (`--prometheus-insecure=true`) | Spoofing | УБИ.4 | Угроза несанкционированной подмены | Внутренний нарушитель / скомпрометированный мониторинг (MITM) | Средний | Ц |
| **Recommender** | Поступающая история метрик | Tampering | УБИ.3 | Угроза несанкционированной модификации (искажения) | Внутренний нарушитель / скомпрометированный источник метрик | Средний | Ц, Д |
| **Recommender** | Получение метрик из внешнего источника | — (Supply of data) | УБИ.9 | Угроза получения информационных ресурсов из недоверенного или скомпрометированного источника | Скомпрометированный модуль `prometheus`/`prometheus-metrics-adapter` | Средний | Ц, Д |
| **Recommender** | Обработка состояния кластера (большой объём) | Denial of Service | УБИ.8 | Угроза нарушения функционирования (работоспособности) | Внутренний нарушитель | Низкий/Средний | Д |
| **Recommender** | SA-токен и широкий доступ на чтение | Information Disclosure | УБИ.1 | Угроза утечки информации | Внутренний нарушитель (при компрометации компонента) | Средний | К |
| **Updater** | `pods/eviction` (кластерно) | Denial of Service | УБИ.6 | Угроза отказа в обслуживании | Внутренний нарушитель (компрометация updater / подмена рекомендаций) | Средний/Высокий | Д |
| **Updater** | `pods/resize`, `pods` patch (кластерно) | Tampering | УБИ.3 | Угроза несанкционированной модификации (искажения) | Внутренний нарушитель | Средний | Ц, Д |
| **Updater** | Широкие кластерные права SA | Elevation of Privilege | УБИ.2 | Угроза несанкционированного доступа | Внутренний нарушитель | Высокий | К, Ц, Д |
| **RBAC / SA компонентов** | `controllers-reader` wildcard `*/scale`; `actor` | Elevation of Privilege | УБИ.2 | Угроза несанкционированного доступа | Внутренний нарушитель | Высокий | К, Ц |
| **Метрики компонентов** | `kube-rbac-proxy:4204` + метки топологии | Information Disclosure | УБИ.11 | Угроза несанкционированного массового сбора информации | Внутренний нарушитель | Низкий/Средний | К |
| **Leader-election Leases** | `coordination.k8s.io/leases` | Denial of Service | УБИ.8 | Угроза нарушения функционирования (работоспособности) | Внутренний нарушитель | Низкий | Д |
| **Cert-ordering hook** | Генерация/ротация CA+cert | Spoofing | УБИ.4 | Угроза несанкционированной подмены | Внутренний нарушитель (компрометация контекста хука) | Высокий | Ц |
| **Аудит действий** | Вытеснение/ресайз/patch VPA-status без атрибуции | Repudiation | УБИ.3 | Угроза несанкционированной модификации (искажения) | Внутренний нарушитель | Средний | Ц |
| **Build/update flow** | `SOURCE_REPO`, `GOPROXY`, патчи, registry | Tampering | УБИ.9 | Угроза получения информационных ресурсов из недоверенного или скомпрометированного источника | Внешний поставщик / внутренний нарушитель сборочной среды | Высокий | К, Ц, Д |
| **Build/update flow** | Локальные патчи `002`–`004` | Repudiation / Tampering | УБИ.3 | Угроза несанкционированной модификации (искажения) | Внутренний нарушитель | Средний | Ц |
| **Кластерные ресурсы** | Нецелевое потребление CPU/памяти компонентами | Denial of Service / Misuse | УБИ.7 | Угроза ненадлежащего (нецелевого) использования | Внутренний нарушитель | Низкий | Д |

**Покрытие STRIDE:**

| STRIDE | Покрытые элементы | Вывод |
| ------ | ----------------- | ----- |
| Spoofing | Канал recommender→Prometheus (insecure TLS), CA-ключ webhook'а, cert-hook | Риск связан с подменой источника метрик и возможностью выпуска валидного сертификата webhook'а при доступе к закрытому ключу CA. |
| Tampering | VPA spec, мутация пода, checkpoint, поступающие метрики, `pods/resize`, патчи сборки | Риск связан с искажением рекомендаций и ресурсов подов через входные данные (метрики, VPA, checkpoint) и через цепочку поставки. |
| Repudiation | Действия updater/recommender, изменения checkpoint/ModuleConfig, локальные патчи | Требует уточнения полноты аудита Kubernetes API и сборочного конвейера; атрибуция автоматических действий модуля ограничена. |
| Information Disclosure | Метрики с метками топологии, чтение pods/nodes/configmaps, checkpoint, SA-токен | Риск связан с внутренней структурой кластера и широкими правами чтения компонентов; внешняя утечка маловероятна из-за отсутствия публичных интерфейсов. |
| Denial of Service | Admission-webhook, eviction/resize, recommender, leases, заниженные/завышенные рекомендации | Наиболее значимый класс: модуль по назначению способен вытеснять/ресайзить поды и влиять на их планируемость; ключевой риск — массовое нарушение доступности рабочих нагрузок. |
| Elevation of Privilege | RBAC `*/scale` wildcard, `pods/eviction`/`pods/resize`, управление `MutatingWebhookConfiguration` | Риск связан с преобразованием компрометации компонента (через supply chain или обработку недоверенных данных) в кластерное воздействие за счёт широких сервисных прав. |

**Примечание о покрытии БДУ/STRIDE:** угроза **УБИ.10** (распространение противоправной информации) рассмотрена и признана **неприменимой** к модулю VPA — у модуля отсутствуют интерфейсы исходящей пересылки контента, проксирования или почты; таким образом, трассируемость по перечню УБИ.1–11 является полной с явным исключением УБИ.10. Перечень БДУ (`Угрозы.csv`) не содержит выделенного класса, эквивалентного STRIDE-категории **Repudiation**; в данной модели Repudiation отображается на ближайший класс УБИ.3 (несанкционированная модификация) и носит качественный характер (покрытие Repudiation — частичное, ограничено отсутствием соответствующего класса в перечне).

## 5. Моделирование сценариев атак

**Сценарий AS-01. Массовое вытеснение/ресайз подов через компрометацию updater или подмену рекомендаций**

| Параметр | Значение |
| -------- | -------- |
| **ID сценария** | AS-01 |
| **Связанная угроза** | УБИ.6, УБИ.8, УБИ.3 |
| **Элемент поверхности атаки** | Действия updater: `pods/eviction` (create), `pods/resize`+`pods` (patch) кластерно; флаг `--in-place-skip-disruption-budget` |
| **Источник угрозы** | Внутренний нарушитель: скомпрометированный под updater (через RCE/supply chain) либо нарушитель, влияющий на входные рекомендации |
| **Начальный уровень доступа** | Выполнение кода в контексте updater либо контроль над данными, формирующими рекомендации |
| **Вектор атаки** | Инициирование вытеснения/in-place-ресайза большого числа подов; для in-place PDB не учитывается |
| **Используемая уязвимость** | Широкие кластерные права SA updater (`pods/eviction`, `pods/resize`); размещение в `kube-system`; отсутствие дополнительной изоляции. Имеющиеся гард-рейлы: `--eviction-rate-limit=1`, `--eviction-tolerance=0.1`, `--min-replicas=2` (в режиме `dev` — `--min-replicas=1`, что снимает защиту нагрузок с одной репликой); для in-place PDB не учитывается (`--in-place-skip-disruption-budget`) |
| **Краткая последовательность действий** | 1. Нарушитель получает выполнение кода в updater или подменяет рекомендации/`status` VPA. 2. Updater трактует поды как требующие обновления. 3. Массово создаются eviction/resize-операции. 4. Контроллеры пересоздают поды; для `replicas: 1` и in-place возникает простой. |
| **Последствия** | Нарушение доступности рабочих нагрузок кластера, каскадные перезапуски, возможная недоступность сервисов с одной репликой. |

**Сценарий AS-02. Искажение рекомендаций через подмену истории метрик (insecure TLS к `aggregating-proxy`)**

| Параметр | Значение |
| -------- | -------- |
| **ID сценария** | AS-02 |
| **Связанная угроза** | УБИ.4, УБИ.3, УБИ.9 |
| **Элемент поверхности атаки** | Канал recommender → `https://aggregating-proxy.d8-monitoring` с `--prometheus-insecure=true` |
| **Источник угрозы** | Внутренний нарушитель с позицией для MITM в кластерной сети либо скомпрометированный модуль мониторинга |
| **Начальный уровень доступа** | Сетевой доступ на пути recommender↔`d8-monitoring` или контроль над источником метрик |
| **Вектор атаки** | Подмена ответов Prometheus (TLS-верификация отключена), внедрение ложной истории потребления |
| **Используемая уязвимость** | `--prometheus-insecure=true` (подтверждено в рантайме); проверено отсутствие `NetworkPolicy`/Cilium-политик между `kube-system` и `d8-monitoring` (модуль их не поставляет) — компенсирующая сетевая изоляция отсутствует |
| **Краткая последовательность действий** | 1. Нарушитель занимает позицию MITM или компрометирует источник метрик. 2. Возвращает заниженные/завышенные значения потребления. 3. Recommender вычисляет некорректные рекомендации. 4. Admission/updater применяют их к подам. |
| **Последствия** | Систематическое занижение requests → OOM-kill/деградация; завышение → перевод подов в `Pending` из-за нехватки ресурсов узлов. |

**Сценарий AS-03. Деградация автоскейлинга через отказ admission-controller (fail-open)**

| Параметр | Значение |
| -------- | -------- |
| **ID сценария** | AS-03 |
| **Связанная угроза** | УБИ.6, УБИ.3 |
| **Элемент поверхности атаки** | Admission-webhook endpoint; `MutatingWebhookConfiguration` с `failurePolicy: Ignore` |
| **Источник угрозы** | Внутренний нарушитель (пользователь с правом create pod/VPA), нагрузочное воздействие |
| **Начальный уровень доступа** | Право создания подов/VPA либо возможность влиять на доступность admission-controller |
| **Вектор атаки** | Доведение admission-controller до недоступности/перегрузки; при `failurePolicy: Ignore` мутация молча пропускается |
| **Используемая уязвимость** | Fail-open политика; единственная функция мутации зависит от доступности компонента |
| **Краткая последовательность действий** | 1. Нарушитель перегружает или выводит из строя admission-controller. 2. Webhook отвечает таймаутом/ошибкой. 3. API-server допускает поды без мутации (fail-open). 4. Поды создаются без рекомендованных ресурсов. |
| **Последствия** | Тихий отказ функции VPA (нарушение доступности функции модуля), потенциальное несоответствие фактических ресурсов подов ожидаемым; кластер при этом не блокируется (положительный эффект fail-open). |

**Сценарий AS-04. Подмена ответов admission-webhook через компрометацию закрытого ключа CA**

| Параметр | Значение |
| -------- | -------- |
| **ID сценария** | AS-04 |
| **Связанная угроза** | УБИ.4, УБИ.1, УБИ.2 |
| **Элемент поверхности атаки** | Secret `vpa-tls-certs` (`caKey.pem`, `serverKey.pem`); `caBundle` webhook'а |
| **Источник угрозы** | Внутренний нарушитель с правом чтения Secret в `kube-system` либо доступом к helm-values `internal` |
| **Начальный уровень доступа** | Доступ на чтение Secret `vpa-tls-certs` |
| **Вектор атаки** | Извлечение закрытого ключа CA и выпуск валидного для `caBundle` сертификата; подмена сервиса webhook'а / MITM admission-трафика |
| **Используемая уязвимость** | Хранение закрытого ключа CA вместе с серверным сертификатом в одном Secret и в helm-values |
| **Краткая последовательность действий** | 1. Нарушитель читает `vpa-tls-certs`. 2. Извлекает `caKey.pem`. 3. Выпускает поддельный серверный сертификат. 4. При возможности перехвата admission-трафика подменяет мутацию подов. |
| **Последствия** | Нарушение целостности мутации подов, раскрытие ключевого материала; область влияния ограничена self-signed CA данного webhook'а (не общий кластерный CA). |

**Сценарий AS-05. Эскалация воздействия через широкие RBAC-права компонента**

| Параметр | Значение |
| -------- | -------- |
| **ID сценария** | AS-05 |
| **Связанная угроза** | УБИ.2, УБИ.1 |
| **Элемент поверхности атаки** | SA компонентов: `controllers-reader` (wildcard `*/scale`, все 3 SA); `actor`/`vpa-status-reader` (recommender/updater); `evictioner`/`in-place` (updater); `checkpoint-actor` (recommender); право управления `MutatingWebhookConfiguration` (admission-controller) |
| **Источник угрозы** | Внутренний нарушитель, получивший выполнение кода в компоненте или его SA-токен |
| **Начальный уровень доступа** | Контроль над подом компонента или его ServiceAccount-токеном |
| **Вектор атаки** | Использование кластерных прав чтения всех `*/scale`/контроллеров/узлов, вытеснения/ресайза подов, создания/удаления webhook-конфигураций |
| **Используемая уязвимость** | Размещение в `kube-system`; wildcard RBAC; совмещение прав чтения и активных действий у updater |
| **Краткая последовательность действий** | 1. Нарушитель получает токен/код компонента. 2. Перечисляет ресурсы кластера через `*/scale`. 3. Выполняет вытеснение/ресайз или манипуляции webhook-конфигурацией. 4. Расширяет воздействие на рабочие нагрузки. |
| **Последствия** | Несанкционированный доступ к информации о структуре кластера и воздействие на доступность/целостность чужих рабочих нагрузок. |

**Сценарий AS-06. Отравление/удаление состояния восстановления через `VerticalPodAutoscalerCheckpoint`**

| Параметр | Значение |
| -------- | -------- |
| **ID сценария** | AS-06 |
| **Связанная угроза** | УБИ.3, УБИ.5 |
| **Элемент поверхности атаки** | CRUD `verticalpodautoscalercheckpoints` (recommender `checkpoint-actor`; пользователи `admin`/`cluster-admin`) |
| **Источник угрозы** | Внутренний нарушитель с правами записи/удаления checkpoint |
| **Начальный уровень доступа** | Права `create/update/patch/delete` на checkpoint (роли `admin`/`cluster-admin`) |
| **Вектор атаки** | Запись искажённых гистограмм потребления либо удаление checkpoint перед/после рестарта recommender |
| **Используемая уязвимость** | Checkpoint используется как source-of-truth восстановления; модель доверия опирается на RBAC уровня администратора |
| **Краткая последовательность действий** | 1. Нарушитель записывает ложные гистограммы или удаляет checkpoint. 2. Recommender при рестарте восстанавливает искажённое/пустое состояние. 3. Рекомендации временно некорректны. |
| **Последствия** | Кратковременное искажение рекомендаций, потеря истории потребления; влияние ограничено и самокорректируется по мере накопления новых метрик. |

**Сценарий AS-07. Само-индуцированный отказ рабочей нагрузки через конфигурацию VPA**

| Параметр | Значение |
| -------- | -------- |
| **ID сценария** | AS-07 |
| **Связанная угроза** | УБИ.6, УБИ.7 |
| **Элемент поверхности атаки** | `VerticalPodAutoscaler` CR (`updateMode: Recreate/InPlaceOrRecreate`, `maxAllowed`, `spec.scope`) |
| **Источник угрозы** | Пользователь Kubernetes namespace с правом `create VerticalPodAutoscaler` |
| **Начальный уровень доступа** | Ограниченный RBAC в собственном namespace |
| **Вектор атаки** | Создание VPA с агрессивным `updateMode` и/или завышенным `maxAllowed`; ресурсоёмкая scoped-агрегация (патч 004) |
| **Используемая уязвимость** | Документированные ограничения VPA (рекомендации могут превышать ресурсы → `Pending`; пересоздание подов при изменении requests) |
| **Краткая последовательность действий** | 1. Пользователь создаёт VPA в своём namespace. 2. Updater вытесняет/ресайзит поды нагрузки. 3. Завышенные рекомендации не размещаются на узлах. 4. Поды переходят в `Pending`/перезапускаются. |
| **Последствия** | Нарушение доступности собственной рабочей нагрузки пользователя; межтенантное воздействие ограничено (VPA namespaced, целит контроллеры своего namespace). |

**Сценарий AS-08. Раскрытие топологии кластера через метрики и широкое чтение ресурсов**

| Параметр | Значение |
| -------- | -------- |
| **ID сценария** | AS-08 |
| **Связанная угроза** | УБИ.11, УБИ.1 |
| **Элемент поверхности атаки** | Метрики компонентов (`kube-rbac-proxy:4204`) с метками namespace/pod/container; чтение pods/nodes/configmaps SA recommender/admission |
| **Источник угрозы** | Внутренний нарушитель с доступом к Prometheus/метрикам или к SA-токену компонента |
| **Начальный уровень доступа** | Доступ к авторизованным метрикам либо к компоненту/SA |
| **Вектор атаки** | Массовый сбор меток метрик и данных чтения ресурсов для построения карты рабочих нагрузок и узлов |
| **Используемая уязвимость** | Метрики и права чтения содержат топологические сведения; авторизация метрик ограничивает, но не исключает доступ при компрометации |
| **Краткая последовательность действий** | 1. Нарушитель получает доступ к метрикам/SA. 2. Собирает имена namespace/pod/container и метки узлов. 3. Использует карту для последующих атак. |
| **Последствия** | Раскрытие внутренней структуры кластера; повышение эффективности последующих атак (внешняя утечка маловероятна — публичные интерфейсы отсутствуют). |

**Сценарий AS-09. Компрометация цепочки поставки образов компонентов**

| Параметр | Значение |
| -------- | -------- |
| **ID сценария** | AS-09 |
| **Связанная угроза** | УБИ.9, УБИ.3, УБИ.2 |
| **Элемент поверхности атаки** | `SOURCE_REPO` (upstream `kubernetes/autoscaler`), `GOPROXY`, локальные патчи `002`–`004`, `werf.inc.yaml`, registry |
| **Источник угрозы** | Внешний поставщик/скомпрометированный upstream, внутренний нарушитель сборочной среды |
| **Начальный уровень доступа** | Доступ к исходному коду, зеркалу зависимостей, патчам или registry |
| **Вектор атаки** | Подмена upstream-тега/кода, Go-модуля, локального патча или OCI-образа |
| **Используемая уязвимость** | Отсутствие полного SBOM/VEX релизных образов в каталоге модуля; зависимость от внешних `SOURCE_REPO`/`GOPROXY`; локальные патчи влияют на критичную логику |
| **Краткая последовательность действий** | 1. Нарушитель внедряет изменение в зависимость/патч. 2. Изменение попадает в сборку. 3. Образ публикуется в registry и разворачивается. 4. Код выполняется в контексте recommender/updater/admission с их RBAC. |
| **Последствия** | Полная компрометация компонента и эскалация через его права (см. AS-05); смягчается распространением distroless+`nobody`, но не устраняется. |

## 6. Оценка актуальности и формирование модели угроз

**Оценка актуальности по сценариям:**

| ID угрозы | ID сценария | Наличие уязвимости | Реализуемость сценария | Потенциальный ущерб | Итоговая категория | Решение | Обоснование |
| --------- | ----------- | ------------------ | ---------------------- | ------------------- | ------------------ | ------- | ----------- |
| **УБИ.6, УБИ.8, УБИ.3** | AS-01 | Да | Средняя | Критический | Критический | **Актуальна** | Updater по назначению обладает кластерными правами `pods/eviction` и `pods/resize`; компрометация компонента или подмена рекомендаций ведёт к массовому нарушению доступности рабочих нагрузок. |
| **УБИ.4, УБИ.3, УБИ.9** | AS-02 | Да | Средняя | Высокий | Высокий | **Актуальна** | `--prometheus-insecure=true` отключает TLS-верификацию (подтверждено в рантайме); проверено, что `NetworkPolicy`/Cilium-политики между `kube-system` и `d8-monitoring` отсутствуют и модулем не поставляются — компенсирующая сетевая изоляция отсутствует. |
| **УБИ.6, УБИ.3** | AS-03 | Да | Средняя | Средний | Средний | **Актуальна** | `failurePolicy: Ignore` обеспечивает fail-open: кластер не блокируется, но функция мутации молча отключается при отказе компонента. Ущерб ограничен функцией модуля. |
| **УБИ.4, УБИ.1, УБИ.2** | AS-04 | Да | Низкая/Средняя | Высокий | Высокий | **Условно актуальна** | Требуется доступ на чтение Secret в `kube-system` (привилегированная позиция); закрытый ключ CA хранится вместе с сертификатом. Область влияния — self-signed CA данного webhook'а. |
| **УБИ.2, УБИ.1** | AS-05 | Да | Низкая/Средняя | Критический | Критический | **Актуальна** | Широкие RBAC-права (`*/scale` wildcard, eviction/resize, управление webhook-конфигурацией) и размещение в `kube-system` превращают компрометацию компонента в кластерное воздействие. |
| **УБИ.3, УБИ.5** | AS-06 | Да | Низкая | Низкий/Средний | Средний | **Условно актуальна** | Запись/удаление checkpoint доступны ролям `admin`/`cluster-admin`; воздействие кратковременно и самокорректируется накоплением метрик. |
| **УБИ.6, УБИ.7** | AS-07 | Да | Высокая | Средний | Средний | **Актуальна** | Любой пользователь с правом `create VerticalPodAutoscaler` может вызвать пересоздание/`Pending` собственной нагрузки; межтенантное воздействие ограничено namespaced-природой VPA. |
| **УБИ.11, УБИ.1** | AS-08 | Требует уточнения | Средняя | Средний | Средний | **Условно актуальна** | Метрики и права чтения содержат топологические сведения; авторизация `kube-rbac-proxy` ограничивает доступ; внешняя утечка маловероятна из-за отсутствия публичных интерфейсов. |
| **УБИ.9, УБИ.3, УБИ.2** | AS-09 | Да | Средняя | Критический | Критический | **Актуальна** | Сборка из внешнего upstream + локальные патчи + `GOPROXY`; полный SBOM/VEX релизных образов в каталоге модуля не предоставлен. |

**Итоговая модель актуальных угроз:**

| ID | Угроза | Актуальность | Основные компоненты | Приоритет нейтрализации |
| -- | ------ | ------------ | ------------------- | ----------------------- |
| TM-01 | Массовое вытеснение/ресайз подов updater'ом | Актуальна | vpa-updater, Kubernetes API, рабочие нагрузки кластера | Критический |
| TM-02 | Искажение рекомендаций через подмену источника метрик | Актуальна | vpa-recommender, `aggregating-proxy`, Metrics API | Высокий (сетевая изоляция отсутствует — проверено) |
| TM-03 | Деградация функции автоскейлинга при отказе admission-controller | Актуальна | vpa-admission-controller, `MutatingWebhookConfiguration` | Средний |
| TM-04 | Подмена ответов webhook через компрометацию закрытого ключа CA | Условно актуальна | Secret `vpa-tls-certs`, cert-hook, admission-controller | Высокий |
| TM-05 | Эскалация воздействия через широкие RBAC-права компонента | Актуальна | SA admission/recommender/updater, RBAC | Критический |
| TM-06 | Отравление/удаление состояния восстановления (checkpoint) | Условно актуальна | vpa-recommender, `VerticalPodAutoscalerCheckpoint` | Средний |
| TM-07 | Само-индуцированный отказ рабочей нагрузки через VPA | Актуальна | VPA CR, vpa-updater, vpa-admission-controller | Средний |
| TM-08 | Раскрытие топологии кластера через метрики/чтение ресурсов | Условно актуальна | kube-rbac-proxy, метрики, SA компонентов | Средний |
| TM-09 | Компрометация цепочки поставки образов | Актуальна | werf build, upstream autoscaler, патчи, registry | Критический |

**Меры по нейтрализации:**

| Угроза | Приоритет | Рекомендуемые меры |
| ------ | --------- | ------------------ |
| TM-01 | Критический | Минимизировать область действия updater (контроль `updateMode` эксплуатируемых VPA); мониторить аномальные всплески eviction/resize; рассмотреть лимиты скорости вытеснения помимо `--eviction-rate-limit=1`; пересмотреть `--in-place-skip-disruption-budget`; тестировать поведение при подмене `status` VPA; усилить изоляцию контейнера updater. |
| TM-02 | Высокий (сетевая изоляция отсутствует — проверено) | Добавить `NetworkPolicy`/Cilium-политику или mTLS между `kube-system` и `d8-monitoring`; по возможности отказаться от `--prometheus-insecure=true` в пользу проверяемой CA; контролировать целостность источника метрик; тестировать устойчивость recommender к аномальным метрикам. |
| TM-03 | Средний | Мониторить доступность и латентность admission-controller; обеспечить HA и `PodDisruptionBudget` (защитен только у admission-controller; у recommender/updater `minAvailable: 0`); учесть **отсутствие liveness/readiness-проб у основных контейнеров** (зависший процесс не перезапускается автоматически — пробы исключены в `.dmtlint.yaml`, `/livez` есть только у sidecar `kube-rbac-proxy`); алертить на массовый пропуск мутаций; задокументировать ожидаемое fail-open-поведение для эксплуатации. |
| TM-04 | Высокий | Ограничить и аудировать чтение Secret `vpa-tls-certs` в `kube-system`; рассмотреть раздельное хранение/недоступность закрытого ключа CA; контролировать ротацию (хук уже выполняет перегенерацию); тестировать корректность `caBundle`/SAN. |
| TM-05 | Критический | Сузить RBAC до минимально необходимого, где возможно (ревизия wildcard `*/scale`, совмещения прав у updater); включить аудит использования SA-токенов; рассмотреть вынос модуля из `kube-system` в выделенный namespace; SAST/ревью обработки недоверенных входных данных компонентами. |
| TM-06 | Средний | Ограничить права записи `verticalpodautoscalercheckpoints` до необходимого; аудировать изменения checkpoint; обеспечить корректное восстановление при пустом/искажённом checkpoint. |
| TM-07 | Средний | Документировать ограничения VPA для пользователей; контролировать выдачу прав `create VerticalPodAutoscaler`; рекомендовать `maxAllowed` и `PodDisruptionBudget`; рассмотреть admission-политики на разумность `resourcePolicy`. |
| TM-08 | Средний | Сохранить ограничение доступа к метрикам через `kube-rbac-proxy`; пересмотреть состав меток с высокой кардинальностью/топологией; ограничить доступ к Prometheus и SA-токенам. |
| TM-09 | Критический | Сформировать и хранить полный SBOM/VEX релизных образов; закреплять и проверять хэши/подписи зависимостей и upstream-тега; защищать `SOURCE_REPO`/`GOPROXY`/registry и сборочные секреты; проводить SAST/SCA для локальных патчей `002`–`004`. |

**Компоненты, подлежащие тестированию:**

| Компонент | Виды тестирования | Связанные угрозы | Цель тестирования |
| --------- | ----------------- | ---------------- | ----------------- |
| vpa-updater (логика eviction/resize) | Регрессионные тесты, тесты RBAC, негативные тесты | TM-01, TM-05 | Корректность и ограниченность вытеснения/ресайза, учёт PDB, поведение при некорректном `status` VPA |
| vpa-recommender (обработка метрик) | Fuzzing метрик/истории, модульные тесты | TM-02, TM-06 | Устойчивость к аномальным/подменённым данным Prometheus и checkpoint, корректность scoped-агрегации (патч 004) |
| vpa-admission-controller (webhook) | DAST (admission), регрессионные тесты | TM-03, TM-04, TM-07 | Корректность мутации, поведение при недоступности (fail-open) и при зависании основного процесса (отсутствие liveness-пробы), валидация TLS/`caBundle` |
| Secret `vpa-tls-certs` + cert-hook | Регрессионные тесты, ревью | TM-04 | Корректность генерации/ротации, SAN, недоступность закрытого ключа CA вне модуля |
| RBAC компонентов | Ревью RBAC, тесты прав | TM-05 | Минимизация прав, проверка влияния wildcard `*/scale` |
| kube-rbac-proxy | Регрессионные тесты | TM-08 | Корректность авторизации доступа к метрикам (`SubjectAccessReview`) |
| Build/update flow (`werf.inc.yaml`, патчи, vendor) | SAST/SCA, supply-chain тесты | TM-09 | Целостность зависимостей, ревью локальных патчей, проверка upstream-тега |

**План проверки безопасности:**

| Направление | Проверки |
| ----------- | -------- |
| DAST/penetration testing | Поведение admission-webhook при недоступности (fail-open), некорректных `AdmissionReview`, граничных VPA; проверка авторизации метрик через `kube-rbac-proxy`. |
| Fuzzing | Fuzzing обработки истории метрик и состояния кластера в recommender, парсинга checkpoint, генерации scoped-группировок (патч 004). |
| Code review | Ревью локальных патчей `002`–`004`, логики updater (eviction/resize), cert-hook, RBAC-шаблонов и обработки недоверенных входных данных. |
| Configuration review | Проверка фактических VPA (`updateMode`, `maxAllowed`), `NetworkPolicy`, RBAC-минимизации, `--prometheus-insecure`, размещения в `kube-system`. |
| Supply-chain review | Проверка SBOM/VEX, закрепления upstream-тега и хэшей зависимостей, источников `SOURCE_REPO`/`GOPROXY`, подписей и provenance образов. |

**Контроль полноты модели угроз (ПРИЛОЖЕНИЕ 2 Методики):**

- Покрытие компонентов: рассмотрены все 3 функциональных компонента (admission-controller, recommender, updater), sidecar `kube-rbac-proxy`, cert-hook, CRD VPA/Checkpoint, RBAC/SA, TLS-материал, цепочка поставки.
- Покрытие STRIDE: все 6 категорий покрыты (см. раздел 4); Repudiation покрыт частично (в перечне БДУ нет выделенного класса, отображается на УБИ.3).
- Покрытие перечня БДУ: использованы УБИ.1–УБИ.9, УБИ.11; УБИ.10 явно исключена как неприменимая (нет интерфейсов исходящей пересылки/проксирования контента).
- Трассируемость «угроза → сценарий → решение»: УБИ → AS-01…AS-09 → TM-01…TM-09 → меры нейтрализации.

**Общий вывод:** модуль `302-vertical-pod-autoscaler` **не имеет внешних (публичных) сетевых интерфейсов** и не лежит на внешней сетевой поверхности атаки; угрозы со стороны внешнего сетевого нарушителя признаны неактуальными. Актуальная модель угроз смещена в сторону **внутреннего нарушителя** (компрометация компонента, недоверенные входные данные, широкие RBAC-права) и **цепочки поставки**. Наиболее критичны TM-01, TM-05 и TM-09. Реализованные меры (distroless, запуск от `nobody`, PSS Restricted, `kube-rbac-proxy`, fail-open webhook, ротация TLS, раздельные SA) существенно снижают, но не устраняют остаточный риск, обусловленный широкими кластерными правами компонентов и размещением в `kube-system`.

## Приложение. Термины и сокращения

Термины и сокращения приведены в соответствии с файлом `abbr.md`. Включены определения, релевантные настоящей модели угроз.

**Согласование терминологии уровней доверия:** согласно `abbr.md`, **«Недоверенный субъект»** определяется наличием открытого сетевого доступа и отсутствием механизма аутентификации. Поскольку модуль VPA не имеет внешних сетевых интерфейсов, в строгом смысле «недоверенные субъекты» (с открытым сетевым доступом) у модуля **отсутствуют** — это согласуется с выводом об отсутствии внешней поверхности атаки. Недоверенные входные данные (`spec` объектов `VerticalPodAutoscaler`/`Pod`) поступают через **аутентифицированный Kubernetes API** от ограниченно доверенных субъектов (пользователей кластера). Категория «недоверенные данные» в разделах 2–3 относится к **оси доверия к данным** (доверенными считаются только данные, созданные самим модулем, либо hardcoded), а не к наличию сетевого доступа. Обозначение внутренних компонентов, обрабатывающих недоверенные данные, как «ограниченно доверенных субъектов» соответствует образцу оформления Методики (раздел 2).

| Термин / сокращение | Определение (по `abbr.md`) |
| ------------------- | -------------------------- |
| **Поверхность атаки** | Множество подпрограмм, функций и модулей ПО, обрабатывающих данные, поступающие через интерфейсы, напрямую или косвенно подверженные риску атаки |
| **Источник угрозы** | Субъект (физическое лицо, материальный объект или физическое явление), являющийся непосредственной причиной возникновения угрозы безопасности информации |
| **Уровень доверия** | Характеристика субъекта взаимодействия с модулем, отражающая степень обоснованного доверия к его идентичности, полномочиям и корректности поведения |
| **Доверенный субъект** | Субъект с механизмом аутентификации по защищённому (криптографическому) каналу, являющийся внутренним компонентом моделируемого модуля |
| **Ограниченно доверенный субъект** | Субъект с механизмом аутентификации, без открытого сетевого доступа, с ограниченными возможностями воздействия на модуль, относящийся к поверхности атаки |
| **Недоверенный субъект** | Субъект без механизма аутентификации, с открытым сетевым доступом, позволяющий произвольно формировать входные данные, внешний по отношению к модулю, относящийся к поверхности атаки |
| **Потенциал нарушителя** | Мера усилий, затрачиваемых нарушителем при реализации угроз; различают высокий, средний и низкий потенциалы |
| **Низкий / Средний / Высокий потенциал нарушителя** | Возможности уровня одного человека / группы лиц или организации / предприятия, группы предприятий или государства по разработке и использованию средств эксплуатации уязвимостей |
| **БДУ ФСТЭК России** | Банк данных угроз безопасности информации ФСТЭК России; общедоступный ресурс со сведениями об угрозах, уязвимостях и векторах атак |
| **ФСТЭК России** | Федеральная служба по техническому и экспортному контролю |
| **Уязвимость** | Недостаток ПО, эксплуатация которого может привести к нарушению конфиденциальности, целостности или доступности |
| **НДВ** | Недекларированные возможности — функции ПО, не описанные в документации, пригодные для НСД или нарушения работоспособности |
| **Цепочка поставок ПО** | Совокупность процессов, организаций, инструментов и каналов, посредством которых компоненты ПО поступают от поставщиков к разработчику и пользователю |
| **Сторонний компонент / поставщик** | Зависимость собираемого ПО, получаемая от внешней организации/проекта/лица; подлежит обязательному входному контролю |
| **Базовый образ** | Исходная основа сборки контейнерных образов (ОС, системные библиотеки, инструменты) |
| **OCI-образ** | Бинарный объект с откомпилированным кодом и конфигурацией; способ хранения частей релиза |
| **Registry / Container Registry** | Хранилище `OCI`-образов как единый доверенный источник поставки ПО |
| **Тег** | Фиксированная ссылка на конкретный коммит, определяющая состав кода релизной сборки |
| **Секреты / Компрометация секрета** | Конфиденциальные значения (ключи, токены, пароли); компрометация — событие, при котором секрет стал известен неуполномоченному лицу/системе |
| **SBOM (ППК/ПКК)** | Перечень компонентов программного изделия, включая зависимости, для анализа применимости уязвимостей |
| **VEX** | Документ при `SBOM` со сведениями о выявленных уязвимостях, их применимости и статусе обработки |
| **SAST / SCA / DAST** | Статический анализ исходного кода / композиционный анализ зависимостей / динамическое тестирование работающего приложения |
| **RBAC** | Управление доступом на основе ролей (`Role-Based Access Control`) |
| **CA** | Удостоверяющий центр (`Certificate Authority`) |
| **CGO** | Механизм вызова кода на C из Go и линковки с C-библиотеками (в сборке используется `CGO_ENABLED=0`) |
| **DKP** | Deckhouse Kubernetes Platform |
| **ПО** | Программное обеспечение |

---

при моделировании угроз использовался ИИ агент Claude Code (Anthropic), модель Claude Opus 4.8 (идентификатор `claude-opus-4-8`).
