---
title: FAQ
permalink: ru/deckhouse-faq.html
lang: ru
---

## Как узнать все параметры Deckhouse?

Deckhouse настраивается с помощью глобальных настроек, настроек модулей и различных сustom resource’ов. Подробнее — [в документации](./).

Вывести глобальные настройки:

```shell
kubectl get mc global -o yaml
```

Вывести список состояния всех модулей (доступно для Deckhouse версии 1.47+):

```shell
kubectl get modules
```

Вывести настройки модуля `user-authn`:

```shell
kubectl get moduleconfigs user-authn -o yaml
```

## Как найти документацию по установленной у меня версии?

Документация запущенной в кластере версии Deckhouse доступна по адресу `documentation.<cluster_domain>`, где `<cluster_domain>` — DNS-имя в соответствии с шаблоном из параметра [modules.publicDomainTemplate](deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) глобальной конфигурации.

{% alert level="warning" %}
Документация доступна, если в кластере включен модуль [documentation](modules/810-documentation/). Он включен по умолчанию, кроме [варианта поставки](modules/002-deckhouse/configuration.html#parameters-bundle) `Minimal`.
{% endalert %}

## Обновление Deckhouse

### Как понять, в каком режиме обновляется кластер?

Посмотреть режим обновления кластера можно в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse`. Для этого выполните следующую команду:

```shell
kubectl get mc deckhouse -oyaml
```

Пример вывода:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  creationTimestamp: "2022-12-14T11:13:03Z"
  generation: 1
  name: deckhouse
  resourceVersion: "3258626079"
  uid: c64a2532-af0d-496b-b4b7-eafb5d9a56ee
spec:
  settings:
    releaseChannel: Stable
    update:
      windows:
      - days:
        - Mon
        from: "19:00"
        to: "20:00"
  version: 1
status:
  state: Enabled
  status: ""
  type: Embedded
  version: "1"
```

Существуют три возможных режима обновления:
* **Автоматический + окна обновлений не заданы.** Кластер обновится сразу после появления новой версии на соответствующем [канале обновлений](https://deckhouse.ru/documentation/deckhouse-release-channels.html).
* **Автоматический + заданы окна обновлений.** Кластер обновится в ближайшее доступное окно после появления новой версии на канале обновлений.
* **Ручной режим.** Для применения обновления требуются [ручные действия](modules/002-deckhouse/usage.html#ручное-подтверждение-обновлений).

### Как установить желаемый канал обновлений?

Чтобы перейти на другой канал обновлений автоматически, нужно в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse` изменить (установить) параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel).

В этом случае включится механизм [автоматической стабилизации релизного канала](#как-работает-автоматическое-обновление-deckhouse).

Пример конфигурации модуля `deckhouse` с установленным каналом обновлений `Stable`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
```

### Как отключить автоматическое обновление?

Чтобы полностью отключить механизм обновления Deckhouse, удалите в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse` параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel).

В этом случае Deckhouse не проверяет обновления и даже обновление на patch-релизы не выполняется.

{% alert level="danger" %}
Крайне не рекомендуется отключать автоматическое обновление! Это заблокирует обновления на patch-релизы, которые могут содержать исправления критических уязвимостей и ошибок.
{% endalert %}

### Как применить обновление минуя окна обновлений?

Чтобы применить обновление немедленно, не дожидаясь ближайшего окна обновлений, установите в соответствующем ресурсе [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease) аннотацию `release.deckhouse.io/apply-now: "true"`.

Пример команды установки аннотации пропуска окон обновлений для версии `v1.56.2`:

```shell
kubectl annotate deckhousereleases v1.56.2 release.deckhouse.io/apply-now="true"
```

Пример ресурса с установленной аннотацией пропуска окон обновлений:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  annotations:
    release.deckhouse.io/apply-now: "true"
...
```

### Как понять, какие изменения содержит обновление и как это повлияет на работу кластера?

Информацию о всех версиях Deckhouse можно найти в [списке релизов](https://github.com/deckhouse/deckhouse/releases) Deckhouse.

Сводную информацию о важных изменениях, об обновлении версий компонентов, а также о том, какие компоненты в кластере буду перезапущены в процессе обновления, можно найти в описании нулевой patch-версии релиза. Например, [v1.46.0](https://github.com/deckhouse/deckhouse/releases/tag/v1.46.0) для релиза v1.46 Deckhouse.

Подробный список изменений можно найти в Changelog, ссылка на который есть в каждом [релизе](https://github.com/deckhouse/deckhouse/releases).

### Как понять, что в кластере идет обновление?

Во время обновления:
- горит алерт `DeckhouseUpdating`;
- под `deckhouse` не в статусе `Ready`. Если под долго не переходит в статус `Ready`, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.

### Как понять, что обновление прошло успешно?

Если алерт `DeckhouseUpdating` погас, значит, обновление завершено.

Вы также можете проверить состояние [релизов](modules/002-deckhouse/cr.html#deckhouserelease) Deckhouse.

Пример:

```console
$ kubectl get deckhouserelease
NAME       PHASE        TRANSITIONTIME   MESSAGE
v1.46.8    Superseded   13d              
v1.46.9    Superseded   11d              
v1.47.0    Superseded   4h12m            
v1.47.1    Deployed     4h12m            
```

Статус `Deployed` у соответствующей версии говорит о том, что переключение на соответствующую версию было выполнено (но это не значит, что оно закончилось успешно).

Проверьте состояние пода Deckhouse:

```shell
$ kubectl -n d8-system get pods -l app=deckhouse
NAME                   READY  STATUS   RESTARTS  AGE
deckhouse-7844b47bcd-qtbx9  1/1   Running  0       1d
```

* Если статус пода `Running` и в колонке READY указано `1/1` — обновление закончилось успешно.
* Если статус пода `Running` и в колонке READY указано `0/1` — обновление еще не закончилось. Если это продолжается более 20–30 минут, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.
* Если статус пода не `Running`, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.

{% alert level="info" %}
Возможные варианты действий, если что-то пошло не так:
- Проверьте логи, используя следующую команду:

  ```shell
  kubectl -n d8-system logs -f -l app=deckhouse | jq -Rr 'fromjson? | .msg'
  ```

- Соберите [отладочную информацию](modules/002-deckhouse/faq.html#как-собрать-информацию-для-отладки) и свяжитесь с технической поддержкой.
- Попросите помощи у [сообщества](https://deckhouse.ru/community/about.html).
{% endalert %}

### Как узнать, что для кластера доступна новая версия?

Как только на установленном в кластере канале обновления появляется новая версия Deckhouse:
- Загорается алерт `DeckhouseReleaseIsWaitingManualApproval`, если кластер использует ручной режим обновлений (параметр [update.mode](modules/002-deckhouse/configuration.html#parameters-update-mode) установлен в `Manual`).
- Появляется новый custom resource [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease). Используйте команду `kubectl get deckhousereleases`, чтобы посмотреть список релизов. Если `DeckhouseRelease` новой версии находится в состоянии `Pending`, указанная версия еще не установлена. Возможные причины, при которых `DeckhouseRelease` может находиться в `Pending`:
  - Установлен ручной режим обновлений (параметр [update.mode](modules/002-deckhouse/configuration.html#parameters-update-mode) установлен в `Manual`).
  - Установлен автоматический режим обновлений и настроены [окна обновлений](modules/002-deckhouse/usage.html#конфигурация-окон-обновлений), интервал которых еще не наступил.
  - Установлен автоматический режим обновлений, окна обновлений не настроены, но применение версии отложено на случайный период времени из-за механизма снижения нагрузки на репозиторий образов контейнеров. В поле `status.message` ресурса `DeckhouseRelease` будет соответствующее сообщение.
  - Установлен параметр [update.notification.minimalNotificationTime](modules/002-deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) и указанное в нем время еще не прошло.

### Как заранее получать информацию о предстоящем обновлении?

Получать заранее информацию об обновлении минорных версий Deckhouse на канале обновлений можно следующими способами:
- Настроить ручной [режим обновлений](modules/002-deckhouse/configuration.html#parameters-update-mode). В этом случае при появлении новой версии на канале обновлений загорится алерт `DeckhouseReleaseIsWaitingManualApproval` и в кластере появится новый custom resource [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease).
- Настроить автоматический [режим обновлений](modules/002-deckhouse/configuration.html#parameters-update-mode) и указать минимальное время в параметре [minimalNotificationTime](modules/002-deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime), на которое будет отложено обновление. В этом случае при появлении новой версии на канале обновлений в кластере появится новый custom resource [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease). А если указать URL в параметре [update.notification.webhook](modules/002-deckhouse/configuration.html#parameters-update-notification-webhook), дополнительно произойдет вызов webhook'а.

### Как узнать, какая версия Deckhouse находится на каком канале обновлений?

Информацию о том, какая версия Deckhouse находится на каком канале обновлений, можно получить на <https://flow.deckhouse.io>.

### Как работает автоматическое обновление Deckhouse?

При указании в конфигурации модуля `deckhouse` параметра `releaseChannel` Deckhouse будет каждую минуту проверять данные о релизе на канале обновлений.

При появлении нового релиза Deckhouse скачивает его в кластер и создает custom resource [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease).

После появления custom resource'а `DeckhouseRelease` в кластере Deckhouse выполняет обновление на соответствующую версию согласно установленным [параметрам обновления](modules/002-deckhouse/configuration.html#parameters-update) (по умолчанию — автоматически, в любое время).

Чтобы посмотреть список и состояние всех релизов, воспользуйтесь командной:

```shell
kubectl get deckhousereleases
```

{% alert %}
Patch-релизы (например, обновление на версию `1.30.2` при установленной версии `1.30.1`) устанавливаются без учета режима и окон обновления, то есть при появлении на канале обновления patch-релиза он всегда будет установлен.
{% endalert %}

### Что происходит при смене канала обновлений?

* При смене канала обновлений на **более стабильный** (например, с `Alpha` на `EarlyAccess`) Deckhouse скачивает данные о релизе (в примере — из канала `EarlyAccess`) и сравнивает их с данными из существующих в кластере custom resource'ов `DeckhouseRelease`:
  * Более *поздние* релизы, которые еще не были применены (в статусе `Pending`), удаляются.
  * Если более *поздние* релизы уже применены (в статусе `Deployed`), смены релиза не происходит. В этом случае Deckhouse останется на таком релизе до тех пор, пока на канале обновлений `EarlyAccess` не появится более поздний релиз.
* При смене канала обновлений на **менее стабильный** (например, с `EarlyAcess` на `Alpha`):
  * Deckhouse скачивает данные о релизе (в примере — из канала `Alpha`) и сравнивает их с данными из существующих в кластере custom resource'ов `DeckhouseRelease`.
  * Затем Deckhouse выполняет обновление согласно установленным [параметрам обновления](modules/002-deckhouse/configuration.html#parameters-update).

{% offtopic title="Схема использования параметра releaseChannel при установке и в процессе работы Deckhouse" %}
![Схема использования параметра releaseChannel при установке и в процессе работы Deckhouse](images/common/deckhouse-update-process.png)
{% endofftopic %}

### Что делать, если Deckhouse не получает обновлений из канала обновлений?

* Проверьте, что [настроен](#как-установить-желаемый-канал-обновлений) нужный канал обновлений.
* Проверьте корректность разрешения DNS-имени хранилища образов Deckhouse.

  Получите и сравните IP-адреса хранилища образов Deckhouse (`registry.deckhouse.ru`) на одном из узлов и в поде Deckhouse. Они должны совпадать.

  Пример получения IP-адреса хранилища образов Deckhouse на узле:

  ```shell
  $ getent ahosts registry.deckhouse.ru
  185.193.90.38    STREAM registry.deckhouse.ru
  185.193.90.38    DGRAM
  185.193.90.38    RAW
  ```

  Пример получения IP-адреса хранилища образов Deckhouse в поде Deckhouse:
  
  ```shell
  $ kubectl -n d8-system exec -ti deploy/deckhouse -c deckhouse -- getent ahosts registry.deckhouse.ru
  185.193.90.38    STREAM registry.deckhouse.ru
  185.193.90.38    DGRAM  registry.deckhouse.ru
  ```
  
  Если полученные IP-адреса не совпадают, проверьте настройки DNS на узле. В частности, обратите внимание на список доменов в параметре `search` файла `/etc/resolv.conf` (он влияет на разрешение имен в поде Deckhouse). Если в параметре `search` файла `/etc/resolv.conf` указан домен, в котором настроено разрешение wildcard-записей, это может привести к неверному разрешению IP-адреса хранилища образов Deckhouse (см. пример).
  
{% offtopic title="Пример настроек DNS, которые могут привести к ошибкам в разрешении IP-адреса хранилища образов Deckhouse..." %}

Далее описан пример, когда настройки DNS приводят к различному результату при разрешении имен на узле и в поде Kubernetes:
- Пример файла `/etc/resolv.conf` на узле:

  ```text
  nameserver 10.0.0.10
  search company.my
  ```

  > Обратите внимание, что по умолчанию на узле параметр `ndot` равен 1 (`options ndots:1`). Но в подах Kubernetes параметр `ndot` равен **5**. Таким образом, логика разрешения DNS-имен, имеющих в имени 5 точек и менее, различается на узле и в поде.

- В DNS-зоне `company.my` настроено разрешение wildcard-записей `*.company.my` в адрес `10.0.0.100`. То есть любое DNS-имя в зоне `company.my`, для которого нет конкретной записи в DNS, разрешается в адрес `10.0.0.100`.

Тогда с учетом параметра `search`, указанного в файле `/etc/resolv.conf`, при обращении на адрес `registry.deckhouse.ru` на узле система попробует получить IP-адрес для имени `registry.deckhouse.ru` (так как считает его полностью определенным, учитывая настройку по умолчанию параметра `options ndots:1`).

При обращении же на адрес `registry.deckhouse.ru` **из пода** Kubernetes, учитывая параметры `options ndots:5` (используется в Kubernetes по умолчанию) и `search`, система первоначально попробует получить IP-адрес для имени `registry.deckhouse.ru.company.my`. Имя `registry.deckhouse.ru.company.my` разрешится в IP-адрес `10.0.0.100`, так как в DNS-зоне `company.my` настроено разрешение wildcard-записей `*.company.my` в адрес `10.0.0.100`. В результате к хосту `registry.deckhouse.ru` будет невозможно подключиться и скачать информацию о доступных обновлениях Deckhouse.  
{% endofftopic %}

## Закрытое окружение, работа через proxy и сторонние registry

### Как установить Deckhouse из стороннего registry?

{% alert level="warning" %}
Доступно только в Enterprise Edition.
{% endalert %}

{% alert level="warning" %}
Deckhouse поддерживает работу только с Bearer token-схемой авторизации в container registry.

Протестирована и гарантируется работа со следующими container registry:
{%- for registry in site.data.supported_versions.registries %}
[{{- registry[1].shortname }}]({{- registry[1].url }})
{%- unless forloop.last %}, {% endunless %}
{%- endfor %}.
{% endalert %}

При установке Deckhouse можно настроить на работу со сторонним registry (например, проксирующий registry внутри закрытого контура).

Установите следующие параметры в ресурсе `InitConfiguration`:

* `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/ee` — адрес образа Deckhouse EE в стороннем registry. Пример: `imagesRepo: registry.deckhouse.ru/deckhouse/ee`;
* `registryDockerCfg: <BASE64>` — права доступа к стороннему registry, зашифрованные в Base64.

Если разрешен анонимный доступ к образам Deckhouse в стороннем registry, `registryDockerCfg` должен выглядеть следующим образом:

```json
{"auths": { "<PROXY_REGISTRY>": {}}}
```

Приведенное значение должно быть закодировано в Base64.

Если для доступа к образам Deckhouse в стороннем registry необходима аутентификация, `registryDockerCfg` должен выглядеть следующим образом:

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
* `registryScheme` — протокол доступа к registry (`HTTP` или `HTTPS`). По умолчанию — `HTTPS`.

<div markdown="0" style="height: 0;" id="особенности-настройки-сторонних-registry"></div>

### Особенности настройки Nexus

При использовании менеджера репозиториев [Nexus](https://github.com/sonatype/nexus-public) должны быть выполнены следующие требования:

* Включен `Docker Bearer Token Realm` (*Administration* -> *Security* -> *Realms*).
* Создан **проксирующий** репозиторий Docker (*Administration* -> *Repository* -> *Repositories*):
  * Параметр `Allow anonymous docker pull` для репозитория должен быть включен. Данный параметр включает поддержку авторизации с помощью Bearer-токенов, при этом анонимный доступ [не будет работать](https://help.sonatype.com/en/docker-authentication.html#unauthenticated-access-to-docker-repositories), если он не был явно включен в *Administration* -> *Security* -> *Anonymous Access* и пользователю `anonymous` не были даны права на доступ к репозиторию.
  * Параметр `Maximum metadata age` для репозитория должен быть установлен в `0`.
* Должен быть настроен контроль доступа:
  * Создана роль **Nexus** (*Administration* -> *Security* -> *Roles*) со следующими полномочиями:
    * `nx-repository-view-docker-<репозиторий>-browse`
    * `nx-repository-view-docker-<репозиторий>-read`
  * Создан пользователь (*Administration* -> *Security* -> *Users*) с ролью, созданной выше.

**Настройка**:

* Включите `Docker Bearer Token Realm` (*Administration* -> *Security* -> *Realms*):
  ![Включение `Docker Bearer Token Realm`](images/registry/nexus/nexus-realm.png)

* Создайте **проксирующий** репозиторий Docker (*Administration* -> *Repository* -> *Repositories*), указывающий на [Deckhouse registry](https://registry.deckhouse.ru/):
  ![Создание проксирующего репозитория Docker](images/registry/nexus/nexus-repository.png)

* Заполните поля страницы создания репозитория следующим образом:
  * `Name` должно содержать имя создаваемого репозитория, например `d8-proxy`.
  * `Repository Connectors / HTTP` или `Repository Connectors / HTTPS` должно содержать выделенный порт для создаваемого репозитория, например `8123` или иной.
  * `Allow anonymous docker pull` должно быть включено, чтобы работала авторизация с помощью Bearer-токенов. При этом анонимный доступ [не будет работать](https://help.sonatype.com/en/docker-authentication.html#unauthenticated-access-to-docker-repositories), если он не был явно включен в *Administration* -> *Security* -> *Anonymous Access* и пользователю `anonymous` не были даны права на доступ к репозиторию.
  * `Remote storage` должно иметь значение `https://registry.deckhouse.ru/`.
  * `Auto blocking enabled` и `Not found cache enabled` могут быть выключены для отладки; в противном случае их следует включить.
  * `Maximum Metadata Age` должно быть равно `0`.
  * Если планируется использовать Deckhouse Enterprise Edition, флажок `Authentication` должен быть включен, а связанные поля должны быть заполнены следующим образом:
    * `Authentication Type` должно иметь значение `Username`.
    * `Username` должно иметь значение `license-token`.
    * `Password` должно содержать ключ лицензии Deckhouse Enterprise Edition.

  ![Пример настроек репозитория 1](images/registry/nexus/nexus-repo-example-1.png)
  ![Пример настроек репозитория 2](images/registry/nexus/nexus-repo-example-2.png)
  ![Пример настроек репозитория 3](images/registry/nexus/nexus-repo-example-3.png)

* Настройте контроль доступа Nexus для доступа Deckhouse к созданному репозиторию:
  * Создайте роль **Nexus** (*Administration* -> *Security* -> *Roles*) с полномочиями `nx-repository-view-docker-<репозиторий>-browse` и `nx-repository-view-docker-<репозиторий>-read`.

    ![Создание роли Nexus](images/registry/nexus/nexus-role.png)

  * Создайте пользователя (*Administration* -> *Security* -> *Users*) с ролью, созданной выше.

    ![Создание пользователя Nexus](images/registry/nexus/nexus-user.png)

### Особенности настройки Harbor

Необходимо использовать такой функционал [Harbor](https://github.com/goharbor/harbor), как Proxy Cache.

* Настройте Registry:
  * `Administration -> Registries -> New Endpoint`.
  * `Provider`: `Docker Registry`.
  * `Name` — укажите любое, на ваше усмотрение.
  * `Endpoint URL`: `https://registry.deckhouse.ru`.
  * Укажите `Access ID` и `Access Secret` для Deckhouse Enterprise Edition.

  ![Настройка Registry](images/registry/harbor/harbor1.png)

* Создайте новый проект:
  * `Projects -> New Project`.
  * `Project Name` будет частью URL. Используйте любой, например, `d8s`.
  * `Access Level`: `Public`.
  * `Proxy Cache` — включите и выберите в списке Registry, созданный на предыдущем шаге.

  ![Создание нового проекта](images/registry/harbor/harbor2.png)

В результате образы Deckhouse будут доступны, например, по следующему адресу: `https://your-harbor.com/d8s/deckhouse/ee:{d8s-version}`.

### Ручная загрузка образов в изолированный приватный registry

{% alert level="warning" %}
Доступно только в Standard Edition (SE), Enterprise Edition (EE) и Certified Security Edition (CSE).
{% endalert %}

1. [Скачайте и установите утилиту Deckhouse CLI](https://github.com/deckhouse/deckhouse-cli/blob/main/README.md#how-to-install).

1. Скачайте образы Deckhouse в выделенную директорию, используя команду `d8 mirror pull`.

   По умолчанию `d8 mirror pull` скачивает только последние доступные патч-версии актуальных релизов Deckhouse и актуальный набор официально поставляемых модулей.
   Например, для Deckhouse 1.52 будет скачана только одна версия `1.52.10`, т. к. этого достаточно для обновления Deckhouse с версии 1.51.

   Следующая команда скачает образы Deckhouse тех версий, которые находятся на каналах обновлений (о текущем статусе версий на каналах обновлений можно узнать на [flow.deckhouse.io](https://flow.deckhouse.io)):

   ```shell
   d8 mirror pull --source="registry.deckhouse.ru/deckhouse/ee" --license="<DECKHOUSE_LICENSE_KEY>" $(pwd)/d8.tar
   ```

   > Если загрузка образов будет прервана, повторный вызов команды проверит загруженные образы и продолжит загрузку с момента ее остановки. Продолжение загрузки возможно только если с момента остановки прошло не более суток.
   > Используйте параметр `--no-pull-resume`, чтобы принудительно начать загрузку сначала.
   >
   > Для пропуска загрузки модулей используйте параметр `--no-modules`.

   Чтобы скачать все версии Deckhouse начиная с конкретной версии, укажите ее в параметре `--min-version` в формате `X.Y`.

   Например, для загрузки всех версий Deckhouse, начиная с версии 1.45, используйте команду:

   ```shell
   d8 mirror pull --source="registry.deckhouse.ru/deckhouse/ee" --license="<DECKHOUSE_LICENSE_KEY>" --min-version=1.45 $(pwd)/d8.tar
   ```

   > Обратите внимание, параметр `--min-version` будет проигнорирован если вы укажете версию выше находящейся в канале обновлений rock-solid.

   Так же возможна загрузка только одного конкретного релиза Deckhouse посредством использования флага `--release=X.Y.Z`.
   В таком случае не будут использоваться релизные каналы, будет скачан только один конкретный релиз.

   > Одновременное использование параметров `--min-version` и `--release` не поддерживается.

   Чтобы загрузить образы Deckhouse из определенного репозитория registry, вы можете указать этот репозиторий с помощью флага `--source`.
   Существуют также дополнительные флаги `--source-login` и `--source-password`, используемые для аутентификации в предоставленном registry.
   Если они не указаны, `d8 mirror pull` будет обращаться к registry анонимно.

   Например, вот как можно загрузить образы из стороннего registry:

   ```shell
   d8 mirror pull --source="corp.company.ru/sys/deckhouse" --source-login="<USER>" --source-password="<PASSWORD>" $(pwd)/d8.tar
   ```

   > Параметр `--license` действует как сокращение для параметров `--source-login` и `--source-password` и предназначен для использования с официальным registry Deckhouse.
   > Если вы укажете и параметр `--license`, и пару логин + пароль одновременно, будет использована последняя.

   `d8 mirror pull` поддерживает расчет контрольных сумм итогового набора образов Deckhouse в формате ГОСТ Р 34.11-2012 (Стрибог) (параметр `--gost-digest`).
   Контрольная сумма будет выведена в лог и записана в файл с расширением `.tar.gostsum` рядом с tar-архивом, содержащим образы Deckhouse.

   А также `d8 mirror pull` поддерживает разделение пакета образов на фрагменты произвольного размера вместо упаковки всех образов в один tar-файл.
   Чтобы использовать эту функцию, передайте желаемый размер фрагмента в гигабайтах флагом `--images-bundle-chunk-size=N`.
   В результате будет создан набор из последовательных фрагментов `.chunk` вместо одного пакета tar.
   Чтобы загрузить такой фрагментированный пакет в свой частный registry, используйте `d8 mirro pushr` как указано далее, передав путь к пакету образов, как если бы вы загружали пакет в виде цельного tar-файла.
   Укажите файл `.tar`, как если бы он был в каталоге пакета, а не какой-либо из файлов фрагментов. `d8 mirror push` автоматически распознает фрагментированный пакет.

1. Передайте загруженный пакет образов Deckhouse на хост с доступом к изолированному registry.
   Для продолжения, установите и используйте утилиту Deckhouse CLI на хосте с доступом к изолированному registry.

1. Загрузите образы Deckhouse в изолированный registry с помощью команды `d8 mirror push`.

   Пример команды для загрузки образов из файла `/tmp/d8-images/d8.tar`:

   ```shell
   d8 mirror push /tmp/d8-images/d8.tar "your.private.registry.com:5000/deckhouse/ee" --registry-login="<USER>" --registry-password="<PASSWORD>"
   ```

   > Обратите внимание, образы будут выгружены в registry по пути, указанному во втором параметре (в примере - /deckhouse/ee).
   > Перед запуском команды убедитесь, что этот путь существует и у используемой учетной записи есть права на запись.

   Если ваш registry не требует авторизации, флаги `--registry-login`/`--registry-password` указывать не нужно.

1. После загрузки образов в изолированный registry можно переходить к установке Deckhouse. Воспользуйтесь [руководством по быстрому старту](/gs/bm-private/step2.html).

   При запуске установщика используйте его образ из registry, в который ранее были загружены образы Deckhouse, а не из публичного registry. Например, используйте адрес вида `your.private.registry.com:5000/deckhouse/ee/install:stable` вместо `registry.deckhouse.ru/deckhouse/ee/install:stable`.

   В ресурсе `InitConfiguration` при установке также используйте адрес вашего registry и данные авторизации (параметры [imagesRepo](/documentation/v1/installing/configuration.html#initconfiguration-deckhouse-imagesrepo), [registryDockerCfg](/documentation/v1/installing/configuration.html#initconfiguration-deckhouse-registrydockercfg) или [шаг 3](/gs/bm-private/step3.html) руководства по быстрому старту).

   После завершения установки примените сгенерированные во время загрузки манифесты DeckhouseReleases к вашему кластеру используя Deckhouse CLI:

   ```shell
   d8 k apply -f ./deckhousereleases.yaml
   ```

### Ручная загрузка образов подключаемых модулей Deckhouse в изолированный приватный registry

Ниже описаны шаги, необходимые для ручной загрузки образов модулей, подключаемых из источника модулей (ресурс [ModuleSource](cr.html#modulesource)):

1. [Скачайте и установите утилиту Deckhouse CLI](https://github.com/deckhouse/deckhouse-cli/blob/main/README.md#how-to-install).

1. Скачайте образы модулей из их источника, описанного в виде ресурса `ModuleSource` в выделенную директорию, используя команду `d8 mirror modules pull`.

   `d8 mirror modules pull` скачивает только версии модулей, доступные в каналах обновлений модуля на момент копирования если не передан параметр `--modules-filter`.

   Следующая команда скачает образы модулей из источника, описанного в ресурсе `ModuleSource`, находящемся в файле `$HOME/module_source.yml`:

   ```shell
   d8 mirror modules pull -d ./d8-modules -m $HOME/module_source.yml
   ```

   Для загрузки только набора из определенных модулей конкретных версий, используйте параметр `--modules-filter`, передав набор необходимых модулей и их версий, разделенных символом `;`.
   Например:

   ```shell
   d8 mirror modules pull -d /tmp/d8-modules -m $HOME/module_source.yml --modules-filter="deckhouse-admin:v1.0.0;deckhouse-admin:v1.3.3; sds-drbd:v0.0.1"
   ```

1. Передайте директорию с загруженными образами модулей на хост с доступом к изолированному registry.
   Для продолжения установки используйте утилиту `d8` на этом хосте.

1. Загрузите образы модулей в изолированный registry с помощью команды `d8 mirror modules push`.

   Пример команды для загрузки образов из директории `/tmp/d8-modules`:

   ```shell
   d8 mirror modules push -d /tmp/d8-modules --registry="your.private.registry.com:5000/deckhouse-modules" --registry-login="<USER>" --registry-password="<PASSWORD>"
   ```

   > Обратите внимание, образы будут выгружены в registry по пути, указанному в параметре `--registry` (в примере - /deckhouse-modules).
   > Перед запуском команды убедитесь, что этот путь существует и у используемой учетной записи есть права на запись.

   Если ваш registry не требует авторизации, флаги `--registry-login`/`--registry-password` указывать не нужно.

1. После загрузки образов в изолированный registry, отредактируйте YAML-манифест `ModuleSource`:

   * Измените поле `.spec.registry.repo` на адрес, который вы указали в параметре `--registry` при выгрузке образов;
   * Измените поле `.spec.registry.dockerCfg` на Base64-строку с данными для авторизации в вашем registry в формате `dockercfg`. Обратитесь к документации вашего registry для получения информации о том, как сгенерировать этот токен.

1. Примените в кластере полученный на прошлом шаге манифест `ModuleSource`:

   ```shell
   d8 k apply -f $HOME/module_source.yml
   ```

   После применения манифеста модули готовы к использованию. Обратитесь к документации разработчика модулей для получения дальнейших инструкций по их настройке и использованию.

### Как переключить работающий кластер Deckhouse на использование стороннего registry?

{% alert level="warning" %}
Использование registry отличных от `registry.deckhouse.io` и `registry.deckhouse.ru` доступно только в Enterprise Edition.
{% endalert %}

Для переключения кластера Deckhouse на использование стороннего registry выполните следующие действия:

* Выполните команду `deckhouse-controller helper change-registry` из пода Deckhouse с параметрами нового registry.
  * Пример запуска:

    ```shell
    kubectl exec -ti -n d8-system deploy/deckhouse -- deckhouse-controller helper change-registry \
      --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee
    ```

  * Если registry использует самоподписные сертификаты, положите корневой сертификат соответствующего сертификата registry в файл `/tmp/ca.crt` в поде Deckhouse и добавьте к вызову опцию `--ca-file /tmp/ca.crt`, или вставьте содержимое CA в переменную, как в примере ниже:

    ```shell
    $ CA_CONTENT=$(cat <<EOF
    -----BEGIN CERTIFICATE-----
    CERTIFICATE
    -----END CERTIFICATE-----
    -----BEGIN CERTIFICATE-----
    CERTIFICATE
    -----END CERTIFICATE-----
    EOF
    )
    $ kubectl exec  -n d8-system deploy/deckhouse -- bash -c "echo '$CA_CONTENT' > /tmp/ca.crt && deckhouse-controller helper change-registry --ca-file /tmp/ca.crt --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee"
    ```
  
  * Просмотреть список доступных ключей команды `deckhouse-controller helper change-registry` можно, выполнив:

    ```shell
    kubectl exec -ti -n d8-system deploy/deckhouse -- deckhouse-controller helper change-registry --help
    ```
    Пример вывода:
    ```shell
    Defaulted container "deckhouse" out of: deckhouse, kube-rbac-proxy, init-external-modules (init)
    usage: deckhouse-controller helper change-registry [<flags>] <new-registry>
    
    Change registry for deckhouse images.
    
    Flags:
    --help               Show context-sensitive help (also try --help-long and --help-man).
    --user=USER          User with pull access to registry.
    --password=PASSWORD  Password/token for registry user.
    --ca-file=CA-FILE    Path to registry CA.
    --insecure           Use HTTP while connecting to new registry.
    --dry-run            Don't change deckhouse resources, only print them.
    --new-deckhouse-tag=NEW-DECKHOUSE-TAG
    New tag that will be used for deckhouse deployment image (by default
    current tag from deckhouse deployment will be used).
    
    Args:
    <new-registry>  Registry that will be used for deckhouse images (example:
    registry.deckhouse.io/deckhouse/ce). By default, https will be used, if you need
    http - provide '--insecure' flag
    ```

* Дождитесь перехода пода Deckhouse в статус `Ready`. Если под будет находиться в статусе `ImagePullBackoff`, перезапустите его.
* Дождитесь применения bashible новых настроек на master-узле. В журнале bashible на master-узле (`journalctl -u bashible`) должно появится сообщение `Configuration is in sync, nothing to do`.
* Если необходимо отключить автоматическое обновление Deckhouse через сторонний registry, удалите параметр `releaseChannel` из конфигурации модуля `deckhouse`.
* Проверьте, не осталось ли в кластере подов с оригинальным адресом registry:

  ```shell
  kubectl get pods -A -o json | jq '.items[] | select(.spec.containers[] | select((.image | contains("deckhouse.io"))))
    | .metadata.namespace + "\t" + .metadata.name' -r
  ```

### Как создать кластер и запустить Deckhouse без использования каналов обновлений?

Данный способ следует использовать только в случае, если в изолированном приватном registry нет образов, содержащих информацию о каналах обновлений.

* Если вы хотите установить Deckhouse с отключенным автоматическим обновлением:
  * Используйте тег образа установщика соответствующей версии. Например, если вы хотите установить релиз `v1.44.3`, используйте образ `your.private.registry.com/deckhouse/install:v1.44.3`.
  * Укажите соответствующий номер версии в параметре [deckhouse.devBranch](installing/configuration.html#initconfiguration-deckhouse-devbranch) в ресурсе [InitConfiguration](installing/configuration.html#initconfiguration).
  * **Не указывайте** параметр [deckhouse.releaseChannel](installing/configuration.html#initconfiguration-deckhouse-releasechannel) в ресурсе [InitConfiguration](installing/configuration.html#initconfiguration).
* Если вы хотите отключить автоматические обновления у уже установленного Deckhouse (включая обновления patch-релизов), удалите параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel) из конфигурации модуля `deckhouse`.

### Использование proxy-сервера

{% alert level="warning" %}
Доступно только в Enterprise Edition.
{% endalert %}

{% offtopic title="Пример шагов по настройке proxy-сервера на базе Squid..." %}
* Подготовьте сервер (или виртуальную машину). Сервер должен быть доступен с необходимых узлов кластера, и у него должен быть выход в интернет.
* Установите Squid (здесь и далее примеры для Ubuntu):

  ```shell
  apt-get install squid
  ```

* Создайте файл конфигурации Squid:

  ```shell
  cat <<EOF > /etc/squid/squid.conf
  auth_param basic program /usr/lib/squid3/basic_ncsa_auth /etc/squid/passwords
  auth_param basic realm proxy
  acl authenticated proxy_auth REQUIRED
  http_access allow authenticated

  # Choose the port you want. Below we set it to default 3128.
  http_port 3128
  ```

* Создайте пользователя и пароль для аутентификации на proxy-сервере:

  Пример для пользователя `test` с паролем `test` (обязательно измените):

  ```shell
  echo "test:$(openssl passwd -crypt test)" >> /etc/squid/passwords
  ```

* Запустите Squid и включите его автоматический запуск при загрузке сервера:

  ```shell
  systemctl restart squid
  systemctl enable squid
  ```

{% endofftopic %}

Для настройки Deckhouse на использование proxy используйте параметр [proxy](installing/configuration.html#clusterconfiguration-proxy) ресурса `ClusterConfiguration`.

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: OpenStack
  prefix: main
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
cri: "Containerd"
clusterDomain: "cluster.local"
proxy:
  httpProxy: "http://user:password@proxy.company.my:3128"
  httpsProxy: "https://user:password@proxy.company.my:8443"
```

## Изменение конфигурации

### Как изменить конфигурацию кластера?

Общие параметры кластера хранятся в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration).

Чтобы изменить общие параметры кластера, выполните команду:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit cluster-configuration
```

После сохранения изменений Deckhouse приведет конфигурацию кластера к измененному состоянию. В зависимости от размеров кластера это может занять какое-то время.

### Как изменить конфигурацию облачного провайдера в кластере?

Настройки используемого облачного провайдера в облачном или гибридном кластере хранятся в структуре `<PROVIDER_NAME>ClusterConfiguration`, где `<PROVIDER_NAME>` — название/код провайдера. Например, для провайдера OpenStack структура будет называться [OpenStackClusterConfiguration]({% if site.mode == 'module' and site.d8Revision == 'CE' %}{{ site.urls[page.lang] }}/documentation/v1/{% endif %}modules/030-cloud-provider-openstack/cluster_configuration.html).

Независимо от используемого облачного провайдера его настройки можно изменить с помощью следующей команды:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

### Как изменить конфигурацию статического кластера?

Настройки статического кластера хранятся в структуре [StaticClusterConfiguration](installing/configuration.html#staticclusterconfiguration).

Чтобы изменить параметры статического кластера, выполните команду:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit static-cluster-configuration
```

### Как переключить Deckhouse EE на CE?

{% alert level="warning" %}
Инструкция подразумевает использование публичного адреса container registry: `registry.deckhouse.ru`. Использование registry отличных от `registry.deckhouse.io` и `registry.deckhouse.ru` доступно только в Enterprise Edition.
{% endalert %}

{% alert level="warning" %}
В Deckhouse CE не поддерживается работа облачных кластеров на OpenStack и VMware vSphere.
{% endalert %}

Для переключения кластера Deckhouse Enterprise Edition на Community Edition выполните следующие действия:

1. Убедитесь, что используемые в кластере модули [поддерживаются в версии CE](revision-comparison.html). Отключите модули, которые не поддерживаются в Deckhouse CE.

1. Выполните следующую команду:

   ```shell
   kubectl exec -ti -n d8-system deploy/deckhouse -- deckhouse-controller helper change-registry registry.deckhouse.ru/deckhouse/ce
   ```

1. Дождитесь перехода пода Deckhouse в статус `Ready`:

   ```shell
   kubectl -n d8-system get po -l app=deckhouse
   ```

1. Если под будет находиться в статусе `ImagePullBackoff`, перезапустите его:

   ```shell
   kubectl -n d8-system delete po -l app=deckhouse
   ```

1. Дождитесь перезапуска Deckhouse и выполнения всех задач в очереди:

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   ```

   Пример вывода, когда в очереди еще есть задания (`length 38`):

   ```console
   # kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   Queue 'main': length 38, status: 'run first task'
   ```

   Пример вывода, когда очередь пуста (`length 0`):

   ```console
   # kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   Queue 'main': length 0, status: 'waiting for task 0s'
   ```

1. На master-узле проверьте применение новых настроек.

   В журнале systemd-сервиса bashible на master-узле должно появиться сообщение `Configuration is in sync, nothing to do`.

   Пример:

   ```console
   # journalctl -u bashible -n 5
   Jan 12 12:38:20 demo-master-0 bashible.sh[868379]: Configuration is in sync, nothing to do.
   Jan 12 12:38:20 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   Jan 12 12:39:18 demo-master-0 systemd[1]: Started Bashible service.
   Jan 12 12:39:19 demo-master-0 bashible.sh[869714]: Configuration is in sync, nothing to do.
   Jan 12 12:39:19 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Проверьте, не осталось ли в кластере подов с адресом registry для Deckhouse EE:

   ```shell
   kubectl get pods -A -o json | jq '.items[] | select(.spec.containers[] | select((.image | contains("deckhouse.io/deckhouse/ee"))))
     | .metadata.namespace + "\t" + .metadata.name' -r | sort | uniq
   ```

   Иногда могут оставаться запущенными некоторые static Pod'ы (например, `kubernetes-api-proxy-*`). Это связанно с тем, что kubelet не перезапускает под несмотря на изменение соответствующего манифеста, так как используемый образ одинаков для Deckhouse CE и EE. Выполните на любом master-узле следующую команду, чтобы убедиться, что соответствующие манифесты также были изменены:

   ```shell
   grep -ri 'deckhouse.io/deckhouse/ee' /etc/kubernetes | grep -v backup
   ```

   Вывод команды должен быть пуст.

### Как переключить Deckhouse CE на EE?

Вам потребуется действующий лицензионный ключ (вы можете [запросить временный ключ](https://deckhouse.ru/products/enterprise_edition.html) при необходимости).

{% alert %}
Инструкция подразумевает использование публичного адреса container registry: `registry.deckhouse.ru`. В случае использования другого адреса container registry измените команды или воспользуйтесь [инструкцией по переключению Deckhouse на использование стороннего registry](#как-переключить-работающий-кластер-deckhouse-на-использование-стороннего-registry).
{% endalert %}

Для переключения кластера Deckhouse Community Edition на Enterprise Edition выполните следующие действия:

1. Выполните следующую команду:

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   kubectl exec -ti -n d8-system deploy/deckhouse -- deckhouse-controller helper change-registry --user license-token --password $LICENSE_TOKEN registry.deckhouse.ru/deckhouse/ee
   ```

1. Дождитесь перехода пода Deckhouse в статус `Ready`:

   ```shell
   kubectl -n d8-system get po -l app=deckhouse
   ```

1. Если под будет находиться в статусе `ImagePullBackoff`, перезапустите его:

   ```shell
   kubectl -n d8-system delete po -l app=deckhouse
   ```

1. Дождитесь перезапуска Deckhouse и выполнения всех задач в очереди:

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   ```

   Пример вывода, когда в очереди еще есть задания (`length 38`):

   ```console
   # kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   Queue 'main': length 38, status: 'run first task'
   ```

   Пример вывода, когда очередь пуста (`length 0`):

   ```console
   # kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   Queue 'main': length 0, status: 'waiting for task 0s'
   ```

1. На master-узле проверьте применение новых настроек.

   В журнале systemd-сервиса bashible на master-узле должно появиться сообщение `Configuration is in sync, nothing to do`.

   Пример:

   ```console
   # journalctl -u bashible -n 5
   Jan 12 12:38:20 demo-master-0 bashible.sh[868379]: Configuration is in sync, nothing to do.
   Jan 12 12:38:20 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   Jan 12 12:39:18 demo-master-0 systemd[1]: Started Bashible service.
   Jan 12 12:39:19 demo-master-0 bashible.sh[869714]: Configuration is in sync, nothing to do.
   Jan 12 12:39:19 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Проверьте, не осталось ли в кластере подов с адресом registry для Deckhouse CE:

   ```shell
   kubectl get pods -A -o json | jq '.items[] | select(.spec.containers[] | select((.image | contains("deckhouse.io/deckhouse/ce"))))
     | .metadata.namespace + "\t" + .metadata.name' -r | sort | uniq
   ```

   Иногда могут оставаться запущенными некоторые static Pod'ы (например, `kubernetes-api-proxy-*`). Это связанно с тем, что kubelet не перезапускает под несмотря на изменение соответствующего манифеста, так как используемый образ одинаков для Deckhouse CE и EE. Выполните на любом master-узле следующую команду, чтобы убедиться, что соответствующие манифесты также были изменены:

   ```shell
   grep -ri 'deckhouse.io/deckhouse/ce' /etc/kubernetes | grep -v backup
   ```

   Вывод команды должен быть пуст.

### Как обновить версию Kubernetes в кластере?

Чтобы обновить версию Kubernetes в кластере, измените параметр [kubernetesVersion](installing/configuration.html#clusterconfiguration-kubernetesversion) в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration) выполнив следующие шаги:
1. Выполните команду:

   ```shell
   kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit cluster-configuration
   ```

1. Измените параметр `kubernetesVersion`.
1. Сохраните изменения. Узлы кластера начнут последовательно обновляться.
1. Дождитесь окончания обновления.  Отслеживать ход обновления можно с помощью команды `kubectl get no`. Обновление можно считать завершенным, когда в выводе команды у каждого узла кластера в колонке `VERSION` появится обновленная версия.

### Как запускать Deckhouse на произвольном узле?

Для запуска Deckhouse на произвольном узле установите у модуля `deckhouse` соответствующий [параметр](modules/002-deckhouse/configuration.html) `nodeSelector` и не задавайте `tolerations`.  Необходимые значения `tolerations` в этом случае будут проставлены автоматически.

{% alert level="warning" %}
Используйте для запуска Deckhouse только узлы с типом **CloudStatic** или **Static**. Также избегайте использования для запуска Deckhouse группы узлов (`NodeGroup`), содержащей только один узел.
{% endalert %}

Пример конфигурации модуля:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    nodeSelector:
      node-role.deckhouse.io/deckhouse: ""
```
