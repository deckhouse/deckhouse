# Node Controller Architecture

Архитектура переноса хуков из modules/040-node-manager/hooks в node-controller на базе controller-runtime.
Все контроллеры работают внутри одного manager.Manager (один бинарник, один pod, shared informer cache).

## Анализ связности хуков

Из 29 хуков без Values проведён анализ data dependencies. Результат:

- **update_approval ↔ handle_draining** — единственная пара с реальной data dependency (approval ставит `draining` annotation → draining выполняет drain → ставит `drained` → approval продолжает).
- **handle_node_templates** — патчит labels/annotations/taints из NodeGroup.spec.nodeTemplate. Не зависит от update_approval и handle_draining. Работает с отдельным набором аннотаций (`node-manager.deckhouse.io/last-applied-node-template`).
- **gpu_enabled** — патчит 3 уникальных GPU-label (`node.deckhouse.io/gpu`, `node.deckhouse.io/device-gpu.config`, `nvidia.com/mig.config`). Не пересекается с другими хуками.
- **set_provider_id_on_static_nodes** — патчит `spec.providerID`. Полностью независим.
- **remove_csi_taints** — удаляет taint `node.deckhouse.io/csi-not-bootstrapped`. Триггерится от CSINode, не от Node events. Независим.
- **remove_bashible_completed_labels_and_taints** — удаляет label `node.deckhouse.io/bashible-first-run-finished` и taint `node.deckhouse.io/bashible-uninitialized`. Независим.
- **change_host_ip** — патчит **Pod** (не Node!). Работает с подами bashible-apiserver. Не должен быть в Node-контроллере.

## Общая схема

```
manager.Manager
├── NodeUpdateReconciler           (primary: Node) — approval + drain workflow
├── NodeTemplateReconciler         (primary: Node) — sync labels/annotations/taints from NodeGroup
├── NodeGPUReconciler              (primary: Node) — GPU labels
├── NodeProviderIDReconciler       (primary: Node) — providerID for static nodes
├── CSITaintReconciler             (primary: CSINode) — remove CSI taints
├── BashibleCleanupReconciler      (primary: Node) — remove bashible labels/taints
├── BashiblePodReconciler          (primary: Pod) — host IP change detection
├── NodeGroupReconciler            (primary: NodeGroup) — status + InstanceClass usage
├── InstanceReconciler             (primary: Instance) — instance lifecycle
├── MachineDeploymentReconciler    (primary: MachineDeployment) — replicas sync
├── CSRApproverReconciler          (primary: CSR) — kubelet CSR approval
├── CRDWebhookReconciler           (primary: Secret) — CA bundle injection
├── DeckhouseControlPlaneReconciler (primary: DeckhouseControlPlane)
├── FencingReconciler              (primary: Node, periodic)
├── ChaosMonkeyReconciler          (primary: NodeGroup, periodic)
├── NodeUserReconciler             (primary: NodeUser, periodic)
├── YCPreemptibleReconciler        (primary: Machine, periodic)
└── BashibleLockReconciler         (primary: Deployment)
```

---

## Структура пакетов

Контроллеры группируются по primary resource в папки. Код фаз и вспомогательная логика размещается в поддиректориях контроллера, а не «плоско».

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
4. Если контроллер простой (один файл) — поддиректория всё равно создаётся для единообразия
5. `reconciler.go` — всегда точка входа: struct, SetupWithManager, Reconcile
6. `predicates.go` — если контроллер имеет нетривиальные предикаты

---

## Структура API типов

Типы объектов группируются по API group и версии. Deckhouse-специфичные типы живут в `api/`, внешние (MCM, CAPI) — импортируются из зависимостей или определяются локально.

```
api/
├── deckhouse.io/
│   ├── v1/                            # Стабильные типы
│   │   ├── groupversion_info.go
│   │   ├── nodegroup_types.go         # NodeGroup (уже есть)
│   │   ├── nodegroup_conversion.go
│   │   ├── nodeuser_types.go          # NodeUser
│   │   ├── nodegroupconfiguration_types.go  # NodeGroupConfiguration
│   │   └── zz_generated.deepcopy.go
│   │
│   ├── v1alpha1/                      # Alpha типы
│   │   ├── groupversion_info.go
│   │   ├── nodegroup_types.go         # NodeGroup v1alpha1 (уже есть)
│   │   ├── instance_types.go          # Instance
│   │   ├── sshcredentials_types.go    # SSHCredentials
│   │   ├── staticinstance_types.go    # StaticInstance
│   │   └── zz_generated.deepcopy.go
│   │
│   └── v1alpha2/                      # (уже есть для NodeGroup)
│       ├── nodegroup_types.go
│       └── zz_generated.deepcopy.go
│
├── mcm.sapcloud.io/
│   └── v1alpha1/                      # MCM (Machine Controller Manager) типы
│       ├── groupversion_info.go
│       ├── machine_types.go           # Machine (MCM)
│       └── machinedeployment_types.go # MachineDeployment (MCM)
│
├── infrastructure.cluster.x-k8s.io/
│   └── v1alpha1/
│       ├── groupversion_info.go
│       └── deckhousecontrolplane_types.go  # DeckhouseControlPlane
│
└── README.md                          # Описание подхода к типам
```

**Правила размещения типов:**

1. **Deckhouse CRDs** (`deckhouse.io/*`) — определяются в `api/deckhouse.io/`. Это полные типы с DeepCopy, используемые для `scheme.AddToScheme()`.

2. **CAPI типы** (`cluster.x-k8s.io/v1beta1`) — **импортируются** из зависимости `sigs.k8s.io/cluster-api`. Machine, MachineDeployment, Cluster — из `sigs.k8s.io/cluster-api/api/v1beta1`. Не дублируются локально.

3. **MCM типы** (`machine.sapcloud.io/v1alpha1`) — определяются локально в `api/mcm.sapcloud.io/v1alpha1/`, т.к. MCM не имеет публичного Go-модуля с типами. Это легковесные структуры с минимумом полей (только те, что нужны контроллерам).

4. **Инфраструктурные типы** (`infrastructure.cluster.x-k8s.io`) — `DeckhouseControlPlane` определяется локально, т.к. это deckhouse-specific CRD.

5. **Kubernetes core типы** (`v1/Node`, `v1/Pod`, `v1/Secret`, `apps/v1/Deployment`, `certificates.k8s.io/v1/CSR`, `storage.k8s.io/v1/CSINode`) — **импортируются** из `k8s.io/api/*`. Не определяются локально.

**Миграция существующих типов:**

Сейчас типы для хуков определены в `hooks/internal/`:
- `hooks/internal/v1alpha1/instance.go` → переносится в `api/deckhouse.io/v1alpha1/instance_types.go`
- `hooks/internal/v1/node_user.go` → переносится в `api/deckhouse.io/v1/nodeuser_types.go`
- `hooks/internal/mcm/v1alpha1/machine.go` → переносится в `api/mcm.sapcloud.io/v1alpha1/machine_types.go`
- `hooks/internal/mcm/v1alpha1/machine_deployment.go` → переносится в `api/mcm.sapcloud.io/v1alpha1/machinedeployment_types.go`
- `hooks/internal/capi/v1beta1/machine_types.go` → **не переносится**, используется `sigs.k8s.io/cluster-api/api/v1beta1`
- `hooks/internal/v1alpha1/ssh_credentials.go` → переносится в `api/deckhouse.io/v1alpha1/sshcredentials_types.go`

**Регистрация схемы в `cmd/main.go`:**

```go
scheme := runtime.NewScheme()
_ = corev1.AddToScheme(scheme)
_ = appsv1.AddToScheme(scheme)
_ = storagev1.AddToScheme(scheme)
_ = certificatesv1.AddToScheme(scheme)
_ = apiextensionsv1.AddToScheme(scheme)
_ = deckhousev1.AddToScheme(scheme)
_ = deckhousev1alpha1.AddToScheme(scheme)
_ = deckhousev1alpha2.AddToScheme(scheme)
_ = mcmv1alpha1.AddToScheme(scheme)
_ = infrastructurev1alpha1.AddToScheme(scheme)
_ = clusterv1beta1.AddToScheme(scheme)  // sigs.k8s.io/cluster-api
```

---

## 1. NodeUpdateReconciler

**Primary resource:** `v1/Node`

**Объединяет хуки:**
- `update_approval.go` — workflow одобрения обновлений
- `handle_draining.go` — drain workflow нод

**Обоснование объединения:** Единственная пара с data dependency: approval ставит `update.node.deckhouse.io/draining` → draining выполняет drain, ставит `update.node.deckhouse.io/drained` → approval продолжает workflow. В одном reconciler это один проход без лишних циклов.

### Watches

```go
ctrl.NewControllerManagedBy(mgr).
    For(&corev1.Node{}, builder.WithPredicates(nodeUpdatePredicate)).
    Watches(&deckhousev1.NodeGroup{}, handler.EnqueueRequestsFromMapFunc(nodeGroupToNodes)).
    Complete(r)
```

### Предикаты

```go
nodeUpdatePredicate = predicate.Funcs{
    CreateFunc: func(e event.CreateEvent) bool {
        return hasLabel(e.Object, "node.deckhouse.io/group")
    },
    UpdateFunc: func(e event.UpdateEvent) bool {
        if !hasLabel(e.ObjectNew, "node.deckhouse.io/group") { return false }
        return updateAnnotationsChanged(e)
    },
    DeleteFunc: func(e event.DeleteEvent) bool { return false },
}

func updateAnnotationsChanged(e event.UpdateEvent) bool {
    prefix := "update.node.deckhouse.io/"
    oldAnn := e.ObjectOld.GetAnnotations()
    newAnn := e.ObjectNew.GetAnnotations()
    // Сравниваем только update.node.deckhouse.io/* аннотации
    for _, key := range []string{
        prefix + "waiting-for-approval",
        prefix + "approved",
        prefix + "draining",
        prefix + "drained",
        prefix + "disruption-required",
        prefix + "disruption-approved",
    } {
        if oldAnn[key] != newAnn[key] { return true }
    }
    return false
}
```

### Ключевые аннотации

| Аннотация | Роль |
|---|---|
| `update.node.deckhouse.io/waiting-for-approval` | Нода ждёт одобрения |
| `update.node.deckhouse.io/approved` | Обновление одобрено |
| `update.node.deckhouse.io/draining` | Approval → Draining запускает drain |
| `update.node.deckhouse.io/drained` | Draining → Approval, drain завершён |
| `update.node.deckhouse.io/disruption-required` | Требуется disruption |
| `update.node.deckhouse.io/disruption-approved` | Одобрен disruptive update |

---

## 2. NodeTemplateReconciler

**Primary resource:** `v1/Node`

**Объединяет хуки:** `handle_node_templates.go`

Синхронизирует labels, annotations, taints из `NodeGroup.spec.nodeTemplate` на Node. Удаляет taint `node.deckhouse.io/uninitialized`. Управляет `node-manager.deckhouse.io/last-applied-node-template` annotation.

### Watches

```go
ctrl.NewControllerManagedBy(mgr).
    For(&corev1.Node{}, builder.WithPredicates(nodeTemplatePredicate)).
    Watches(&deckhousev1.NodeGroup{}, handler.EnqueueRequestsFromMapFunc(nodeGroupToNodes),
        builder.WithPredicates(nodeGroupTemplatePredicate)).
    Complete(r)
```

### Предикаты

```go
nodeTemplatePredicate = predicate.Funcs{
    CreateFunc: func(e event.CreateEvent) bool {
        return hasLabel(e.Object, "node.deckhouse.io/group")
    },
    UpdateFunc: func(e event.UpdateEvent) bool {
        if !hasLabel(e.ObjectNew, "node.deckhouse.io/group") { return false }
        return labelsChanged(e) || annotationsChanged(e) || taintsChanged(e)
    },
    DeleteFunc: func(e event.DeleteEvent) bool { return false },
}

nodeGroupTemplatePredicate = predicate.Funcs{
    UpdateFunc: func(e event.UpdateEvent) bool {
        // Реконсилим только при изменении spec.nodeTemplate
        return nodeTemplateSpecChanged(e)
    },
}
```

---

## 3. NodeGPUReconciler

**Primary resource:** `v1/Node`

**Объединяет хуки:** `gpu_enabled.go`

Патчит GPU-специфичные labels: `node.deckhouse.io/gpu`, `node.deckhouse.io/device-gpu.config`, `nvidia.com/mig.config`.

### Watches

```go
ctrl.NewControllerManagedBy(mgr).
    For(&corev1.Node{}, builder.WithPredicates(gpuNodePredicate)).
    Watches(&deckhousev1.NodeGroup{}, handler.EnqueueRequestsFromMapFunc(nodeGroupToNodes),
        builder.WithPredicates(gpuNodeGroupPredicate)).
    Complete(r)
```

### Предикаты

```go
gpuNodePredicate = predicate.Funcs{
    CreateFunc: func(e event.CreateEvent) bool {
        return hasLabel(e.Object, "node.deckhouse.io/group")
    },
    UpdateFunc: func(e event.UpdateEvent) bool {
        if !hasLabel(e.ObjectNew, "node.deckhouse.io/group") { return false }
        return gpuLabelsChanged(e) // node.deckhouse.io/gpu, device-gpu.config, nvidia.com/mig.config
    },
    DeleteFunc: func(e event.DeleteEvent) bool { return false },
}

gpuNodeGroupPredicate = predicate.NewPredicateFuncs(func(obj client.Object) bool {
    ng := obj.(*deckhousev1.NodeGroup)
    return ng.Spec.GPU.Sharing != ""
})
```

---

## 4. NodeProviderIDReconciler

**Primary resource:** `v1/Node`

**Объединяет хуки:** `set_provider_id_on_static_nodes.go`

Устанавливает `spec.providerID` на статических нодах (NodeType == Static), если оно пустое.

### Watches

```go
ctrl.NewControllerManagedBy(mgr).
    For(&corev1.Node{}, builder.WithPredicates(providerIDPredicate)).
    Complete(r)
```

### Предикаты

```go
providerIDPredicate = predicate.Funcs{
    CreateFunc: func(e event.CreateEvent) bool {
        node := e.Object.(*corev1.Node)
        return hasLabel(node, "node.deckhouse.io/group") && node.Spec.ProviderID == ""
    },
    UpdateFunc: func(e event.UpdateEvent) bool {
        node := e.ObjectNew.(*corev1.Node)
        return hasLabel(node, "node.deckhouse.io/group") && node.Spec.ProviderID == ""
    },
    DeleteFunc: func(e event.DeleteEvent) bool { return false },
}
```

---

## 5. CSITaintReconciler

**Primary resource:** `storage.k8s.io/v1/CSINode`

**Объединяет хуки:** `remove_csi_taints.go`

Удаляет taint `node.deckhouse.io/csi-not-bootstrapped` с Node, когда CSINode появляется с драйверами.

### Watches

```go
ctrl.NewControllerManagedBy(mgr).
    For(&storagev1.CSINode{}).
    Complete(r)
```

### Reconcile

```go
func (r *CSITaintReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    csiNode := &storagev1.CSINode{}
    if err := r.Get(ctx, req.NamespacedName, csiNode); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    if len(csiNode.Spec.Drivers) == 0 { return ctrl.Result{}, nil }
    node := &corev1.Node{}
    if err := r.Get(ctx, req.NamespacedName, node); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    // Удалить taint node.deckhouse.io/csi-not-bootstrapped если есть
    return ctrl.Result{}, nil
}
```

---

## 6. BashibleCleanupReconciler

**Primary resource:** `v1/Node`

**Объединяет хуки:** `remove_bashible_completed_labels_and_taints.go`

Удаляет label `node.deckhouse.io/bashible-first-run-finished` и taint `node.deckhouse.io/bashible-uninitialized`.

### Watches

```go
ctrl.NewControllerManagedBy(mgr).
    For(&corev1.Node{}, builder.WithPredicates(bashibleCleanupPredicate)).
    Complete(r)
```

### Предикаты

```go
bashibleCleanupPredicate = predicate.Funcs{
    CreateFunc: func(e event.CreateEvent) bool {
        return hasLabel(e.Object, "node.deckhouse.io/bashible-first-run-finished")
    },
    UpdateFunc: func(e event.UpdateEvent) bool {
        return hasLabel(e.ObjectNew, "node.deckhouse.io/bashible-first-run-finished")
    },
    DeleteFunc: func(e event.DeleteEvent) bool { return false },
}
```

---

## 7. BashiblePodReconciler

**Primary resource:** `v1/Pod`

**Объединяет хуки:** `change_host_ip.go`

Отслеживает поды bashible-apiserver. Если HostIP изменился — удаляет под для рестарта.

### Watches

```go
ctrl.NewControllerManagedBy(mgr).
    For(&corev1.Pod{}, builder.WithPredicates(bashiblePodPredicate)).
    Complete(r)
```

### Предикаты

```go
bashiblePodPredicate = predicate.NewPredicateFuncs(func(obj client.Object) bool {
    return obj.GetNamespace() == "d8-cloud-instance-manager" &&
        obj.GetLabels()["app"] == "bashible-apiserver"
})
```

---

## 8. NodeGroupReconciler

**Primary resource:** `deckhouse.io/v1/NodeGroup`

**Объединяет хуки:**
- `update_node_group_status.go` — вычисление и патч статуса NodeGroup
- `create_master_node_group.go` — создание master NodeGroup при startup
- `set_instance_class_ng_usage.go` — патч статуса InstanceClass

### Watches

```go
ctrl.NewControllerManagedBy(mgr).
    For(&deckhousev1.NodeGroup{}).
    Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(nodeToNodeGroup),
        builder.WithPredicates(nodeForNGStatusPredicate)).
    Watches(&deckhousev1alpha1.Instance{}, handler.EnqueueRequestsFromMapFunc(instanceToNodeGroup)).
    Watches(&machineV1alpha1.MachineDeployment{}, handler.EnqueueRequestsFromMapFunc(mdToNodeGroup),
        builder.WithPredicates(inNamespace("d8-cloud-instance-manager"))).
    Complete(r)
```

### Предикаты

```go
nodeForNGStatusPredicate = predicate.Funcs{
    CreateFunc:  func(e event.CreateEvent) bool { return hasNGLabel(e.Object) },
    DeleteFunc:  func(e event.DeleteEvent) bool { return hasNGLabel(e.Object) },
    UpdateFunc:  func(e event.UpdateEvent) bool {
        return hasNGLabel(e.ObjectNew) && (nodeConditionsChanged(e) || nodeReadyChanged(e))
    },
}
```

---

## 9. InstanceReconciler

**Primary resource:** `deckhouse.io/v1alpha1/Instance`

**Объединяет хуки:** `instance_controller.go`

### Watches

```go
ctrl.NewControllerManagedBy(mgr).
    For(&deckhousev1alpha1.Instance{}).
    Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(nodeToInstance)).
    Watches(&machineV1alpha1.Machine{}, handler.EnqueueRequestsFromMapFunc(machineToInstance),
        builder.WithPredicates(inNamespace("d8-cloud-instance-manager"))).
    Watches(&capiV1beta1.Machine{}, handler.EnqueueRequestsFromMapFunc(capiMachineToInstance),
        builder.WithPredicates(inNamespace("d8-cloud-instance-manager"))).
    Watches(&deckhousev1.NodeGroup{}, handler.EnqueueRequestsFromMapFunc(ngToInstances)).
    Complete(r)
```

---

## 10. MachineDeploymentReconciler

**Primary resource:** `machine.sapcloud.io/v1alpha1/MachineDeployment`

**Объединяет хуки:** `set_replicas_on_machine_deployment.go`

### Watches

```go
ctrl.NewControllerManagedBy(mgr).
    For(&machineV1alpha1.MachineDeployment{},
        builder.WithPredicates(inNamespace("d8-cloud-instance-manager"))).
    Watches(&deckhousev1.NodeGroup{}, handler.EnqueueRequestsFromMapFunc(ngToMachineDeployments)).
    Complete(r)
```

---

## 11. CSRApproverReconciler

**Primary resource:** `certificates.k8s.io/v1/CertificateSigningRequest`

**Объединяет хуки:** `kubelet_csr_approver.go`

### Предикаты

```go
csrPredicate = predicate.Funcs{
    CreateFunc: func(e event.CreateEvent) bool {
        csr := e.Object.(*certificatesv1.CertificateSigningRequest)
        return !isApproved(csr) && isKubeletCSR(csr)
    },
    UpdateFunc: func(e event.UpdateEvent) bool {
        csr := e.ObjectNew.(*certificatesv1.CertificateSigningRequest)
        return !isApproved(csr) && isKubeletCSR(csr)
    },
    DeleteFunc: func(e event.DeleteEvent) bool { return false },
}
```

---

## 12. CRDWebhookReconciler

**Primary resource:** `v1/Secret` (webhook TLS secrets)

**Объединяет хуки:**
- `capi_crds_cabundle_injection.go`
- `sshcredentials_crd_cabundle_injection.go`
- `nodegroup_crd_conversion_webhook.go`

### Предикаты

```go
webhookSecretPredicate = predicate.NewPredicateFuncs(func(obj client.Object) bool {
    if obj.GetNamespace() != "d8-cloud-instance-manager" { return false }
    name := obj.GetName()
    return name == "capi-webhook-tls" ||
        name == "caps-controller-manager-webhook-tls" ||
        name == "node-controller-webhook-tls"
})
```

**Маппинг Secret → CRDs:**
- `capi-webhook-tls` → clusters, machines, machinesets, machinedeployments, machinepools, machinehealthchecks, machinedrainrules, extensionconfigs
- `caps-controller-manager-webhook-tls` → sshcredentials.deckhouse.io
- `node-controller-webhook-tls` → nodegroups.deckhouse.io

---

## 13. DeckhouseControlPlaneReconciler

**Primary:** `infrastructure.cluster.x-k8s.io/v1alpha1/DeckhouseControlPlane`

**Хук:** `cluster_api_deckhouse_control_plane.go`

---

## 14. FencingReconciler

**Primary:** `v1/Node` (label `node-manager.deckhouse.io/fencing-enabled`)

**Хук:** `fencing_controller.go`

Skip если аннотации: `disruption-approved`, `approved`, `fencing-disable`. Проверяет Lease, если `time.Since(lease.RenewTime) > 60s` → force delete pods + delete Node.

`RequeueAfter: 1 * time.Minute`

---

## 15. ChaosMonkeyReconciler

**Primary:** `deckhouse.io/v1/NodeGroup`

**Хук:** `chaos_monkey.go`

Предикат: `ng.Spec.Chaos.Mode == "DrainAndDelete"`. Выбирает случайную Machine, удаляет.

`RequeueAfter: ng.Spec.Chaos.Period` (default 6h)

---

## 16. NodeUserReconciler

**Primary:** `deckhouse.io/v1/NodeUser`

**Хук:** `clear_nodeuser_errors.go`

Очищает ошибки в статусе старше 30 минут. `RequeueAfter: 30 * time.Minute`

---

## 17. YCPreemptibleReconciler

**Primary:** `machine.sapcloud.io/v1alpha1/Machine` (label `node.deckhouse.io/preemptible`)

**Хук:** `yc_delete_preemptible_instances.go`

`RequeueAfter: 1 * time.Minute`

---

## 18. BashibleLockReconciler

**Primary:** `apps/v1/Deployment` (name `bashible-apiserver`, ns `d8-cloud-instance-manager`)

**Хук:** `lock_bashible_apiserver.go`

---

## Метрики (5 отдельных reconcilers)

| Хук | Watch | Данные |
|---|---|---|
| `machine_deployments_caps_metrics.go` | MachineDeployment (label `app=caps-controller`) | replicas/ready/unavailable/phase |
| `metrics_node_group_configurations.go` | NodeGroupConfiguration | count per NG |
| `cntrd_v2_support.go` | Node (label `node.deckhouse.io/group`) | containerd/cgroup + requirements |
| `minimal_node_os_version.go` | Node (label `node.deckhouse.io/group`) | min OS version → requirements |
| `check_unmet_conditions.go` | ConfigMap `d8-cloud-provider-conditions` | cloud conditions → requirements |

---

## Хуки НЕ переносимые (обновляют Values)

23 хука обновляют `nodeManager.internal.*` Values и остаются в deckhouse-controller:

control_plane_arguments, convert_static_cluster_configuration, create_rbac_and_certificate_for_kubernetes_api_proxy, deployment_required, discover_apiserver_endpoints, discover_cloud_provider, discover_kubernetes_ca, discover_packages_proxy_addresses, discover_standby_ng, enable_cluster_api_cloud_and_static, gen_bashible_apiserver_certs, generate_capi_webhook_certs, generate_caps_webhook_certs, generate_node_webhook_certs, get_crds, get_packages_proxy_token, machineclass_checksum_assign, machineclass_checksum_collect, mig_custom_config_name, order_bootstrap_token, set_instance_prefix, set_ng_priorities, upmeter_discovery.

---

## Сводная таблица

| # | Controller | Primary GVK | Hooks | Watches (secondary) | Periodic |
|---|---|---|---|---|---|
| 1 | NodeUpdateReconciler | v1/Node | update_approval, handle_draining | NodeGroup | нет |
| 2 | NodeTemplateReconciler | v1/Node | handle_node_templates | NodeGroup | нет |
| 3 | NodeGPUReconciler | v1/Node | gpu_enabled | NodeGroup | нет |
| 4 | NodeProviderIDReconciler | v1/Node | set_provider_id_on_static_nodes | — | нет |
| 5 | CSITaintReconciler | CSINode | remove_csi_taints | — | нет |
| 6 | BashibleCleanupReconciler | v1/Node | remove_bashible_completed_labels_and_taints | — | нет |
| 7 | BashiblePodReconciler | v1/Pod | change_host_ip | — | нет |
| 8 | NodeGroupReconciler | NodeGroup | update_node_group_status, create_master_node_group, set_instance_class_ng_usage | Node, Instance, MD | нет |
| 9 | InstanceReconciler | Instance | instance_controller | Node, Machine, NodeGroup | нет |
| 10 | MachineDeploymentReconciler | MachineDeployment | set_replicas_on_machine_deployment | NodeGroup | нет |
| 11 | CSRApproverReconciler | CSR | kubelet_csr_approver | — | нет |
| 12 | CRDWebhookReconciler | Secret | 3x CA bundle injection | Service | нет |
| 13 | DeckhouseControlPlaneReconciler | DeckhouseControlPlane | cluster_api_deckhouse_control_plane | — | нет |
| 14 | FencingReconciler | v1/Node | fencing_controller | Lease | 1m |
| 15 | ChaosMonkeyReconciler | NodeGroup | chaos_monkey | Machine, Node | period |
| 16 | NodeUserReconciler | NodeUser | clear_nodeuser_errors | — | 30m |
| 17 | YCPreemptibleReconciler | Machine | yc_delete_preemptible_instances | — | 1m |
| 18 | BashibleLockReconciler | Deployment | lock_bashible_apiserver | Secret | нет |
| — | Metrics (5 reconcilers) | various | 5 metric hooks | — | нет |

**Итого:** 18 контроллеров + 5 metric reconcilers = 29 хуков перенесено.

---

## Shared Informer Cache

Все контроллеры делят один кэш через Manager. Ресурсы, используемые несколькими контроллерами:
- **Node:** NodeUpdateReconciler, NodeTemplateReconciler, NodeGPUReconciler, NodeProviderIDReconciler, BashibleCleanupReconciler, FencingReconciler, NodeGroupReconciler, metrics
- **NodeGroup:** NodeUpdateReconciler, NodeTemplateReconciler, NodeGPUReconciler, NodeGroupReconciler, InstanceReconciler, ChaosMonkeyReconciler, MachineDeploymentReconciler
- **Machine:** InstanceReconciler, ChaosMonkeyReconciler, YCPreemptibleReconciler
- **Secret:** CRDWebhookReconciler, BashibleLockReconciler

Один informer на GVR — нет дублирования.

---

## Почему не один большой NodeReconciler?

Хуки, работающие с Node, **не связаны между собой** (за исключением пары approval/draining):

| Хук | Патчит на Node | Зависит от |
|---|---|---|
| handle_node_templates | labels, annotations, taints из NodeGroup.spec.nodeTemplate | NodeGroup.spec.nodeTemplate |
| gpu_enabled | 3 GPU-label | NodeGroup.spec.gpu |
| set_provider_id_on_static_nodes | spec.providerID | — |
| remove_csi_taints | taint csi-not-bootstrapped | CSINode |
| remove_bashible_completed_labels_and_taints | label bashible-first-run-finished, taint bashible-uninitialized | — |
| update_approval + handle_draining | update.node.deckhouse.io/* annotations | друг от друга |

Каждый работает с **уникальным набором полей**. Нет конфликтов при параллельном выполнении (при использовании SSA или merge patches на разные поля). Разделение даёт:
- Более точные предикаты (меньше лишних reconcile)
- Изоляция ошибок
- Независимое тестирование
- Возможность настройки MaxConcurrentReconciles per controller
