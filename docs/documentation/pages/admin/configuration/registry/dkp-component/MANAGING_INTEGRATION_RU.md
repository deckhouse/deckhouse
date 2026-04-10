---
title: Кластер, управляемый DKP
permalink: ru/admin/configuration/registry/dkp-component/managing-interaction.html
description: "Как настроить хранилище образов компонентов платформы в кластере, полностью управляемом DKP."
lang: ru
search: registry, dkp registry, direct, proxy, local, unmanaged, module registry, хранилище образов, управляемый кластер
---

Если кластер полностью управляется DKP, настройки registry для компонентов платформы задаёт модуль [`registry`](..//modules/registry/).

На этой странице собраны основные режимы работы, ограничения и ссылки на детальные инструкции.

## Что можно настроить

DKP поддерживает четыре режима работы с registry компонентов платформы:

- `Direct` — платформа обращается к внешнему registry через внутренний виртуальный адрес.
- `Proxy` — DKP поднимает кеширующий proxy и обращается к внешнему registry через него.
- `Local` — платформа использует локальный registry внутри кластера.
- `Unmanaged` — DKP обращается к внешнему registry напрямую, без внутреннего registry-механизма.

Во всех режимах, кроме `Unmanaged`, DKP использует виртуальный адрес `registry.d8-system.svc:5001/system/deckhouse`. Это помогает менять настройки registry без повторной загрузки всех образов и без полного перезапуска компонентов в типовых сценариях.

## Как выбрать режим

### `Direct`

Подходит, если нужен простой и понятный сценарий работы с внешним registry.

Когда выбирать:
- кластеру нужен доступ к внешнему registry;
- не нужен локальный registry внутри кластера;
- важно упростить дальнейшую смену адреса registry.

### `Proxy`

Подходит, если нужно сократить число запросов во внешний registry.

Когда выбирать:
- внешний registry доступен, но хочется снизить сетевую нагрузку;
- кластер статичный;
- подходит запуск proxy на control-plane-узлах.

### `Local`

Подходит для изолированных сред.

Когда выбирать:
- кластер должен работать без постоянного доступа к внешнему registry;
- вы готовы заранее загрузить нужные образы в локальный registry;
- кластер статичный.

### `Unmanaged`

Подходит, если нужен прямой доступ к внешнему registry без внутреннего registry-механизма DKP.

Когда выбирать:
- не нужен внутренний virtual endpoint;
- допустимы более жёсткие последствия при смене registry;
- вы понимаете ограничения этого режима.

## На что обратить внимание перед переключением

Перед сменой режима проверьте:

- bootstrap кластера завершён;
- модуль `registry` включён и работает;
- все control-plane-узлы находятся в статусе `Ready`;
- на узлах нет статуса `SchedulingDisabled`;
- очередь Deckhouse пуста;
- в кластере используется CRI containerd или containerd v2;
- для режимов `Local` и `Proxy` кластер должен быть статичным.

## Ограничения

### Общие ограничения

- Управлять registry через модуль `registry` можно только в кластерах, полностью управляемых DKP.
- Для `Direct` нужен CRI containerd или containerd v2 на всех узлах.
- `Local` и `Proxy` поддерживаются только в статичных кластерах.
- Переключение доступно только после завершения bootstrap.

### Ограничения по переходам

- Нельзя переключаться между `Local` и `Proxy` напрямую.
- Для такого перехода используйте промежуточный режим: `Direct` или `Unmanaged`.
- Если кластер использует старый неконфигурируемый `Unmanaged`, сначала выполните миграцию на модуль `registry`.

## Последствия переключения

При смене режима возможны перезапуски компонентов платформы.

Особенно важно учитывать:

- при первом переходе из `Unmanaged` в `Direct`, `Proxy` или `Local` DKP может перезапустить все свои компоненты;
- в сценариях с containerd v1 возможен перезапуск containerd;
- в `Local` режиме нужно отдельно подготовить и загрузить образы.

Поэтому такие работы лучше планировать заранее.

## Как отслеживать статус

DKP сохраняет статус переключения в секрете `registry-state`.

Проверьте его командой:

```bash
d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
```

Основные условия, на которые стоит смотреть:

- `ContainerdConfigPreflightReady` — проверка конфигурации containerd;
- `RegistryContainsRequiredImages` — есть ли все нужные образы;
- `Ready` — переключение завершено.

## Детальные инструкции

Используйте нужный раздел:

- [Переключение на режим `Direct`](/#переключение-на-режим-direct)
- [Переключение на режим `Proxy`](/#переключение-на-режим-proxy)
- [Переключение на режим `Local`](/#переключение-на-режим-local)
- [Переключение на режим `Unmanaged`](/#переключение-на-режим-unmanaged)
- [Миграция на модуль `registry`](/#миграция-на-модуль-registry)
- [Просмотр статуса переключения](/#просмотр-статуса-переключения)

---

## Переключение на режим `Direct`

1. Убедитесь, что модуль `registry` включён:

   ```bash
   d8 k get module registry -o wide
   ```

1. Проверьте состояние узлов:

   ```bash
   d8 k get nodes
   ```

1. Убедитесь, что очередь Deckhouse пуста:

   ```bash
   d8 system queue list
   ```

1. Примените `ModuleConfig` с режимом `Direct`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Direct
         direct:
           imagesRepo: registry.deckhouse.ru/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY>
   ```

1. Дождитесь состояния `Ready` в `registry-state`.

## Переключение на режим `Proxy`

1. Убедитесь, что модуль `registry` включён:

   ```bash
   d8 k get module registry -o wide
   ```

1. Проверьте состояние узлов:

   ```bash
   d8 k get nodes
   ```

1. Убедитесь, что очередь Deckhouse пуста:

   ```bash
   d8 system queue list
   ```

1. Примените `ModuleConfig` с режимом `Proxy`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Proxy
         proxy:
           imagesRepo: registry.deckhouse.ru/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY>
   ```

1. Дождитесь состояния `Ready` в `registry-state`.

## Переключение на режим `Local`

> Этот сценарий длиннее остальных. Сначала подготовьте образы, затем включите `Local`, после этого загрузите bundle в локальный registry.

1. Убедитесь, что модуль `registry` включён:

   ```bash
   d8 k get module registry -o wide
   ```

1. Проверьте состояние узлов:

   ```bash
   d8 k get nodes
   ```

1. Убедитесь, что очередь Deckhouse пуста:

   ```bash
   d8 system queue list
   ```

1. Определите текущую версию DKP и редакцию:

   ```bash
   TAG=$(
   d8 k -n d8-system get deployment/deckhouse -o yaml \
   | yq -r '.spec.template.spec.containers[] | select(.name == "deckhouse").image | split(":")[-1]'
   ) && echo "TAG: $TAG"

   EDITION=$(
   d8 k -n d8-system exec -it svc/deckhouse-leader -- deckhouse-controller global values -o yaml \
   | yq .deckhouseEdition
   ) && echo "EDITION: $EDITION"
   ```

1. Скачайте bundle с образами:

   ```bash
   d8 mirror pull \
     --license="<LICENSE_KEY>" \
     --source="registry.deckhouse.ru/deckhouse/$EDITION" \
     --deckhouse-tag="$TAG" \
     /home/user/d8-bundle
   ```

1. Включите режим `Local`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Local
   ```

1. Проверьте статус `RegistryContainsRequiredImages` в `registry-state`.

1. Получите учётные данные для загрузки образов:

   ```bash
   d8 k -n d8-system get secret/registry-user-rw -o json | jq -r '.data | to_entries[] | "\(.key): \(.value | @base64d)"'
   ```

1. Загрузите bundle в локальный registry:

   ```bash
   d8 mirror push \
     --registry-login="rw" \
     --registry-password="<PASSWORD>" \
     /home/user/d8-bundle \
     registry.${PUBLIC_DOMAIN}/system/deckhouse
   ```

1. Дождитесь состояния `Ready` в `registry-state`.

## Переключение на режим `Unmanaged`

1. Убедитесь, что модуль `registry` включён:

   ```bash
   d8 k get module registry -o wide
   ```

1. Убедитесь, что очередь Deckhouse пуста:

   ```bash
   d8 system queue list
   ```

1. Примените `ModuleConfig` с режимом `Unmanaged`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           imagesRepo: registry.deckhouse.ru/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY>
   ```

1. Дождитесь состояния `Ready` в `registry-state`.

## Миграция на модуль `registry`

Этот раздел нужен, если кластер до сих пор использует старый неконфигурируемый `Unmanaged`.

### Если у вас containerd v2

1. Посмотрите текущие настройки registry:

   ```bash
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller global values | yq e '.modulesImages.registry' -
   ```

1. Перенесите эти значения в `ModuleConfig` `deckhouse` и включите режим `Unmanaged`.

1. Дождитесь состояния `Ready` в `registry-state`.

### Если у вас containerd v1

Сценарий сложнее. Перед началом проверьте пользовательские конфигурации в `/etc/containerd/conf.d` и перенесите их в новый формат `/etc/containerd/registry.d`.

После этого:
1. примените новые конфигурации через `NodeGroupConfiguration`;
1. проверьте pull образов через `ctr`;
1. включите режим `Unmanaged` через модуль `registry`;
1. удалите старые конфигурации из `/etc/containerd/conf.d`;
1. дождитесь завершения переключения.

> В этом сценарии containerd может перезапуститься. Планируйте работы заранее.

## Просмотр статуса переключения

Используйте команду:

```bash
d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
```

Пример вывода:

```yaml
conditions:
  - lastTransitionTime: "2025-07-15T12:52:46Z"
    message: 'registry.deckhouse.ru: all 157 items are checked'
    reason: Ready
    status: "True"
    type: RegistryContainsRequiredImages
  - lastTransitionTime: "2025-07-15T12:55:48Z"
    message: ""
    reason: ""
    status: "True"
    type: Ready
mode: Direct
target_mode: Direct
```

Если `type: Ready` имеет статус `True`, переключение завершено.

## Что дальше

- Если кластер работает в Managed Kubernetes, используйте инструкцию [Managed Kubernetes: сторонний registry](../third-party).
- Если нужно хранить образы приложений внутри кластера, перейдите в раздел [Payload registry](../payload-registry).
