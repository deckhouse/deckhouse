# Node Controller — План реализации для суб-агентов

## Контекст

Существующий node-controller: `modules/040-node-manager/images/node-controller/src/`

Уже реализовано:
- `cmd/main.go` — точка входа с manager.Manager
- `internal/controllers/register.go` — регистрация контроллеров
- `internal/controllers/nodegroup/controller.go` + `reconcilers/status.go` — NodeGroup controller (частично)
- `internal/webhook/` — конвертирующий вебхук NodeGroup (v1/v1alpha1/v1alpha2)
- `api/deckhouse.io/v1,v1alpha1,v1alpha2` — типы NodeGroup
- `internal/dynr/` — динамический reconciler framework

Паттерн: Kubebuilder-style с `SetupWithManager(mgr)`.

---

## Порядок реализации

Контроллеры упорядочены по приоритету (от критичных к вспомогательным) и независимости (можно делать параллельно).

### Волна 1 — Критичные контроллеры (параллельно)

#### Задача 1.1: NodeUpdateReconciler
**Файл:** `internal/controllers/node/update_reconciler.go`
**Исходные хуки:** `hooks/update_approval.go`, `hooks/handle_draining.go`
**Что делать:**
1. Прочитать `hooks/update_approval.go` и `hooks/handle_draining.go` полностью
2. Прочитать `hooks/pkg/` — там может быть shared drain logic
3. Создать `internal/controllers/node/update_reconciler.go` с `NodeUpdateReconciler` struct
4. Реализовать `SetupWithManager`: `For(&corev1.Node{})`, `Watches(&NodeGroup{})`, предикат на `update.node.deckhouse.io/*` аннотации
5. Реализовать `Reconcile`: approval workflow → drain → drained feedback loop
6. Drain логику вынести в `internal/drain/drainer.go` (shared)
7. Зарегистрировать в `internal/controllers/register.go`
8. Написать тесты в `internal/controllers/node/update_reconciler_test.go`

**Зависимости:** Нужен shared drain package.

#### Задача 1.2: NodeTemplateReconciler
**Файл:** `internal/controllers/node/template_reconciler.go`
**Исходный хук:** `hooks/handle_node_templates.go`
**Что делать:**
1. Прочитать `hooks/handle_node_templates.go` полностью
2. Создать `internal/controllers/node/template_reconciler.go`
3. Реализовать `SetupWithManager`: `For(&corev1.Node{})`, `Watches(&NodeGroup{})` с предикатом на nodeTemplate changes
4. Реализовать `Reconcile`: diff labels/annotations/taints между NodeGroup.spec.nodeTemplate и Node, apply merge patch
5. Управление `node-manager.deckhouse.io/last-applied-node-template` annotation
6. Удаление taint `node.deckhouse.io/uninitialized`
7. Тесты

**Зависимости:** Нет.

#### Задача 1.3: NodeGroupReconciler (расширение существующего)
**Файл:** `internal/controllers/nodegroup/controller.go` (уже существует)
**Исходные хуки:** `hooks/update_node_group_status.go`, `hooks/create_master_node_group.go`, `hooks/set_instance_class_ng_usage.go`
**Что делать:**
1. Прочитать все три хука полностью
2. Прочитать существующий `internal/controllers/nodegroup/controller.go` и `reconcilers/status.go`
3. Расширить существующий контроллер: добавить Watches на Node, Instance, MachineDeployment
4. Добавить reconciler для InstanceClass status update
5. Добавить startup логику для создания master NodeGroup (если не существует)
6. Тесты

**Зависимости:** Нет (расширяет существующий код).

#### Задача 1.4: InstanceReconciler
**Файл:** `internal/controllers/instance/reconciler.go`
**Исходный хук:** `hooks/instance_controller.go`
**Что делать:**
1. Прочитать `hooks/instance_controller.go` полностью (самый сложный хук)
2. Прочитать `hooks/internal/` — там может быть shared logic для Instance
3. Создать `internal/controllers/instance/reconciler.go`
4. Реализовать `SetupWithManager`: `For(&Instance{})`, Watches на Node, Machine (MCM), Machine (CAPI), NodeGroup
5. Реализовать lifecycle: создание Instance по Machine/Node, обновление статуса, финалайзеры, каскадное удаление
6. Тесты

**Зависимости:** Нужны API types для Instance (проверить есть ли в `api/`).

---

### Волна 2 — Node контроллеры (параллельно)

#### Задача 2.1: NodeGPUReconciler
**Файл:** `internal/controllers/node/gpu_reconciler.go`
**Исходный хук:** `hooks/gpu_enabled.go`
**Что делать:**
1. Прочитать `hooks/gpu_enabled.go`
2. Создать reconciler: `For(&corev1.Node{})`, Watches на NodeGroup (предикат gpu.sharing != "")
3. Реализовать: патч GPU labels на ноды по NodeGroup.spec.gpu
4. Тесты

#### Задача 2.2: NodeProviderIDReconciler
**Файл:** `internal/controllers/node/provider_id_reconciler.go`
**Исходный хук:** `hooks/set_provider_id_on_static_nodes.go`
**Что делать:**
1. Прочитать хук
2. Создать reconciler: `For(&corev1.Node{})` с предикатом providerID == ""
3. Реализовать: установка providerID на статических нодах
4. Тесты

#### Задача 2.3: BashibleCleanupReconciler
**Файл:** `internal/controllers/node/bashible_cleanup_reconciler.go`
**Исходный хук:** `hooks/remove_bashible_completed_labels_and_taints.go`
**Что делать:**
1. Прочитать хук
2. Создать reconciler: `For(&corev1.Node{})` с предикатом на label bashible-first-run-finished
3. Реализовать: удаление label и taint
4. Тесты

#### Задача 2.4: FencingReconciler
**Файл:** `internal/controllers/node/fencing_reconciler.go`
**Исходный хук:** `hooks/fencing_controller.go`
**Что делать:**
1. Прочитать хук
2. Создать reconciler: `For(&corev1.Node{})` с предикатом на label fencing-enabled
3. Реализовать: проверка Lease → force delete pods → delete Node
4. RequeueAfter: 1m
5. Тесты

#### Задача 2.5: CSITaintReconciler
**Файл:** `internal/controllers/csinode/taint_reconciler.go`
**Исходный хук:** `hooks/remove_csi_taints.go`
**Что делать:**
1. Прочитать хук
2. Создать reconciler: `For(&CSINode{})`
3. Реализовать: при появлении CSINode с драйверами удалить taint csi-not-bootstrapped с Node
4. Тесты

---

### Волна 3 — Вспомогательные контроллеры (параллельно)

#### Задача 3.1: MachineDeploymentReconciler
**Файл:** `internal/controllers/machinedeployment/reconciler.go`
**Исходный хук:** `hooks/set_replicas_on_machine_deployment.go`

#### Задача 3.2: CSRApproverReconciler
**Файл:** `internal/controllers/csr/approver_reconciler.go`
**Исходный хук:** `hooks/kubelet_csr_approver.go`

#### Задача 3.3: CRDWebhookReconciler
**Файл:** `internal/controllers/secret/crd_webhook_reconciler.go`
**Исходные хуки:** `hooks/capi_crds_cabundle_injection.go`, `hooks/sshcredentials_crd_cabundle_injection.go`, `hooks/nodegroup_crd_conversion_webhook.go`

#### Задача 3.4: BashiblePodReconciler
**Файл:** `internal/controllers/pod/bashible_pod_reconciler.go`
**Исходный хук:** `hooks/change_host_ip.go`

#### Задача 3.5: ChaosMonkeyReconciler
**Файл:** `internal/controllers/nodegroup/chaos_monkey_reconciler.go`
**Исходный хук:** `hooks/chaos_monkey.go`

#### Задача 3.6: BashibleLockReconciler
**Файл:** `internal/controllers/deployment/bashible_lock_reconciler.go`
**Исходный хук:** `hooks/lock_bashible_apiserver.go`

#### Задача 3.7: DeckhouseControlPlaneReconciler
**Файл:** `internal/controllers/controlplane/reconciler.go`
**Исходный хук:** `hooks/cluster_api_deckhouse_control_plane.go`

#### Задача 3.8: NodeUserReconciler
**Файл:** `internal/controllers/nodeuser/reconciler.go`
**Исходный хук:** `hooks/clear_nodeuser_errors.go`

#### Задача 3.9: YCPreemptibleReconciler
**Файл:** `internal/controllers/machine/yc_preemptible_reconciler.go`
**Исходный хук:** `hooks/yc_delete_preemptible_instances.go`

---

### Волна 4 — Метрики

#### Задача 4.1: Metric reconcilers
**Файлы:** `internal/controllers/metrics/`
**Исходные хуки:**
- `hooks/machine_deployments_caps_metrics.go`
- `hooks/metrics_node_group_configurations.go`
- `hooks/cntrd_v2_support.go`
- `hooks/minimal_node_os_version.go`
- `hooks/check_unmet_conditions.go`

---

## Шаблон промпта для суб-агента

Каждая задача выполняется одним суб-агентом с промптом по следующему шаблону:

```
Ты реализуешь контроллер [ИМЯ] для node-controller в deckhouse.

Рабочая директория: /Users/pallam/GolandProjects/deckhouse/modules/040-node-manager/images/node-controller/src/

КОНТЕКСТ:
1. Прочитай исходный хук: /Users/pallam/GolandProjects/deckhouse/modules/040-node-manager/hooks/[ИМЯ_ХУКА].go
2. Прочитай тесты хука: /Users/pallam/GolandProjects/deckhouse/modules/040-node-manager/hooks/[ИМЯ_ХУКА]_test.go
   — Тесты хука содержат реальные сценарии и edge-cases. Используй их как спецификацию для переноса логики.
   — Каждый тест-кейс из хука должен быть покрыт аналогичным тестом в новом контроллере.
   — Проанализируй все BeforeEach/It блоки, чтобы не потерять граничные случаи.
3. Прочитай существующую структуру контроллера: internal/controllers/register.go, cmd/main.go
4. Прочитай архитектуру: docs/node-controller-architecture.md (секция N)

ЗАДАЧА:
1. Создай файл [ПУТЬ_К_ФАЙЛУ] с reconciler struct
2. Реализуй SetupWithManager с watches и предикатами из архитектурного документа
3. Реализуй Reconcile, перенеся логику из хука
4. Добавь регистрацию в internal/controllers/register.go
5. Создай тесты в [ПУТЬ_К_ТЕСТАМ], покрывающие ВСЕ сценарии из оригинальных тестов хука

ПРАВИЛА:
- Используй Kubebuilder-style паттерн (SetupWithManager)
- Reconciler struct: embedded client.Client + Scheme
- Все public функции принимают context.Context первым аргументом
- Не добавляй комментарии, которые повторяют код
- Оборачивай ошибки: fmt.Errorf("do something: %w", err)
- Используй SSA (Server-Side Apply) или merge patches
- Следуй code style из /Users/pallam/Documents/Cline/Rules/codestyle.md

ВАЖНО — ТЕСТЫ:
- Тесты оригинальных хуков (*_test.go) — это СПЕЦИФИКАЦИЯ поведения. Прочитай их ПОЛНОСТЬЮ.
- Для каждого test case из хука создай эквивалентный test case с envtest.
- Если тесты хука проверяют edge case — он ОБЯЗАТЕЛЬНО должен быть покрыт в новых тестах.
- Не придумывай новое поведение — переноси существующее 1:1.

ВАЖНО — ФРЕЙМВОРК:
- Если существующий фреймворк (internal/dynr/, текущая структура) мешает реализации — МЕНЯЙ его.
- Ты МОЖЕШЬ и ДОЛЖЕН менять internal/dynr/, internal/controllers/register.go, cmd/main.go
  или любой другой инфраструктурный код, если он не позволяет решить задачу.
- Не подстраивай логику контроллера под ограничения фреймворка — подстраивай фреймворк под контроллер.
```

---

## Граф зависимостей

```
Волна 1 (параллельно):
  1.1 NodeUpdateReconciler ──→ internal/drain/ (shared)
  1.2 NodeTemplateReconciler
  1.3 NodeGroupReconciler (extend)
  1.4 InstanceReconciler ──→ api/ types (Instance, Machine)

Волна 2 (параллельно, после волны 1):
  2.1 NodeGPUReconciler
  2.2 NodeProviderIDReconciler
  2.3 BashibleCleanupReconciler
  2.4 FencingReconciler
  2.5 CSITaintReconciler

Волна 3 (параллельно, после волны 1):
  3.1-3.9 — все независимы

Волна 4 (после волн 1-3):
  4.1 Metric reconcilers
```

Волны 2 и 3 можно запускать параллельно с волной 1, если нет конфликтов по register.go.

---

## Предварительные задачи (до волн)

### Задача 0.1: Подготовка структуры директорий

Код фаз и вспомогательная логика каждого контроллера размещается в поддиректориях пакета контроллера, а не «плоско» в одном файле. Каждый reconciler с несколькими фазами/подзадачами выносит их в отдельные файлы внутри своей директории.

```
internal/
├── controller/
│   ├── node/                              # Primary: v1/Node
│   │   ├── update/                        # NodeUpdateReconciler
│   │   │   ├── reconciler.go              # struct + SetupWithManager + Reconcile
│   │   │   ├── approval.go               # фаза: approval workflow logic
│   │   │   ├── draining.go               # фаза: drain orchestration
│   │   │   └── predicates.go             # предикаты этого контроллера
│   │   │
│   │   ├── template/                      # NodeTemplateReconciler
│   │   │   ├── reconciler.go
│   │   │   ├── diff.go                   # diff labels/annotations/taints
│   │   │   └── predicates.go
│   │   │
│   │   ├── gpu/                           # NodeGPUReconciler
│   │   │   ├── reconciler.go
│   │   │   └── predicates.go
│   │   │
│   │   ├── providerid/                    # NodeProviderIDReconciler
│   │   │   └── reconciler.go
│   │   │
│   │   ├── bashiblecleanup/              # BashibleCleanupReconciler
│   │   │   └── reconciler.go
│   │   │
│   │   └── fencing/                       # FencingReconciler
│   │       ├── reconciler.go
│   │       └── lease.go                  # lease checking logic
│   │
│   ├── nodegroup/                         # Primary: NodeGroup
│   │   ├── status/                        # NodeGroupReconciler — status phase
│   │   │   ├── reconciler.go
│   │   │   └── aggregator.go            # агрегация данных по нодам
│   │   ├── instanceclass/                 # фаза: InstanceClass usage update
│   │   │   └── updater.go
│   │   ├── master/                        # фаза: создание master NodeGroup
│   │   │   └── creator.go
│   │   └── chaosmonkey/                   # ChaosMonkeyReconciler
│   │       ├── reconciler.go
│   │       └── selector.go              # выбор жертвы
│   │
│   ├── instance/                          # Primary: Instance
│   │   ├── reconciler.go
│   │   ├── lifecycle.go                  # создание/удаление Instance
│   │   ├── status.go                     # обновление статуса
│   │   └── finalizer.go                  # управление финалайзерами
│   │
│   ├── machinedeployment/                 # Primary: MachineDeployment
│   │   └── reconciler.go
│   │
│   ├── machine/                           # Primary: Machine
│   │   └── ycpreemptible/
│   │       └── reconciler.go
│   │
│   ├── csinode/                           # Primary: CSINode
│   │   └── reconciler.go
│   │
│   ├── csr/                               # Primary: CertificateSigningRequest
│   │   ├── reconciler.go
│   │   └── validation.go                # проверка CSR (signerName, etc.)
│   │
│   ├── secret/                            # Primary: Secret
│   │   └── crdwebhook/
│   │       ├── reconciler.go
│   │       └── injection.go             # логика патча CRDs
│   │
│   ├── pod/                               # Primary: Pod
│   │   └── bashible/
│   │       └── reconciler.go
│   │
│   ├── deployment/                        # Primary: Deployment
│   │   └── bashiblelock/
│   │       └── reconciler.go
│   │
│   ├── controlplane/                      # Primary: DeckhouseControlPlane
│   │   └── reconciler.go
│   │
│   ├── nodeuser/                          # Primary: NodeUser
│   │   └── reconciler.go
│   │
│   └── metrics/                           # Metric-only reconcilers
│       ├── caps/
│       │   └── reconciler.go
│       ├── nodegroup_configurations/
│       │   └── reconciler.go
│       ├── containerd/
│       │   └── reconciler.go
│       ├── osversion/
│       │   └── reconciler.go
│       └── cloudconditions/
│           └── reconciler.go
│
├── drain/                                 # Shared drain logic
│   └── drainer.go
│
└── predicate/                             # Shared predicates
    └── predicates.go
```

**Правила структуры:**
1. **Папка = primary resource GVK** (`node/`, `nodegroup/`, `instance/` и т.д.)
2. **Поддиректория = контроллер** (`node/update/`, `node/template/`, `node/gpu/`)
3. **Файлы внутри поддиректории = фазы/подзадачи** контроллера (`approval.go`, `draining.go`, `diff.go`)
4. Если контроллер простой (один файл) — поддиректория всё равно создаётся для единообразия и места для будущих тестов
5. `reconciler.go` — всегда точка входа: struct, SetupWithManager, Reconcile
6. `predicates.go` — если контроллер имеет нетривиальные предикаты

### Задача 0.2: API types

Типы группируются по API group в `api/`. Подробная структура — в `docs/node-controller-architecture.md`, секция "Структура API типов".

Что нужно создать (миграция из `hooks/internal/`):

| Источник (hooks/internal/) | Цель (api/) | Тип |
|---|---|---|
| `v1alpha1/instance.go` | `api/deckhouse.io/v1alpha1/instance_types.go` | Instance |
| `v1/node_user.go` | `api/deckhouse.io/v1/nodeuser_types.go` | NodeUser |
| `v1alpha1/ssh_credentials.go` | `api/deckhouse.io/v1alpha1/sshcredentials_types.go` | SSHCredentials |
| — (новый) | `api/deckhouse.io/v1/nodegroupconfiguration_types.go` | NodeGroupConfiguration |
| — (новый) | `api/deckhouse.io/v1alpha1/staticinstance_types.go` | StaticInstance |
| `mcm/v1alpha1/machine.go` | `api/mcm.sapcloud.io/v1alpha1/machine_types.go` | Machine (MCM) |
| `mcm/v1alpha1/machine_deployment.go` | `api/mcm.sapcloud.io/v1alpha1/machinedeployment_types.go` | MachineDeployment (MCM) |
| — (новый) | `api/infrastructure.cluster.x-k8s.io/v1alpha1/deckhousecontrolplane_types.go` | DeckhouseControlPlane |

CAPI типы (`cluster.x-k8s.io/v1beta1` Machine, MachineDeployment) — **не создаются**, импортируются из `sigs.k8s.io/cluster-api/api/v1beta1`. Добавить зависимость в `go.mod`.

Kubernetes core типы (`Node`, `Pod`, `Secret`, `Deployment`, `CSINode`, `CSR`) — **не создаются**, импортируются из `k8s.io/api/*`.

Каждый пакет в `api/` должен содержать:
- `groupversion_info.go` — `SchemeGroupVersion`, `SchemeBuilder`, `AddToScheme`
- `*_types.go` — struct + `+kubebuilder:object:root=true`
- `zz_generated.deepcopy.go` — генерируется через `make generate`

### Задача 0.3: Shared predicates
Создать `internal/predicate/predicates.go` с общими предикатами:
- `hasLabel(obj, key)` → bool
- `inNamespace(ns)` → predicate
- `labelsChanged(e)`, `annotationsChanged(e)`, `taintsChanged(e)` → bool
