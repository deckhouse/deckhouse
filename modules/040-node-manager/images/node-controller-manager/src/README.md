# Node Controller

Kubernetes контроллер для управления NodeGroups в Deckhouse.

## Цель

Замена хуков addon-operator на нативный Go-контроллер для:
- Уменьшения нагрузки на API-сервер
- Повышения отзывчивости
- Использования event-driven подхода вместо polling

## Структура проекта

```
node-controller/
├── api/                           # API определения
│   ├── v1/                        # Текущая версия (storage)
│   │   ├── groupversion_info.go
│   │   └── nodegroup_types.go
│   ├── v1alpha1/                  # Старая версия
│   │   ├── groupversion_info.go
│   │   ├── nodegroup_types.go
│   │   └── nodegroup_conversion.go  # Конверсия в v1
│   └── v1alpha2/                  # Старая версия
│       ├── groupversion_info.go
│       ├── nodegroup_types.go
│       └── nodegroup_conversion.go  # Конверсия в v1
├── cmd/
│   └── controller/
│       └── main.go                # Точка входа
├── internal/
│   ├── controller/
│   │   └── nodegroup_controller.go  # Reconciler
│   └── webhook/
│       ├── setup.go
│       ├── nodegroup_validator.go   # Validation webhook
│       └── conversion.go            # Conversion webhook
├── config/
│   ├── rbac/                      # RBAC манифесты
│   ├── manager/                   # Deployment манифесты
│   └── webhook/                   # Webhook конфигурации
├── Dockerfile
├── Makefile
└── go.mod
```

## Версии API

| Версия | Storage | NodeType значения |
|--------|---------|-------------------|
| v1 | ✓ (основная) | CloudEphemeral, CloudPermanent, CloudStatic, Static |
| v1alpha2 | - | Cloud, Static, Hybrid |
| v1alpha1 | - | Cloud, Static, Hybrid |

### Маппинг nodeType при конверсии

```
v1alpha1/v1alpha2  →  v1
─────────────────────────
Cloud              →  CloudEphemeral
Static             →  Static
Hybrid             →  CloudStatic

v1                 →  v1alpha1/v1alpha2
─────────────────────────
CloudEphemeral     →  Cloud
CloudPermanent     →  Hybrid
CloudStatic        →  Hybrid
Static             →  Static
```

## Разработка

### Требования

- Go 1.22+
- Docker
- kubectl
- Доступ к Kubernetes кластеру

### Сборка

```bash
# Локальная сборка
make build

# Docker образ
make docker-build IMG=my-registry/node-controller:latest

# Запустить локально
make run
```

### Тестирование

```bash
make test
make test-verbose
```

### Генерация кода

```bash
# Генерация DeepCopy методов
make generate

# Генерация CRD (если нужно)
make manifests
```

## Миграция хуков

### Текущий статус

Контроллер является заготовкой. Логика из хуков переносится постепенно.

### Места для добавления логики

1. **internal/controller/nodegroup_controller.go** - `reconcileNode()`
   - TODO: MIGRATE FROM HOOK: set_node_labels
   - TODO: MIGRATE FROM HOOK: set_node_annotations
   - TODO: MIGRATE FROM HOOK: handle_node_taints
   - TODO: Add more hook logic

2. **internal/webhook/nodegroup_validator.go** - `validateCreate()`, `validateUpdate()`
   - TODO: MIGRATE VALIDATION FROM PYTHON/BASH WEBHOOKS

### Пример добавления логики из хука

```go
// В reconcileNode()

// ==========================================
// MIGRATED FROM HOOK: set_node_labels
// Original: hooks/set_node_labels.py
// ==========================================
if ng.Spec.NodeTemplate.Labels != nil {
    // Получаем "managed" labels из annotation
    managedLabels := getManagedLabels(nodeCopy)
    
    // Удаляем старые managed labels
    for key := range managedLabels {
        if _, exists := ng.Spec.NodeTemplate.Labels[key]; !exists {
            delete(nodeCopy.Labels, key)
            needsUpdate = true
        }
    }
    
    // Добавляем/обновляем labels из NodeGroup
    for key, value := range ng.Spec.NodeTemplate.Labels {
        if nodeCopy.Labels[key] != value {
            nodeCopy.Labels[key] = value
            needsUpdate = true
        }
    }
    
    // Обновляем annotation с managed labels
    setManagedLabels(nodeCopy, ng.Spec.NodeTemplate.Labels)
}
```

## Деплой

### В Deckhouse

Контроллер деплоится как часть модуля `node-manager`:

1. Добавить в `modules/040-node-manager/templates/`:
   - `deployment.yaml` (из config/manager/)
   - `rbac.yaml` (из config/rbac/)
   - `webhook-configuration.yaml` (из config/webhook/)

2. Добавить флаг для постепенной миграции:
   ```yaml
   # values.yaml
   nodeManager:
     useNewController: false
   ```

3. Отключать хуки при `useNewController: true`

### Ручной деплой (для тестирования)

```bash
# Применить RBAC
kubectl apply -f config/rbac/

# Применить Deployment
kubectl apply -f config/manager/

# Применить Webhooks
kubectl apply -f config/webhook/
```

## Метрики

Контроллер экспортирует стандартные метрики controller-runtime:

```
# Reconcile операции
controller_runtime_reconcile_total{controller="nodegroup",result="success"}
controller_runtime_reconcile_total{controller="nodegroup",result="error"}

# Время reconcile
controller_runtime_reconcile_time_seconds_bucket{controller="nodegroup"}

# Размер очереди
workqueue_depth{name="nodegroup"}
workqueue_adds_total{name="nodegroup"}
```

## Webhook сертификаты

Для работы webhooks требуются TLS сертификаты. Варианты:

1. **cert-manager** - автоматическая генерация
2. **Deckhouse webhook handler** - использует существующую инфраструктуру
3. **Ручная генерация** - для тестирования

## Отладка

```bash
# Логи контроллера
kubectl logs -n d8-cloud-instance-manager -l app=node-controller -f

# Увеличить verbosity
./controller --zap-log-level=debug

# Отключить webhooks для отладки
./controller --enable-webhooks=false
```
