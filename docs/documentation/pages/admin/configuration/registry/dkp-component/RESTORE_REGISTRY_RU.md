---
title: Восстановление подключения к registry
permalink: ru/admin/configuration/registry/dkp-component/restore-token.html
description: "Как восстановить подключение Deckhouse Kubernetes Platform к registry, если изменились учётные данные, лицензия или сертификаты."
lang: ru
search: registry access, restore registry access, license key, registry credentials, ca certificate, восстановление подключения к registry
---

{% alert level="warning" %}
Эта страница помогает восстановить доступ к registry **компонентов DKP**.

Если вы настраиваете registry для пользовательских приложений, используйте разделы [«Payload registry»](payload-registry.html) или [«Внутренний registry»](internal.html).
{% endalert %}

{% alert level="warning" %}
Способ восстановления зависит от типа кластера:

- для кластера, полностью управляемого DKP, используйте модуль `registry`;
- для Managed Kubernetes-кластера используйте `helper change-registry`.
{% endalert %}

Если DKP больше не может скачивать образы, причина обычно одна из этих:

- изменились логин или пароль к registry;
- истёк или сменился токен;
- изменился CA-сертификат;
- сменился адрес registry;
- указан неверный `license` для доступа к образам DKP;
- у узлов больше нет сетевого доступа к registry.

На этой странице собраны быстрые шаги, которые помогают вернуть подключение.

## С чего начать

Сначала проверьте, к какому типу относится ваш кластер:

- **кластер полностью управляется DKP** → переходите в раздел [Восстановление в кластере, управляемом DKP](#восстановление-в-кластере-управляемом-dkp);
- **Managed Kubernetes-кластер** → переходите в раздел [Восстановление в Managed Kubernetes-кластере](#восстановление-в-managed-kubernetes-кластере).

## Что проверить в любом случае

Перед изменением настроек проверьте:

- какой адрес registry должен использовать кластер;
- есть ли у вас актуальные логин, пароль или токен;
- нужен ли отдельный CA-сертификат;
- доступен ли registry по сети с control-plane-узлов;
- не закончилась ли лицензия, если вы используете образы DKP из официального registry.

## Восстановление в кластере, управляемом DKP

В таких кластерах DKP хранит настройки registry в `ModuleConfig` `deckhouse`.

### 1. Проверьте текущие настройки

Посмотрите текущую конфигурацию:

```bash
d8 k get mc deckhouse -o yaml
```

Обратите внимание на секцию:

```yaml
spec:
  settings:
    registry:
```

Проверьте:

- `mode`;
- адрес `imagesRepo`;
- `scheme`;
- `license`;
- дополнительные параметры, если вы используете свой registry.

### 2. Обновите параметры доступа

Если изменились адрес registry, лицензия или другие параметры, обновите `ModuleConfig`.

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

Если кластер работает в другом режиме, обновите параметры в нужной секции:

- `direct`
- `proxy`
- `unmanaged`

### 3. Проверьте статус переключения

После изменения конфигурации проверьте статус:

```bash
d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
```

Если всё прошло успешно, в conditions появится:

```yaml
type: Ready
status: "True"
```

### 4. Если вы используете свой CA-сертификат

Проверьте, что сертификат:

- актуален;
- соответствует адресу registry;
- доверен всем нужным узлам.

Если сертификат обновился, примените новую конфигурацию CA по вашему стандартному процессу управления сертификатами в кластере.

### 5. Если проблема осталась

Проверьте:

- может ли узел открыть registry по сети;
- не изменились ли правила firewall или proxy;
- нет ли ошибок в логах container runtime;
- не зависло ли переключение в `registry-state`.

Если кластер полностью управляется DKP и вы меняете сам режим работы registry, используйте подробную инструкцию [«Кластер, управляемый DKP»](../managing-interaction).

## Восстановление в Managed Kubernetes-кластере

В Managed Kubernetes-кластерах нужно заново применить настройки через `helper change-registry`.

### 1. Повторно задайте актуальные параметры

Пример:

```bash
d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- \
  deckhouse-controller helper change-registry \
  --user MY-USER \
  --password MY-PASSWORD \
  registry.example.com/deckhouse/ee
```

Если registry использует собственный CA-сертификат, добавьте `--ca-file`.

Пример:

```bash
d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- \
  deckhouse-controller helper change-registry \
  --ca-file /tmp/ca.crt \
  --user MY-USER \
  --password MY-PASSWORD \
  registry.example.com/deckhouse/ee
```

### 2. Дождитесь применения настроек

Проверьте:

- pod'ы, которые используют образы DKP;
- состояние pod'а registry;
- журнал `bashible` на master-узле.

Команда для проверки журнала:

```bash
journalctl -u bashible -n 20
```

Успешный результат выглядит так:

```text
Configuration is in sync, nothing to do
```

### 3. Убедитесь, что кластер больше не использует старый адрес

```bash
d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
  | select(.image | startswith("registry.deckhouse"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
```

Если список пустой, кластер больше не тянет образы со старого адреса.

Подробный сценарий описан в разделе [«Managed Kubernetes: сторонний registry»](../third-party).

## Частые причины проблем

### Неверный логин, пароль или токен

Проверьте учётные данные и примените их заново.

### Неверный адрес registry

Проверьте `imagesRepo` или значение `<new-registry>` в `helper change-registry`.

### Проблема с сертификатом

Если registry использует свой CA, проверьте сертификат и цепочку доверия.

### Проблема с лицензией

Если вы используете официальный registry DKP, проверьте, что указали актуальный `license`.

### Сетевой доступ

Проверьте DNS, маршрутизацию, proxy и firewall между узлами кластера и registry.

## Как понять, что доступ восстановлен

Обычно это видно по трём признакам:

- новые pod'ы успешно скачивают образы;
- в логах нет новых ошибок `ImagePullBackOff` или `ErrImagePull`;
- статус `registry-state` показывает `Ready=True`, если кластер управляется через модуль `registry`.

## Что дальше

- Если нужно полностью сменить режим работы registry, используйте раздел [«Кластер, управляемый DKP»](../managing-interaction).
- Если кластер работает в Managed Kubernetes, откройте [«Managed Kubernetes: сторонний registry»](../third-party.html).
- Если нужно хранить образы приложений внутри кластера, используйте [«Payload registry»](../payload-registry.html).
