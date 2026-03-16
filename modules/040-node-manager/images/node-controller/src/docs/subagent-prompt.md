# Промпт для запуска реализации node-controller

Вставь этот промпт в новый чат с Cline для запуска реализации.

---

```
Ты выполняешь перенос хуков из deckhouse module 040-node-manager в отдельный node-controller на базе controller-runtime.

## Документы

Прочитай эти документы ПОЛНОСТЬЮ перед началом работы:
1. Архитектура: modules/040-node-manager/images/node-controller/src/docs/node-controller-architecture.md
2. План реализации: modules/040-node-manager/images/node-controller/src/docs/node-controller-implementation-plan.md

## Рабочая директория

modules/040-node-manager/images/node-controller/src/

## Что уже реализовано

- cmd/main.go — точка входа
- internal/controllers/register.go — регистрация контроллеров
- internal/controllers/nodegroup/ — частичный NodeGroup controller
- internal/webhook/ — NodeGroup conversion webhook
- api/deckhouse.io/v1,v1alpha1,v1alpha2 — типы NodeGroup

## Задача

Реализуй контроллеры из плана, используя суб-агентов для параллельной работы.

### Порядок выполнения

**Шаг 0 (подготовка):** Выполни задачи 0.1, 0.2, 0.3 из плана — создай структуру директорий, API types, shared predicates. Это делается последовательно, т.к. волны зависят от этого.

**Шаг 1 (Волна 1):** Запусти до 4 суб-агентов параллельно для задач 1.1-1.4. Каждый суб-агент получает промпт из шаблона в плане реализации (секция "Шаблон промпта для суб-агента"), с подставленными значениями для конкретной задачи.

**Шаг 2 (Волны 2+3):** После завершения волны 1, запусти до 5 суб-агентов для задач из волн 2 и 3. Волны 2 и 3 независимы друг от друга и могут выполняться одновременно.

**Шаг 3 (Волна 4):** Метрики — после всех предыдущих волн.

### Правила для суб-агентов

Для каждого суб-агента формируй промпт по шаблону из плана реализации. В промпте ОБЯЗАТЕЛЬНО укажи:

1. Конкретные файлы для чтения:
   - Исходный хук: /Users/pallam/GolandProjects/deckhouse/modules/040-node-manager/hooks/[ИМЯ].go
   - Тесты хука: /Users/pallam/GolandProjects/deckhouse/modules/040-node-manager/hooks/[ИМЯ]_test.go
   - Архитектура: docs/node-controller-architecture.md (номер секции)
   - Существующий код: internal/controllers/register.go, cmd/main.go

2. Конкретные файлы для создания:
   - Reconciler: internal/controller/[resource]/[subdir]/reconciler.go
   - Фазы: internal/controller/[resource]/[subdir]/[фаза].go
   - Тесты: internal/controller/[resource]/[subdir]/reconciler_test.go

3. Блоки "ВАЖНО — ТЕСТЫ" и "ВАЖНО — ФРЕЙМВОРК" из шаблона.

### После каждой волны

1. Проверь что register.go обновлён
2. Запусти `make build` для проверки компиляции
3. Запусти `make test` для проверки тестов
4. Если есть ошибки — исправь их до перехода к следующей волне

### Конкретные промпты для суб-агентов Волны 1

**Суб-агент 1.1 (NodeUpdateReconciler):**
Прочитай hooks/update_approval.go, hooks/update_approval_test.go, hooks/handle_draining.go, hooks/handle_draining_test.go, hooks/pkg/ (drain logic). Создай internal/controller/node/update/ с reconciler.go, approval.go, draining.go, predicates.go. Тесты в reconciler_test.go. Shared drain logic в internal/drain/drainer.go. Секция 1 архитектуры.

**Суб-агент 1.2 (NodeTemplateReconciler):**
Прочитай hooks/handle_node_templates.go, hooks/handle_node_templates_test.go. Создай internal/controller/node/template/ с reconciler.go, diff.go, predicates.go. Секция 2 архитектуры.

**Суб-агент 1.3 (NodeGroupReconciler):**
Прочитай hooks/update_node_group_status.go, hooks/update_node_group_status_test.go, hooks/create_master_node_group.go, hooks/create_master_node_group_test.go, hooks/set_instance_class_ng_usage.go, hooks/set_instance_class_ng_usage_test.go. Расширь существующий internal/controllers/nodegroup/. Создай поддиректории status/, instanceclass/, master/. Секция 8 архитектуры.

**Суб-агент 1.4 (InstanceReconciler):**
Прочитай hooks/instance_controller.go, hooks/instance_controller_test.go, hooks/internal/ (Instance types). Создай internal/controller/instance/ с reconciler.go, lifecycle.go, status.go, finalizer.go. Секция 9 архитектуры.
```
