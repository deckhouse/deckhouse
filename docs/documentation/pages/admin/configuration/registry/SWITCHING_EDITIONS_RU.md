---
title: Переключение редакций
permalink: ru/admin/configuration/registry/switching-editions.html
description: "Как сменить редакцию Deckhouse Kubernetes Platform и что проверить перед переключением."
lang: ru
search: deckhouse edition, switch edition, CE, BE, EE, SE, edition change, переключение редакций
---

{% alert level="warning" %}
Перед переключением редакции проверьте лицензию и доступ к registry с образами нужной редакции.

Если кластер не сможет скачать образы новой редакции, переключение не завершится.
{% endalert %}

{% alert level="warning" %}
Смена редакции может изменить состав доступных модулей и функций.

Перед переключением проверьте, какие возможности нужны вашей команде и какие модули используются в кластере сейчас.
{% endalert %}

Эта инструкция помогает сменить редакцию Deckhouse Kubernetes Platform в уже работающем кластере.

## Когда это нужно

Редакцию обычно меняют в трёх случаях:

- вы переходите на коммерческую редакцию;
- меняется лицензия и нужно использовать другой набор возможностей;
- вы приводите кластер к новой целевой конфигурации по требованиям компании.

## Что проверить перед началом

Перед переключением проверьте:

- какая редакция используется сейчас;
- на какую редакцию вы хотите перейти;
- есть ли действующая лицензия для новой редакции;
- доступен ли registry с образами новой редакции;
- не зависят ли ваши рабочие процессы от модулей, которых не будет в новой редакции;
- пуста ли очередь Deckhouse;
- все ли control-plane-узлы находятся в состоянии `Ready`.

## Как узнать текущую редакцию

Проверьте глобальные значения Deckhouse:

```bash
d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- \
  deckhouse-controller global values -o yaml | yq '.deckhouseEdition'
```

Команда вернёт текущую редакцию, например:

```text
EE
```

## Как проходит переключение

Редакция DKP зависит от двух вещей:

- от лицензии;
- от registry с образом Deckhouse нужной редакции.

На практике это значит, что нужно:

1. указать registry с образами новой редакции;
1. передать корректную лицензию, если она нужна;
1. дождаться, пока Deckhouse применит изменения.

## Переключение в кластере, управляемом DKP

Если кластер полностью управляется DKP, редакцию обычно меняют через `ModuleConfig` `deckhouse` в секции `registry`.

Пример для режима `Direct`:

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

Что важно проверить:

- путь `imagesRepo` указывает на нужную редакцию;
- значение `license` соответствует новой редакции;
- режим registry подходит вашему кластеру.

После изменения конфигурации проверьте статус:

```bash
d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
```

Успешное переключение выглядит так:

```yaml
conditions:
  - type: Ready
    status: "True"
mode: Direct
target_mode: Direct
```

## Переключение в Managed Kubernetes-кластере

В Managed Kubernetes-кластерах редакцию меняют через `helper change-registry`.

Пример:

```bash
d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- \
  deckhouse-controller helper change-registry \
  --user MY-USER \
  --password MY-PASSWORD \
  registry.example.com/deckhouse/ee
```

Если registry использует свой CA-сертификат, добавьте параметр `--ca-file`.

После применения настроек:

- дождитесь, пока pod'ы обновят образы;
- проверьте журнал `bashible`;
- убедитесь, что в кластере не осталось pod'ов со старым адресом registry.

Подробный сценарий описан в разделе [«Managed Kubernetes: сторонний registry»](../dkp_component/third-party).

## Как понять, что редакция уже сменилась

Снова проверьте глобальные значения:

```bash
d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- \
  deckhouse-controller global values -o yaml | yq '.deckhouseEdition'
```

Если команда показывает новую редакцию, переключение завершилось.

Дополнительно проверьте:

- cluster-wide состояние Deckhouse;
- доступность нужных модулей;
- отсутствие ошибок загрузки образов.

## Что может пойти не так

### Неверная лицензия

Если лицензия не подходит для новой редакции, DKP не сможет перейти на неё корректно.

Проверьте:
- срок действия лицензии;
- поддерживаемую редакцию;
- правильность значения `license` в конфигурации.

### Недоступен registry новой редакции

Проверьте:
- сетевой доступ;
- логин и пароль;
- CA-сертификаты;
- правильность адреса `imagesRepo` или параметра `<new-registry>`.

### После смены редакции пропали ожидаемые функции

Скорее всего, новая редакция не включает часть функций или модулей, которые использовались раньше.

Перед переключением лучше заранее сверить состав возможностей и зависимости кластера.

## После переключения

После смены редакции рекомендуем:

- проверить состояние модулей:

  ```bash
  d8 k get modules
  ```

- убедиться, что очередь Deckhouse пуста:

  ```bash
  d8 system queue list
  ```

- проверить ключевые приложения и системные компоненты.

## Что дальше

- Если вы меняете registry в кластере, полностью управляемом DKP, откройте раздел [«Кластер, управляемый DKP»](../dkp_component/managing-interaction).
- Если кластер работает в Managed Kubernetes, используйте раздел [«Managed Kubernetes: сторонний registry»](../dkp_component/third-party).
- Если нужно восстановить доступ к registry, перейдите в раздел [«Восстановление подключения к registry»](../custom_image_storage/restore-token).
