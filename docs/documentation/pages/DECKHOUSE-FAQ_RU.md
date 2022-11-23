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

> Документация доступна внутри кластера при включенном модуле [deckhouse-web](modules/810-deckhouse-web/) (включен по умолчанию, кроме [варианта поставки](modules/002-deckhouse/configuration.html#parameters-bundle) `Minimal`).

Документация запущенной в кластере версии Deckhouse доступна по адресу `deckhouse.<cluster_domain>`, где `<cluster_domain>` — DNS имя в соответствии с шаблоном из параметра `global.modules.publicDomainTemplate` конфигурации.

## Как установить желаемый канал обновлений?

Чтобы перейти на другой канал обновлений автоматически, нужно в [конфигурации](modules/002-deckhouse/configuration.html#parameters-releasechannel) модуля `deckhouse` изменить
(установить) параметр `releaseChannel`.

В этом случае включится механизм [автоматической стабилизации релизного канала](#как-работает-автоматическое-обновление-deckhouse).

Пример конфигурации модуля:

```yaml
deckhouse: |
  releaseChannel: Stable
```

## Как отключить автоматическое обновление?

Чтобы полностью отключить механизм обновления Deckhouse, удалите в [конфигурации](modules/002-deckhouse/configuration.html#parameters-releasechannel) модуля `deckhouse` параметр `releaseChannel`.

В этом случае Deckhouse не проверяет обновления, и даже обновление на patch-релизы не выполняется.

> Крайне не рекомендуется отключать автоматическое обновление! Это заблокирует обновления на patch-релизы, которые могут содержать исправления критических уязвимостей и ошибок.

## Как работает автоматическое обновление Deckhouse?

При указании в конфигурации модуля `deckhouse` параметра `releaseChannel`, Deckhouse будет каждую минуту проверять данные о релизе на канале обновлений.

При появлении нового релиза Deckhouse скачивает его в кластер и создает custom resource `DeckhouseRelease`.

После появления custom resource'а `DeckhouseRelease` в кластере Deckhouse выполняет обновление на соответствующую версию согласно установленным [параметрам обновления](modules/002-deckhouse/configuration.html#parameters-update) (по-умолчанию — автоматически, в любое время).

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
  * Затем Deckhouse выполняет обновление согласно установленным [параметрам обновления](modules/002-deckhouse/configuration.html#parameters-update).

## Как запускать Deckhouse на произвольном узле?

Для запуска Deckhouse на произвольном узле установите у модуля `deckhouse` соответствующий [параметр](modules/002-deckhouse/configuration.html) `nodeSelector` и не задавайте `tolerations`.  Необходимые значения `tolerations` в этом случае будут проставлены автоматически.

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
* `registryScheme` — протокол доступа к registry (`HTTP` или `HTTPS`). По умолчанию - `HTTPS`.

### Особенности настройки сторонних registry

**Внимание:** Deckhouse поддерживает работу только с Bearer token-схемой авторизации в registry.

#### Nexus

##### Требования

При использовании менеджера репозиториев [Nexus](https://github.com/sonatype/nexus-public) должны быть выполнены следующие требования:

* Включен `Docker Bearer Token Realm`.
* Создан проксирующий репозиторий Docker.
* Параметр `Allow anonymous docker pull` должен быть включен.
* Должен быть настроен контроль доступа:
  * Создана роль Nexus с полномочиями `nx-repository-view-docker-<репозиторий>-browse` и `nx-repository-view-docker-<репозиторий>-read`.
  * Создан пользователь Nexus с ролью, созданной выше.
* Параметр `Maximum metadata age` созданного репозитория должен быть установлен в 0.

##### Настройка

* Включите `Docker Bearer Token Realm`:
  ![Включение `Docker Bearer Token Realm`](images/registry/nexus/nexus-realm.png)

* Создайте проксирующий репозиторий Docker, указывающий на [Deckhouse registry](https://registry.deckhouse.io/):
  ![Создание проксирующего репозитория Docker](images/registry/nexus/nexus-repository.png)

* Заполните поля страницы создания следующим образом:
  * `Name` должно содержать имя создаваемого репозитория, например `d8-proxy`.
  * `Repository Connectors / HTTP` или `Repository Connectors / HTTPS` должно содержать выделенный порт для создаваемого репозитория, например, `8123` или иной.
  * `Allow anonymous docker pull` должно быть включено, чтобы [работала](https://help.sonatype.com/repomanager3/system-configuration/user-authentication#UserAuthentication-security-realms) авторизация с помощью Bearer-токенов, при этом анонимный доступ [не будет работать](https://help.sonatype.com/repomanager3/nexus-repository-administration/formats/docker-registry/docker-authentication#DockerAuthentication-UnauthenticatedAccesstoDockerRepositories), если он не был явно включен в Settings -> Security -> Anonymous Access, и пользователю `anonymous` не были даны права на доступ к репозиторию.
  * `Remote storage` должно иметь значение `https://registry.deckhouse.io/`.
  * `Auto blocking enabled` и `Not found cache enabled` могут быть выключены для отладки; в противном случае их следует включить.
  * `Maximum Metadata Age` должно быть равно 0.
  * Если планируется использовать Deckhouse Enterprise Edition, флажок `Authentication` должен быть включен, а связанные поля должны быть заполнены следующим образом:
    * `Authentication Type` должно иметь значение `Username`.
    * `Username` должно иметь значение `license-token`.
    * `Password` должно содержать ключ лицензии Deckhouse Enterprise Edition.

  ![Пример настроек репозитория 1](images/registry/nexus/nexus-repo-example-1.png)
  ![Пример настроек репозитория 2](images/registry/nexus/nexus-repo-example-2.png)
  ![Пример настроек репозитория 3](images/registry/nexus/nexus-repo-example-3.png)

* Настройте контроль доступа Nexus для доступа Deckhouse к созданному репозиторию:
  * Создайте роль Nexus с полномочиями `nx-repository-view-docker-<репозиторий>-browse` и `nx-repository-view-docker-<репозиторий>-read`.
    ![Создание роли Nexus](images/registry/nexus/nexus-role.png)
  * Создайте пользователя Nexus с ролью, созданной выше.
    ![Создание пользователя Nexus](images/registry/nexus/nexus-user.png)

#### Harbor

Необходимо использовать такой функционал [Harbor](https://github.com/goharbor/harbor), как Proxy Cache.

* Настройте Registry:
  * `Administration -> Registries -> New Endpoint`.
  * `Provider`: `Docker Registry`.
  * `Name` — укажите любое, на ваше усмотрение.
  * `Endpoint URL`: `https://registry.deckhouse.io`.
  * Укажите `Access ID` и `Access Secret`, если используете Deckhouse Enterprise Edition, иначе оставьте пустыми.

![Настройка Registry](images/registry/harbor/harbor1.png)

* Создайте новый проект:
  * `Projects -> New Project`.
  * `Project Name` будет частью URL. Используйте любой, например, `d8s`.
  * `Access Level`: `Public`.
  * `Proxy Cache` — включите и выберите в списке Registry, созданный на предыдущем шаге.

![Создание нового проекта](images/registry/harbor/harbor2.png)

В результате образы Deckhouse будут доступны по адресу, например, `https://your-harbor.com/d8s/deckhouse/{d8s-edition}:{d8s-version}`.

### Ручная загрузка образов в изолированный приватный registry

Загрузите скрипт на хост, с которого есть доступ до `registry.deckhouse.io`. `Docker`, `crane` и `jq` должны быть установлены на хосте.

```shell
curl -fsSL -o d8-pull.sh https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/release/d8-pull.sh
chmod 700 d8-pull.sh
```

Пример вызова команды скачивания образов:

```shell
./d8-pull.sh --license YOUR_DECKHOUSE_LICENSE_KEY --output-dir /your/output-dir/
```

Загрузите директорию с образами, полученную на предыдущем шаге, на хост, с которого есть доступ до изолированного приватного registry. `Crane` должен быть установлен на хосте.
Загрузите скрипт на этот хост:

```shell
curl -fsSL -o d8-push.sh https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/release/d8-push.sh
chmod 700 d8-push.sh
```

Пример вызова команды загрузки образов в изолированный приватный registry:

```shell
./d8-push.sh --source-dir /your/source-dir/ --path your.private.registry.com/deckhouse --username YOUR_USERNAME --password YOUR_PASSWORD
```

## Как создать кластер и запустить Deckhouse без использования каналов обновлений?

Данный способ следует использовать только в случае, если в изолированном приватном registry нет образов, содержащих информацию о каналах обновлений.

* Если вы хотите создать кластер, то вы должны использовать точный тег образа Deckhouse, чтобы установить Deckhouse Platform.
Например, если вы хотите установить релиз v1.32.13, то вы должны использовать образ `your.private.registry.com/deckhouse/install:v1.32.13`. Также вы должны указать `devBranch: v1.32.13` вместо `releaseChannel: XXX` в `config.yml`.
* Если у вас уже есть рабочий кластер, то вы должны удалить `releaseChannel` из ConfigMap `d8-system/deckhouse` и указать выбранный образ Deckhouse в поле `image` в Deployment `d8-system/deckhouse`. Дальнейшее обновление необходимо производить также изменяя образ вручную.
Например, для релиза v1.32.13 следует указывать в поле `image` значение `your.private.registry.com/deckhouse:v1.32.13`.

## Как переключить работающий кластер Deckhouse на использование стороннего registry?

Для переключения кластера Deckhouse на использование стороннего registry выполните следующие действия:

* Измените поле `image` в Deployment `d8-system/deckhouse` на адрес образа Deckhouse в новом registry;
* Скачайте скрипт на мастер-узел и запустите его с параметрами нового registry.
  * Пример запуска:

  ```shell
  curl -fsSL -o change-registry.sh https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/change-registry.sh
  chmod 700 change-registry.sh
  ./change-registry.sh --registry-url https://my-new-registry/deckhouse --user my-user --password my-password
  ```

  * Если registry использует самоподписные сертификаты, то положите корневой сертификат соответствующего сертификата registry в файл `ca.crt` возле скрипта и добавьте к вызову опцию `--ca-file ca.crt`.
* Дождитесь перехода Pod'а Deckhouse в статус `Ready`. Если Pod будет находиться в статусе `ImagePullBackoff`, то перезапустите его.
* Дождитесь применения bashible новых настроек на master-узле. В журнале bashible на master-узле (`journalctl -u bashible`) должно появится сообщение `Configuration is in sync, nothing to do`.
* Если необходимо отключить автоматическое обновление Deckhouse через сторонний registry, то удалите параметр `releaseChannel` из ConfigMap `d8-system/deckhouse`.
* Проверьте, не осталось ли в кластере Pod'ов с оригинальным адресом registry:

  ```shell
  kubectl get pods -A -o json | jq '.items[] | select(.spec.containers[] | select((.image | contains("deckhouse.io")))) | .metadata.namespace + "\t" + .metadata.name' -r
  ```

## Как изменить конфигурацию кластера

Общие параметры кластера хранятся в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration).

Чтобы изменить общие параметры кластера, выполните команду:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit cluster-configuration
```

После сохранения изменений Deckhouse приведет конфигурацию кластера к измененному состоянию. В зависимости от размеров кластера это может занять какое-то время.

## Как изменить конфигурацию облачного провайдера в кластере?

Настройки используемого облачного провайдера в облачном или гибридном кластере хранятся в структуре `<PROVIDER_NAME>ClusterConfiguration`, где `<PROVIDER_NAME>` — название/код провайдера. Например, для провайдера OpenStack структура будет называться [OpenStackClusterConfiguration]({% if site.mode == 'local' and site.d8Revision == 'CE' %}{{ site.urls[page.lang] }}/documentation/v1/{% endif %}modules/030-cloud-provider-openstack/cluster_configuration.html).

Независимо от используемого облачного провайдера, его настройки можно изменить с помощью команды:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

## Как изменить конфигурацию статического кластера?

Настройки статического кластера хранятся в структуре [StaticClusterConfiguration](installing/configuration.html#staticclusterconfiguration).

Чтобы изменить параметры статического кластера, выполните команду:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit static-cluster-configuration
```

## Как обновить версию Kubernetes в кластере?

Чтобы обновить версию Kubernetes в кластере, измените параметр [kubernetesVersion](installing/configuration.html#parameters-kubernetesversion) в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration) выполнив следующие шаги:
1. Выполните команду:

   ```shell
   kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit cluster-configuration
   ```

1. Измените параметр `kubernetesVersion`.
1. Сохраните изменения. Узлы кластера начнут последовательно обновляться.
1. Дождитесь окончания обновления.  Отслеживать ход обновления можно с помощью команды `kubectl get no`. Обновление можно считать завершенным, когда в выводе команды у каждого узла кластера в колонке `VERSION` появится обновленная версия.
