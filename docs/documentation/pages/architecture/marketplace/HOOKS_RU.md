---
title: Хуки
permalink: ru/architecture/marketplace/hooks.html
description: "Go-хуки для Applications в Deckhouse Platform Marketplace: бинарный файл хуков на module-sdk, привязки и порядок выполнения, ApplicationHookInput, привязки Kubernetes, операции с объектами, values и хук валидации настроек."
lang: ru
search: application hooks, ApplicationHookInput, ApplicationHookConfig, settingscheck, beforeDeleteHelm, хуки приложения, валидация настроек, привязки хуков
---

Хуки Application пишутся на Go с использованием [module-sdk](https://github.com/deckhouse/module-sdk). Deckhouse Platform (DP) запускает их на определённых этапах жизненного цикла приложения, при изменении объектов Kubernetes и по расписанию. Хуки подготавливают values для шаблонов, создают и изменяют объекты в неймспейсе приложения и проверяют настройки.

## Бинарный файл хуков

Все хуки пакета компилируются в один бинарный файл Go. В заготовке пакета исходный код находится в `hooks/batch/`, а `d8 package build` собирает бинарный файл по инструкциям из `hooks/hooks.yaml` и помещает его в bundle пакета.

Каждый хук регистрируется с помощью `registry.RegisterFunc`, а функция `main` вызывает `app.Run`:

```go
// hooks/batch/main.go
package main

import (
    "github.com/deckhouse/module-sdk/pkg/app"

    "hooks/settings"
    _ "hooks/triggers"
)

func main() {
    app.Run(app.WithSettingsCheck(settings.Check))
}
```

Требования к бинарному файлу:

- Файлы с исходным кодом хуков должны находиться в каталоге с именем `hooks`: module-sdk формирует имя хука по пути к файлу.
- Бинарный файл должен регистрировать хотя бы один хук. DP игнорирует бинарный файл без хуков, даже если он содержит [хук валидации настроек](#хук-валидации-настроек).
- Собирайте статический бинарный файл для Linux (`CGO_ENABLED=0`), как это сделано в заготовке.

DP находит хуки, запуская каждый исполняемый файл без расширения из каталога `hooks/` bundle с аргументом `hook list`. Бинарный файл, который завершается с ошибкой на этом этапе, например, из-за некорректной конфигурации хука, пропускается без ошибки в статусе Application. Чтобы убедиться, что хуки зарегистрированы, проверяйте в CI вывод следующих команд:

```bash
cd hooks/batch
CGO_ENABLED=0 go build -o /tmp/hooks .
/tmp/hooks hook list
/tmp/hooks hook config
```

## Пример хука

```go
package state

import (
    "context"

    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

    "github.com/deckhouse/module-sdk/pkg"
    objectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
    "github.com/deckhouse/module-sdk/pkg/registry"
)

var _ = registry.RegisterFunc(&pkg.ApplicationHookConfig{
    OnBeforeHelm: &pkg.OrderedConfig{Order: 10},
    Kubernetes: []pkg.ApplicationKubernetesConfig{
        {
            Name:       "credentials",
            APIVersion: "v1",
            Kind:       "Secret",
            LabelSelector: &metav1.LabelSelector{
                MatchLabels: map[string]string{"example.com/credentials": "true"},
            },
            JqFilter: `{"name": .metadata.name}`,
        },
    },
}, handle)

type secret struct {
    Name string `json:"name"`
}

func handle(_ context.Context, input *pkg.ApplicationHookInput) error {
    secrets, err := objectpatch.UnmarshalToStruct[secret](input.Snapshots, "credentials")
    if err != nil {
        return err
    }

    // The path must be declared in openapi/values.yaml.
    input.Values.Set("internal.credentialsCount", len(secrets))

    // The ConfigMap is created as d8a-<INSTANCE_NAME>-state in the application namespace.
    input.PatchCollector.CreateOrUpdate(&corev1.ConfigMap{
        TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
        ObjectMeta: metav1.ObjectMeta{Name: "state"},
        Data:       map[string]string{"instance": input.Instance.Name()},
    })

    return nil
}
```

## Привязки и порядок выполнения

Структура `pkg.ApplicationHookConfig` определяет, когда запускается хук:

| Поле | Когда запускается хук |
|---|---|
| `OnStartup` | Один раз после чтения пакета: при установке, после изменения версии пакета и после перезапуска DP. Хук не получает снапшоты |
| `OnBeforeHelm` | В каждом цикле согласования перед применением шаблонов |
| `OnAfterHelm` | В каждом цикле согласования после применения шаблонов. Если хук меняет values, DP применяет шаблоны ещё раз |
| `OnBeforeDeleteHelm` | Перед удалением Helm-релиза: при удалении Application или когда требования пакета перестают выполняться. При изменении версии пакета хук не запускается |
| `OnAfterDeleteHelm` | После удаления Helm-релиза в тех же случаях, что и `OnBeforeDeleteHelm` |
| `Schedule` | По расписанию в формате crontab |
| `Kubernetes` | При изменении отслеживаемых объектов в неймспейсе приложения и при синхронизации привязки (см. [«Привязки Kubernetes»](#привязки-kubernetes)) |

Другие поля:

- `Queue` — имя очереди для привязок `Schedule` и `Kubernetes` хука (по умолчанию `main`). Эти привязки выполняются в очереди, отдельной от этапов жизненного цикла, поэтому могут выполняться одновременно с хуками `OnBeforeHelm` или `OnAfterHelm`.
- `AllowFailure` и `Settings` для приложений не поддерживаются.

Хуки с одинаковой привязкой запускаются в порядке возрастания `Order` (поле `pkg.OrderedConfig`). Последовательность этапов жизненного цикла описана в разделе [«Жизненный цикл и отладка»](lifecycle.html#этапы-установки).

Хук не может сочетать `OnStartup` с привязками `Kubernetes`: при регистрации такого хука module-sdk завершается с паникой, и DP пропускает весь бинарный файл.

### Обработка ошибок

- Если хук завершается с ошибкой, этап, на котором он запускался, завершается с ошибкой и повторяется с увеличивающейся задержкой — от 15 секунд до 2 часов. Следующие задачи приложения ждут. Ошибка отражается в условиях и summary ресурса Application.
- Если с ошибкой завершается хук `OnBeforeDeleteHelm`, DP не удаляет Helm-релиз и повторяет хук. Application не удаляется, пока хук не выполнится успешно.
- Если операция хука с объектом завершается с ошибкой, запуск хука завершается с ошибкой, а изменения values, сделанные этим запуском, отбрасываются. Уже выполненные операции не откатываются.

{% alert level="warning" %}
module-sdk перехватывает панику в обработчиках хуков, и запуск считается успешным, но ни на что не влияет: не применяются ни изменения values, ни операции с объектами. Возвращайте ошибки вместо паники.
{% endalert %}

## ApplicationHookInput

Обработчик хука имеет сигнатуру `func(ctx context.Context, input *pkg.ApplicationHookInput) error`. `pkg.ApplicationHookInput` содержит следующие поля:

| Поле | Описание |
|---|---|
| `Instance` | Параметры экземпляра: `Name()` и `Namespace()` |
| `Snapshots` | Снапшоты привязок Kubernetes хука |
| `Values` | Values приложения: чтение (`Get`, `GetOk`, `Exists`, `ArrayCount`) и изменение (`Set`, `Remove`) |
| `Settings` | Итоговые настройки приложения, только для чтения |
| `PatchCollector` | Операции с объектами в неймспейсе приложения (см. [«Операции с объектами»](#операции-с-объектами)) |
| `DC` | Зависимости: HTTP-клиент (`GetHTTPClient`), клиент реестра образов контейнеров (`GetRegistryClient`) и часы (`GetClock`) |
| `Logger` | Логгер. Вывод записывается в лог DP |
| `MetricsCollector` | Для приложений не поддерживается: DP игнорирует метрики |

Хуки приложений не получают клиент Kubernetes. Чтобы читать объекты, используйте привязки Kubernetes, а чтобы изменять их — `PatchCollector`.

Имя и неймспейс экземпляра — единственные параметры экземпляра, доступные хукам. Имя, версия и образы пакета доступны только [шаблонам](templates.html#application).

## Привязки Kubernetes

Структура `pkg.ApplicationKubernetesConfig` описывает объекты, которые отслеживает хук:

| Поле | Описание |
|---|---|
| `Name` | Имя привязки, ключ в `Snapshots` |
| `APIVersion`, `Kind` | Версия API и тип объектов. Поддерживаются только типы уровня неймспейса |
| `NameSelector`, `LabelSelector`, `FieldSelector` | Выбор объектов |
| `JqFilter` | Выражение jq, которое применяется к каждому объекту. Снапшот содержит результаты выражения. Без выражения снапшот содержит объекты целиком |
| `ExecuteHookOnEvents` | `false` — обновлять снапшот без запуска хука при изменении объектов. По умолчанию: `true` |
| `ExecuteHookOnSynchronization` | `false` — не запускать хук при синхронизации. По умолчанию: `true` |
| `WaitForSynchronization` | `false` — не ждать синхронизации привязки перед следующими этапами. Действует, только если задано поле `Queue` хука. По умолчанию: `true` |
| `AllowFailure` | Игнорировать ошибки хука при синхронизации |

Поведение:

- DP всегда ограничивает привязки неймспейсом приложения, поэтому в структуре нет селектора неймспейсов.
- При каждом запуске хук получает снапшоты всех своих привязок: текущее состояние объектов, а не событие, вызвавшее запуск.
- Хуки `OnBeforeHelm`, `OnAfterHelm`, `OnBeforeDeleteHelm` и `OnAfterDeleteHelm` тоже получают снапшоты.
- DP синхронизирует привязки в начале каждого цикла согласования: хуки с включённым `ExecuteHookOnSynchronization` запускаются с полным списком объектов.
- `NameSelector` сравнивается с фактическими именами объектов, включая префикс `d8a-<INSTANCE_NAME>-`.

Конфигурация хука одинакова для всех экземпляров пакета, поэтому селекторы не могут ссылаться на имя экземпляра. Если в один неймспейс может быть установлено несколько экземпляров пакета, добавьте метку экземпляра в снапшот и сравнивайте её с `input.Instance.Name()`. Объекты, отрендеренные из шаблонов, имеют метку `packages.deckhouse.io/instance`:

```go
JqFilter: `{"name": .metadata.name, "instance": .metadata.labels["packages.deckhouse.io/instance"]}`,
```

## Операции с объектами

`input.PatchCollector` собирает операции с объектами. DP выполняет их после завершения хука:

| Метод | Описание |
|---|---|
| `Create` | Создать объект. Операция завершается с ошибкой, если объект существует |
| `CreateIfNotExists` | Создать объект, если его нет |
| `CreateOrUpdate` | Создать объект или заменить существующий. Поля, которые хук не задаёт, удаляются из объекта |
| `Delete`, `DeleteInBackground`, `DeleteNonCascading` | Удалить объект с каскадным удалением зависимых объектов на переднем плане, в фоне или без удаления зависимых объектов. Отсутствующий объект игнорируется |
| `PatchWithJSON`, `PatchWithMerge`, `PatchWithJQ` | Изменить объект с помощью JSON Patch (RFC 6902), JSON Merge Patch (RFC 7396) или выражения jq. Чтобы изменить подресурс, передайте опцию `objectpatch.WithSubresource("status")` |

Поведение:

- module-sdk подставляет неймспейс приложения в каждую операцию независимо от неймспейса, указанного в объекте.
- У типизированных объектов должно быть заполнено поле `TypeMeta` с `apiVersion` и `kind`. Иначе операция не выполняется.
- Изменение отсутствующего объекта приводит к ошибке хука. Перед изменением проверяйте, что объект существует, например, по снапшоту.
- Объекты, созданные хуками, не входят в Helm-релиз: DP не удаляет их при удалении приложения и не добавляет к ним [метки платформы](templates.html#метки-и-защита-объектов). Удаляйте такие объекты в хуках `OnBeforeDeleteHelm` или `OnAfterDeleteHelm` либо задавайте им ссылку на владельца (owner reference) — объект релиза.

### Имена объектов

DP добавляет префикс `d8a-<INSTANCE_NAME>-` к имени объекта в каждой операции, если имя ещё не начинается с него. Например, для экземпляра `myapp`:

- `Create` для ConfigMap `state` создаёт `d8a-myapp-state`;
- `PatchWithMerge` для Deployment `server` изменяет `d8a-myapp-server`, то есть Deployment с именем `d8a-<INSTANCE_NAME>-server` в шаблонах.

В результате хук может изменять только объекты своего экземпляра. Объекты с префиксом `d8a-` защищены от изменений пользователями (см. [«Шаблоны»](templates.html#метки-и-защита-объектов)).

## Values

`input.Values` содержит values приложения — тот же документ, который шаблоны получают как `.Values`. Настройки находятся в корне документа, без ключа с именем пакета.

- `Set(path, value)` и `Remove(path)` принимают путь с ключами, разделёнными точками, например, `internal.credentialsCount`. Родительский объект пути должен существовать, поэтому описывайте промежуточные объекты в `openapi/values.yaml` с `default: {}`.
- После завершения хука DP проверяет values по схеме `openapi/values.yaml`. Схема должна включать схему настроек через `x-extend` (см. [«Настройки приложения»](settings.html#файлы-схем)): настройки находятся в корне values, а незадекларированные поля не проходят валидацию.
- Изменения values хранятся в памяти. После перезапуска DP или изменения версии пакета они теряются, пока хуки не запустятся снова.

Что происходит после изменения values, зависит от привязки:

| Привязка | Результат |
|---|---|
| `Kubernetes`, `Schedule` | DP запускает новый цикл согласования |
| `OnAfterHelm` | DP применяет шаблоны ещё раз |
| `OnStartup`, `OnBeforeHelm`, синхронизация | Values используются при применении шаблонов в том же цикле |

`input.Settings` доступны только для чтения: хуки не могут менять настройки приложения.

## Хук валидации настроек

Application может содержать хук валидации настроек для проверок, которые нельзя выразить OpenAPI-схемой, например, для проверки бизнес-логики между несколькими полями настроек.

Хук реализует функцию `Check` с сигнатурой:

```go
func Check(_ context.Context, input settingscheck.Input) settingscheck.Result
```

`settingscheck.Input` предоставляет доступ к настройкам (`input.Settings`) с подставленными значениями по умолчанию из `openapi/settings.yaml`, а также к логгеру (`input.Logger`).

`settingscheck.Result` — одно из:

- `settingscheck.Allow(warnings...)` — настройки валидны; опционально можно добавить предупреждения.
- `settingscheck.Reject(reason)` — настройки невалидны.

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

Чтобы зарегистрировать проверку, передайте её в `app.Run` в бинарном файле хуков: `app.Run(app.WithSettingsCheck(Check))`. Пакет может содержать только один хук валидации настроек.

DP вызывает проверку в следующих случаях:

- Когда меняется Application установленного приложения, validating-вебхук выполняет проверку установленной версии. Отказ отклоняет запрос с указанной причиной. Предупреждения возвращаются клиенту, например, `d8 k` выводит их.
- Когда DP применяет изменённые настройки. При отказе новые настройки не применяются, а ошибка отражается в условиях Application. Предупреждения игнорируются.

Проверка не выполняется в validating-вебхуке при создании Application: в этом случае запрос проверяется только по схеме настроек. При изменении `spec.packageVersion` вебхук выполняет проверку установленной версии, а не новой. Описывайте критичные ограничения в схеме, например, с помощью [правил CEL](settings.html#правила-cel-x-deckhouse-validations), чтобы отклонять некорректные настройки ещё до установки.

## Ограничения

- Привязки Kubernetes и операции с объектами ограничены неймспейсом приложения, а операции с объектами — ещё и объектами с префиксом имени `d8a-<INSTANCE_NAME>-`.
- Хуки не могут создавать или изменять cluster-wide-объекты.
- Проверки готовности (`app.WithReadiness`) предназначены для модулей. Не используйте их в приложениях: DP определяет состояние приложения по его [рабочим нагрузкам](templates.html#состояние-рабочих-нагрузок).
- Метрики хуков не собираются.
