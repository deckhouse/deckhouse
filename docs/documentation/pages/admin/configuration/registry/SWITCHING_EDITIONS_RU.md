---
title: Переключение редакций
permalink: ru/admin/configuration/registry/switching-editions.html
description: "Как сменить редакцию Deckhouse Kubernetes Platform и что проверить перед переключением."
lang: ru
search: deckhouse edition, switch edition, CE, BE, EE, SE, edition change, переключение редакций
---
{% alert level="warning" %}
Перед переключением редакции проверьте лицензию и доступ к хранилищу образов контейнеров с образами нужной редакции.
Если кластер не сможет скачать образы новой редакции, переключение не завершится.
{% endalert %}

{% alert level="warning" %}
Смена редакции может изменить состав доступных модулей и функций.
Перед переключением проверьте, какие возможности нужны вашей команде и какие модули используются в кластере сейчас.
{% endalert %}

## Что проверить перед началом

Перед переключением проверьте:
- какая редакция используется сейчас;
- на какую редакцию вы хотите перейти;
- есть ли действующая лицензия для новой редакции, если она нужна;
- доступно ли хранилище образов контейнеров с образами новой редакции;
- не зависят ли ваши рабочие процессы от модулей, которых не будет в новой редакции;
- пуста ли очередь Deckhouse;
- все ли master-узлы находятся в состоянии `Ready`;
- готовы ли вы к возможной временной недоступности компонентов кластера во время переключения.

## Как узнать текущую редакцию

Проверьте глобальные значения Deckhouse:

```bash
d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller global values -o yaml | yq '.deckhouseEdition'
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
1. проверить, какие модули доступны в новой редакции;
1. отключить модули, которые новая редакция не поддерживает;
1. указать registry с образами новой редакции;
1. передать корректную лицензию, если она нужна;
1. дождаться, пока Deckhouse применит изменения.

## Подготовка перед переключением

### Для CE и коммерческих редакций

Подготовьте переменные:
- `NEW_EDITION` — целевая редакция:
  - `ce`
  - `be`
  - `se`
  - `se-plus`
  - `ee`
- `LICENSE_TOKEN` — лицензионный токен для коммерческих редакций.

Для CE лицензионный токен не нужен.

Пример:

```bash
NEW_EDITION=<PUT_YOUR_EDITION_HERE>
LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
```


### Проверьте, какие модули недоступны в новой редакции

Этот шаг лучше не пропускать. Он помогает понять, какие модули придётся отключить до переключения.

#### В кластере, управляемом DKP

Для CE:

```bash
DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | \
  jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | \
  awk -F: '{print $NF}')

d8 k run $NEW_EDITION-image \
  --image=registry.deckhouse.ru/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION \
  --command sleep -- infinity
```

Для коммерческих редакций:

```bash
d8 k create secret docker-registry $NEW_EDITION-image-pull-secret \
  --docker-server=registry.deckhouse.ru \
  --docker-username=license-token \
  --docker-password=${LICENSE_TOKEN}

DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | \
  jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | \
  awk -F: '{print $NF}')

d8 k run $NEW_EDITION-image \
  --image=registry.deckhouse.ru/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION \
  --overrides="{\"spec\": {\"imagePullSecrets\":[{\"name\": \"$NEW_EDITION-image-pull-secret\"}]}}" \
  --command sleep -- infinity
```

Когда под перейдёт в состояние `Running`, выполните:

```bash
NEW_EDITION_MODULES=$(d8 k exec $NEW_EDITION-image -- ls -l deckhouse/modules/ | \
  grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)

USED_MODULES=$(d8 k get modules \
  -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | \
  grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})

MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | \
  grep -Fxv -f <(echo $NEW_EDITION_MODULES | tr ' ' '\n'))
```

Проверьте список:

```bash
echo $MODULES_WILL_DISABLE
```

Если список не пустой, отключите эти модули до переключения:

```bash
echo $MODULES_WILL_DISABLE | tr ' ' '\n' | \
  awk {'print "d8 platform module disable",$1'} | bash
```

После этого:
- дождитесь состояния `Ready` у пода Deckhouse;
- ещё раз проверьте очередь Deckhouse;
- убедитесь, что модули перешли в состояние `Disabled`:

  ```bash
  d8 k get modules
  ```

Удалите временные ресурсы:

```bash
d8 k delete pod/$NEW_EDITION-image
d8 k delete secret/$NEW_EDITION-image-pull-secret
```

#### В Managed Kubernetes-кластере

Логика проверки такая же: нужно запустить временный под новой редакции, получить список встроенных модулей и сравнить его с текущим набором модулей в кластере.

Для коммерческих редакций перед этим подготовьте доступ к registry. Если нужно, используйте временную конфигурацию узлов по аналогии с инструкцией из старой документации. После проверки модулей отключите те, которых не будет в новой редакции, и только потом переходите к смене registry.

## Переключение в кластере, управляемом DKP

Если кластер полностью управляется DKP, редакцию меняют через `ModuleConfig` `deckhouse` в секции `registry`.

### Что важно перед переключением

1. Убедитесь, что кластер уже использует модуль [`registry`](/modules/registry/).
   Если модуль не используется, сначала выполните миграцию по инструкции из раздела [«Настройки в кластере, управляемом DKP»](./dkp-component/managing-interaction.html).

1. Убедитесь, что очередь Deckhouse пуста:

   ```bash
   d8 system queue list
   ```

1. Убедитесь, что все master-узлы находятся в состоянии `Ready`.

### Как переключить редакцию

При переключении между редакциями используйте `checkMode: Relax`. Этот режим нужен, чтобы DKP сначала проверил доступность текущей версии Deckhouse в новом registry и смог безопасно начать переключение.

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
        checkMode: Relax
        imagesRepo: registry.deckhouse.ru/deckhouse/<NEW_EDITION>
        scheme: HTTPS
        license: <LICENSE_TOKEN>
```

Пример для режима `Unmanaged`:

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
        checkMode: Relax
        imagesRepo: registry.deckhouse.ru/deckhouse/<NEW_EDITION>
        scheme: HTTPS
        license: <LICENSE_TOKEN>
```

Для CE удалите параметр `license`.

Где:
- `imagesRepo` должен указывать на образы нужной редакции;
- `license` должен соответствовать новой редакции;
- `checkMode: Relax` нужен только на время переключения.

### Как проверить статус переключения

После изменения конфигурации проверьте статус:

```bash
d8 k -n d8-system -o yaml get secret registry-state | \
  yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
```

Пример успешного переключения:

```yaml
conditions:
  - lastTransitionTime: "..."
    message: |-
      Mode: Relax
      registry.deckhouse.ru: all 1 items are checked
    reason: Ready
    status: "True"
    type: RegistryContainsRequiredImages
  - lastTransitionTime: "..."
    message: ""
    reason: ""
    status: "True"
    type: Ready
mode: Direct
target_mode: Direct
```

После переключения удалите из `ModuleConfig` `deckhouse` параметр `checkMode: Relax`. Это включит стандартную проверку наличия критически важных образов в registry.

Затем снова проверьте статус переключения. В успешном случае `RegistryContainsRequiredImages` будет выполняться уже в обычном режиме проверки.

### Что проверить после переключения

После переключения:
- снова проверьте текущую редакцию через `deckhouseEdition`;
- убедитесь, что очередь Deckhouse пуста:

  ```bash
  d8 system queue list
  ```

- проверьте состояние модулей:

  ```bash
  d8 k get modules
  ```

Проверьте, не осталось ли в кластере подов со старым адресом registry.

Для режима `Unmanaged`:

```bash
d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
  | select(.image | contains("deckhouse.ru/deckhouse/<YOUR-PREVIOUS-EDITION>"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
```

Для режимов с фиксированным внутренним адресом:

```bash
IMAGES_DIGESTS=$(d8 k -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- \
  cat /deckhouse/modules/images_digests.json | jq -r '.[][]' | sort -u)

d8 k get pods -A -o json |
jq -r --argjson digests "$(printf '%s\n' $IMAGES_DIGESTS | jq -R . | jq -s .)" '
  .items[]
  | {name: .metadata.name, namespace: .metadata.namespace, containers: .spec.containers}
  | select(.containers != null)
  | select(
      .containers[]
      | select(.image | test("registry.d8-system.svc:5001/system/deckhouse") and test("@sha256:"))
      | .image as $img
      | ($img | split("@") | last) as $digest
      | ($digest | IN($digests[]) | not)
    )
  | .namespace + "\t" + .name
' | sort -u
```

Если в выводе остаются поды со старыми образами, проверьте их отдельно. При необходимости воспользуйтесь [инструкцией](../../../faq.html#что-делать-если-образ-модуля-не-скачался-и-модуль-не-переустанов).

## Переключение в Managed Kubernetes-кластере

В Managed Kubernetes-кластерах редакцию меняют через `helper change-registry`.

### Что важно перед переключением

Перед переключением:
1. проверьте, что очередь Deckhouse пуста:

   ```bash
   d8 system queue list
   ```

1. убедитесь, что у вас есть доступ к registry новой редакции;
1. проверьте, какие модули будут недоступны после смены редакции;
1. отключите модули, которых не будет в новой редакции.

### Как переключить редакцию

Для коммерческих редакций:

```bash
d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- \
  deckhouse-controller helper change-registry \
  --user license-token \
  --password ${LICENSE_TOKEN} \
  registry.deckhouse.ru/deckhouse/${NEW_EDITION}
```

Для CE:

```bash
DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | \
  jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | \
  awk -F: '{print $NF}')

d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- \
  deckhouse-controller helper change-registry \
  --new-deckhouse-tag=$DECKHOUSE_VERSION \
  registry.deckhouse.ru/deckhouse/ce
```

Если registry использует самоподписанный сертификат, добавьте параметр `--ca-file`.

Пример:

```bash
CA_CONTENT=$(cat <<EOF
-----BEGIN CERTIFICATE-----
CERTIFICATE
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
CERTIFICATE
-----END CERTIFICATE-----
EOF
)

d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- bash -c \
"echo '$CA_CONTENT' > /tmp/ca.crt && \
deckhouse-controller helper change-registry \
  --ca-file /tmp/ca.crt \
  --user license-token \
  --password ${LICENSE_TOKEN} \
  registry.deckhouse.ru/deckhouse/${NEW_EDITION}"
```

### Что проверить после применения настроек

После применения настроек:
- дождитесь, пока образы обновятся;
- проверьте журнал `bashible`:

  ```bash
  journalctl -u bashible -n 20
  ```

- убедитесь, что в журнале появилась строка:

  ```text
  Configuration is in sync, nothing to do
  ```

Проверьте, что в кластере не осталось подов со старым адресом registry:

```bash
d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
  | select(.image | contains("deckhouse.ru/deckhouse/<YOUR-PREVIOUS-EDITION>"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
```

Если список не пустой, дождитесь завершения обновления или разберите проблемные поды отдельно.

## Особый сценарий: переход с EE на CSE

Если вы переводите кластер с EE на CSE, учитывайте дополнительные ограничения из старой документации:

- переход возможен только с поддерживаемых версий DKP EE;
- переход поддерживается только между одинаковыми минорными версиями;
- в некоторых случаях нужна промежуточная миграция через другие минорные версии;
- в CSE не поддерживаются некоторые модули и сценарии;
- перед переключением нужно отдельно проверить совместимость по версии Kubernetes и составу модулей.

Если этот сценарий актуален для вашей инфраструктуры, используйте документацию той версии DKP, для которой переход описан и поддерживается.

## Как понять, что редакция уже сменилась

Снова проверьте глобальные значения:

```bash
d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- \
  deckhouse-controller global values -o yaml | yq '.deckhouseEdition'
```

Если команда показывает новую редакцию, переключение завершилось.

Дополнительно проверьте:
- общее состояние Deckhouse;
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

### После смены редакции отключились ожидаемые функции

Скорее всего, новая редакция не включает часть функций или модулей, которые использовались раньше.

Перед переключением заранее проверьте состав модулей и зависимости кластера. Если модуль не поддерживается в новой редакции, отключите его до переключения.

### Образы части модулей не обновились

Если после переключения отдельные модули не скачали новые образы и не переустановились, воспользуйтесь [инструкцией](../../../faq.html#что-делать-если-образ-модуля-не-скачался-и-модуль-не-переустанов).

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

- Если вы меняете registry в кластере, полностью управляемом DKP, откройте раздел [«Настройки в кластере, управляемом DKP»](./dkp-component/managing-interaction.html).
- Если кластер работает в Managed Kubernetes, используйте раздел [«Настройки в Managed Kubernetes-кластере»](./dkp-component/third-party.html).
