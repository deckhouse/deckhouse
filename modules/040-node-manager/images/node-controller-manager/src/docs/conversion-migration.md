# Миграция Conversion и Validation Hooks из Python/Bash в Go

## Обзор

Этот документ описывает как хуки из `modules/040-node-manager/hooks/` были переписаны на Go:

1. **node_group.py** — Conversion Webhook (Python)
2. **node_group** — Validation Webhook (Bash)

## Часть 1: Conversion Hooks (node_group.py)

## Сравнение: Python Hook vs Go Conversion

### Python Hook (shell-operator)

```python
# node_group.py
kubernetesCustomResourceConversion:
  - name: alpha1_to_alpha2
    crdName: nodegroups.deckhouse.io
    conversions:
    - fromVersion: deckhouse.io/v1alpha1
      toVersion: deckhouse.io/v1alpha2
```

**Как работает:**
1. Shell-operator регистрирует Conversion Webhook в API Server
2. API Server вызывает webhook через HTTP при конверсии
3. Python код обрабатывает объект и возвращает результат

**Проблемы:**
- HTTP вызов на каждую конверсию
- Отдельный процесс для обработки
- Парсинг JSON/YAML
- Нет типизации

### Go Conversion (controller-runtime)

```go
// api/deckhouse.io/v1alpha1/nodegroup_conversion.go
func (src *NodeGroup) ConvertTo(dstRaw conversion.Hub) error {
    dst := dstRaw.(*v1.NodeGroup)
    // ... конверсия
    return nil
}
```

**Как работает:**
1. Webhook сервер встроен в контроллер
2. API Server вызывает webhook через HTTP
3. Go код обрабатывает типизированный объект

**Преимущества:**
- Типизация на этапе компиляции
- Один процесс (контроллер)
- Нет парсинга JSON (используется схема)
- Тестируемость

## Маппинг функций

| Python функция | Go функция | Файл |
|----------------|------------|------|
| `alpha1_to_alpha2()` | `ConvertTo()` + `Convert_v1alpha1_*` | `v1alpha1/nodegroup_conversion.go`, `v1alpha1/conversion.go` |
| `alpha2_to_alpha1()` | `ConvertFrom()` + `Convert_v1_*` | `v1alpha1/nodegroup_conversion.go`, `v1alpha1/conversion.go` |
| `alpha2_to_v1()` | `ConvertTo()` | `v1alpha2/nodegroup_conversion.go` |
| `v1_to_alpha2()` | `ConvertFrom()` | `v1alpha2/nodegroup_conversion.go` |

## Детальное сравнение логики

### 1. alpha1_to_alpha2: docker → cri.docker

**Python:**
```python
def alpha1_to_alpha2(self, o):
    obj.apiVersion = "deckhouse.io/v1alpha2"
    if "docker" in obj.spec:
        if "cri" not in obj.spec:
            obj.spec.cri = {}
        obj.spec.cri.docker = obj.spec.docker
        del obj.spec.docker
    if "kubernetesVersion" in obj.spec:
        del obj.spec.kubernetesVersion
    if "static" in obj:
        del obj.static
```

**Go (v1alpha1/conversion.go):**
```go
func Convert_v1alpha1_NodeGroupSpec_To_v1_NodeGroupSpec(in *NodeGroupSpec, out *v1.NodeGroupSpec, s conversion.Scope) error {
    // Docker field in v1alpha1 maps to CRI.Docker in v1
    if in.Docker != nil {
        if out.CRI == nil {
            out.CRI = &v1.CRISpec{}
        }
        if out.CRI.Type == "" {
            out.CRI.Type = v1.CRITypeDocker
        }
        out.CRI.Docker = &v1.DockerSpec{
            MaxConcurrentDownloads: in.Docker.MaxConcurrentDownloads,
            Manage:                 in.Docker.Manage,
        }
    }
    // kubernetesVersion и static просто не конвертируются (теряются)
    return nil
}
```

### 2. alpha2_to_v1: nodeType маппинг + cluster config

**Python:**
```python
def alpha2_to_v1(self, o):
    obj.apiVersion = "deckhouse.io/v1"
    
    # Читаем cluster config из Secret
    provider_config = get_from_secret("d8-provider-cluster-configuration")
    
    ng_name = obj.metadata.name
    ng_type = obj.spec.nodeType
    
    if ng_type == "Cloud":
        ng_type = "CloudEphemeral"
    elif ng_type == "Hybrid":
        # Определяем CloudPermanent vs CloudStatic
        found_in_permanent = False
        if ng_name == "master":
            found_in_permanent = True
        else:
            for ng in provider_config["nodeGroups"]:
                if ng["name"] == ng_name:
                    found_in_permanent = True
                    break
        ng_type = "CloudPermanent" if found_in_permanent else "CloudStatic"
    
    obj.spec.nodeType = ng_type
```

**Go (v1alpha2/nodegroup_conversion.go):**
```go
func (src *NodeGroup) ConvertTo(dstRaw conversion.Hub) error {
    dst := dstRaw.(*v1.NodeGroup)
    
    switch src.Spec.NodeType {
    case NodeTypeCloud:
        dst.Spec.NodeType = v1.NodeTypeCloudEphemeral
    case NodeTypeStatic:
        dst.Spec.NodeType = v1.NodeTypeStatic
    case NodeTypeHybrid:
        // Default to CloudStatic
        // CloudPermanent detection requires cluster config lookup
        // which is handled by ConversionWebhook with cluster config access
        dst.Spec.NodeType = v1.NodeTypeCloudStatic
    }
    
    return nil
}
```

### 3. Проблема: Hybrid → CloudPermanent требует cluster config

**В Python хуке:**
```python
# Хук имеет доступ к Secret через snapshot
includeSnapshotsFrom: ["cluster_config"]

# В коде:
provider_config = base64.decode(self._snapshots["cluster_config"][0]["filterResult"])
```

**В Go есть два варианта:**

**Вариант A: Validating Webhook с доступом к cluster config**
```go
// api/deckhouse.io/v1alpha2/nodegroup_webhook.go

type NodeGroupWebhook struct {
    Client client.Client
}

func (w *NodeGroupWebhook) ConvertTo(src *NodeGroup, dst *v1.NodeGroup) error {
    if src.Spec.NodeType == NodeTypeHybrid {
        // Читаем Secret
        secret := &corev1.Secret{}
        w.Client.Get(ctx, types.NamespacedName{
            Namespace: "kube-system",
            Name:      "d8-provider-cluster-configuration",
        }, secret)
        
        providerConfig := parseConfig(secret.Data["cloud-provider-cluster-configuration.yaml"])
        
        if isCloudPermanent(src.Name, providerConfig) {
            dst.Spec.NodeType = v1.NodeTypeCloudPermanent
        } else {
            dst.Spec.NodeType = v1.NodeTypeCloudStatic
        }
    }
    return nil
}
```

**Вариант B: Использовать annotation для передачи информации**
```yaml
# При создании NodeGroup, внешний компонент добавляет annotation
metadata:
  annotations:
    node.deckhouse.io/permanent-node-group: "true"
```

```go
func (src *NodeGroup) ConvertTo(dstRaw conversion.Hub) error {
    if src.Spec.NodeType == NodeTypeHybrid {
        if src.Annotations["node.deckhouse.io/permanent-node-group"] == "true" {
            dst.Spec.NodeType = v1.NodeTypeCloudPermanent
        } else {
            dst.Spec.NodeType = v1.NodeTypeCloudStatic
        }
    }
    return nil
}
```

### 4. v1_to_alpha2: Обратный маппинг

**Python:**
```python
def v1_to_alpha2(self, o):
    obj.apiVersion = "deckhouse.io/v1alpha2"
    
    ng_type = obj.spec.nodeType
    if ng_type == "CloudEphemeral":
        ng_type = "Cloud"
    elif ng_type in ["CloudPermanent", "CloudStatic"]:
        ng_type = "Hybrid"
    
    obj.spec.nodeType = ng_type
```

**Go (v1alpha2/nodegroup_conversion.go):**
```go
func (dst *NodeGroup) ConvertFrom(srcRaw conversion.Hub) error {
    src := srcRaw.(*v1.NodeGroup)
    
    switch src.Spec.NodeType {
    case v1.NodeTypeCloudEphemeral:
        dst.Spec.NodeType = NodeTypeCloud
    case v1.NodeTypeStatic:
        dst.Spec.NodeType = NodeTypeStatic
    case v1.NodeTypeCloudPermanent, v1.NodeTypeCloudStatic:
        dst.Spec.NodeType = NodeTypeHybrid
    }
    
    return nil
}
```

## Цепочка конверсий

### Python (shell-operator)

```
v1alpha1 → v1alpha2 → v1 (hub)
    │          │
    └── alpha1_to_alpha2()
               └── alpha2_to_v1()
```

Shell-operator реализует **прямые конверсии** между соседними версиями.

### Go (controller-runtime)

```
v1alpha1 ─────────────────► v1 (hub)
              ConvertTo()

v1alpha2 ─────────────────► v1 (hub)
              ConvertTo()
```

Controller-runtime реализует **hub-and-spoke**: каждая версия конвертируется напрямую в hub.

## Потерянные поля при конверсии

### v1alpha1 → v1 (потеряно)

| Поле | Причина |
|------|---------|
| `spec.kubernetesVersion` | Deprecated, управляется кластером |
| `spec.static.internalNetworkCIDRs` | Нет эквивалента в v1 |

### v1 → v1alpha1 (потеряно)

| Поле | Причина |
|------|---------|
| `spec.gpu` | Новое в v1 |
| `spec.fencing` | Новое в v1 |
| `spec.update` | Новое в v1 |
| `spec.staticInstances` | Новое в v1 |
| `spec.cri.containerdV2` | Новое в v1 (downgrade to containerd) |
| `spec.cri.notManaged` | Новое в v1 |
| `spec.kubelet.resourceReservation` | Новое в v1 |
| `spec.kubelet.topologyManager` | Новое в v1 |

## Тестирование

```go
// api/deckhouse.io/v1alpha1/conversion_test.go

func TestConvertTo(t *testing.T) {
    src := &v1alpha1.NodeGroup{
        Spec: v1alpha1.NodeGroupSpec{
            NodeType: v1alpha1.NodeTypeCloud,
            Docker: &v1alpha1.DockerSpec{
                MaxConcurrentDownloads: ptr.To(5),
            },
        },
    }
    
    dst := &v1.NodeGroup{}
    err := src.ConvertTo(dst)
    
    assert.NoError(t, err)
    assert.Equal(t, v1.NodeTypeCloudEphemeral, dst.Spec.NodeType)
    assert.Equal(t, v1.CRITypeDocker, dst.Spec.CRI.Type)
    assert.Equal(t, 5, *dst.Spec.CRI.Docker.MaxConcurrentDownloads)
}
```

## Итог

| Аспект | Python Hook | Go Conversion |
|--------|-------------|---------------|
| Типизация | Нет | Да |
| Тестируемость | Сложно | Просто |
| Производительность | HTTP + JSON | Прямой вызов |
| Доступ к cluster config | Через snapshots | Через webhook с client |
| Сложность | Низкая | Средняя |

## Часть 2: Validation Webhook (node_group bash hook)

### Исходный хук (Bash)

```bash
# modules/040-node-manager/hooks/node_group

kubernetesValidating:
- name: nodegroup-policy.deckhouse.io
  group: main
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["nodegroups"]
```

### Что читает хук

| Snapshot | Ресурс | Что извлекает |
|----------|--------|---------------|
| `endpoints` | Endpoints/kubernetes | Количество API server endpoints |
| `cluster_config` | Secret/d8-cluster-configuration | defaultCRI, clusterPrefixLen, clusterType, podSubnetNodeCIDRPrefix |
| `provider_cluster_config` | Secret/d8-provider-cluster-configuration | Доступные зоны |
| `deckhouse_config` | ModuleConfig/global | customTolerationKeys |
| `nodes_with_containerd_custom_conf` | Nodes с label `containerd-config=custom` | Ноды с кастомным containerd |
| `nodes_without_containerd_support` | Nodes с label `containerd-v2-unsupported` | Ноды без поддержки containerd v2 |

### Все проверки

| # | Проверка | Когда | Go реализация |
|---|----------|-------|---------------|
| 1 | `clusterPrefix + ngName <= 42` | CREATE, Cloud | `NodeGroupValidator.Handle()` |
| 2 | `maxPerZone >= minPerZone` | CREATE/UPDATE | `NodeGroupValidator.Handle()` |
| 3 | `maxPods` vs subnet size | CREATE/UPDATE | Warning в `NodeGroupValidator` |
| 4 | Zone exists in provider | CREATE/UPDATE | `loadProviderClusterConfig()` |
| 5 | `cri.type != Docker` | CREATE/UPDATE | `NodeGroupValidator.Handle()` |
| 6 | CRI change on master < 3 endpoints | UPDATE | Warning, `getKubernetesEndpointsCount()` |
| 7 | Taints in customTolerationKeys | CREATE/UPDATE | `loadCustomTolerationKeys()` |
| 8 | RollingUpdate only for CloudEphemeral | CREATE/UPDATE | `NodeGroupValidator.Handle()` |
| 9 | `nodeType` is immutable | UPDATE | `NodeGroupValidator.Handle()` |
| 10 | No duplicate taints | CREATE/UPDATE | `NodeGroupValidator.Handle()` |
| 11 | topologyManager requires resourceReservation | CREATE/UPDATE | `NodeGroupValidator.Handle()` |
| 12 | CRI change blocked by custom containerd | UPDATE | `getNodesWithCustomContainerd()` |
| 13 | ContainerdV2 blocked by unsupported nodes | UPDATE | `getNodesWithoutContainerdV2Support()` |
| 14 | memorySwap requires cgroup v2 | UPDATE | `getNodesWithoutContainerdV2Support()` |

### Go реализация

```go
// internal/webhook/validator.go

type NodeGroupValidator struct {
    Client  client.Client
    decoder *admission.Decoder
}

func (v *NodeGroupValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
    // Загружаем cluster config
    clusterConfig, _ := v.loadClusterConfig(ctx)
    providerConfig, _ := v.loadProviderClusterConfig(ctx)
    
    // Проверка 1: prefix + name <= 42
    if req.Operation == "CREATE" && clusterConfig.ClusterType == "Cloud" {
        if 63-clusterConfig.ClusterPrefixLen-1-len(ng.Name)-21 < 0 {
            return admission.Denied("...")
        }
    }
    
    // Проверка 2: maxPerZone >= minPerZone
    if ng.Spec.CloudInstances != nil {
        if ng.Spec.CloudInstances.MaxPerZone < ng.Spec.CloudInstances.MinPerZone {
            return admission.Denied("...")
        }
    }
    
    // ... остальные проверки
    
    return admission.Allowed("")
}
```

### Регистрация webhook

```go
// cmd/main.go

mgr.GetWebhookServer().Register("/validate-nodegroup-policy", &webhook.Admission{
    Handler: &validator.NodeGroupValidator{
        Client: mgr.GetClient(),
    },
})
```

### Сравнение: Bash vs Go

| Аспект | Bash Hook | Go Validator |
|--------|-----------|--------------|
| Типизация | Нет (JSON/jq) | Да (Go structs) |
| Доступ к кластеру | Через snapshots | Через client.Client |
| Ошибки | Runtime | Compile-time |
| Тестируемость | Сложно | Unit tests |
| Производительность | Новый процесс | В памяти |

## Файлы проекта

```
node-controller/
├── api/deckhouse.io/
│   ├── v1/
│   │   ├── nodegroup_webhook.go      # Simple defaulting/validation
│   │   └── nodegroup_conversion.go   # Hub() marker
│   ├── v1alpha1/
│   │   ├── nodegroup_conversion.go   # ConvertTo/ConvertFrom
│   │   └── conversion.go             # docker → cri.docker
│   └── v1alpha2/
│       └── nodegroup_conversion.go   # ConvertTo/ConvertFrom
│
├── internal/
│   ├── webhook/
│   │   └── validator.go              # Custom validator with cluster access
│   ├── controller/
│   │   └── nodegroup_controller.go   # Reconciliation logic
│   └── ...
│
└── cmd/
    └── main.go                       # Registers both webhooks
```

## Webhook конфигурация

```yaml
# config/webhook/webhook-configuration.yaml

apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: node-controller-validating
webhooks:
  # Simple validation (from nodegroup_webhook.go)
  - name: vnodegroup.deckhouse.io
    rules:
      - apiGroups: ["deckhouse.io"]
        resources: ["nodegroups"]
        operations: ["CREATE", "UPDATE", "DELETE"]
    clientConfig:
      service:
        path: /validate-deckhouse-io-v1-nodegroup

  # Policy validation (from validator.go) 
  - name: nodegroup-policy.deckhouse.io
    rules:
      - apiGroups: ["deckhouse.io"]
        resources: ["nodegroups"]
        operations: ["CREATE", "UPDATE"]
    clientConfig:
      service:
        path: /validate-nodegroup-policy
```
