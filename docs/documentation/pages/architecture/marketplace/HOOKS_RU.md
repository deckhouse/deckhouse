---
title: Хуки
permalink: ru/architecture/marketplace/hooks.html
description: "Написание Go-хуков для Applications в Deckhouse Kubernetes Platform Marketplace с использованием ApplicationHookInput. ObjectPatcher, ограниченный неймспейсом, и хуки валидации настроек."
lang: ru
search: application hooks, ApplicationHookInput, settingscheck, хуки приложения, валидация настроек
---

Хуки Application пишутся на Go с использованием того же module-sdk, что и хуки модулей Deckhouse Kubernetes Platform (DKP). Ключевое отличие в том, что хуки Application используют `ApplicationHookInput` вместо `HookInput` — это добавляет возможности, специфичные для Application, и обеспечивает изоляцию неймспейса.

## ApplicationHookInput

`ApplicationHookInput` предоставляет все стандартные методы hook input, а также:

- **`Instance()`** — возвращает метаданные об экземпляре Application, включая его имя и неймспейс. Используйте для ограничения операций нужным неймспейсом.
- **`ObjectPatcher()`** — возвращает ObjectPatcher, ограниченный неймспейсом. Операции создания и патча ограничены неймспейсом экземпляра Application. Хуки не могут создавать или изменять cluster-wide ресурсы.

## Пример: базовый хук

```go
package main

import (
    "context"

    "github.com/deckhouse/module-sdk/pkg"
    applicationhook "github.com/deckhouse/module-sdk/pkg/app-hook"
)

func main() {
    applicationhook.Run(onSync)
}

func onSync(ctx context.Context, input applicationhook.ApplicationHookInput) error {
    instance := input.Instance()
    // instance.Name — имя ресурса Application
    // instance.Namespace — неймспейс, в котором установлен Application

    patcher := input.ObjectPatcher()
    // patcher работает только внутри instance.Namespace

    return nil
}
```

## Хук валидации настроек

Application может содержать хук валидации настроек, выполняющийся до того, как DKP применяет изменения. Используется когда валидации через OpenAPI-схему недостаточно — например, для проверки бизнес-логики между несколькими полями настроек.

Хук реализует функцию `Check` с сигнатурой:

```go
func Check(_ context.Context, input settingscheck.Input) settingscheck.Result
```

`settingscheck.Input` предоставляет доступ к значениям `Application.spec.settings`.

`settingscheck.Result` — одно из:

- `settingscheck.Allow(warnings...)` — настройки валидны; опционально можно добавить предупреждения.
- `settingscheck.Reject(reason)` — настройки невалидны; ресурс Application не будет применён.

**Пример:**

```go
func Check(_ context.Context, input settingscheck.Input) settingscheck.Result {
    replicas := input.Settings.Get("replicas").Int()
    if replicas == 0 {
        return settingscheck.Reject("replicas cannot be 0")
    }

    var warnings []string
    if replicas == 2 {
        warnings = append(warnings, "an even number of replicas may cause split-brain in some configurations")
    }

    if replicas > 3 {
        return settingscheck.Reject("replicas cannot be greater than 3")
    }

    return settingscheck.Allow(warnings...)
}
```

## Гарантия изоляции неймспейса

`ObjectPatcher`, возвращаемый `ApplicationHookInput`, обеспечивает, что:

- Все операции `Create`, `Patch` и `Update` направлены в неймспейс самого Application.
- Обращения к другим неймспейсам или cluster-wide ресурсам отклоняются в рантайме.

Это архитектурное ограничение, а не просто соглашение. Хуки, которым нужно читать (но не писать) cluster-wide ресурсы, могут по-прежнему использовать `d8 k` или Kubernetes API-клиент напрямую, но запись за пределами неймспейса заблокирована.
