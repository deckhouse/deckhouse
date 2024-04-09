
---
title: Установка обновлений DKP в закрытое окружение
permalink: ru/update/notifications/dkp-notice/
lang: ru
---

# 

## Доставка образов поставки в закрытое окружение

Для установки обновлений DKP (от текущей версии до последней доступной) в закрытом окружении необходимо наличие образов последних патч версий для каждой минорной версии платформы.

Доставка образов платформы в закрытое окружение осуществляется либо в виде готовой поставки платформы на USB-носителе, либо с помощью утилиты `dhctl mirror` (требуется доступ в Интернет).

Поставка на USB-носителе включает в себя все необходимые данные для установки обновлений в закрытых окружениях. В состав поставки входят:

- архив с образами контейнеров платформы `d8.tar`, содержащий все необходимые промежуточные версии, начиная от заданной минимальной версии и заканчивая последней доступной.
- манифесты релизов DKP, соответствующие версиям образов поставки, в файле `deckhousereleases.yaml`
- исполняемый файл `dhctl`

В случае использования `dhctl mirror` указанные выше артефакты будут созданы в процессе работы утилиты.

**Важно:** в случае использования `dhctl mirror` необходима версия **1.58.3** платформы.

```bash
# Выполните аутентификацию на registry.deckhouse.ru
docker login -u license-token registry.deckhouse.ru

# Запустите образ установщика версии 1.58.3, указав подходящий каталог рабочей станции для проброса в контейнер 
docker run -ti --pull=always -v $(pwd)/d8-images:/tmp/d8-images registry.deckhouse.ru/deckhouse/ee/install:v1.58.3 bash
```

Подробнее об использовании `dhctl mirror` для выгрузки образов читайте в [документации на сайте](https://deckhouse.ru/documentation/v1/deckhouse-faq.html#%D1%80%D1%83%D1%87%D0%BD%D0%B0%D1%8F-%D0%B7%D0%B0%D0%B3%D1%80%D1%83%D0%B7%D0%BA%D0%B0-%D0%BE%D0%B1%D1%80%D0%B0%D0%B7%D0%BE%D0%B2-%D0%B2-%D0%B8%D0%B7%D0%BE%D0%BB%D0%B8%D1%80%D0%BE%D0%B2%D0%B0%D0%BD%D0%BD%D1%8B%D0%B9-%D0%BF%D1%80%D0%B8%D0%B2%D0%B0%D1%82%D0%BD%D1%8B%D0%B9-registry).

## Подготовительные шаги

1. Убедитесь, что все обновляемые кластеры не имеют заданного канала обновлений `ReleaseChannel`. Чтобы проверить, выполните команду ниже:

```bash
kubectl get mc deckhouse -o yaml | grep releaseChannel
```

В случае, если канал обновлений указан, его необходимо удалить, отредактировав конфигурацию модуля Deckhouse:

```bash
kubectl edit mc deckhouse -o yaml
```

После внесения изменений дождитесь завершения обработки очереди Deckhouse. Проверить очередь можно командой:

```bash
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller queue list
```

2. Переведите установку обновлений платформы в ручной режим. Для этого отредактируйте конфигурацию модуля Deckhouse командой:

```bash
kubectl edit mc deckhouse -o yaml
```

Пример корректной конфигурации модуля Deckhouse после шагов 1 и 2:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"deckhouse.io/v1alpha1","kind":"ModuleConfig","metadata":{"annotations":{},"name":"deckhouse"},"spec":{"settings":{"update":{"mode":"Manual"}},"version":1}}
  creationTimestamp: "2024-03-11T10:28:47Z"
  generation: 3
  name: deckhouse
  resourceVersion: "538605"
  uid: 39114274-a091-4bf0-8506-3a224917a725
spec:
  settings:
    bundle: Default
    logLevel: Info
    update:
      mode: Manual
  version: 1
status:
  state: Enabled
  status: Ready
  type: ""
  version: "1"
```

После внесения изменений дождитесь завершения обработки очереди Deckhouse. Проверить очередь можно командой:

```bash
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller queue list
```

3. Загрузите все образы поставки DKP в реестр образов контейнеров, находящийся в закрытом окружении.

Для этого перейдите в каталог с содержимым поставки и выполните команду:

```bash
./dhctl mirror -i ./d8.tar -r "REGISTRY.EXAMPLE.COM:5000/path/to/deckhouse/ee" -u "ПОЛЬЗОВАТЕЛЬ" -p "ПАРОЛЬ"
```

В случае использования самоподписанных сертификатов для реестра образов контейнеров используйте переменные окружения `SSL_CERT_FILE` и `SSL_CERT_DIR`, чтобы задать пути к СА сертификату и сертификатам реестра образов контейнеров. Пример:

```bash
export SSL_CERT_FILE="/etc/docker/certs.d/REGISTRY.EXAMPLE.COM/registry.example.com.cert"
export SSL_CERT_DIR="/etc/docker/certs.d/REGISTRY.EXAMPLE.COM"
```

Подробнее об использовании `dhctl mirror` для загрузки образов в закрытый реестр образов контейнеров читайте в [документации на сайте](https://deckhouse.ru/documentation/v1/deckhouse-faq.html#%D1%80%D1%83%D1%87%D0%BD%D0%B0%D1%8F-%D0%B7%D0%B0%D0%B3%D1%80%D1%83%D0%B7%D0%BA%D0%B0-%D0%BE%D0%B1%D1%80%D0%B0%D0%B7%D0%BE%D0%B2-%D0%B2-%D0%B8%D0%B7%D0%BE%D0%BB%D0%B8%D1%80%D0%BE%D0%B2%D0%B0%D0%BD%D0%BD%D1%8B%D0%B9-%D0%BF%D1%80%D0%B8%D0%B2%D0%B0%D1%82%D0%BD%D1%8B%D0%B9-registry).

4. Установите канал обновлений, например, `Stable`. Для этого отредактируйте конфигурацию модуля Deckhouse командой:

```bash
kubectl edit mc deckhouse -o yaml
```

Добавьте `releaseChannel: Stable` в блок `settings`.

Пример корректной конфигурации модуля Deckhouse после шага 5:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"deckhouse.io/v1alpha1","kind":"ModuleConfig","metadata":{"annotations":{},"name":"deckhouse"},"spec":{"settings":{"update":{"mode":"Manual"}},"version":1}}
  creationTimestamp: "2024-03-11T10:28:47Z"
  generation: 3
  name: deckhouse
  resourceVersion: "538605"
  uid: 39114274-a091-4bf0-8506-3a224917a725
spec:
  settings:
    bundle: Default
    logLevel: Info
    releaseChannel: Stable
    update:
      mode: Manual
  version: 1
status:
  state: Enabled
  status: Ready
  type: ""
  version: "1"
```

После внесения изменений дождитесь завершения обработки очереди Deckhouse. Проверить очередь можно командой:

```bash
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller queue list
```

5. Загрузите манифесты `DeckhouseReleases` из файла `deckhousereleases.yaml` командой:

```bash
kubectl apply -f deckhousereleases.yaml
```

6. Проверьте наличие релизов Deckhouse командой:

```bash
kubectl get deckhousereleases.deckhouse.io
```

Пример вывода команды:

```text
$ kubectl get deckhousereleases.deckhouse.io 
NAME       PHASE     TRANSITIONTIME   MESSAGE
v1-57-5    Pending   48s              "k8s" requirement for DeckhouseRelease "1.57.5" not met: current kubernetes version is lower then required
v1.45.11   Pending   4s               Waiting for manual approval
v1.46.12   Pending   34s              
v1.47.5    Pending   34s              
v1.48.9    Pending   34s              
v1.49.6    Pending   34s              
v1.50.6    Pending   34s              
v1.51.10   Pending   34s              
v1.52.10   Pending   34s              
v1.53.3    Pending   34s              
v1.54.7    Pending   34s              
v1.55.7    Pending   34s              
v1.56.9    Pending   34s              
v1.57.5    Pending   34s              
v1.58.3    Pending   34s
```

**Важно:** в случае обнаружения в списке релиза с нестандартным названием без точек (из примера выше: `v1-57-5`) удалите его командой:

```bash
kubectl delete deckhousereleases v1-57-5
```

## Установка обновлений

Так как установка обновлений осуществляется в ручном режиме, необходимо вручную одобрять каждый устанавливаемый релиз.

В среднем установка каждого релиза занимает около 30 минут для кластера с 3-мя мастерами и 2-мя воркерами.

1. Получите список доступных релизов Deckhouse командой:

```bash
kubectl get deckhousereleases.deckhouse.io
```

Пример вывода команды:

```text
$ kubectl get deckhousereleases.deckhouse.io 
NAME       PHASE     TRANSITIONTIME   MESSAGE
v1.45.11   Pending   4s               Waiting for manual approval
v1.46.12   Pending   34s              
v1.47.5    Pending   34s              
v1.48.9    Pending   34s              
v1.49.6    Pending   34s              
v1.50.6    Pending   34s              
v1.51.10   Pending   34s              
v1.52.10   Pending   34s              
v1.53.3    Pending   34s              
v1.54.7    Pending   34s              
v1.55.7    Pending   34s              
v1.56.9    Pending   34s              
v1.57.5    Pending   34s              
v1.58.3    Pending   34s
```

2. Найдите в списке релиз с сообщением `Waiting for manual approval`, либо зайдите в раздел Alerts сервиса Prometheus и найдите алерт `DeckhouseReleaselsWaitingManualApproval`. Развернув этот алерт, можно будет узнать ожидаемый для одобрения релиз.

3. Проверьте, что ваш кластер соответствует требованиям для выполнения обновлений. Для этого выполните команду ниже и ознакомьтесь с секцией Requirements:

```bash
kubectl describe deckhouserelease ВЕРСИЯ_РЕЛИЗА
```

Пример вывода команды выше для релиза `v1.45.11`:

```yaml
Name:         v1.45.11
Namespace:    
Labels:       <none>
Annotations:  <none>
API Version:  deckhouse.io/v1alpha1
Approved:     false
Kind:         DeckhouseRelease
Metadata:
  Creation Timestamp:  2024-03-11T13:07:04Z
  Generation:          1
  Managed Fields:
    API Version:  deckhouse.io/v1alpha1
    Fields Type:  FieldsV1
    fieldsV1:
      f:approved:
      f:metadata:
        f:annotations:
          .:
          f:kubectl.kubernetes.io/last-applied-configuration:
      f:spec:
        .:
        f:changelog:
          .:
          f:helm:
            .:
            f:fixes:
          f:ingress-nginx:
            .:
            f:fixes:
        f:changelogLink:
        f:requirements:
          .:
          f:ingressNginx:
          f:k8s:
          f:nodesMinimalOSVersionUbuntu:
        f:version:
    Manager:      kubectl-client-side-apply
    Operation:    Update
    Time:         2024-03-11T13:07:04Z
    API Version:  deckhouse.io/v1alpha1
    Fields Type:  FieldsV1
    fieldsV1:
      f:status:
        .:
        f:approved:
        f:message:
        f:phase:
        f:transitionTime:
    Manager:         deckhouse-controller
    Operation:       Update
    Subresource:     status
    Time:            2024-03-11T13:07:15Z
  Resource Version:  124704
  UID:               bdde9d57-6d94-47e3-8316-c038081b01ed
Spec:
  Changelog:
    Helm:
      Fixes:
        pull_request:  https://github.com/deckhouse/deckhouse/pull/4751
        Summary:       Fix deprecated k8s resources metrics.
    Ingress - Nginx:
      Fixes:
        pull_request:  https://github.com/deckhouse/deckhouse/pull/4734
        Summary:       Add protection for ingress-nginx-controller daemonset migration.
  Changelog Link:      https://github.com/deckhouse/deckhouse/releases/tag/v1.45.11
  Requirements:
    Ingress Nginx:                    1.1
    k8s:                              1.22.0
    Nodes Minimal OS Version Ubuntu:  18.04
  Version:                            v1.45.11
Status:
  Approved:         false
  Message:          Waiting for manual approval
  Phase:            Pending
  Transition Time:  2024-03-11T13:10:00.117064727Z
Events:             <none>
```

4. Если все требования соблюдены, одобрите установку обновлений, выполнив команду:

```bash
kubectl patch DeckhouseRelease ВЕРСИЯ_РЕЛИЗА --type=merge -p='{"approved": true}'
```

5. Дождитесь завершения установки релиза. Определить успешность операции можно по следующим признакам:

- в разделе Alerts сервиса Prometheus погас алерт `DeckhouseUpdating`
- в Grafana отображается желаемая версия Deckhouse
- в очереди Deckhouse нет задач для обработки
- релиз перешел из статуса `Pending` в статус `Deployed`

Пример вывода для установленного релиза `v1.45.11`:

```text
$ kubectl get deckhousereleases
NAME       PHASE      TRANSITIONTIME   MESSAGE
v1.45.11   Deployed   55s              
v1.46.12   Pending    10s              Waiting for manual approval
v1.47.5    Pending    5m55s            
v1.48.9    Pending    5m...
```
