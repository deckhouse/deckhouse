---
title: FAQ
permalink: ru/deckhouse-faq.html
lang: ru
---

## Как узнать все параметры Deckhouse?

Deckhouse настраивается с помощью глобальных настроек, настроек модулей и различных custom resource’ов. Подробнее — [в документации](./).

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
Документация доступна, если в кластере включен модуль [documentation](modules/documentation/). Он включен по умолчанию, кроме [варианта поставки](modules/deckhouse/configuration.html#parameters-bundle) `Minimal`.
{% endalert %}

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
  * Параметр `Maximum metadata age` для репозитория должен быть установлен в `0`.
* Должен быть настроен контроль доступа:
  * Создана роль **Nexus** (*Administration* -> *Security* -> *Roles*) со следующими полномочиями:
    * `nx-repository-view-docker-<репозиторий>-browse`
    * `nx-repository-view-docker-<репозиторий>-read`
  * Создан пользователь (*Administration* -> *Security* -> *Users*) с ролью, созданной выше.

**Настройка**:

* Создайте **проксирующий** репозиторий Docker (*Administration* -> *Repository* -> *Repositories*), указывающий на [Deckhouse registry](https://registry.deckhouse.ru/):
  ![Создание проксирующего репозитория Docker](images/registry/nexus/nexus-repository.png)

* Заполните поля страницы создания репозитория следующим образом:
  * `Name` должно содержать имя создаваемого репозитория, например `d8-proxy`.
  * `Repository Connectors / HTTP` или `Repository Connectors / HTTPS` должно содержать выделенный порт для создаваемого репозитория, например `8123` или иной.
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

В результате образы Deckhouse будут доступны, например, по следующему адресу: `https://<NEXUS_HOST>:<REPOSITORY_PORT>/deckhouse/ee:<d8s-version>`.

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

{% alert level="info" %}
О текущем статусе версий на каналах обновлений можно узнать на [releases.deckhouse.ru](https://releases.deckhouse.ru).
{% endalert %}

1. [Скачайте и установите утилиту Deckhouse CLI](deckhouse-cli/).

1. Скачайте образы Deckhouse в выделенную директорию, используя команду `d8 mirror pull`.

   По умолчанию `d8 mirror pull` скачивает только актуальные версии Deckhouse и официально поставляемых модулей.
   Например, для Deckhouse 1.59 будет скачана только версия `1.59.12`, т. к. этого достаточно для обновления Deckhouse с 1.58 до 1.59.

   Выполните следующую команду (укажите код редакции и лицензионный ключ), чтобы скачать образы актуальных версий:

   ```shell
   d8 mirror pull \
     --source='registry.deckhouse.ru/deckhouse/<EDITION>' \
     --license='<LICENSE_KEY>' $(pwd)/d8.tar
   ```

   где:
   - `<EDITION>` — код редакции Deckhouse Kubernetes Platform (например, `ee`, `se`, `cse`);
   - `<LICENSE_KEY>` — лицензионный ключ Deckhouse Kubernetes Platform.

   > Если загрузка образов будет прервана, повторный вызов команды продолжит загрузку, если с момента ее остановки прошло не более суток.

   Вы также можете использовать следующие параметры команды:
   - `--no-pull-resume` — чтобы принудительно начать загрузку сначала;
   - `--no-modules` — для пропуска загрузки модулей;
   - `--min-version=X.Y` — чтобы скачать все версии Deckhouse, начиная с указанной минорной версии. Параметр будет проигнорирован, если указана версия выше чем версия находящаяся на канале обновлений Rock Solid. Параметр не может быть использован одновременно с параметром `--release`;
   - `--release=X.Y.Z` — чтобы скачать только конкретную версию Deckhouse (без учета каналов обновлений). Параметр не может быть использован одновременно с параметром `--min-version`;
   - `--gost-digest` — для расчета контрольной суммы итогового набора образов Deckhouse в формате ГОСТ Р 34.11-2012 (Стрибог). Контрольная сумма будет отображена и записана в файл с расширением `.tar.gostsum` в папке с tar-архивом, содержащим образы Deckhouse;
   - `--source` — чтобы указать адрес источника хранилища образов Deckhouse;
      - Для аутентификации в официальном хранилище образов Deckhouse нужно использовать лицензионный ключ и параметр `--license`;
      - Для аутентификации в стороннем хранилище образов нужно использовать параметры `--source-login` и `--source-password`;
   - `--images-bundle-chunk-size=N` — для указания максимального размера файла (в ГБ), на которые нужно разбить архив образов. В результате работы вместо одного файла архива образов будет создан набор `.chunk`-файлов (например, `d8.tar.NNNN.chunk`). Чтобы загрузить образы из такого набора файлов, укажите в команде `d8 mirror push` имя файла без суффикса `.NNNN.chunk` (например, `d8.tar` для файлов `d8.tar.NNNN.chunk`).

   Дополнительные параметры конфигурации для семейства команд `d8 mirror` доступны в виде переменных окружения:
   - `HTTP_PROXY`/`HTTPS_PROXY` — URL прокси-сервера для запросов к HTTP(S) хостам, которые не указаны в списке хостов в переменной `$NO_PROXY`;
   - `NO_PROXY` — список хостов, разделенных запятыми, которые следует исключить из проксирования. Каждое значение может быть представлено в виде IP-адреса (`1.2.3.4`), CIDR (`1.2.3.4/8`), домена или символа (`*`). IP-адреса и домены также могут включать номер порта (`1.2.3.4:80`). Доменное имя соответствует как самому себе, так и всем поддоменам. Доменное имя начинающееся с `.`, соответствует только поддоменам. Например, `foo.com` соответствует `foo.com` и `bar.foo.com`; `.y.com` соответствует `x.y.com`, но не соответствует `y.com`. Символ `*` отключает проксирование;
   - `SSL_CERT_FILE` — указывает путь до сертификата SSL. Если переменная установлена, системные сертификаты не используются;
   - `SSL_CERT_DIR` — список каталогов, разделенный двоеточиями. Определяет, в каких каталогах искать файлы сертификатов SSL. Если переменная установлена, системные сертификаты не используются. [Подробнее...](https://www.openssl.org/docs/man1.0.2/man1/c_rehash.html);
   - `TMPDIR (*nix)`/`TMP (Windows)` — путь к директории для временных файлов, который будет использоваться во время операций загрузки и выгрузки образов. Вся обработка выполняется в этом каталоге. Он должен иметь достаточное количество свободного дискового пространства, чтобы вместить весь загружаемый пакет образов;
   - `MIRROR_BYPASS_ACCESS_CHECKS` — установите для этого параметра значение `1`, чтобы отключить проверку корректности переданных учетных данных для registry;

   Пример команды для загрузки всех версий Deckhouse EE начиная с версии 1.59 (укажите лицензионный ключ):

   ```shell
   d8 mirror pull \
     --source='registry.deckhouse.ru/deckhouse/ee' \
     --license='<LICENSE_KEY>' --min-version=1.59 $(pwd)/d8.tar
   ```

   Пример команды для загрузки образов Deckhouse из стороннего хранилища образов:

   ```shell
   d8 mirror pull \
     --source='corp.company.com:5000/sys/deckhouse' \
     --source-login='<USER>' --source-password='<PASSWORD>' $(pwd)/d8.tar
   ```

1. На хост с доступом к хранилищу, куда нужно загрузить образы Deckhouse, скопируйте загруженный пакет образов Deckhouse и установите [Deckhouse CLI](deckhouse-cli/).

1. Загрузите образы Deckhouse в хранилище с помощью команды `d8 mirror push`.

   Пример команды для загрузки образов из файла `/tmp/d8-images/d8.tar` (укажите данные для авторизации при необходимости):

   ```shell
   d8 mirror push /tmp/d8-images/d8.tar 'corp.company.com:5000/sys/deckhouse' \
     --registry-login='<USER>' --registry-password='<PASSWORD>'
   ```

   > Перед загрузкой образов убедитесь, что путь для загрузки в хранилище образов существует (в примере — `/sys/deckhouse`) и у используемой учетной записи есть права на запись.
   > Если вы используете Harbor, вы не сможете выгрузить образы в корень проекта, используйте выделенный репозиторий в проекте для размещения образов Deckhouse.

1. После загрузки образов в хранилище можно переходить к установке Deckhouse. Воспользуйтесь [руководством по быстрому старту](/products/kubernetes-platform/gs/bm-private/step2.html).

   При запуске установщика используйте не официальное публичное хранилище образов Deckhouse, а хранилище в которое ранее были загружены образы Deckhouse. Для примера выше адрес запуска установщика будет иметь вид `corp.company.com:5000/sys/deckhouse/install:stable`, вместо `registry.deckhouse.ru/deckhouse/ee/install:stable`.

   В ресурсе [InitConfiguration](installing/configuration.html#initconfiguration) при установке также используйте адрес вашего хранилища и данные авторизации (параметры [imagesRepo](installing/configuration.html#initconfiguration-deckhouse-imagesrepo), [registryDockerCfg](installing/configuration.html#initconfiguration-deckhouse-registrydockercfg) или [шаг 3]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/products/kubernetes-platform/gs/bm-private/step3.html) руководства по быстрому старту).

   После завершения установки примените сгенерированные во время загрузки манифесты [DeckhouseReleases](cr.html#deckhouserelease) к вашему кластеру, используя [Deckhouse CLI](deckhouse-cli/):

   ```shell
   d8 k apply -f ./deckhousereleases.yaml
   ```

### Ручная загрузка образов подключаемых модулей Deckhouse в изолированный приватный registry

Для ручной загрузки образов модулей, подключаемых из источника модулей (ресурс [ModuleSource](cr.html#modulesource)), выполните следующие шаги:

1. [Скачайте и установите утилиту Deckhouse CLI](deckhouse-cli/).

1. Создайте строку аутентификации для официального хранилища образов `registry.deckhouse.ru`, выполнив следующую команду (укажите лицензионный ключ):

   ```shell
   LICENSE_KEY='<LICENSE_KEY>'
   base64 -w0 <<EOF
     {
       "auths": {
         "registry.deckhouse.ru": {
           "auth": "$(echo -n license-token:${LICENSE_KEY} | base64 -w0)"
         }
       }
     }
   EOF
   ```

1. Скачайте образы модулей из их источника, описанного в виде ресурса `ModuleSource`, в выделенную директорию, используя команду `d8 mirror modules pull`.

   Если не передан параметр `--filter`, то `d8 mirror modules pull` скачивает только версии модулей, доступные в каналах обновлений модуля на момент копирования.

   - Создайте файл с описанием ресурса `ModuleSource` (например, `$HOME/module_source.yml`).

     Пример ресурса ModuleSource:

     ```yaml
     apiVersion: deckhouse.io/v1alpha1
     kind: ModuleSource
     metadata:
       name: deckhouse
     spec:
       registry:
         # Укажите строку аутентификации для официального хранилища образов, полученную в п. 2
         dockerCfg: <BASE64_REGISTRY_CREDENTIALS>
         repo: registry.deckhouse.ru/deckhouse/ee/modules
         scheme: HTTPS
       # Выберите подходящий канал обновлений: Alpha, Beta, EarlyAccess, Stable, RockSolid
       releaseChannel: "Stable"
     ```

   - Скачайте образы модулей из источника, описанного в ресурсе `ModuleSource`, в выделенную директорию, используя команду `d8 mirror modules pull`.

     Пример команды:

     ```shell
     d8 mirror modules pull -d ./d8-modules -m $HOME/module_source.yml
     ```

     Для загрузки только набора из определенных модулей конкретных версий используйте параметр `--filter`, передав набор необходимых модулей и их минимальных версий, разделенных символом `;`.

     Пример:

     ```shell
     d8 mirror modules pull -d /tmp/d8-modules -m $HOME/module_source.yml \
       --filter='deckhouse-admin@1.3.3; sds-drbd@0.0.1'
     ```

     Команда выше загрузит только модули `deckhouse-admin` и `sds-drbd`. Для `deckhouse-admin` будут загружены все доступные версии начиная с `1.3.3`, для `sds-drbd` — все доступные версии начиная с `0.0.1`.

1. На хост с доступом к хранилищу, куда нужно загрузить образы, скопируйте директорию с загруженными образами модулей Deckhouse и установите [Deckhouse CLI](deckhouse-cli/).

1. Загрузите образы модулей в хранилище с помощью команды `d8 mirror modules push`.

   Пример команды для загрузки образов из директории `/tmp/d8-modules`:

   ```shell
   d8 mirror modules push \
     -d /tmp/d8-modules --registry='corp.company.com:5000/sys/deckhouse/modules' \
     --registry-login='<USER>' --registry-password='<PASSWORD>'
   ```

   > Перед загрузкой образов убедитесь, что путь для загрузки в хранилище образов существует (в примере — `/sys/deckhouse/modules`) и у используемой учетной записи есть права на запись.

1. Отредактируйте YAML-манифест `ModuleSource`, подготовленный на шаге 3:

   * Измените поле `.spec.registry.repo` на адрес, который вы указали в параметре `--registry` при загрузке образов.
   * Измените поле `.spec.registry.dockerCfg` на Base64-строку с данными для авторизации в вашем хранилище образов в формате `dockercfg`. Обратитесь к документации вашего registry для получения информации о том, как сгенерировать этот токен.

   Пример `ModuleSource`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleSource
   metadata:
     name: deckhouse
   spec:
     registry:
       # Укажите строку аутентификации для вашего хранилища образов.
       dockerCfg: <BASE64_REGISTRY_CREDENTIALS>
       repo: 'corp.company.com:5000/sys/deckhouse/modules'
       scheme: HTTPS
     # Выберите подходящий канал обновлений: Alpha, Beta, EarlyAccess, Stable, RockSolid
     releaseChannel: "Stable"
   ```

1. Примените в кластере исправленный манифест `ModuleSource`:

   ```shell
   d8 k apply -f $HOME/module_source.yml
   ```

   После применения манифеста модули готовы к использованию. Обратитесь к [документации по разработке модуля](./module-development/) для получения дополнительной информации.

### Как переключить работающий кластер Deckhouse на использование стороннего registry?

{% alert level="warning" %}
Использование registry отличных от `registry.deckhouse.io` и `registry.deckhouse.ru` доступно только в Enterprise Edition.
{% endalert %}

Для переключения кластера Deckhouse на использование стороннего registry выполните следующие действия:

* Выполните команду `deckhouse-controller helper change-registry` из пода Deckhouse с параметрами нового registry.
  * Пример запуска:

    ```shell
    kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee
    ```

  * Если registry использует самоподписанные сертификаты, положите корневой сертификат соответствующего сертификата registry в файл `/tmp/ca.crt` в поде Deckhouse и добавьте к вызову опцию `--ca-file /tmp/ca.crt` или вставьте содержимое CA в переменную, как в примере ниже:

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
    $ kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- bash -c "echo '$CA_CONTENT' > /tmp/ca.crt && deckhouse-controller helper change-registry --ca-file /tmp/ca.crt --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee"
    ```

  * Просмотреть список доступных ключей команды `deckhouse-controller helper change-registry` можно, выполнив следующую команду:

    ```shell
    kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --help
    ```

    Пример вывода:

    ```shell
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

* Дождитесь перехода пода Deckhouse в статус `Ready`. Если под будет находиться в статусе `ImagePullBackoff`, перезапустите его.
* Дождитесь применения bashible новых настроек на master-узле. В журнале bashible на master-узле (`journalctl -u bashible`) должно появится сообщение `Configuration is in sync, nothing to do`.
* Если необходимо отключить автоматическое обновление Deckhouse через сторонний registry, удалите параметр `releaseChannel` из конфигурации модуля `deckhouse`.
* Проверьте, не осталось ли в кластере подов с оригинальным адресом registry:

  ```shell
  kubectl get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
    | select(.image | startswith("registry.deckhouse"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
  ```

### Как создать кластер и запустить Deckhouse без использования каналов обновлений?

Данный способ следует использовать только в случае, если в изолированном приватном registry нет образов, содержащих информацию о каналах обновлений.

* Если вы хотите установить Deckhouse с отключенным автоматическим обновлением:
  * Используйте тег образа установщика соответствующей версии. Например, если вы хотите установить релиз `v1.60.5`, используйте образ `your.private.registry.com/deckhouse/install:v1.60.5`.
  * **Не указывайте** параметр [deckhouse.releaseChannel](modules/deckhouse/configuration.html#parameters-releasechannel).
* Если вы хотите отключить автоматические обновления у уже установленного Deckhouse, ознакомьтесь с документацией [по закреплению релиза](modules/deckhouse/#закрепление-релиза].

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

{% alert level="warning" %}
Для применения изменений конфигурации узлов необходимо выполнить команду  `dhctl converge`, запустив инсталлятор Deckhouse. Эта команда синхронизирует состояние узлов с указанным в конфигурации.
{% endalert %}

### Как изменить конфигурацию кластера?

Общие параметры кластера хранятся в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration).

Чтобы изменить общие параметры кластера, выполните команду:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit cluster-configuration
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

### Как получить доступ к контроллеру Deckhouse в multi-master-кластере?

В кластерах с несколькими master-узлами Deckhouse запускается в режиме высокой доступности (в нескольких экземплярах). Для доступа к активному контроллеру Deckhouse можно использовать следующую команду (на примере команды `deckhouse-controller queue list`):

```shell
kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
```

### Как обновить версию Kubernetes в кластере?

Чтобы обновить версию Kubernetes в кластере, измените параметр [kubernetesVersion](installing/configuration.html#clusterconfiguration-kubernetesversion) в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration), выполнив следующие шаги:
1. Выполните команду:

   ```shell
   kubectl -n d8-system exec -ti svc/deckhouse-leader \
     -c deckhouse -- deckhouse-controller edit cluster-configuration
   ```

1. Измените параметр `kubernetesVersion`.
1. Сохраните изменения. Узлы кластера начнут последовательно обновляться.
1. Дождитесь окончания обновления. Отслеживать ход обновления можно с помощью команды `kubectl get no`. Обновление можно считать завершенным, когда в выводе команды у каждого узла кластера в колонке `VERSION` появится обновленная версия.

### Как запускать Deckhouse на произвольном узле?

Для запуска Deckhouse на произвольном узле установите у модуля `deckhouse` соответствующий [параметр](modules/deckhouse/configuration.html) `nodeSelector` и не задавайте `tolerations`.  Необходимые значения `tolerations` в этом случае будут проставлены автоматически.

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
