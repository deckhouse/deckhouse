---
title: FAQ
permalink: ru/deckhouse-faq.html
lang: ru
---

## Как узнать все параметры Deckhouse?

Все ключевые настройки конфигурации Deckhouse, включая параметры модулей, хранятся в ConfigMap'е `deckhouse` пространства имён `d8-system`.

Чтобы посмотреть конфигурацию Deckhouse, воспользуйтесь командной:

```shell
kubectl -n d8-system get cm deckhouse -o yaml
```

## Как найти документацию по установленной у меня версии?

> Документация доступна внутри кластера при включенном модуле [deckhouse-web](modules/810-deckhouse-web/) (включен по умолчанию, кроме [варианта поставки](modules/020-deckhouse/configuration.html#parameters-bundle) `Minimal`).

Документация запущенной в кластере версии Deckhouse доступна по адресу `deckhouse.<cluster_domain>`, где `<cluster_domain>` — DNS имя в соответствии с шаблоном из параметра `global.modules.publicDomainTemplate` конфигурации.

## Как установить желаемый канал обновлений?

Чтобы перейти на другой канал обновлений автоматически, нужно в [конфигурации](modules/020-deckhouse/configuration.html#parameters-releasechannel) модуля `deckhouse` изменить
(установить) параметр `releaseChannel`.

В этом случае включится механизм [автоматической стабилизации релизного канала](#как-работает-автоматическое-обновление-deckhouse).

Пример конфигурации модуля:

```yaml
deckhouse: |
  releaseChannel: Stable
```

## Как отключить автоматическое обновление?

Чтобы полностью отключить механизм обновления Deckhouse, удалите в [конфигурации](modules/020-deckhouse/configuration.html#parameters-releasechannel) модуля `deckhouse` параметр `releaseChannel`.

В этом случае Deckhouse не проверяет обновления, и даже обновление на patch-релизы не выполняется.

> Крайне не рекомендуется отключать автоматическое обновление! Это заблокирует обновления на patch-релизы, которые могут содержать исправления критических уязвимостей и ошибок.

## Как работает автоматическое обновление Deckhouse?

При указании в конфигурации модуля `deckhouse` параметра `releaseChannel`, Deckhouse будет каждую минуту проверять данные о релизе на канале обновлений.

При появлении нового релиза Deckhouse скачивает его в кластер и создает custom resource `DeckhouseRelease`.

После появления custom resource'а `DeckhouseRelease` в кластере Deckhouse выполняет обновление на соответствующую версию согласно установленным [параметрам обновления](modules/020-deckhouse/configuration.html#parameters-update) (по-умолчанию — автоматически, в любое время).

Чтобы посмотреть список и состояние всех релизов, воспользуйтесь командной:

```shell
kubectl get deckhousereleases
```

> Patch-релизы (например, обновление на версию `1.30.2` при установленной версии `1.30.1`) устанавливаются без учета режима и окон обновления, т.е. при появлении на канале обновления patch-релиза, он всегда будет установлен.

### Что происходит при смене канала обновлений?

* При смене канала обновлений на **более стабильный** (например с `Alpha` на `EarlyAccess`) Deckhouse скачивает данные о релизе (в примере — из канала `EarlyAccess`) и сравнивает их с данными из существующих в кластере custom resouce'ов `DeckhouseRelease`:
  * Более *поздние* релизы, которые еще не были применены (в статусе `Pending`), — удаляются.
  * Если более *поздние* релизы уже применены (в статусе `Deployed`), то смены релиза не происходит. В этом случае Deckhouse останется на таком релизе до тех пор, пока на канале обновлений `EarlyAccess` не появится более поздний релиз.
* При смене канала обновлений на **менее стабильный** (например с `EarlyAcess` на `Alpha`):
  * Deckhouse скачивает данные о релизе (в примере — из канала `Alpha`) и сравнивает их с данными из существующих в кластере custom resource'ов `DeckhouseRelease`.
  * Затем Deckhouse выполняет обновление согласно установленным [параметрам обновления](modules/020-deckhouse/configuration.html#parameters-update).

## Как запускать Deckhouse на произвольном узле?

Для запуска Deckhouse на произвольном узле установите у модуля `deckhouse` соответствующий [параметр](modules/020-deckhouse/configuration.html) `nodeSelector` и не задавайте `tolerations`.  Необходимые значения `tolerations` в этом случае будут проставлены автоматически.

Также стоит избегать использования узлов **CloudEphemeral**. В противном случае может произойти ситуация, когда целевого узла нет в кластере, и его заказ по какой-то причине невозможен.

Пример конфигурации модуля:

```yaml
deckhouse: |
  nodeSelector:
    node-role.deckhouse.io/deckhouse: ""
```

## Как установить Deckhouse из стороннего registry?

При установке Deckhouse можно настроить на работу со сторонним registry (например, проксирующий registry внутри закрытого контура).

### Подготовка конфигурации

Установите следующие параметры в ресурсе `InitConfiguration`:

* `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/<DECKHOUSE_REVISION>` — адрес образа Deckhouse в стороннем registry с учетом используемой редакции - CE или EE. Пример: `imagesRepo: registry.deckhouse.io/deckhouse/ce`;
* `registryDockerCfg: <BASE64>` — права доступа к стороннему registry, зашифрованные в Base64.

Если разрешен анонимный доступ к образам Deckhouse в стороннем registry, то `registryDockerCfg` должен выглядеть следующим образом:

```json
{"auths": { "<PROXY_REGISTRY>": {}}}
```

Приведенное значение должно быть закодировано в Base64.

Если для доступа к образам Deckhouse в стороннем registry необходима аутентификация, то `registryDockerCfg` должен выглядеть следующим образом:

```json
{"auths": { "<PROXY_REGISTRY>": {"username":"<PROXY_USERNAME>","password":"<PROXY_PASSWORD>","auth":"<AUTH_BASE64>"}}}
```

где:

* `<PROXY_USERNAME>` — имя пользователя для аутентификации на `<PROXY_REGISTRY>`;
* `<PROXY_PASSWORD>` — пароль пользователя для аутентификации на `<PROXY_REGISTRY>`;
* `<PROXY_REGISTRY>` — адрес стороннего registry в виде `<HOSTNAME>[:PORT]`;
* `<AUTH_BASE64>` — строка вида `<PROXY_USERNAME>:<PROXY_PASSWORD>`, закодированная в Base64.

Итоговое значение для `registryDockerCfg` должно быть также закодировано в Base64.

Для настройки нестандартных конфигураций сторонних registry в ресурсе `InitConfiguration` предусмотрены еще два параметра:

* `registryCA` — корневой сертификат, которым можно проверить сертификат registry (если registry использует самоподписанные сертификаты);
* `registryScheme` — протокол доступа к registry (`http` или `https`). По умолчанию - `https`.

### Особенности настройки сторонних registry

**Внимание:** Deckhouse поддерживает работу только с Bearer token-схемой авторизации в registry.

#### Nexus

При использовании [Nexus](https://github.com/sonatype/nexus-public) в режиме registry-прокси необходимо соблюдение нескольких условий:

* Включить `Docker Bearer Token Realm`:
![](images/registry/nexus/nexus1.png)

* Включить анонимный доступ к registry (иначе [не будет работать](https://help.sonatype.com/repomanager3/system-configuration/user-authentication#UserAuthentication-security-realms) Bearer token-авторизация):
![](images/registry/nexus/nexus2.png)

* Установить `Maximum metadata age` в 0 (иначе автоматическое обновление Deckhouse не будет работать корректно из-за кеширования):
![](images/registry/nexus/nexus3.png)

#### Harbor

Необходимо использовать такой функционал [Harbor](https://github.com/goharbor/harbor), как Proxy Cache.

* Настройте Registry:
  * `Administration -> Registries -> New Endpoint`.
  * `Provider`: `Docker Registry`.
  * `Name` — укажите любое, на ваше усмотрение.
  * `Endpoint URL`: `https://registry.deckhouse.io`.
  * Укажите `Access ID` и `Access Secret`, если используете Deckhouse Enterprise Edition, иначе оставьте пустыми.

![](images/registry/harbor/harbor1.png)

* Создайте новый проект:
  * `Projects -> New Project`.
  * `Project Name` будет частью URL. Используйте любой, например, `d8s`.
  * `Access Level`: `Public`.
  * `Proxy Cache` — включите и выберите в списке Registry, созданный на предыдущем шаге.

![](images/registry/harbor/harbor2.png)

В результате образы Deckhouse будут доступны по адресу, например, `https://your-harbor.com/d8s/deckhouse/{d8s-edition}:{d8s-version}`.

## Как переключить работающий кластер Deckhouse на использование стороннего registry?

Для переключения кластера Deckhouse на использование стороннего registry необходимо выполнить следующие действия:

* Изменить поле `image` в Deployment `d8-system/deckhouse` на адрес образа Deckhouse в новом registry;
* Изменить Secret `d8-system/deckhouse-registry` (все параметры хранятся в кодировке Base64):
  * Исправить `.dockerconfigjson` с учетом авторизации в новом registry.
  * Исправить `address` на адрес нового registry (например, `registry.example.com`).
  * Исправить `path` на путь к репозиторию Deckhouse в новом registry (например, `/deckhouse/ee`).
  * При необходимости изменить `scheme` на `http` (если используется HTTP registry).
  * Если registry использует самоподписные сертификаты, то изменить или добавить поле `ca`, куда внести корневой сертификат соответствующего сертификата registry;
* Дождаться перехода Pod'а Deckhouse в статус `Ready`. Если Pod будет находиться в статусе `ImagePullBackoff`, то перезапустите его.
* Дождаться применения bashible новых настроек на master-узле. В журнале bashible на master-узле (`journalctl -u bashible`) должно появится сообщение `Configuration is in sync, nothing to do`.
* Только если обновление Deckhouse через сторонний registry не планируется, то следует удалить `releaseChannel` из конфигмапа `d8-system/deckhouse`.
