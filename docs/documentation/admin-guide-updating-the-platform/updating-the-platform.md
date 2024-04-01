---
title: Обновление платформы
permalink: ru/guides/...html
lang: ru
---

Оглавление. Переосмысление:
- [not in order][search info] Гарантия совместимости при обновлении - мы гарантируем такие обновления
- [not in order][search info] Единые обновления и ""фундамента"" и ""узлов"" и ""решений" - автоматизация DKP по всем фронтам обновлений
- [not in order][search info] Полностью автоматический процесс обновления (и четкий контракт с разработчиками) - работа с разработчиком-пользователем
- [Маша time] Каналы обновлений - инфы достаточно
- [Маша time] Выбор окон для обновлений - уточнить достаточно ли инфы
- [Маша time] Обновление в закрытом контуре - инфы достаточно
- [rename][search info - уточнение сценария - достаточно ли информации?] Предоставление LTS обновлений на твёрдых носителях – При невозможности выдачи ограниченного доступа в закрытый контур для настройки проксирующего registry.
- [Маша time] Отправка уведомлений об обновлениях - инфы достаточно
- [Маша time] Автоматические обновления - описательная часть - need help may be
- [Маша time] Ручное одобрение обновлений - описательная часть - need help may be
- [search info] Поддержка более 2-ух последних версий Kubernetes и версий Deckhouse + LTS → раскрывашка CE и EE
- [Маша time] История применения релизов в кластере - уточнить достаточно ли инфы

## Для readme-file

Deckhouse Kubernetes Platform предоставляет возможноси управления запуском обновлений: от автоматического запуска при поступлении обновления в канал (early-access, stable, lts), включая такие инструменты как ""окна обновлений"", до полностью контролируемого режима, где обновления утверждаются вручную. Сам процесс обновления осуществляется полностью автоматически, включая как инфраструктурные компоненты и ПО на узлах, так и всю высокоуровневую функциональность (платформа Kubernetes, аутентификация, мониторинг, журналирование, управление сертификатами, балансировка трафика, безопасность, виртуализация и др.). Deckhouse обеспечивает стабильность работы платформы и приложений при обновлениях, при условии соблюдения последними рекомендаций отказаустойчевости: наличие более чем одной реплики и указания максимального количества недоступных реплик (PDB). Для обеспечения обратной совместимости Deckhouse применяет трёхкомпонентный подход: сохранение обратной совместимости между версиями, правила устаревания (deprecation policy) и автоматическую блокировку запуска обновлений при возникновении проблем. Всегда поддерживается не менее 4 версий Kubernetes. Платформа поддерживает обновления в изолированных средах и при помощи твердых носителей. Все эти возможности гарантируют предсказуемость обновления, обеспечивая баланс между свежестью и стабильностью."							

## Каналы обновлений

Существует несколько каналов обновления для кластера. К кластерам, как элементам инфраструктуры, обычно предъявляются различные требования.

Например, production-кластер, в отличие от кластера разработки, более требователен к надежности: в нем нежелательно часто обновлять или изменять какие-либо компоненты без особой необходимости, при этом сами компоненты должны быть тщательно протестированы.
По возможности используйте разные каналы обновлений для кластеров. Для кластера разработки используйте менее стабильный канал обновлений, нежели для тестового или stage-кластера (pre-production-кластер).

Мы рекомендуем использовать канал обновлений `Early Access` или `Stable` для production-кластеров. Если в production-окружении более одного кластера, предпочтительно использовать для них разные каналы обновлений. Например, `Early Access` для одного, а `Stable` — для другого. Если использовать разные каналы обновлений по каким-либо причинам невозможно, рекомендуется устанавливать разные окна обновлений.

Deckhouse Kubernetes Platform использует пять каналов обновлений. Между ними можно переключаться с помощью модуля *deckhouse*: достаточно указать желаемый канал обновлений в конфигурации модуля:

1. **Rock Solid**. Наиболее стабильный канал обновлений. Подойдет для кластеров, которым необходимо обеспечить повышенный уровень стабильности. Обновления функционала до этого канала доходят не ранее чем через месяц после их появления в релизе.
2. **Stable**. Стабильный канал обновлений для кластеров, в которых закончена активная работа и преимущественно осуществляется эксплуатация. Обновления функционала до этого канала обновлений доходят не ранее чем через две недели после их появления в релизе.
3. **Early Access**. Рекомендуемый канал обновлений, если вы не уверены в выборе. Подойдет для кластеров, в которых идет активная работа (запускаются, дорабатываются новые приложения и т. п.). Обновления функционала до этого канала обновлений доходят не ранее чем через одну неделю после их появления в релизе.
4. **Beta**. Ориентирован на кластеры разработки, как и канал обновлений Alpha. Получает версии, предварительно опробованные на канале обновлений Alpha.
5. **Alpha**. Наименее стабильный канал обновлений с наиболее частым появлением новых версий. Ориентирован на кластеры разработки с небольшим количеством разработчиков.

{% alert %}
Используйте канал обновлений `Early Access` или `Stable`. Установите [окно автоматических обновлений](/documentation/v1/modules/002-deckhouse/usage.html#конфигурация-окон-обновлений) или [ручной режим](/documentation/v1/modules/002-deckhouse/usage.html#ручное-подтверждение-обновлений).
{% endalert %}

Выберите необходимый канал обновлений и [режим обновлений](/documentation/v1/modules/002-deckhouse/configuration.html#parameters-releasechannel -дать ссылку на раздел этой доки), который соответствует вашим ожиданиям. Чем стабильнее канал обновлений, тем позже до него доходит новая функциональность.

{% alert level="warning" %}
Даже в очень нагруженных и критичных кластерах не стоит отключать использование канала обновлений. Лучшая стратегия — плановое обновление. В инсталляциях Deckhouse, которые не обновлялись полгода или более, могут присутствовать ошибки. Как правило, эти ошибки давно устранены в новых версиях. В этом случае оперативно решить возникшую проблему будет непросто.
{% endalert %}

#### Установка желаемого канала обновлений

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

#### Версия Deckhouse Kubernetes Platform на канале обновлений

Информацию о том, какая версия Deckhouse находится на каком канале обновлений, можно получить на <https://flow.deckhouse.io>.

#### Cмена канала обновлений

* При смене канала обновлений на **более стабильный** (например, с `Alpha` на `EarlyAccess`) Deckhouse скачивает данные о релизе (в примере — из канала `EarlyAccess`) и сравнивает их с данными из существующих в кластере custom resouce'ов `DeckhouseRelease`:
  * Более *поздние* релизы, которые еще не были применены (в статусе `Pending`), удаляются.
  * Если более *поздние* релизы уже применены (в статусе `Deployed`), смены релиза не происходит. В этом случае Deckhouse останется на таком релизе до тех пор, пока на канале обновлений `EarlyAccess` не появится более поздний релиз.
* При смене канала обновлений на **менее стабильный** (например, с `EarlyAcess` на `Alpha`):
  * Deckhouse скачивает данные о релизе (в примере — из канала `Alpha`) и сравнивает их с данными из существующих в кластере custom resource'ов `DeckhouseRelease`.
  * Затем Deckhouse выполняет обновление согласно установленным [параметрам обновления](modules/002-deckhouse/configuration.html#parameters-update).

{% offtopic title="Схема использования параметра releaseChannel при установке и в процессе работы Deckhouse" %}
![Схема использования параметра releaseChannel при установке и в процессе работы Deckhouse](images/common/deckhouse-update-process.png)
{% endofftopic %}

### Что делать, если Deckhouse не получает обновлений из канала обновлений? - перефразировать

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

## Окна обновлений

Управление [окнами обновлений](/documentation/v1/modules/002-deckhouse/configuration.html#parameters-update-windows) позволяет планово обновлять релизы Deckhouse в автоматическом режиме в периоды «затишья», когда нагрузка на кластер далека от пиковой.

  В Deckhouse реализован механизм автоматического обновления. Этот механизм использует [5 каналов обновлений](../../deckhouse-release-channels.html), различающиеся стабильностью и частотой выхода версий. Ознакомьтесь подробнее с тем, [как работает механизм автоматического обновления](../../deckhouse-faq.html#как-работает-автоматическое-обновление-deckhouse) и [как установить желаемый канал обновлений](../../deckhouse-faq.html#как-установить-желаемый-канал-обновлений).
- **[Режим обновлений](configuration.html#parameters-update-mode)** и **[окна обновлений](configuration.html#parameters-update-windows)**

### Конфигурация окон обновлений

Настроить время, когда Deckhouse будет устанавливать обновления, можно в параметре [update.windows](configuration.html#parameters-update-windows) конфигурации модуля.

Пример настройки двух ежедневных окон обновлений: с 8:00 до 10:00 и c 20:00 до 22:00 (UTC):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: EarlyAccess
    update:
      windows: 
        - from: "8:00"
          to: "10:00"
        - from: "20:00"
          to: "22:00"
```

Также можно настроить обновления в определенные дни, например по вторникам и субботам с 18:00 до 19:30 (UTC):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      windows: 
        - from: "18:00"
          to: "19:30"
          days:
            - Tue
            - Sat
```

## Обновление в закрытом контуре

Deckhouse Kubernetes Platform отвечает за то, чтобы кластер одинаково работал на любой поддерживаемой инфраструктуре:

* в облаках (смотри информацию по соответствующему cloud провайдеру - добавить ссылку);
* на виртуальных машинах или железе (включая on-premises);
* в гибридной инфраструктуре.

Образы всех компонентов Deckhouse Kubernetes Platform, включая control plane, хранятся в высокодоступном и геораспределенном container registry. Для организации доступа из изолированных контуров container registry доступен с фиксированного набора IP-адресов.

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

### Настройка Nexus

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

1. Включите `Docker Bearer Token Realm` (*Administration* -> *Security* -> *Realms*):
  ![Включение `Docker Bearer Token Realm`](images/registry/nexus/nexus-realm.png)

2. Создайте **проксирующий** репозиторий Docker (*Administration* -> *Repository* -> *Repositories*), указывающий на [Deckhouse registry](https://registry.deckhouse.ru/):
  ![Создание проксирующего репозитория Docker](images/registry/nexus/nexus-repository.png)

3. Заполните поля страницы создания репозитория следующим образом:
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

### Настройки Harbor

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
Доступно только в Enterprise Edition.
{% endalert %}

1. При необходимости авторизуйтесь в container registry `registry.deckhouse.ru` (или `registry.deckhouse.io`) с помощью вашего лицензионного ключа.

   ```shell
   docker login -u license-token registry.deckhouse.ru
   ```

1. Запустите установщик Deckhouse версии 1.56.3 или выше.

   ```shell
   docker run -ti --pull=always -v $(pwd)/d8-images:/tmp/d8-images registry.deckhouse.ru/deckhouse/ee/install:v1.58.4 bash
   ```

   Обратите внимание, что в контейнер установщика монтируется директория с файловой системы хоста, в которую будут загружены образы Deckhouse.

1. Скачайте образы Deckhouse в выделенную директорию, используя команду `dhctl mirror`.

   `dhctl mirror` скачивает только последнюю доступную патч-версию минорного релиза Deckhouse. Например, для Deckhouse 1.52 будет скачана только одна версия `1.52.10`, т. к. этого достаточно для обновления Deckhouse с версии 1.51.

   Следующая команда скачает образы Deckhouse тех версий, которые находятся на каналах обновлений (о текущем статусе версий на каналах обновлений можно узнать на [flow.deckhouse.io](https://flow.deckhouse.io)):

   ```shell
   DHCTL_CLI_MIRROR_LICENSE="<DECKHOUSE_LICENSE_KEY>" dhctl mirror --source="registry.deckhouse.ru/deckhouse/ee" --images-bundle-path /tmp/d8-images/d8.tar
   ```

   > Если загрузка образов будет прервана, повторный вызов команды проверит загруженные образы и продолжит загрузку с момента ее остановки. Продолжение загрузки возможно только если с момента остановки прошло не более суток.
   > Используйте параметр `--no-pull-resume`, чтобы принудительно начать загрузку сначала.

   Чтобы скачать все версии Deckhouse начиная с конкретной версии, укажите ее в параметре `--min-version` в формате `X.Y`.

   Например, для загрузки всех версий Deckhouse, начиная с версии 1.45, используйте команду:

   ```shell
   DHCTL_CLI_MIRROR_LICENSE="<DECKHOUSE_LICENSE_KEY>" dhctl mirror --source="registry.deckhouse.ru/deckhouse/ee" --images-bundle-path /tmp/d8-images/d8.tar --min-version=1.45
   ```

   > Обратите внимание, параметр `--min-version` будет проигнорирован если вы укажете версию выше находящейся в канале обновлений rock-solid.

   Чтобы загрузить образы Deckhouse из определенного репозитория registry, вы можете указать этот репозиторий с помощью флага `--source`.
   Существуют также дополнительные флаги `--source-login` и `--source-password`, используемые для аутентификации в предоставленном registry.
   Если они не указаны, `dhctl mirror` будет обращаться к registry анонимно.

   Например, вот как можно загрузить образы из стороннего registry:

   ```shell
   DHCTL_CLI_MIRROR_SOURCE_LOGIN="user" DHCTL_CLI_MIRROR_SOURCE_PASSWORD="password" dhctl mirror --source="corp.company.ru/sys/deckhouse" --images-bundle-path /tmp/d8-images/d8.tar
   ```

   > Параметр `--license` действует как сокращение для параметров `--source-login` и `--source-password` и предназначен для использования с официальным registry Deckhouse.
   > Если вы укажете и параметр `--license`, и пару логин + пароль одновременно, будет использована последняя.

   `dhctl mirror` поддерживает расчет контрольных сумм итогового набора образов Deckhouse в формате ГОСТ Р 34.11-2012 (Стрибог) (параметр `--gost-digest`).
   Контрольная сумма будет выведена в лог и записана в файл с расширением `.tar.gostsum` рядом с tar-архивом, содержащим образы Deckhouse.

1. Опционально: Скопируйте утилиту `dhctl` из контейнера в директорию со скачанными образами Deckhouse.

   ```shell
   cp /usr/bin/dhctl /tmp/d8-images/dhctl
   ```

1. Передайте директорию с загруженными образами Deckhouse на хост с доступом к изолированному registry.
   Для продолжения установки используйте скопированную ранее утилиту `dhctl` или запустите установщик Deckhouse аналогично пунктам 1 и 2 на хосте с доступом к изолированному registry. Не забудьте смонтировать директорию с загруженными образами Deckhouse в контейнер установщика.

1. Загрузите образы Deckhouse с помощью команды `dhctl mirror` в изолированный registry.

   Пример команды для загрузки образов из файла `/tmp/d8-images/d8.tar`:

   ```shell
   DHCTL_CLI_MIRROR_USER="<USERNAME>" DHCTL_CLI_MIRROR_PASS="<PASSWORD>" dhctl mirror --images-bundle-path /tmp/d8-images/d8.tar --registry="your.private.registry.com:5000/deckhouse/ee"
   ```

   > Обратите внимание, образы будут выгружены в registry по пути, указанному в параметре `--registry` (в примере - /deckhouse/ee).
   > Перед запуском команды убедитесь, что этот путь существует и у используемой учетной записи есть права на запись.

   Если ваш registry не требует авторизации, флаги `--registry-login`/`--registry-password` указывать не нужно.

1. После загрузки образов в изолированный registry можно переходить к установке Deckhouse (доступно только в Enterprise Edition). Воспользуйтесь [руководством по быстрому старту](/gs/bm-private/step2.html).

   При запуске установщика используйте его образ из registry, в который ранее были загружены образы Deckhouse, а не из публичного registry. Например, используйте адрес вида `your.private.registry.com:5000/deckhouse/ee/install:stable` вместо `registry.deckhouse.ru/deckhouse/ee/install:stable`.

   В ресурсе `InitConfiguration` при установке также используйте адрес вашего registry и данные авторизации (параметры [imagesRepo](/documentation/v1/installing/configuration.html#initconfiguration-deckhouse-imagesrepo), [registryDockerCfg](/documentation/v1/installing/configuration.html#initconfiguration-deckhouse-registrydockercfg) или [шаг 3](/gs/bm-private/step3.html) руководства по быстрому старту).

### Ручная загрузка образов подключаемых модулей Deckhouse Kubernetes Platform в изолированный приватный registry

Ниже описаны шаги, необходимые для ручной загрузки образов модулей, подключаемых из источника модулей (ресурс [ModuleSource](cr.html#modulesource)):

1. Запустите установщик Deckhouse версии 1.56.0 или выше.

   ```shell
   docker run -ti --pull=always -v $(HOME)/d8-modules:/tmp/d8-modules -v $(HOME)/module_source.yml:/tmp/module_source.yml registry.deckhouse.ru/deckhouse/ce/install:v1.58.4 bash
   ```

   Обратите внимание, что в контейнер установщика монтируется директория с файловой системы хоста, в которую будут загружены образы модулей и YAML-манифест [ModuleSource](cr.html#modulesource), описывающий источник модулей.

1. Скачайте образы модулей из их источника, описанного в виде ресурса `ModuleSource` в выделенную директорию, используя команду `dhctl mirror-modules`.

   `dhctl mirror-modules` скачивает только версии модулей, доступные в каналах обновлений модуля на момент копирования.

   Следующая команда скачает образы модулей из источника, описанного в ресурсе `ModuleSource`, находящемся в файле `$HOME/module_source.yml`:

   ```shell
   dhctl mirror-modules -d /tmp/d8-modules -m /tmp/module_source.yml
   ```

1. Опционально: Скопируйте утилиту `dhctl` из контейнера в директорию со скачанными образами Deckhouse.

   ```shell
   cp /usr/bin/dhctl /tmp/d8-modules/dhctl
   ```

1. Передайте директорию с загруженными образами модулей на хост с доступом к изолированному registry.
   Для продолжения установки используйте скопированную ранее утилиту `dhctl` или запустите установщик Deckhouse аналогично пунктам 1 и 2 на хосте с доступом к изолированному registry. Не забудьте смонтировать директорию с загруженными образами модулей в контейнер установщика.

1. Загрузите образы модулей в изолированный registry с помощью команды `dhctl mirror-modules`.

   Пример команды для загрузки образов из директории `/tmp/d8-modules`:

   ```shell
   DHCTL_CLI_MIRROR_USER="<USERNAME>" DHCTL_CLI_MIRROR_PASS="<PASSWORD>" dhctl mirror-modules -d /tmp/d8-modules --registry="your.private.registry.com:5000/deckhouse-modules"
   ```

   > Обратите внимание, образы будут выгружены в registry по пути, указанному в параметре `--registry` (в примере - /deckhouse-modules).
   > Перед запуском команды убедитесь, что этот путь существует и у используемой учетной записи есть права на запись.

   Если ваш registry не требует авторизации, флаги `--registry-login`/`--registry-password` указывать не нужно.

1. После загрузки образов в изолированный registry, отредактируйте YAML-манифест `ModuleSource`:

   * Измените поле `.spec.registry.repo` на адрес, который вы указали в параметре `--registry` при выгрузке образов;
   * Измените поле `.spec.registry.dockerCfg` на Base64-строку с данными для авторизации в вашем registry в формате `dockercfg`. Обратитесь к документации вашего registry для получения информации о том, как сгенерировать этот токен.

1. Примените в кластере полученный на прошлом шаге манифест `ModuleSource`:

   ```shell
   kubectl apply -f $HOME/module_source.yml
   ```

   После применения манифеста модули готовы к использованию. Обратитесь к документации разработчика модулей для получения дальнейших инструкций по их настройке и использованию.

### Закрытое окружение, работа через proxy и сторонние registry

#### Уставновка Deckhouse Kubernetes Platform из стороннего registry

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

#### Переключение работающиго кластера Deckhouse на использование стороннего registry

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

* Дождитесь перехода пода Deckhouse в статус `Ready`. Если под будет находиться в статусе `ImagePullBackoff`, перезапустите его.
* Дождитесь применения bashible новых настроек на master-узле. В журнале bashible на master-узле (`journalctl -u bashible`) должно появится сообщение `Configuration is in sync, nothing to do`.
* Если необходимо отключить автоматическое обновление Deckhouse через сторонний registry, удалите параметр `releaseChannel` из конфигурации модуля `deckhouse`.
* Проверьте, не осталось ли в кластере подов с оригинальным адресом registry:

  ```shell
  kubectl get pods -A -o json | jq '.items[] | select(.spec.containers[] | select((.image | contains("deckhouse.io"))))
    | .metadata.namespace + "\t" + .metadata.name' -r
  ```

#### Создание кластера и запуск Deckhouse Kubernetes Platform без использования каналов обновлений

Данный способ следует использовать только в случае, если в изолированном приватном registry нет образов, содержащих информацию о каналах обновлений.

* Если вы хотите установить Deckhouse с отключенным автоматическим обновлением:
  * Используйте тег образа установщика соответствующей версии. Например, если вы хотите установить релиз `v1.44.3`, используйте образ `your.private.registry.com/deckhouse/install:v1.44.3`.
  * Укажите соответствующий номер версии в параметре [deckhouse.devBranch](installing/configuration.html#initconfiguration-deckhouse-devbranch) в ресурсе [InitConfiguration](installing/configuration.html#initconfiguration).
  * **Не указывайте** параметр [deckhouse.releaseChannel](installing/configuration.html#initconfiguration-deckhouse-releasechannel) в ресурсе [InitConfiguration](installing/configuration.html#initconfiguration).
* Если вы хотите отключить автоматические обновления у уже установленного Deckhouse (включая обновления patch-релизов), удалите параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel) из конфигурации модуля `deckhouse`.

#### Использование proxy-сервера

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

## Предоставление LTS обновлений на твёрдых носителях

Долгосрочная поддержка (LTS) — это период времени, в течение которого поставщик программного обеспечения предоставляет регулярные обновления безопасности и функциональности для операционной системы или приложения. Предоставление LTS обновлений на твёрдых носителях обычно означает физическую доставку обновлений на CD, DVD или USB-накопителе.

Преимущества получения LTS обновлений на твёрдых носителях включают следующее:

* Безопасность: Получение обновлений на физических носителях позволяет убедиться, что они не были изменены или повреждены при передаче через интернет.
* Доступность: В некоторых регионах или организациях может отсутствовать стабильное интернет-соединение, поэтому получение обновлений на твёрдых носителях является единственным способом получить последние версии ПО.
* Контроль: Физическое владение обновлением даёт возможность контролировать процесс его установки и тестирования перед внедрением в производственную среду.
* Поддержка: Некоторые поставщики программного обеспечения предлагают дополнительную поддержку клиентам, которые получают LTS обновления на твёрдых носителях.

При невозможности выдачи ограниченного доступа в закрытый контур для настройки проксирующего registry.						

## Отправка уведломления об обновлении

Состав уведомлений может варьироваться в зависимости от конкретной системы и приложения. Обычно они состоят из следующих элементов:
* Заголовок уведомления - содержит информацию о типе уведомления и его источнике.
* Текст уведомления - содержит описание обновления или изменения, которое было сделано.
* Ссылка на обновление - если доступно, ссылка на страницу или приложение, где можно получить более подробную информацию об обновлении.
* Действия пользователя - какие действия пользователь может предпринять в ответ на уведомление, например, обновить страницу или установить обновление.

### Оповещение об обновлении Deckhouse Kubernetes Platform

В режиме обновлений `Auto` можно [настроить](configuration.html#parameters-update-notification) вызов webhook'а для получения оповещения о предстоящем обновлении минорной версии Deckhouse.

Пример настройки оповещения:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    update:
      releaseChannel: Stable
      mode: Auto
      notification:
        webhook: https://release-webhook.mydomain.com
```

После появления новой минорной версии Deckhouse на используемом канале обновлений, но до момента применения ее в кластере на адрес webhook'а будет выполнен [POST-запрос](configuration.html#parameters-update-notification-webhook).

Чтобы всегда иметь достаточно времени для реакции на оповещение об обновлении Deckhouse, достаточно настроить параметр [minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime). В этом случае обновление случится по прошествии указанного времени с учетом окон обновлений.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    update:
      releaseChannel: Stable
      mode: Auto
      notification:
        webhook: https://release-webhook.mydomain.com
        minimalNotificationTime: 8h
```

{% alert %}
Если не указать адрес в параметре [update.notification.webhook](configuration.html#parameters-update-notification-webhook), но указать время в параметре [update.notification.minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime), применение новой версии все равно будет отложено как минимум на указанное в параметре `minimalNotificationTime` время. В этом случае оповещением о появлении новой версии можно считать появление в кластере ресурса [DeckhouseRelease](cr.html#deckhouserelease), имя которого соответствует новой версии.
{% endalert %}

## Режимы обновлений

Существуют три возможных режима обновления:
* **Автоматический + окна обновлений не заданы.** Кластер обновится сразу после появления новой версии на соответствующем [канале обновлений](https://deckhouse.ru/documentation/deckhouse-release-channels.html).
* **Автоматический + заданы окна обновлений.** Кластер обновится в ближайшее доступное окно после появления новой версии на канале обновлений.
* **Ручной режим.** Для применения обновления требуются [ручные действия](modules/002-deckhouse/usage.html#ручное-подтверждение-обновлений).

### Автоматический режим обновления

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

**Отключение автоматического обновления**

Чтобы полностью отключить механизм обновления Deckhouse, удалите в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse` параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel).

В этом случае Deckhouse не проверяет обновления и даже обновление на patch-релизы не выполняется.

{% alert level="danger" %}
Крайне не рекомендуется отключать автоматическое обновление! Это заблокирует обновления на patch-релизы, которые могут содержать исправления критических уязвимостей и ошибок.
{% endalert %}

### Настройка режима обновления

Если в автоматическом режиме окна обновлений не заданы, Deckhouse обновится сразу, как только новый релиз станет доступен.

Patch-версии (например, обновления с `1.26.1` до `1.26.2`) устанавливаются без подтверждения и без учета окон обновлений.

{% alert %}
Вы также можете настраивать окна disruption-обновлений узлов в custom resource'ах [NodeGroup](../040-node-manager/cr.html#nodegroup) (параметр `disruptions.automatic.windows`).
{% endalert %}

### Ручной режим обновления

#### Ручное подтверждение обновлений

При необходимости возможно включить ручное подтверждение обновлений. Сделать это можно следующим образом:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      mode: Manual
```

В этом режиме необходимо подтверждать каждое минорное обновление Deckhouse (без учета patch-версий).

Пример подтверждения обновления на версию `v1.43.2`:

```shell
kubectl patch DeckhouseRelease v1.43.2 --type=merge -p='{"approved": true}'
```

#### Ручное подтверждение потенциально опасных (disruptive) обновлений

При необходимости возможно включить ручное подтверждение потенциально опасных (disruptive) обновлений (которые меняют значения по умолчанию или поведение некоторых модулей). Сделать это можно следующим образом:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      disruptionApprovalMode: Manual
```

В этом режиме необходимо подтверждать каждое минорное потенциально опасное (disruptive) обновление Deckhouse (без учета patch-версий) с помощью аннотации `release.deckhouse.io/disruption-approved=true` на соответствующем ресурсе [DeckhouseRelease](cr.html#deckhouserelease).

Пример подтверждения минорного потенциально опасного обновления Deckhouse `v1.36.4`:

```shell
kubectl annotate DeckhouseRelease v1.36.4 release.deckhouse.io/disruption-approved=true
```

## Поддержка последних версий Kubernetes

Как только на установленном в кластере канале обновления появляется новая версия Deckhouse:
- Загорается алерт `DeckhouseReleaseIsWaitingManualApproval`, если кластер использует ручной режим обновлений (параметр [update.mode](modules/002-deckhouse/configuration.html#parameters-update-mode) установлен в `Manual`).
- Появляется новый custom resource [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease). Используйте команду `kubectl get deckhousereleases`, чтобы посмотреть список релизов. Если `DeckhouseRelease` новой версии находится в состоянии `Pending`, указанная версия еще не установлена. Возможные причины, при которых `DeckhouseRelease` может находиться в `Pending`:
  - Установлен ручной режим обновлений (параметр [update.mode](modules/002-deckhouse/configuration.html#parameters-update-mode) установлен в `Manual`).
  - Установлен автоматический режим обновлений и настроены [окна обновлений](modules/002-deckhouse/usage.html#конфигурация-окон-обновлений), интервал которых еще не наступил.
  - Установлен автоматический режим обновлений, окна обновлений не настроены, но применение версии отложено на случайный период времени из-за механизма снижения нагрузки на репозиторий образов контейнеров. В поле `status.message` ресурса `DeckhouseRelease` будет соответствующее сообщение.
  - Установлен параметр [update.notification.minimalNotificationTime](modules/002-deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) и указанное в нем время еще не прошло.

### Информация о предстоящем обновлении

Получать заранее информацию об обновлении минорных версий Deckhouse на канале обновлений можно следующими способами:
- Настроить ручной [режим обновлений](modules/002-deckhouse/configuration.html#parameters-update-mode). В этом случае при появлении новой версии на канале обновлений загорится алерт `DeckhouseReleaseIsWaitingManualApproval` и в кластере появится новый custom resource [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease).
- Настроить автоматический [режим обновлений](modules/002-deckhouse/configuration.html#parameters-update-mode) и указать минимальное время в параметре [minimalNotificationTime](modules/002-deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime), на которое будет отложено обновление. В этом случае при появлении новой версии на канале обновлений в кластере появится новый custom resource [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease). А если указать URL в параметре [update.notification.webhook](modules/002-deckhouse/configuration.html#parameters-update-notification-webhook), дополнительно произойдет вызов webhook'а.

### Поддержка более 2-ух последних версий Kubernetes (CE)

В базовых редакциях поддерживается только две последних версии Kubernetes, в более продвинутых поддерживается 4 последние вышедшие версии Kubernetes.						

### Поддержка LTS (EE)

Информацию о всех версиях Deckhouse можно найти в [списке релизов](https://github.com/deckhouse/deckhouse/releases) Deckhouse.

Сводную информацию о важных изменениях, об обновлении версий компонентов, а также о том, какие компоненты в кластере буду перезапущены в процессе обновления, можно найти в описании нулевой patch-версии релиза. Например, [v1.46.0](https://github.com/deckhouse/deckhouse/releases/tag/v1.46.0) для релиза v1.46 Deckhouse.

Подробный список изменений можно найти в Changelog, ссылка на который есть в каждом [релизе](https://github.com/deckhouse/deckhouse/releases).

### Как понять, что в кластере идет обновление? - перефразировать

Во время обновления:
- горит алерт `DeckhouseUpdating`;
- под `deckhouse` не в статусе `Ready`. Если под долго не переходит в статус `Ready`, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.

### Как понять, что обновление прошло успешно?  перефразировать

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

## Нерассортированны вопросы

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