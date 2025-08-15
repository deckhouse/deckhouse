---
title: FAQ
permalink: ru/deckhouse-faq.html
lang: ru
---

## Как узнать все параметры Deckhouse?

Deckhouse настраивается с помощью глобальных настроек, настроек модулей и различных кастомных ресурсов. Подробнее — [в документации](./).

1. Выведите глобальные настройки:

   ```shell
   kubectl get mc global -o yaml
   ```

1. Выведите список состояния всех модулей (доступно для Deckhouse версии 1.47+):

   ```shell
   kubectl get modules
   ```

1. Выведите настройки модуля `user-authn`:

   ```shell
   kubectl get moduleconfigs user-authn -o yaml
   ```

## Как найти документацию по установленной у меня версии?

Документация запущенной в кластере версии Deckhouse доступна по адресу `documentation.<cluster_domain>`, где `<cluster_domain>` — DNS-имя в соответствии с шаблоном из параметра [modules.publicDomainTemplate](deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) глобальной конфигурации.

{% alert level="warning" %}
Документация доступна, если в кластере включен модуль [documentation](modules/documentation/). Он включен по умолчанию, кроме [варианта поставки](modules/deckhouse/configuration.html#parameters-bundle) `Minimal`.
{% endalert %}

## Обновление Deckhouse

### Как понять, в каком режиме обновляется кластер?

Посмотреть режим обновления кластера можно в [конфигурации](modules/deckhouse/configuration.html) модуля `deckhouse`. Для этого выполните следующую команду:

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
* **Ручной режим.** Для применения обновления требуются [ручные действия](modules/deckhouse/usage.html#ручное-подтверждение-обновлений).

### Как установить желаемый канал обновлений?

Чтобы перейти на другой канал обновлений автоматически, нужно в [конфигурации](modules/deckhouse/configuration.html) модуля `deckhouse` изменить (установить) параметр [releaseChannel](modules/deckhouse/configuration.html#parameters-releasechannel).

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

Чтобы полностью отключить механизм обновления Deckhouse, удалите в [конфигурации](modules/deckhouse/configuration.html) модуля `deckhouse` параметр [releaseChannel](modules/deckhouse/configuration.html#parameters-releasechannel).

В этом случае Deckhouse не проверяет обновления и не выполняется обновление на patch-релизы.

{% alert level="danger" %}
Крайне не рекомендуется отключать автоматическое обновление! Это заблокирует обновления на patch-релизы, которые могут содержать исправления критических уязвимостей и ошибок.
{% endalert %}

### Как применить обновление минуя окна обновлений, canary-release и ручной режим обновлений?

Чтобы применить обновление немедленно, установите в соответствующем ресурсе [DeckhouseRelease](cr.html#deckhouserelease) аннотацию `release.deckhouse.io/apply-now: "true"`.

{% alert level="info" %}
**Обратите внимание!** В этом случае будут проигнорированы окна обновления, настройки [canary-release](cr.html#deckhouserelease-v1alpha1-spec-applyafter) и режим [ручного обновления кластера](modules/deckhouse/configuration.html#parameters-update-disruptionapprovalmode). Обновление применится сразу после установки аннотации.
{% endalert %}

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

- отображается алерт `DeckhouseUpdating`;
- под `deckhouse` не в статусе `Ready`. Если под долго не переходит в статус `Ready`, это может говорить о проблеме. Необходима диагностика.

### Как понять, что обновление прошло успешно?

Если алерт `DeckhouseUpdating` перестал отображаться, значит, обновление завершено.

Также вы можете проверить состояние [релизов](cr.html#deckhouserelease) Deckhouse, выполнив следующую команду:

```shell
kubectl get deckhouserelease
```

Пример вывода:

```console
NAME       PHASE        TRANSITIONTIME   MESSAGE
v1.46.8    Superseded   13d
v1.46.9    Superseded   11d
v1.47.0    Superseded   4h12m
v1.47.1    Deployed     4h12m
```

Статус `Deployed` у соответствующей версии говорит о том, что переключение на неё было выполнено (но это не равнозначно успешному завершению).

Проверьте состояние пода Deckhouse:

```shell
kubectl -n d8-system get pods -l app=deckhouse
```

Пример вывода команды:

```console
NAME                   READY  STATUS   RESTARTS  AGE
deckhouse-7844b47bcd-qtbx9  1/1   Running  0       1d
```

* Если статус пода `Running` и в колонке READY указано `1/1` — обновление закончилось успешно.
* Если статус пода `Running` и в колонке READY указано `0/1` — обновление еще не закончилось. Если это продолжается более 20–30 минут, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.
* Если статус пода не `Running`, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.

{% alert level="info" %}
Возможные варианты действий, если что-то пошло не так:

1. Проверьте логи, используя следующую команду:

   ```shell
   kubectl -n d8-system logs -f -l app=deckhouse | jq -Rr 'fromjson? | .msg'
   ```

1. Соберите [отладочную информацию](modules/deckhouse/faq.html#как-собрать-информацию-для-отладки) и свяжитесь с технической поддержкой.
1. Попросите помощи у [сообщества](https://deckhouse.ru/community/about.html).
{% endalert %}

### Как узнать, что для кластера доступна новая версия?

Как только на установленном в кластере канале обновления появляется новая версия Deckhouse:

- Отображается алерт `DeckhouseReleaseIsWaitingManualApproval`, если кластер использует ручной режим обновлений (параметр [update.mode](modules/deckhouse/configuration.html#parameters-update-mode) установлен в `Manual`).
- Появляется новый кастомный ресурс [DeckhouseRelease](cr.html#deckhouserelease). Используйте команду `kubectl get deckhousereleases`, чтобы посмотреть список релизов. Если `DeckhouseRelease` новой версии находится в состоянии `Pending`, указанная версия еще не установлена. Возможные причины, при которых `DeckhouseRelease` может находиться в `Pending`:
  - Установлен ручной режим обновлений (параметр [update.mode](modules/deckhouse/configuration.html#parameters-update-mode) установлен в `Manual`).
  - Установлен автоматический режим обновлений и настроены [окна обновлений](modules/deckhouse/usage.html#конфигурация-окон-обновлений), интервал которых еще не наступил.
  - Установлен автоматический режим обновлений, окна обновлений не настроены, но применение версии отложено на случайный период времени из-за механизма снижения нагрузки на репозиторий образов контейнеров. В поле `status.message` ресурса `DeckhouseRelease` будет соответствующее сообщение.
  - Установлен параметр [update.notification.minimalNotificationTime](modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) и указанное в нем время еще не прошло.

### Как заранее получать информацию о предстоящем обновлении?

Заранее получать информацию об обновлении минорных версий Deckhouse на канале обновлений можно следующими способами:

- Настройте ручной [режим обновлений](modules/deckhouse/configuration.html#parameters-update-mode). В этом случае при появлении новой версии на канале обновлений отобразится алерт `DeckhouseReleaseIsWaitingManualApproval` и в кластере появится новый кастомный ресурс [DeckhouseRelease](cr.html#deckhouserelease).
- Настройте автоматический [режим обновлений](modules/deckhouse/configuration.html#parameters-update-mode) и укажите минимальное время в параметре [minimalNotificationTime](modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime), на которое будет отложено обновление. В этом случае при появлении новой версии на канале обновлений в кластере появится новый кастомный ресурс [DeckhouseRelease](cr.html#deckhouserelease). А если указать URL в параметре [update.notification.webhook](modules/deckhouse/configuration.html#parameters-update-notification-webhook), дополнительно произойдет вызов webhook'а.

### Как узнать, какая версия Deckhouse находится на каком канале обновлений?

Информацию о версиях Deckhouse и их связях с каналами обновлений можно получить на <https://releases.deckhouse.ru>.

### Как работает автоматическое обновление Deckhouse?

Если в конфигурации модуля `deckhouse` указан параметр `releaseChannel`, то Deckhouse каждую минуту будет проверять данные о релизе на канале обновлений.

При появлении нового релиза Deckhouse скачает его в кластер и создаст кастомный ресурс [DeckhouseRelease](cr.html#deckhouserelease).

После появления кастомного ресурса `DeckhouseRelease` в кластере, Deckhouse выполнит обновление на соответствующую версию согласно установленным [параметрам обновления](modules/deckhouse/configuration.html#parameters-update) (по умолчанию — автоматически, в любое время).

Чтобы посмотреть список и состояние всех релизов, воспользуйтесь командой:

```shell
kubectl get deckhousereleases
```

{% alert level="warning" %}
Начиная с версии DKP 1.70, patch-версии обновлений (например, обновление на версию 1.70.2 при установленной версии 1.70.1) устанавливаются с учетом окон обновлений. До версии DKP 1.70 patch-версии обновлений устанавливаются без учета режима и окон обновления.
{% endalert %}

### Что происходит при смене канала обновлений?

* При смене канала обновлений на **более стабильный** (например, с `Alpha` на `EarlyAccess`) Deckhouse скачивает данные о релизе (в примере — из канала `EarlyAccess`) и сравнивает их с данными из существующих в кластере кастомных ресурсов `DeckhouseRelease`:
  * Более *поздние* релизы, которые не были применены (в статусе `Pending`), удаляются.
  * Если более *поздние* релизы применены (в статусе `Deployed`), смены релиза не происходит. Релиз остается прежним до тех пор, пока на канале обновлений `EarlyAccess` не появится более поздняя версия.
* При смене канала обновлений на **менее стабильный** (например, с `EarlyAccess` на `Alpha`):
  * Deckhouse скачивает данные о релизе (в примере — из канала `Alpha`) и сравнивает их с данными из существующих в кластере кастомных ресурсов `DeckhouseRelease`.
  * Затем Deckhouse выполняет обновление согласно установленным [параметрам обновления](modules/deckhouse/configuration.html#parameters-update).

{% offtopic title="Схема использования параметра releaseChannel при установке и в процессе работы Deckhouse" %}
![Схема использования параметра releaseChannel при установке и в процессе работы Deckhouse](images/common/deckhouse-update-process.png)
{% endofftopic %}

### Что делать, если Deckhouse не получает обновлений из канала обновлений?

1. Проверьте, что [настроен](#как-установить-желаемый-канал-обновлений) нужный канал обновлений.
1. Проверьте корректность разрешения DNS-имени хранилища образов Deckhouse.

1. Получите и сравните IP-адреса хранилища образов Deckhouse (`registry.deckhouse.ru`) на одном из узлов и в поде Deckhouse. Они должны совпадать.

   Чтобы получить IP-адрес хранилища образов Deckhouse на узле, выполните следующую команду:

   ```shell
   getent ahosts registry.deckhouse.ru
   ```

   Пример вывода:

   ```console
   185.193.90.38    STREAM registry.deckhouse.ru
   185.193.90.38    DGRAM
   185.193.90.38    RAW
   ```

   Чтобы получить IP-адрес хранилища образов Deckhouse в поде Deckhouse, выполните следующую команду:

   ```shell
   kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- getent ahosts registry.deckhouse.ru
   ```

   Пример вывода:

   ```console
   185.193.90.38    STREAM registry.deckhouse.ru
   185.193.90.38    DGRAM  registry.deckhouse.ru
   ```

   Если полученные IP-адреса не совпадают, проверьте настройки DNS на узле.
   Обратите внимание на список доменов в параметре `search` файла `/etc/resolv.conf` (он влияет на разрешение имен в поде Deckhouse). Если в параметре `search` файла `/etc/resolv.conf` указан домен, в котором настроено разрешение wildcard-записей, это может привести к неверному разрешению IP-адреса хранилища образов Deckhouse (см. пример).

{% offtopic title="Пример настроек DNS, которые могут привести к ошибкам в разрешении IP-адреса хранилища образов Deckhouse..." %}

Пример, когда настройки DNS приводят к различному результату при разрешении имен на узле и в поде Kubernetes:

- Пример файла `/etc/resolv.conf` на узле:

  ```text
  nameserver 10.0.0.10
  search company.my
  ```

  > Обратите внимание, что по умолчанию на узле параметр `ndot` равен 1 (`options ndots:1`). Но в подах Kubernetes параметр `ndot` равен **5**. Таким образом, логика разрешения DNS-имен, имеющих в имени 5 точек и менее, различается на узле и в поде.

- В DNS-зоне `company.my` настроено разрешение wildcard-записей `*.company.my` в адрес `10.0.0.100`. То есть любое DNS-имя в зоне `company.my`, для которого нет конкретной записи в DNS, разрешается в адрес `10.0.0.100`.

С учетом параметра `search`, указанного в файле `/etc/resolv.conf`, при обращении на адрес `registry.deckhouse.ru` на узле, система пытается получить IP-адрес для имени `registry.deckhouse.ru` (так как считает его полностью определенным, учитывая настройку по умолчанию параметра `options ndots:1`).

При обращении на адрес `registry.deckhouse.ru` **из пода** Kubernetes, учитывая параметры `options ndots:5` (используется в Kubernetes по умолчанию) и `search`, система первоначально попытается получить IP-адрес для имени `registry.deckhouse.ru.company.my`. Это имя будет разрешено для IP-адреса `10.0.0.100`, так как в DNS-зоне `company.my` настроено разрешение wildcard-записей `*.company.my` для адреса `10.0.0.100`. В результате к хосту `registry.deckhouse.ru` будет невозможно подключиться и скачать информацию о доступных обновлениях Deckhouse.
{% endofftopic %}

### Как проверить очередь заданий в Deckhouse?

Для просмотра состояния всех очередей заданий Deckhouse, выполните следующую команду:

```shell
kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
```

Пример вывода (очереди пусты):

```console
Summary:
- 'main' queue: empty.
- 88 other queues (0 active, 88 empty): 0 tasks.
- no tasks to handle.
```

Для просмотра состояния очереди заданий `main` Deckhouse, выполните следующую команду:

```shell
kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue main
```

Пример вывода (в очереди `main` 38 заданий):

```console
Queue 'main': length 38, status: 'run first task'
```

Пример вывода (очередь `main` пуста):

```console
Queue 'main': length 0, status: 'waiting for task 0s'
```

## Закрытое окружение, работа через proxy и сторонние registry

### Как установить Deckhouse из стороннего registry?

{% alert level="warning" %}
Доступно в следующих редакциях: BE, SE, SE+, EE, CSE Lite (1.67), CSE Pro (1.67).
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

Вы можете использовать следующий скрипт для генерации `registryDockerCfg`:

```shell
declare MYUSER='<PROXY_USERNAME>'
declare MYPASSWORD='<PROXY_PASSWORD>'
declare MYREGISTRY='<PROXY_REGISTRY>'

MYAUTH=$(echo -n "$MYUSER:$MYPASSWORD" | base64 -w0)
MYRESULTSTRING=$(echo -n "{\"auths\":{\"$MYREGISTRY\":{\"username\":\"$MYUSER\",\"password\":\"$MYPASSWORD\",\"auth\":\"$MYAUTH\"}}}" | base64 -w0)

echo "$MYRESULTSTRING"
```

Для настройки нестандартных конфигураций сторонних registry в ресурсе `InitConfiguration` предусмотрены еще два параметра:

* `registryCA` — корневой сертификат, которым можно проверить сертификат registry (если registry использует самоподписанные сертификаты);
* `registryScheme` — протокол доступа к registry (`HTTP` или `HTTPS`). По умолчанию — `HTTPS`.

<div markdown="0" style="height: 0;" id="особенности-настройки-сторонних-registry"></div>

### Особенности настройки Nexus

{% alert level="warning" %}
При взаимодействии с репозиторием типа `docker` расположенным в Nexus (например, при выполнении команд `docker pull`, `docker push`) требуется указывать адрес в формате `<NEXUS_URL>:<REPOSITORY_PORT>/<PATH>`.

Использование значения `URL` из параметров репозитория Nexus **недопустимо**
{% endalert %}

При использовании менеджера репозиториев [Nexus](https://github.com/sonatype/nexus-public) должны быть выполнены следующие требования:

* Создан **проксирующий** репозиторий Docker (*Administration* -> *Repository* -> *Repositories*):
  * Установлен в `0` параметр `Maximum metadata age` для репозитория.
* Настроен контроль доступа:
  * Создана роль **Nexus** (*Administration* -> *Security* -> *Roles*) со следующими полномочиями:
    * `nx-repository-view-docker-<репозиторий>-browse`
    * `nx-repository-view-docker-<репозиторий>-read`
  * Создан пользователь (*Administration* -> *Security* -> *Users*) с ролью **Nexus**.

**Настройка**:

1. Создайте **проксирующий** репозиторий Docker (*Administration* -> *Repository* -> *Repositories*), указывающий на [Deckhouse registry](https://registry.deckhouse.ru/):
   ![Создание проксирующего репозитория Docker](images/registry/nexus/nexus-repository.png)

1. Заполните поля страницы создания репозитория следующим образом:
   * `Name` должно содержать имя создаваемого репозитория, например `d8-proxy`.
   * `Repository Connectors / HTTP` или `Repository Connectors / HTTPS` должно содержать выделенный порт для создаваемого репозитория, например `8123` или иной.
   * `Remote storage` должно иметь значение `https://registry.deckhouse.ru/`.
   * `Auto blocking enabled` и `Not found cache enabled` могут быть выключены для отладки; в противном случае их следует включить.
   * `Maximum Metadata Age` должно быть равно `0`.
   * Если планируется использовать коммерческую редакцию Deckhouse Kubernetes Platform, флажок `Authentication` должен быть включен, а связанные поля должны быть заполнены следующим образом:
     * `Authentication Type` должно иметь значение `Username`.
     * `Username` должно иметь значение `license-token`.
     * `Password` должно содержать ключ лицензии Deckhouse Kubernetes Platform.

    ![Пример настроек репозитория 1](images/registry/nexus/nexus-repo-example-1.png)
    ![Пример настроек репозитория 2](images/registry/nexus/nexus-repo-example-2.png)
    ![Пример настроек репозитория 3](images/registry/nexus/nexus-repo-example-3.png)

1. Настройте контроль доступа Nexus для доступа Deckhouse к созданному репозиторию:
   * Создайте роль **Nexus** (*Administration* -> *Security* -> *Roles*) с полномочиями `nx-repository-view-docker-<репозиторий>-browse` и `nx-repository-view-docker-<репозиторий>-read`.

     ![Создание роли Nexus](images/registry/nexus/nexus-role.png)

   * Создайте пользователя (*Administration* -> *Security* -> *Users*) с ролью, созданной выше.

     ![Создание пользователя Nexus](images/registry/nexus/nexus-user.png)

В результате образы Deckhouse будут доступны, например, по следующему адресу: `https://<NEXUS_HOST>:<REPOSITORY_PORT>/deckhouse/ee:<d8s-version>`.

### Особенности настройки Harbor

Используйте функцию [Harbor Proxy Cache](https://github.com/goharbor/harbor).

* Настройте Registry:
  * `Administration -> Registries -> New Endpoint`.
  * `Provider`: `Docker Registry`.
  * `Name` — укажите любое, на ваше усмотрение.
  * `Endpoint URL`: `https://registry.deckhouse.ru`.
  * Укажите `Access ID` и `Access Secret` (лицензионный ключ для Deckhouse Kubernetes Platform).

    ![Настройка Registry](images/registry/harbor/harbor1.png)

* Создайте новый проект:
  * `Projects -> New Project`.
  * `Project Name` будет частью URL. Используйте любой, например, `d8s`.
  * `Access Level`: `Public`.
  * `Proxy Cache` — включите и выберите в списке Registry, созданный на предыдущем шаге.

    ![Создание нового проекта](images/registry/harbor/harbor2.png)

В результате настройки, образы Deckhouse станут доступны, например, по следующему адресу: `https://your-harbor.com/d8s/deckhouse/ee:{d8s-version}`.

### Как сгенерировать самоподписанный сертификат?

При самостоятельной генерации сертификатов важно корректно заполнить все поля запроса на сертификат, чтобы итоговый сертификат был правильно издан и гарантированно проходил валидацию в различных сервисах.  

Важно придерживаться следующих правил:

1. Указывать доменные имена в поле `SAN` (Subject Alternative Name).

   Поле `SAN` является более современным и распространенным методом указания доменных имен, на которые распространяется сертификат.
   Некоторые сервисы на данный момент уже не рассматривают поле `CN` (Common Name) как источник для доменных имен.

2. Корректно заполнять поля `keyUsage`, `basicConstraints`, `extendedKeyUsage`, а именно:
   - `basicConstraints = CA:FALSE`  

     Данное поле определяет, относится ли сертификат к конечному пользователю (end-entity certificate) или к центру сертификации (CA certificate). CA-сертификат не может использоваться в качестве сертификата сервиса.

   - `keyUsage = digitalSignature, keyEncipherment`  

     Поле `keyUsage` ограничивает допустимые сценарии использования данного ключа:

     - `digitalSignature` — позволяет использовать ключ для подписи цифровых сообщений и обеспечения целостности соединения.
     - `keyEncipherment` — позволяет использовать ключ для шифрования других ключей, что необходимо для безопасного обмена данными с помощью TLS (Transport Layer Security).

   - `extendedKeyUsage = serverAuth`  

     Поле `extendedKeyUsage` уточняет дополнительные сценарии использования ключа, которые могут требоваться конкретными протоколами или приложениями:

     - `serverAuth` — указывает, что сертификат предназначен для использования на сервере для аутентификации сервера перед клиентом в процессе установления защищенного соединения.

Также рекомендуется:

1. Издать сертификат на срок не более 1 года (365 дней).

   Срок действия сертификата влияет на его безопасность. Срок в 1 год позволяет обеспечить актуальность криптографических методов и своевременно обновлять сертификаты в случае возникновения угроз.
   Также некоторые современные браузеры на текущий момент отвергают сертификаты со сроком действия более 1 года.

2. Использовать стойкие криптографические алгоритмы, например, алгоритмы на основе эллиптических кривых (в т.ч. `prime256v1`).

   Алгоритмы на основе эллиптических кривых (ECC) предоставляют высокий уровень безопасности при меньшем размере ключа по сравнению с традиционными методами, такими как RSA. Это делает сертификаты более эффективными по производительности и безопасными в долгосрочной перспективе.

3. Не указывать домены в поле `CN` (Common Name).

   Исторически поле `CN` использовалось для указания основного доменного имени, для которого выдается сертификат. Однако современные стандарты, такие как [RFC 2818](https://datatracker.ietf.org/doc/html/rfc2818), рекомендуют использовать поле `SAN` (Subject Alternative Name) для этой цели.
   Если сертификат распространяется на несколько доменных имен, указанных в поле `SAN`, то при дополнительном указании одного из доменов в `CN` в некоторых сервисах может возникнуть ошибка валидации при обращении к домену, не указанному в `CN`.
   Если указывать в `CN` информацию, не относящуюся напрямую к доменным именам (например, идентификатор или имя сервиса), то сертификат также будет распространяться на эти имена, что может быть использовано для вредоносных целей.

#### Пример создания сертификата

Для генерации сертификата воспользуемся утилитой `openssl`.

1. Заполните конфигурационный файл `cert.cnf`:

   ```ini
   [ req ]
   default_bits       = 2048
   default_md         = sha256
   prompt             = no
   distinguished_name = dn
   req_extensions     = req_ext

   [ dn ]
   C = RU
   ST = Moscow
   L = Moscow
   O = Example Company
   OU = IT Department
   # CN = Не указывайте поле CN.

   [ req_ext ]
   subjectAltName = @alt_names

   [ alt_names ]
   # Укажите все доменные имена.
   DNS.1 = example.com
   DNS.2 = www.example.com
   DNS.3 = api.example.com
   # Укажите IP-адреса (если требуется).
   IP.1 = 192.0.2.1
   IP.2 = 192.0.4.1

   [ v3_ca ]
   basicConstraints = CA:FALSE
   keyUsage = digitalSignature, keyEncipherment
   extendedKeyUsage = serverAuth

   [ v3_req ]
   basicConstraints = CA:FALSE
   keyUsage = digitalSignature, keyEncipherment
   extendedKeyUsage = serverAuth
   subjectAltName = @alt_names

   # Параметры эллиптических кривых.
   [ ec_params ]
   name = prime256v1
   ```

2. Сгенерируйте ключ на основе эллиптических кривых:

   ```shell
   openssl ecparam -genkey -name prime256v1 -noout -out ec_private_key.pem
   ```

3. Создайте запрос на сертификат:

   ```shell
   openssl req -new -key ec_private_key.pem -out example.csr -config cert.cnf
   ```

4. Сгенерируйте самоподписанный сертификат:

   ```shell
   openssl x509 -req -in example.csr -signkey ec_private_key.pem -out example.crt -days 365 -extensions v3_req -extfile cert.cnf
   ```

### Ручная загрузка образов Deckhouse Kubernetes Platform, БД сканера уязвимостей и модулей Deckhouse в приватный registry

{% alert level="warning" %}
Утилита `d8 mirror` недоступна для использования с редакциями Community Edition (CE) и Basic Edition (BE).
{% endalert %}

{% alert level="info" %}
О текущем статусе версий на каналах обновлений можно узнать на [releases.deckhouse.ru](https://releases.deckhouse.ru).
{% endalert %}

1. [Скачайте и установите утилиту Deckhouse CLI](deckhouse-cli/).

1. Скачайте образы Deckhouse в выделенную директорию, используя команду `d8 mirror pull`.

   По умолчанию `d8 mirror pull` скачивает только актуальные версии Deckhouse, базы данных сканера уязвимостей (если они входят в редакцию DKP) и официально поставляемых модулей.
   Например, для Deckhouse 1.59 будет скачана только версия 1.59.12, т. к. этого достаточно для обновления Deckhouse с 1.58 до 1.59.

   Выполните следующую команду (укажите код редакции и лицензионный ключ), чтобы скачать образы актуальных версий:

   ```shell
   d8 mirror pull \
     --source='registry.deckhouse.ru/deckhouse/<EDITION>' \
     --license='<LICENSE_KEY>' /home/user/d8-bundle
   ```

   где:

   - `<EDITION>` — код редакции Deckhouse Kubernetes Platform (например, `ee`, `se`, `se-plus`). По умолчанию параметр `--source` ссылается на редакцию Enterprise Edition (`ee`) и может быть опущен;
   - `<LICENSE_KEY>` — лицензионный ключ Deckhouse Kubernetes Platform;
   - `/home/user/d8-bundle` — директория, в которой будут расположены пакеты образов. Будет создана, если не существует.

   > Если загрузка образов будет прервана, повторный вызов команды продолжит загрузку, если с момента ее остановки прошло не более суток.

   Вы также можете использовать следующие параметры команды:

   - `--no-pull-resume` — чтобы принудительно начать загрузку сначала;
   - `--no-platform` — для пропуска загрузки пакета образов Deckhouse Kubernetes Platform (platform.tar);
   - `--no-modules` — для пропуска загрузки пакетов модулей (module-*.tar);
   - `--no-security-db` — для пропуска загрузки пакета баз данных сканера уязвимостей (security.tar);
   - `--include-module` / `-i` = `name[@Major.Minor]` — для загрузки только определенного набора модулей по принципу белого списка (и, при необходимости, их минимальных версий). Укажите несколько раз, чтобы добавить в белый список больше модулей. Эти флаги игнорируются, если используются совместно с `--no-modules`.
   - `--exclude-module` / `-e` = `name` — для пропуска загрузки определенного набора модулей по принципу черного списка. Укажите несколько раз, чтобы добавить в черный список больше модулей. Игнорируется, если используются `--no-modules` или `--include-module`.
   - `--modules-path-suffix` — для изменения суффикса пути к репозиторию модулей в основном репозитории Deckhouse. По умолчанию используется суффикс `/modules` (так, например, полный путь к репозиторию с модулями будет выглядеть как `registry.deckhouse.ru/deckhouse/EDITION/modules`).
   - `--since-version=X.Y` — чтобы скачать все версии Deckhouse, начиная с указанной минорной версии. Параметр будет проигнорирован, если указана версия выше чем версия находящаяся на канале обновлений Rock Solid. Параметр не может быть использован одновременно с параметром `--deckhouse-tag`;
   - `--deckhouse-tag` — чтобы скачать только конкретную версию Deckhouse (без учета каналов обновлений). Параметр не может быть использован одновременно с параметром `--since-version`;
   - `--gost-digest` — для расчета контрольной суммы итогового набора образов Deckhouse в формате ГОСТ Р 34.11-2012 (Стрибог). Контрольная сумма будет отображена и записана в файл с расширением `.tar.gostsum` в папке с tar-архивом, содержащим образы Deckhouse;
   - `--source` — чтобы указать адрес источника хранилища образов Deckhouse (по умолчанию `registry.deckhouse.ru/deckhouse/ee`);
   - Для аутентификации в официальном хранилище образов Deckhouse нужно использовать лицензионный ключ и параметр `--license`;
   - Для аутентификации в стороннем хранилище образов нужно использовать параметры `--source-login` и `--source-password`;
   - `--images-bundle-chunk-size=N` — для указания максимального размера файла (в ГБ), на которые нужно разбить архив образов. В результате работы вместо одного файла архива образов будет создан набор `.chunk`-файлов (например, `d8.tar.NNNN.chunk`). Чтобы загрузить образы из такого набора файлов, укажите в команде `d8 mirror push` имя файла без суффикса `.NNNN.chunk` (например, `d8.tar` для файлов `d8.tar.NNNN.chunk`);
   - `--tmp-dir` — путь к директории для временных файлов, который будет использоваться во время операций загрузки и выгрузки образов. Вся обработка выполняется в этом каталоге. Он должен иметь достаточное количество свободного дискового пространства, чтобы вместить весь загружаемый пакет образов. По умолчанию используется поддиректория `.tmp` в директории с пакетами образов.

   Дополнительные параметры конфигурации для семейства команд `d8 mirror` доступны в виде переменных окружения:

   - `HTTP_PROXY`/`HTTPS_PROXY` — URL прокси-сервера для запросов к HTTP(S) хостам, которые не указаны в списке хостов в переменной `$NO_PROXY`;
   - `NO_PROXY` — список хостов, разделенных запятыми, которые следует исключить из проксирования. Каждое значение может быть представлено в виде IP-адреса (`1.2.3.4`), CIDR (`1.2.3.4/8`), домена или символа (`*`). IP-адреса и домены также могут включать номер порта (`1.2.3.4:80`). Доменное имя соответствует как самому себе, так и всем поддоменам. Доменное имя начинающееся с `.`, соответствует только поддоменам. Например, `foo.com` соответствует `foo.com` и `bar.foo.com`; `.y.com` соответствует `x.y.com`, но не соответствует `y.com`. Символ `*` отключает проксирование;
   - `SSL_CERT_FILE` — указывает путь до сертификата SSL. Если переменная установлена, системные сертификаты не используются;
   - `SSL_CERT_DIR` — список каталогов, разделенный двоеточиями. Определяет, в каких каталогах искать файлы сертификатов SSL. Если переменная установлена, системные сертификаты не используются. [Подробнее...](https://www.openssl.org/docs/man1.0.2/man1/c_rehash.html);
   - `MIRROR_BYPASS_ACCESS_CHECKS` — установите для этого параметра значение `1`, чтобы отключить проверку корректности переданных учетных данных для registry;

   Пример команды для загрузки всех версий Deckhouse EE начиная с версии 1.59 (укажите лицензионный ключ):

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --since-version=1.59 /home/user/d8-bundle
   ```

   Пример команды для загрузки актуальных версий Deckhouse SE (укажите лицензионный ключ):

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --source='registry.deckhouse.ru/deckhouse/se' \
   /home/user/d8-bundle
   ```

   Пример команды для загрузки образов Deckhouse из стороннего хранилища образов:

   ```shell
   d8 mirror pull \
   --source='corp.company.com:5000/sys/deckhouse' \
   --source-login='<USER>' --source-password='<PASSWORD>' /home/user/d8-bundle
   ```

   Пример команды для загрузки пакета баз данных сканера уязвимостей:

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --no-platform --no-modules /home/user/d8-bundle
   ```

   Пример команды для загрузки пакетов всех доступных дополнительных модулей:

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --no-platform --no-security-db /home/user/d8-bundle
   ```

   Пример команды для загрузки пакетов модулей `stronghold` и `secrets-store-integration`:

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --no-platform --no-security-db \
   --include-module stronghold \
   --include-module secrets-store-integration \
   /home/user/d8-bundle
   ```

1. На хост с доступом к хранилищу, куда нужно загрузить образы Deckhouse, скопируйте загруженный пакет образов Deckhouse и установите [Deckhouse CLI](deckhouse-cli/).

1. Загрузите образы Deckhouse в хранилище с помощью команды `d8 mirror push`.

   Команда `d8 mirror push` загружает в репозиторий образы из всех пакетов, которые присутствуют в переданной директории.
   При необходимости выгрузить в репозиторий только часть пакетов, вы можете либо выполнить команду для каждого необходимого пакета образов передав ей прямой путь до пакета tar вместо директории, либо убрав расширение `.tar` у ненужных пакетов или переместив их вне директории.

   Пример команды для загрузки пакетов образов из директории `/mnt/MEDIA/d8-images` (укажите данные для авторизации при необходимости):

   ```shell
   d8 mirror push /mnt/MEDIA/d8-images 'corp.company.com:5000/sys/deckhouse' \
     --registry-login='<USER>' --registry-password='<PASSWORD>'
   ```

   > Перед загрузкой образов убедитесь, что путь для загрузки в хранилище образов существует (в примере — `/sys/deckhouse`) и у используемой учетной записи есть права на запись.
   > Если вы используете Harbor, вы не сможете выгрузить образы в корень проекта, используйте выделенный репозиторий в проекте для размещения образов Deckhouse.

1. После загрузки образов в хранилище можно переходить к установке Deckhouse. Воспользуйтесь [руководством по быстрому старту](/products/kubernetes-platform/gs/bm-private/step2.html).

   При запуске установщика используйте не официальное публичное хранилище образов Deckhouse, а хранилище в которое ранее были загружены образы Deckhouse. Для примера выше адрес запуска установщика будет иметь вид `corp.company.com:5000/sys/deckhouse/install:stable`, вместо `registry.deckhouse.ru/deckhouse/ee/install:stable`.

   В ресурсе [InitConfiguration](installing/configuration.html#initconfiguration) при установке также используйте адрес вашего хранилища и данные авторизации (параметры [imagesRepo](installing/configuration.html#initconfiguration-deckhouse-imagesrepo), [registryDockerCfg](installing/configuration.html#initconfiguration-deckhouse-registrydockercfg) или [шаг 3]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/products/kubernetes-platform/gs/bm-private/step3.html) руководства по быстрому старту).

### Как переключить работающий кластер Deckhouse на использование стороннего registry?

{% alert level="warning" %}
При использовании модуля [registry](modules/registry/) смена адреса и параметров registry выполняется в секции [registry](modules/deckhouse/configuration.html#parameters-registry) конфигурации модуля `deckhouse`. Пример настройки приведен в документации модуля [registry](modules/registry/examples.html).
{% endalert %}

{% alert level="warning" %}
Использование registry, отличных от `registry.deckhouse.io` и `registry.deckhouse.ru`, доступно только в коммерческих редакциях Deckhouse Kubernetes Platform.
{% endalert %}

Для переключения кластера Deckhouse на использование стороннего registry выполните следующие действия:

1. Выполните команду `deckhouse-controller helper change-registry` из пода Deckhouse с параметрами нового registry.
   Пример запуска:

   ```shell
   kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee
   ```

1. Если registry использует самоподписанные сертификаты, положите корневой сертификат соответствующего сертификата registry в файл `/tmp/ca.crt` в поде Deckhouse и добавьте к вызову опцию `--ca-file /tmp/ca.crt` или вставьте содержимое CA в переменную, как в примере ниже:

    ```shell
    CA_CONTENT=$(cat <<EOF
    -----BEGIN CERTIFICATE-----
    CERTIFICATE
    -----END CERTIFICATE-----
    -----BEGIN CERTIFICATE-----
    CERTIFICATE
    -----END CERTIFICATE-----
    EOF
    )
    kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- bash -c "echo '$CA_CONTENT' > /tmp/ca.crt && deckhouse-controller helper change-registry --ca-file /tmp/ca.crt --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee"
    ```

   Просмотреть список доступных ключей команды `deckhouse-controller helper change-registry` можно, выполнив следующую команду:

    ```shell
    kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --help
    ```

    Пример вывода:

    ```console
    usage: deckhouse-controller helper change-registry [<flags>] <new-registry>

    Change registry for deckhouse images.

    Flags:
      --help               Show context-sensitive help (also try --help-long and --help-man).
      --user=USER          User with pull access to registry.
      --password=PASSWORD  Password/token for registry user.
      --ca-file=CA-FILE    Path to registry CA.
      --scheme=SCHEME      Used scheme while connecting to registry, http or https.
      --dry-run            Don't change deckhouse resources, only print them.
      --new-deckhouse-tag=NEW-DECKHOUSE-TAG
                          New tag that will be used for deckhouse deployment image (by default
                          current tag from deckhouse deployment will be used).

    Args:
      <new-registry>  Registry that will be used for deckhouse images (example:
                      registry.deckhouse.io/deckhouse/ce). By default, https will be used, if you need
                      http - provide '--scheme' flag with http value
    ```

1. Дождитесь перехода пода Deckhouse в статус `Ready`. Если под находится в статусе `ImagePullBackoff`, перезапустите его.
1. Дождитесь применения bashible новых настроек на master-узле. В журнале bashible на master-узле (`journalctl -u bashible`) должно появится сообщение `Configuration is in sync, nothing to do`.
1. Если необходимо отключить автоматическое обновление Deckhouse через сторонний registry, удалите параметр `releaseChannel` из конфигурации модуля `deckhouse`.
1. Проверьте, не осталось ли в кластере подов с оригинальным адресом registry:

   ```shell
   kubectl get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
     | select(.image | startswith("registry.deckhouse"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

### Как создать кластер и запустить Deckhouse без использования каналов обновлений?

{% alert level="warning" %}
Этот способ следует использовать только в случае, если в изолированном приватном registry нет образов, содержащих информацию о каналах обновлений.
{% endalert %}

Если необходимо установить Deckhouse с отключенным автоматическим обновлением:

1. Используйте тег образа установщика соответствующей версии. Например, если вы хотите установить релиз `v1.44.3`, используйте образ `your.private.registry.com/deckhouse/install:v1.44.3`.
1. Укажите соответствующий номер версии в параметре [deckhouse.devBranch](installing/configuration.html#initconfiguration-deckhouse-devbranch) в ресурсе [InitConfiguration](installing/configuration.html#initconfiguration).
    > **Не указывайте** параметр [deckhouse.releaseChannel](installing/configuration.html#initconfiguration-deckhouse-releasechannel) в ресурсе [InitConfiguration](installing/configuration.html#initconfiguration).

Если вы хотите отключить автоматические обновления у уже установленного Deckhouse (включая обновления patch-релизов), удалите параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel) из конфигурации модуля `deckhouse`.

### Использование proxy-сервера

{% alert level="warning" %}
Доступно в следующих редакциях: BE, SE, SE+, EE, CSE Lite (1.67), CSE Pro (1.67).
{% endalert %}

{% offtopic title="Пример шагов по настройке proxy-сервера на базе Squid..." %}

1. Подготовьте сервер (или виртуальную машину). Сервер должен быть доступен с необходимых узлов кластера, и у него должен быть выход в интернет.
1. Установите Squid (здесь и далее примеры для Ubuntu):

   ```shell
   apt-get install squid
   ```

1. Создайте файл конфигурации Squid:

   ```shell
   cat <<EOF > /etc/squid/squid.conf
   auth_param basic program /usr/lib/squid3/basic_ncsa_auth /etc/squid/passwords
   auth_param basic realm proxy
   acl authenticated proxy_auth REQUIRED
   http_access allow authenticated

   # Укажите необходимый порт. Порт 3128 используется по умолчанию.
   http_port 3128
   ```

1. Создайте пользователя и пароль для аутентификации на proxy-сервере:

   Пример для пользователя `test` с паролем `test` (обязательно измените):

   ```shell
   echo "test:$(openssl passwd -crypt test)" >> /etc/squid/passwords
   ```

1. Запустите Squid и включите его автоматический запуск при загрузке сервера:

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

{% raw %}

### Автозагрузка переменных proxy пользователям в CLI

Начиная с версии платформы DKP v1.67 больше не настраивается файл `/etc/profile.d/d8-system-proxy.sh`, который устанавливал переменные proxy для пользователей. Для автозагрузки переменных proxy пользователям в CLI используйте ресурс `NodeGroupConfiguration`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: profile-proxy.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 99
  content: |
    {{- if .proxy }}
      {{- if .proxy.httpProxy }}
    export HTTP_PROXY={{ .proxy.httpProxy | quote }}
    export http_proxy=${HTTP_PROXY}
      {{- end }}
      {{- if .proxy.httpsProxy }}
    export HTTPS_PROXY={{ .proxy.httpsProxy | quote }}
    export https_proxy=${HTTPS_PROXY}
      {{- end }}
      {{- if .proxy.noProxy }}
    export NO_PROXY={{ .proxy.noProxy | join "," | quote }}
    export no_proxy=${NO_PROXY}
      {{- end }}
    bb-sync-file /etc/profile.d/profile-proxy.sh - << EOF
    export HTTP_PROXY=${HTTP_PROXY}
    export http_proxy=${HTTP_PROXY}
    export HTTPS_PROXY=${HTTPS_PROXY}
    export https_proxy=${HTTPS_PROXY}
    export NO_PROXY=${NO_PROXY}
    export no_proxy=${NO_PROXY}
    EOF
    {{- else }}
    rm -rf /etc/profile.d/profile-proxy.sh
    {{- end }}
```

{% endraw %}

## Изменение конфигурации

{% alert level="warning" %}
Для применения изменений конфигурации узлов необходимо выполнить команду `dhctl converge`, запустив инсталлятор Deckhouse. Эта команда синхронизирует состояние узлов с указанным в конфигурации.
{% endalert %}

### Как изменить конфигурацию кластера?

Общие параметры кластера хранятся в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration).

Чтобы изменить общие параметры кластера, выполните команду:

```shell
d8 platform edit cluster-configuration
```

После сохранения изменений Deckhouse приведет конфигурацию кластера к измененному состоянию. В зависимости от размеров кластера это может занять какое-то время.

### Как изменить конфигурацию облачного провайдера в кластере?

Настройки используемого облачного провайдера в облачном или гибридном кластере хранятся в структуре `<PROVIDER_NAME>ClusterConfiguration`, где `<PROVIDER_NAME>` — название/код провайдера. Например, для провайдера OpenStack структура будет называться [OpenStackClusterConfiguration]({% if site.mode == 'module' and site.d8Revision == 'CE' %}{{ site.urls[page.lang] }}/products/kubernetes-platform/documentation/v1/{% endif %}modules/cloud-provider-openstack/cluster_configuration.html).

Независимо от используемого облачного провайдера его настройки можно изменить с помощью следующей команды:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

### Как изменить конфигурацию статического кластера?

Настройки статического кластера хранятся в структуре [StaticClusterConfiguration](installing/configuration.html#staticclusterconfiguration).

Чтобы изменить параметры статического кластера, выполните команду:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit static-cluster-configuration
```

### Как переключить редакцию Deckhouse на CE/BE/SE/SE+/EE?

{% alert level="warning" %}
- Работоспособность инструкции подтверждена только для версий Deckhouse от `v1.70`. Если ваша версия младше, используйте соответствующую ей документацию.
- Для коммерческих изданий требуется действующий лицензионный ключ с поддержкой нужного издания. При необходимости можно [запросить временный ключ](https://deckhouse.ru/products/enterprise_edition.html).
- Инструкция подразумевает использование публичного адреса container registry: `registry.deckhouse.ru`. В случае использования другого адреса container registry измените команды или воспользуйтесь [инструкцией по переключению Deckhouse на использование стороннего registry](#как-переключить-работающий-кластер-deckhouse-на-использование-стороннего-registry).
- В редакциях Deckhouse CE/BE/SE/SE+ не поддерживается работа облачных провайдеров `dynamix`, `openstack`, `VCD`, `vSphere` (vSphere поддерживается в редакции SE+) и ряда модулей, подробное сравнение в [документации](revision-comparison.html).
- Все команды выполняются на master-узле существующего кластера под пользователем `root`.
{% endalert %}

Ниже описаны шаги для переключения кластера с любой редакцию на одну из поддерживаемых: Community Edition, Basic Edition, Standard Edition, Standard Edition+, Enterprise Edition.

#### Как переключиться с помощью модуля Registry?

1. Убедитесь, что кластер был переключен под управление [модулем registry](modules/registry/faq.html#как-мигрировать-на-модуль-registry). Если кластер не находится под управлемнием модуля registry, перейдите к инструкции ниже "Как переключиться без использования модуля Registry?".

1. Подготовьте переменные с токеном лицензии и названием новой редакции:

   > Заполнять переменную `LICENSE_TOKEN` при переключении на редакцию Deckhouse CE не требуется.
   > Значение переменной `NEW_EDITION` должно быть равно желаемой редакции Deckhouse, например для переключения на редакцию:
   > - CE, переменная должна быть `ce`;
   > - BE, переменная должна быть `be`;
   > - SE, переменная должна быть `se`;
   > - SE+, переменная должна быть `se-plus`;
   > - EE, переменная должна быть `ee`.

   ```shell
   NEW_EDITION=<PUT_YOUR_EDITION_HERE>
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   ```

1. Проверьте, чтобы [очередь Deckhouse](#как-проверить-очередь-заданий-в-deckhouse) была пустой и без ошибок.

1. Запустите временный под Deckhouse новой редакции, чтобы получить актуальные дайджесты и список модулей:

   для CE редакции:

   ```shell
   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
   kubectl run $NEW_EDITION-image --image=registry.deckhouse.ru/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   для других редакций:

   ```shell
   kubectl create secret docker-registry $NEW_EDITION-image-pull-secret \
    --docker-server=registry.deckhouse.ru \
    --docker-username=license-token \
    --docker-password=${LICENSE_TOKEN}

   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
   kubectl run $NEW_EDITION-image \
    --image=registry.deckhouse.ru/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION \
    --overrides="{\"spec\": {\"imagePullSecrets\":[{\"name\": \"$NEW_EDITION-image-pull-secret\"}]}}" \
    --command sleep -- infinity
   ```

   Как только под перейдёт в статус `Running`, выполните следующие команды:

   ```shell
   NEW_EDITION_MODULES=$(kubectl exec $NEW_EDITION-image -- ls -l deckhouse/modules/ | grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
   USED_MODULES=$(kubectl get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $NEW_EDITION_MODULES | tr ' ' '\n'))
   ```

1. Убедитесь, что используемые в кластере модули поддерживаются в желаемой редакции.

   Посмотреть список модулей, которые не поддерживаются в новой редакции и будут отключены:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   > Проверьте полученный список и убедитесь, что функциональность указанных модулей не используется вами в кластере и вы готовы их отключить.

   Отключите неподдерживаемые новой редакцией модули:

   ```shell
   echo $MODULES_WILL_DISABLE | tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
   ```

   Дождитесь, пока под Deckhouse перейдёт в состояние `Ready` и [убедитесь в выполнении всех задач в очереди](#как-проверить-очередь-заданий-в-deckhouse).

1. Удалите созданный секрет и под:

   ```shell
   kubectl delete pod/$NEW_EDITION-image
   kubectl delete secret/$NEW_EDITION-image-pull-secret
   ```

1. Выполните переключение на новую редакцию. Для этого укажите следующие параметр в ModuleConfig `deckhouse`. Для подробной настройки ознакомьтесь с конфигурацией модуля [deckhouse](modules/deckhouse/).

   ```yaml
   ---
   # Пример для Direct режима
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
           # Relax mode используется для проверки наличия текущей версии Deckhouse в указанном registry
           # Для переключения между редакциями необходимо использовать данный режим проверки registry
           checkMode: Relax
           # Укажите свой параметр <NEW_EDITION>
           imagesRepo: registry.deckhouse.ru/deckhouse/<NEW_EDITION>
           scheme: HTTPS
           # Укажите свой параметр <LICENSE_TOKEN>
           # Если переключение выполняется на CE редакцию, удалите данный параметр
           license: <LICENSE_TOKEN>
   ---
   # Пример для Unmanaged режима
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
           # Relax mode используется для проверки наличия текущей версии Deckhouse в указанном registry
           # Для переключения между редакциями необходимо использовать данный режим проверки
           checkMode: Relax
           # Укажите свой параметр <NEW_EDITION>
           imagesRepo: registry.deckhouse.ru/deckhouse/<NEW_EDITION>
           scheme: HTTPS
           # Укажите свой параметр <LICENSE_TOKEN>
           # Если переключение выполняется на CE редакцию, удалите данный параметр
           license: <LICENSE_TOKEN>
   ```

1. Дождитесь переключения registry. Для проверки выполнения переключения воспользуйтесь [инструкцией](modules/registry/faq.html#как-посмотреть-статус-переключения-режима-registry).

   Пример вывода:

   ```yaml
   conditions:
     - lastTransitionTime: "..."
       message: |-
         Mode: Relax
         registry.deckhouse.ru: all 1 items are checked
       reason: Ready
       status: "True"
       type: RegistryContainsRequiredImages
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. После переключения, удалите из ModuleConfig `deckhouse` параметр `checkMode: Relax`, чтобы переключить на проверку по-умолчанию. Удаление запустит проверку наличия критически важных компонентов в registry.

1. Дождитесь выполнения проверки, для этого воспользуйтесь [инструкцией](modules/registry/faq.html#как-посмотреть-статус-переключения-режима-registry).

   Пример вывода:

   ```yaml
   conditions:
     - lastTransitionTime: "..."
       message: |-
         Mode: Default
         registry.deckhouse.ru: all 155 items are checked
       reason: Ready
       status: "True"
       type: RegistryContainsRequiredImages
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. Проверьте, не осталось ли в кластере подов со старым адресом registry, где `<YOUR-PREVIOUS-EDITION>` — название вашей прошлой редакции:

   Для Unmanaged режима:

   ```shell
   kubectl get pods -A -o json | jq -r '.items[] | select(.spec.containers[] | select(.image | contains("deckhouse.ru/deckhouse/<YOUR-PREVIOUS-EDITION>"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

   Для других режимов, использующих фиксированный адрес.
   Данная проверка не учитывает внешние модули:

   ```shell
   # Получаем список актуальных digest'ов из файла images_digests.json внутри Deckhouse
   IMAGES_DIGESTS=$(kubectl -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- cat /deckhouse/modules/images_digests.json | jq -r '.[][]' | sort -u)

   # Проверяем, есть ли Pod'ы, использующие образы Deckhouse по адресу `registry.d8-system.svc:5001/system/deckhouse`
   # с digest'ом, отсутствующим в списке актуальных digest'ов из IMAGES_DIGESTS
   kubectl get pods -A -o json |
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

#### Как переключиться без использования модуля Registry?

1. Перед началом, отключите модуль Registry с [помощью инструкции](modules/registry/faq.html#как-мигрировать-обратно-с-модуля-registry).

1. Подготовьте переменные с токеном лицензии и названием новой редакции:

   > Заполнять переменные `NEW_EDITION` и `AUTH_STRING` при переключении на редакцию Deckhouse CE не требуется.
   > Значение переменной `NEW_EDITION` должно быть равно желаемой редакции Deckhouse, например для переключения на редакцию:
   > - CE, переменная должна быть `ce`;
   > - BE, переменная должна быть `be`;
   > - SE, переменная должна быть `se`;
   > - SE+, переменная должна быть `se-plus`;
   > - EE, переменная должна быть `ee`.

   ```shell
   NEW_EDITION=<PUT_YOUR_EDITION_HERE>
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
   ```

1. Проверьте, чтобы [очередь Deckhouse](#как-проверить-очередь-заданий-в-deckhouse) была пустой и без ошибок.

1. Создайте ресурс `NodeGroupConfiguration` для переходной авторизации в `registry.deckhouse.ru`:

   > Перед созданием ресурса ознакомьтесь с разделом [«Как добавить конфигурацию для дополнительного registry»](modules/node-manager/faq.html#как-добавить-конфигурацию-для-дополнительного-registry).
   >
   > При переходе на редакцию Deckhouse CE пропустите этот шаг.

   ```shell
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-$NEW_EDITION-config.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 30
     content: |
       _on_containerd_config_changed() {
         bb-flag-set containerd-need-restart
       }
       bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'
       mkdir -p /etc/containerd/conf.d
       bb-sync-file /etc/containerd/conf.d/$NEW_EDITION-registry.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry.configs]
             [plugins."io.containerd.grpc.v1.cri".registry.configs."registry.deckhouse.ru".auth]
               auth = "$AUTH_STRING"
       EOF_TOML
   EOF
   ```

   Дождитесь появления файла `/etc/containerd/conf.d/$NEW_EDITION-registry.toml` на узлах и завершения синхронизации bashible. Чтобы отследить статус синхронизации, проверьте значение `UPTODATE` (число узлов в этом статусе должно совпадать с общим числом узлов (`NODES`) в группе):

   ```shell
   kubectl get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Пример вывода:
  
   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   Также в журнале systemd-сервиса bashible должно появиться сообщение `Configuration is in sync, nothing to do` в результате выполнения следующей команды:

   ```shell
   journalctl -u bashible -n 5
   ```

   Пример вывода:

   ```console
   Aug 21 11:04:28 master-ee-to-se-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-se-0 bashible.sh[53407]: Annotate node master-ee-to-se-0 with annotation node.deckhouse.io/   configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master ee-to-se-0 bashible.sh[53407]: Successful annotate node master-ee-to-se-0 with annotation node.deckhouse.io/   configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-se-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Запустите временный под Deckhouse новой редакции, чтобы получить актуальные дайджесты и список модулей:

   ```shell
   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
   kubectl run $NEW_EDITION-image --image=registry.deckhouse.ru/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

1. После перехода пода в статус `Running` выполните следующие команды:

   ```shell
   NEW_EDITION_MODULES=$(kubectl exec $NEW_EDITION-image -- ls -l deckhouse/modules/ | grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
   USED_MODULES=$(kubectl get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $NEW_EDITION_MODULES | tr ' ' '\n'))
   ```

1. Убедитесь, что используемые в кластере модули поддерживаются в желаемой редакции.

   Посмотреть список модулей, которые не поддерживаются в новой редакции и будут отключены:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   > Проверьте полученный список и убедитесь, что функциональность указанных модулей не используется вами в кластере и вы готовы их отключить.

   Отключите неподдерживаемые новой редакцией модули:

   ```shell
   echo $MODULES_WILL_DISABLE | tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
   ```

   Дождитесь, пока под Deckhouse перейдёт в состояние `Ready` и [убедитесь в выполнении всех задач в очереди](#как-проверить-очередь-заданий-в-deckhouse).

1. Выполните команду `deckhouse-controller helper change-registry` из пода Deckhouse с параметрами новой редакции:

   Для переключения на BE/SE/SE+/EE издания:

   ```shell
   kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user=license-token --password=$LICENSE_TOKEN --new-deckhouse-tag=$DECKHOUSE_VERSION registry.deckhouse.ru/deckhouse/$NEW_EDITION
   ```

   Для переключения на CE издание:

   ```shell
   kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --new-deckhouse-tag=$DECKHOUSE_VERSION registry.deckhouse.ru/deckhouse/ce
   ```

1. Проверьте, не осталось ли в кластере подов со старым адресом registry, где `<YOUR-PREVIOUS-EDITION>` — название вашей прошлой редакции:

   ```shell
   kubectl get pods -A -o json | jq -r '.items[] | select(.spec.containers[] | select(.image | contains("deckhouse.ru/deckhouse/<YOUR-PREVIOUS-EDITION>"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

1. Удалите временные файлы, ресурс `NodeGroupConfiguration` и переменные:

   > При переходе на редакцию Deckhouse CE пропустите этот шаг.

   ```shell
   kubectl delete ngc containerd-$NEW_EDITION-config.sh
   kubectl delete pod $NEW_EDITION-image
   kubectl apply -f - <<EOF
       apiVersion: deckhouse.io/v1alpha1
       kind: NodeGroupConfiguration
       metadata:
         name: del-temp-config.sh
       spec:
         nodeGroups:
         - '*'
         bundles:
         - '*'
         weight: 90
         content: |
           if [ -f /etc/containerd/conf.d/$NEW_EDITION-registry.toml ]; then
             rm -f /etc/containerd/conf.d/$NEW_EDITION-registry.toml
           fi
   EOF
   ```

   После завершения синхронизации bashible (статус синхронизации на узлах отображается по значению `UPTODATE` у NodeGroup) удалите созданный ресурс NodeGroupConfiguration:

   ```shell
   kubectl delete ngc del-temp-config.sh
   ```

### Как переключить Deckhouse EE на CSE?

{% alert level="warning" %}
- Инструкция подразумевает использование публичного адреса container registry: `registry-cse.deckhouse.ru`. В случае использования другого адреса container registry измените команды или воспользуйтесь [инструкцией по переключению Deckhouse на использование стороннего registry](#как-переключить-работающий-кластер-deckhouse-на-использование-стороннего-registry).
- В Deckhouse CSE не поддерживается работа облачных кластеров и некоторых модулей. Подробнее о поддерживаемых модулях можно узнать на странице [сравнения редакций](revision-comparison.html).
- Миграция на Deckhouse CSE возможна только с версии Deckhouse EE 1.58, 1.64 или 1.67.
- Актуальные версии Deckhouse CSE: 1.58.2 для релиза 1.58, 1.64.1 для релиза 1.64 и 1.67.0 для релиза 1.67. Эти версии потребуется использовать далее для указания переменной `DECKHOUSE_VERSION`.
- Переход поддерживается только между одинаковыми минорными версиями, например, с Deckhouse EE 1.64 на Deckhouse CSE 1.64. Переход с версии EE 1.58 на CSE 1.67 потребует промежуточной миграции: сначала на EE 1.64, затем на EE 1.67, и только после этого — на CSE 1.67. Попытки обновить версию на несколько релизов сразу могут привести к неработоспособности кластера.
- Deckhouse CSE 1.58 и 1.64 поддерживает Kubernetes версии 1.27, Deckhouse CSE 1.67 поддерживает Kubernetes версий 1.27 и 1.29.
- При переключении на Deckhouse CSE возможна временная недоступность компонентов кластера.
{% endalert %}

Для переключения кластера Deckhouse Enterprise Edition на Certified Security Edition выполните следующие действия (все команды выполняются на master-узле кластера от имени пользователя с настроенным контекстом `kubectl` или от имени суперпользователя):

#### Как переключиться с помощью модуля Registry?

1. Убедитесь, что кластер был переключен под управление [модулем registry](modules/registry/faq.html#как-мигрировать-на-модуль-registry). Если кластер не находится под управлемнием модуля registry, перейдите к инструкции ниже "Как переключиться без использования модуля Registry?".

1. Настройте кластер на использование необходимой версии Kubernetes (см. примечание выше про доступные версии Kubernetes). Для этого:
   1. Выполните команду:

      ```shell
      d8 platform edit cluster-configuration
      ```

   1. Измените параметр `kubernetesVersion` на необходимое значение, например, `"1.27"` (в кавычках) для Kubernetes 1.27.
   1. Сохраните изменения. Узлы кластера начнут последовательно обновляться.
   1. Дождитесь окончания обновления. Отслеживать ход обновления можно с помощью команды `kubectl get no`. Обновление можно считать завершенным, когда в выводе команды у каждого узла кластера в колонке `VERSION` появится обновленная версия.

1. Подготовьте переменные с токеном лицензии:

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   ```

1. Запустите временный под Deckhouse новой редакции, чтобы получить актуальные дайджесты и список модулей:

   ```shell
   kubectl create secret docker-registry cse-image-pull-secret \
    --docker-server=registry-cse.deckhouse.ru \
    --docker-username=license-token \
    --docker-password=${LICENSE_TOKEN}

   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
   kubectl run cse-image \
    --image=registry-cse.deckhouse.ru/deckhouse/cse/install:$DECKHOUSE_VERSION \
    --overrides="{\"spec\": {\"imagePullSecrets\":[{\"name\": \"cse-image-pull-secret\"}]}}" \
    --command sleep -- infinity
   ```

   Как только под перейдёт в статус `Running`, выполните следующие команды:

   ```shell
   CSE_MODULES=$(kubectl exec cse-image -- ls -l deckhouse/modules/ | awk {'print $9'} |grep -oP "\d.*-\w*" | cut -c5-)
   USED_MODULES=$(kubectl get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $CSE_MODULES | tr ' ' '\n'))
   ```

1. Убедитесь, что используемые в кластере модули поддерживаются в Deckhouse CSE.
   Например, в Deckhouse CSE 1.58 и 1.64 отсутствует модуль cert-manager. Поэтому, перед отключением модуля cert-manager необходимо перевести режим работы HTTPS некоторых компонентов (например [user-authn](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.58/modules/user-authn/configuration.html#parameters-https-mode) или [prometheus](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.58/modules/prometheus/configuration.html#parameters-https-mode)) на альтернативные варианты работы, либо изменить [глобальный параметр](deckhouse-configure-global.html#parameters-modules-https-mode) отвечающий за режим работы HTTPS в кластере.

   Отобразить список модулей, которые не поддерживаются в Deckhouse CSE и будут отключены, можно следующей командой:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   > Проверьте список и убедитесь, что функциональность указанных модулей не задействована вами в кластере, и вы готовы к их отключению.

   Отключите неподдерживаемые в Deckhouse CSE модули:

   ```shell
   echo $MODULES_WILL_DISABLE | 
     tr ' ' '\n' | awk {'print "kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module disable",$1'} | bash
   ```

   В Deckhouse CSE не поддерживается компонент earlyOOM. Отключите его с помощью [настройки](modules/node-manager/configuration.html#parameters-earlyoomenabled).

   Дождитесь перехода пода Deckhouse в статус `Ready` и выполнения всех задач в очереди.

   ```shell
   kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

   Проверьте, что отключенные модули перешли в состояние `Disabled`.

   ```shell
   kubectl get modules
   ```

1. Удалите созданный секрет и под:

   ```shell
   kubectl delete pod/cse-image
   kubectl delete secret/cse-image-pull-secret
   ```

1. Выполните переключение на новую редакцию. Для этого укажите следующие параметр в ModuleConfig `deckhouse`. Для подробной настройки ознакомьтесь с конфигурацией модуля [deckhouse](modules/deckhouse/).

   ```yaml
   ---
   # Пример для Direct режима
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
           # Relax mode используется для проверки наличия текущей версии Deckhouse в указанном registry
           # Для переключения между редакциями необходимо использовать данный режим проверки registry
           checkMode: Relax
           imagesRepo: registry-cse.deckhouse.ru/deckhouse/cse
           scheme: HTTPS
           # Укажите свой параметр <LICENSE_TOKEN>
           license: <LICENSE_TOKEN>
   ---
   # Пример для Unmanaged режима
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
           # Relax mode используется для проверки наличия текущей версии Deckhouse в указанном registry
           # Для переключения между редакциями необходимо использовать данный режим проверки
           checkMode: Relax
           imagesRepo: registry-cse.deckhouse.ru/deckhouse/cse
           scheme: HTTPS
           # Укажите свой параметр <LICENSE_TOKEN>
           license: <LICENSE_TOKEN>
   ```

1. Дождитесь переключения registry. Для проверки выполнения переключения воспользуйтесь [инструкцией](modules/registry/faq.html#как-посмотреть-статус-переключения-режима-registry).

   Пример вывода:

   ```yaml
   conditions:
     - lastTransitionTime: "..."
       message: |-
         Mode: Relax
         registry-cse.deckhouse.ru: all 1 items are checked
       reason: Ready
       status: "True"
       type: RegistryContainsRequiredImages
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. После переключения, удалите из ModuleConfig `deckhouse` параметр `checkMode: Relax`, чтобы переключить на проверку по-умолчанию. Удаление запустит проверку наличия критически важных компонентов в registry.

1. Дождитесь выполнения проверки, для этого воспользуйтесь [инструкцией](modules/registry/faq.html#как-посмотреть-статус-переключения-режима-registry).

   Пример вывода:

   ```yaml
   conditions:
     - lastTransitionTime: "..."
       message: |-
         Mode: Default
         registry-cse.deckhouse.ru: all 155 items are checked
       reason: Ready
       status: "True"
       type: RegistryContainsRequiredImages
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. Проверьте, не осталось ли в кластере подов с адресом registry для Deckhouse EE:

   Для Unmanaged режима:

   ```shell
   kubectl get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
     | select(.image | contains("deckhouse.ru/deckhouse/ee"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

   Для других режимов, использующих фиксированный адрес.
   Данная проверка не учитывает внешние модули:

   ```shell
   # Получаем список актуальных digest'ов из файла images_digests.json внутри Deckhouse
   IMAGES_DIGESTS=$(kubectl -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- cat /deckhouse/modules/images_digests.json | jq -r '.[][]' | sort -u)

   # Проверяем, есть ли Pod'ы, использующие образы Deckhouse по адресу `registry.d8-system.svc:5001/system/deckhouse`
   # с digest'ом, отсутствующим в списке актуальных digest'ов из IMAGES_DIGESTS
   kubectl get pods -A -o json |
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

   Если в выводе присутствуют поды модуля chrony, заново включите данный модуль (в Deckhouse CSE этот модуль по умолчанию выключен):

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module enable chrony
   ```

#### Как переключиться без использования модуля Registry?

1. Перед началом, отключите модуль Registry с [помощью инструкции](modules/registry/faq.html#как-мигрировать-обратно-с-модуля-registry).

1. Настройте кластер на использование необходимой версии Kubernetes (см. примечание выше про доступные версии Kubernetes). Для этого:
   1. Выполните команду:

      ```shell
      d8 platform edit cluster-configuration
      ```

   1. Измените параметр `kubernetesVersion` на необходимое значение, например, `"1.27"` (в кавычках) для Kubernetes 1.27.
   1. Сохраните изменения. Узлы кластера начнут последовательно обновляться.
   1. Дождитесь окончания обновления. Отслеживать ход обновления можно с помощью команды `kubectl get no`. Обновление можно считать завершенным, когда в выводе команды у каждого узла кластера в колонке `VERSION` появится обновленная версия.

1. Подготовьте переменные с токеном лицензии и создайте NodeGroupConfiguration для переходной авторизации в `registry-cse.deckhouse.ru`:

   > Перед созданием ресурса ознакомьтесь с разделом [«Как добавить конфигурацию для дополнительного registry»](modules/node-manager/faq.html#как-добавить-конфигурацию-для-дополнительного-registry)

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
   kubectl apply -f - <<EOF
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-cse-config.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 30
     content: |
       _on_containerd_config_changed() {
         bb-flag-set containerd-need-restart
       }
       bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

       mkdir -p /etc/containerd/conf.d
       bb-sync-file /etc/containerd/conf.d/cse-registry.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry]
             [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
           [plugins."io.containerd.grpc.v1.cri".registry.mirrors."registry-cse.deckhouse.ru"]
             endpoint = ["https://registry-cse.deckhouse.ru"]
           [plugins."io.containerd.grpc.v1.cri".registry.configs]
             [plugins."io.containerd.grpc.v1.cri".registry.configs."registry-cse.deckhouse.ru".auth]
               auth = "$AUTH_STRING"
       EOF_TOML
   EOF
   ```

   Дождитесь завершения синхронизации и появления файла `/etc/containerd/conf.d/cse-registry.toml` на узлах.

   Статус синхронизации можно отследить по значению `UPTODATE` (отображаемое число узлов в этом статусе должно совпадать с общим числом узлов (`NODES`) в группе):

   ```shell
   kubectl get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Пример вывода:

   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   В журнале systemd-сервиса bashible должно появиться сообщение `Configuration is in sync, nothing to do` в результате выполнения следующей команды:

   ```shell
   journalctl -u bashible -n 5
   ```

   Пример вывода:

   ```console
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 bashible.sh[53407]: Successful annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Выполните следующие команды для запуска временного пода Deckhouse CSE для получения актуальных дайджестов и списка модулей:

   ```shell
   DECKHOUSE_VERSION=v<ВЕРСИЯ_DECKHOUSE_CSE>
   # Например, DECKHOUSE_VERSION=v1.58.2
   kubectl run cse-image --image=registry-cse.deckhouse.ru/deckhouse/cse/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   Как только под перейдёт в статус `Running`, выполните следующие команды:

   ```shell
   CSE_SANDBOX_IMAGE=$(kubectl exec cse-image -- cat deckhouse/candi/images_digests.json | grep pause | grep -oE 'sha256:\w*')
   CSE_K8S_API_PROXY=$(kubectl exec cse-image -- cat deckhouse/candi/images_digests.json | grep kubernetesApiProxy | grep -oE 'sha256:\w*')
   CSE_MODULES=$(kubectl exec cse-image -- ls -l deckhouse/modules/ | awk {'print $9'} |grep -oP "\d.*-\w*" | cut -c5-)
   USED_MODULES=$(kubectl get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $CSE_MODULES | tr ' ' '\n'))
   CSE_DECKHOUSE_KUBE_RBAC_PROXY=$(kubectl exec cse-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
   ```

   > Дополнительная команда, которая необходима только при переключении на Deckhouse CSE версии 1.64:
   >
   > ```shell
   > CSE_DECKHOUSE_INIT_CONTAINER=$(kubectl exec cse-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.init")
   > ```

1. Убедитесь, что используемые в кластере модули поддерживаются в Deckhouse CSE.
   Например, в Deckhouse CSE 1.58 и 1.64 отсутствует модуль cert-manager. Поэтому, перед отключением модуля cert-manager необходимо перевести режим работы HTTPS некоторых компонентов (например [user-authn](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.58/modules/user-authn/configuration.html#parameters-https-mode) или [prometheus](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.58/modules/prometheus/configuration.html#parameters-https-mode)) на альтернативные варианты работы, либо изменить [глобальный параметр](deckhouse-configure-global.html#parameters-modules-https-mode) отвечающий за режим работы HTTPS в кластере.

   Отобразить список модулей, которые не поддерживаются в Deckhouse CSE и будут отключены, можно следующей командой:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   > Проверьте список и убедитесь, что функциональность указанных модулей не задействована вами в кластере, и вы готовы к их отключению.

   Отключите неподдерживаемые в Deckhouse CSE модули:

   ```shell
   echo $MODULES_WILL_DISABLE | 
     tr ' ' '\n' | awk {'print "kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module disable",$1'} | bash
   ```

   В Deckhouse CSE не поддерживается компонент earlyOOM. Отключите его с помощью [настройки](modules/node-manager/configuration.html#parameters-earlyoomenabled).

   Дождитесь перехода пода Deckhouse в статус `Ready` и выполнения всех задач в очереди.

   ```shell
   kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

   Проверьте, что отключенные модули перешли в состояние `Disabled`.

   ```shell
   kubectl get modules
   ```

1. Создайте NodeGroupConfiguration:

   ```shell
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: cse-set-sha-images.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 50
     content: |
        _on_containerd_config_changed() {
          bb-flag-set containerd-need-restart
        }
        bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

        bb-sync-file /etc/containerd/conf.d/cse-sandbox.toml - containerd-config-file-changed << "EOF_TOML"
        [plugins]
          [plugins."io.containerd.grpc.v1.cri"]
            sandbox_image = "registry-cse.deckhouse.ru/deckhouse/cse@$CSE_SANDBOX_IMAGE"
        EOF_TOML

        sed -i 's|image: .*|image: registry-cse.deckhouse.ru/deckhouse/cse@$CSE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh
        sed -i 's|crictl pull .*|crictl pull registry-cse.deckhouse.ru/deckhouse/cse@$CSE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh
   EOF
   ```

   Дождитесь завершения синхронизации bashible на всех узлах.

   Состояние синхронизации можно отследить по значению `UPTODATE` статуса (отображаемое число узлов в этом статусе должно совпадать с общим числом узлов (`NODES`) в группе):

   ```shell
   kubectl get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   В журнале systemd-сервиса bashible на узлах должно появиться сообщение `Configuration is in sync, nothing to do` в результате выполнения следующей команды:

   ```shell
   journalctl -u bashible -n 5
   ```

   Пример вывода:

   ```console
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 bashible.sh[53407]: Successful annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Актуализируйте секрет доступа к registry Deckhouse CSE, выполнив следующую команду:

   ```shell
   kubectl -n d8-system create secret generic deckhouse-registry \
     --from-literal=".dockerconfigjson"="{\"auths\": { \"registry-cse.deckhouse.ru\": { \"username\": \"license-token\", \"password\": \"$LICENSE_TOKEN\", \"auth\": \"$AUTH_STRING\" }}}" \
     --from-literal="address"=registry-cse.deckhouse.ru \
     --from-literal="path"=/deckhouse/cse \
     --from-literal="scheme"=https \
     --type=kubernetes.io/dockerconfigjson \
     --dry-run='client' \
     -o yaml | kubectl -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- kubectl replace -f -
   ```

1. Измените образ Deckhouse на образ Deckhouse CSE:

   Команда для Deckhouse CSE версии 1.58:

   ```shell
   kubectl -n d8-system set image deployment/deckhouse kube-rbac-proxy=registry-cse.deckhouse.ru/deckhouse/cse@$CSE_DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry-cse.deckhouse.ru/deckhouse/cse:$DECKHOUSE_VERSION
   ```

   Команда для Deckhouse CSE версии 1.64 и 1.67:

   ```shell
   kubectl -n d8-system set image deployment/deckhouse init-downloaded-modules=registry-cse.deckhouse.ru/deckhouse/cse@$CSE_DECKHOUSE_INIT_CONTAINER kube-rbac-proxy=registry-cse.deckhouse.ru/deckhouse/cse@$CSE_DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry-cse.deckhouse.ru/deckhouse/cse:$DECKHOUSE_VERSION
   ```

1. Дождитесь перехода пода Deckhouse в статус `Ready` и [выполнения всех задач в очереди](#как-проверить-очередь-заданий-в-deckhouse). Если в процессе возникает ошибка `ImagePullBackOff`, подождите автоматического перезапуска пода.

   Посмотреть статус пода Deckhouse:

   ```shell
   kubectl -n d8-system get po -l app=deckhouse
   ```

   Проверить состояние очереди Deckhouse:

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -c deckhouse -- deckhouse-controller queue list
   ```

1. Проверьте, не осталось ли в кластере подов с адресом registry для Deckhouse EE:

   ```shell
   kubectl get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
     | select(.image | contains("deckhouse.ru/deckhouse/ee"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

   Если в выводе присутствуют поды модуля chrony, заново включите данный модуль (в Deckhouse CSE этот модуль по умолчанию выключен):

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module enable chrony
   ```

1. Очистите временные файлы, ресурс `NodeGroupConfiguration` и переменные:

   ```shell
   rm /tmp/cse-deckhouse-registry.yaml

   kubectl delete ngc containerd-cse-config.sh cse-set-sha-images.sh

   kubectl delete pod cse-image

   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: del-temp-config.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 90
     content: |
       if [ -f /etc/containerd/conf.d/cse-registry.toml ]; then
         rm -f /etc/containerd/conf.d/cse-registry.toml
       fi
       if [ -f /etc/containerd/conf.d/cse-sandbox.toml ]; then
         rm -f /etc/containerd/conf.d/cse-sandbox.toml
       fi
   EOF
   ```

   После синхронизации (статус синхронизации на узлах можно отследить по значению `UPTODATE` у NodeGroup) удалите созданный ресурс NodeGroupConfiguration:

   ```shell
   kubectl delete ngc del-temp-config.sh
   ```

### Как получить доступ к контроллеру Deckhouse в multi-master-кластере?

В кластерах с несколькими master-узлами Deckhouse запускается в режиме высокой доступности (в нескольких экземплярах). Для доступа к активному контроллеру Deckhouse можно использовать следующую команду (на примере команды `deckhouse-controller queue list`):

```shell
kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
```

### Как обновить версию Kubernetes в кластере?

Чтобы обновить версию Kubernetes в кластере, измените параметр [kubernetesVersion](installing/configuration.html#clusterconfiguration-kubernetesversion) в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration), выполнив следующие шаги:

1. Выполните команду:

   ```shell
   d8 platform edit cluster-configuration
   ```

1. Измените параметр `kubernetesVersion`.
1. Сохраните изменения. Узлы кластера начнут последовательно обновляться.
1. Дождитесь окончания обновления. Отслеживать ход обновления можно с помощью команды `kubectl get no`. Обновление можно считать завершенным, когда в выводе команды у каждого узла кластера в колонке `VERSION` появится обновленная версия.

### Как запускать Deckhouse на произвольном узле?

Для запуска Deckhouse на произвольном узле установите у модуля `deckhouse` соответствующий [параметр](modules/deckhouse/configuration.html) `nodeSelector` и не задавайте `tolerations`. Необходимые значения `tolerations` в этом случае будут проставлены автоматически.

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

### Как принудительно отключить IPv6 на узлах кластера Deckhouse?

> Внутреннее взаимодействие между компонентами кластера Deckhouse осуществляется по протоколу IPv4. Однако, на уровне операционной системы узлов кластера, как правило, по умолчанию активен IPv6. Это приводит к автоматическому присвоению IPv6-адресов всем сетевым интерфейсам, включая интерфейсы подов. В результате возникает нежелательный сетевой трафик — например, избыточные DNS-запросы типа `AAAA`, которые могут повлиять на производительность и усложнить отладку сетевых взаимодействий.

Для корректного отключения IPv6 на уровне узлов в кластере, управляемом Deckhouse, достаточно задать необходимые параметры через ресурс [NodeGroupConfiguration](./modules/node-manager/cr.html#nodegroupconfiguration):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: disable-ipv6.sh
spec:
  nodeGroups:
  - '*'
  bundles:
  - '*'
  weight: 50
  content: |
    GRUB_FILE_PATH="/etc/default/grub"
    
    if ! grep -q "ipv6.disable" "$GRUB_FILE_PATH"; then
      sed -E -e 's/^(GRUB_CMDLINE_LINUX_DEFAULT="[^"]*)"/\1 ipv6.disable=1"/' -i "$GRUB_FILE_PATH"
      update-grub
      
      bb-flag-set reboot
    fi
```

{% alert level="warning" %}
После применения ресурса настройки GRUB будут обновлены, и узлы кластера начнут последовательную перезагрузку для применения изменений.
{% endalert %}

### Как изменить container runtime на containerd v2 на узлах?

Миграцию на containerd v` можно выполнить одним из следующих способов:

* Указав значение `ContainerdV2` для параметра [`defaultCRI`](./installing/configuration.html#clusterconfiguration-defaultcri) в общих параметрах кластера. В этом случае container runtime будет изменен во всех группах узлов, для которых он явно не определен с помощью параметра [`spec.cri.type`](./modules/node-manager/cr.html#nodegroup-v1-spec-cri-type).
* Указав значение `ContainerdV2` для параметра [`spec.cri.type`](./modules/node-manager/cr.html#nodegroup-v1-spec-cri-type) для конкретной группы узлов.

{% alert level="info" %}
Миграция на containerd v2 возможна при выполнении следующих условий:

* Узлы соответствуют требованиям, описанным [в общих параметрах кластера](./installing/configuration.html#clusterconfiguration-defaultcri).
* На сервере нет кастомных конфигураций в `/etc/containerd/conf.d` ([пример кастомной конфигурации](./modules/node-manager/faq.html#как-использовать-containerd-с-поддержкой-nvidia-gpu)).
{% endalert %}

При миграции на containerd v2 очищается папка `/var/lib/containerd`. Для containerd используется папка `/etc/containerd/conf.d`. Для containerd v2 используется `/etc/containerd/conf2.d`.
