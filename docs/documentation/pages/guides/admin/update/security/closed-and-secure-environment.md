---
title: Обновление в закрытом контуре
permalink: ru/update/security/closed-and-secure-environment/
lang: ru
---

Deckhouse Kubernetes Platform использует актуальные версии компонентов для обеспечения стабильности и безопасности системы. Обновления могут включать исправления уязвимостей, улучшение производительности и добавление новых функций.
Закрытый контур может требовать использования специфических версий компонентов или патчей, которые не доступны в стандартных репозиториях. Поэтому можно настроить Deckhouse Kubernetes Platform на работу со сторонним реестром, который содержит необходимые образы. Также обновления в закрытом контуре необходимы для обеспечения совместимости с другими компонентами в системе или для поддержки новых функций, так как Deckhouse Kubernetes Platform отвечает за то, чтобы кластер одинаково работал на любой поддерживаемой инфраструктуре из следующих:

* в [облаках](смотри информацию по соответствующему cloud провайдеру - добавить ссылку);
* на виртуальных машинах или bare metall (включая on-premises);
* в гибридной инфраструктуре.

### Ручная загрузка образов в изолированный приватный registry

{% alert level="warning" %}
Доступно только в Enterprise Edition.
{% endalert %}

1. При необходимости авторизуйтесь в container registry `registry.deckhouse.ru` или `registry.deckhouse.io` с помощью вашего лицензионного ключа.

   ```shell
   docker login -u license-token registry.deckhouse.ru
   ```

1. Запустите установщик Deckhouse версии 1.58.6 или выше.

   ```shell
   docker run -ti --pull=always -v $(pwd)/d8-images:/tmp/d8-images registry.deckhouse.ru/deckhouse/ee/install:v1.58.6 bash
   ```

   Обратите внимание, что в контейнер установщика монтируется директория с файловой системы хоста, в которую будут загружены образы Deckhouse Kubernetes Platform и сгенерированы манифесты [DeckhouseReleases](modules/002-deckhouse/cr.html#deckhouserelease).

1. Скачайте образы Deckhouse в выделенную директорию, используя команду `dhctl mirror`.

   `dhctl mirror` скачивает только последнюю доступную патч-версию минорного релиза Deckhouse Kubernetes Platform. Например, для Deckhouse 1.58 будет скачана только одна версия `1.58.10`, т. к. этого достаточно для обновления Deckhouse Kubernetes Platform с версии 1.57.

   Следующая команда скачает образы Deckhouse тех версий, которые находятся на каналах обновлений (о текущем статусе версий на каналах обновлений можно узнать на [flow.deckhouse.io](https://flow.deckhouse.io)):

   ```shell
   DHCTL_CLI_MIRROR_LICENSE="<DECKHOUSE_LICENSE_KEY>" dhctl mirror --source="registry.deckhouse.ru/deckhouse/ee" --images-bundle-path /tmp/d8-images/d8.tar
   ```

   > Если загрузка образов будет прервана, повторный вызов команды проверит загруженные образы и продолжит загрузку с момента ее остановки. Продолжение загрузки возможно только если с момента остановки прошло не более суток.
   > Используйте параметр `--no-pull-resume`, чтобы принудительно начать загрузку сначала.

   1. Чтобы скачать все версии Deckhouse начиная с конкретной версии, укажите ее в параметре `--min-version` в формате `X.Y`. Например, для загрузки всех версий Deckhouse, начиная с версии 1.45, используйте команду:

      ```shell
      DHCTL_CLI_MIRROR_LICENSE="<DECKHOUSE_LICENSE_KEY>" dhctl mirror --source="registry.deckhouse.ru/deckhouse/ee" --images-bundle-path /tmp/d8-images/d8.tar --min-version=1.45
      ```

   > Обратите внимание, параметр `--min-version` будет проигнорирован если вы укажете версию выше находящейся в канале обновлений rock-solid.

   Чтобы загрузить образы Deckhouse Kubernetes Platform из определенного репозитория registry, вы можете указать этот репозиторий с помощью флага `--source`.
   Существуют также дополнительные флаги `--source-login` и `--source-password`, используемые для аутентификации в предоставленном registry.
   Если они не указаны, `dhctl mirror` будет обращаться к registry анонимно.

   Например, вот как можно загрузить образы из стороннего registry:

   ```shell
   DHCTL_CLI_MIRROR_SOURCE_LOGIN="user" DHCTL_CLI_MIRROR_SOURCE_PASSWORD="password" dhctl mirror --source="corp.company.ru/sys/deckhouse" --images-bundle-path /tmp/d8-images/d8.tar
   ```

   > Параметр `--license` действует как сокращение для параметров `--source-login ($DHCTL_CLI_MIRROR_SOURCE_LOGIN)` и `--source-password ($DHCTL_CLI_MIRROR_SOURCE_PASSWORD)` и предназначен для использования с официальным registry Deckhouse.
   > Если вы укажете и параметр `--license`, и пару логин + пароль одновременно, будет использована последняя.

   `dhctl mirror` поддерживает расчет контрольных сумм итогового набора образов Deckhouse в формате ГОСТ Р 34.11-2012 (Стрибог) (параметр `--gost-digest`).
   Контрольная сумма будет выведена в лог и записана в файл с расширением `.tar.gostsum` рядом с tar-архивом, содержащим образы Deckhouse.

1. Опционально: Скопируйте утилиту `dhctl` из контейнера в директорию со скачанными образами Deckhouse Kubernetes Platform.

   ```shell
   cp /usr/bin/dhctl /tmp/d8-images/dhctl
   ```

1. Передайте директорию с загруженными образами Deckhouse Kubernetes Platform на хост с доступом к изолированному registry.
   Для продолжения установки используйте скопированную ранее утилиту `dhctl` или запустите установщик Deckhouse аналогично пунктам 1 и 2 на хосте с доступом к изолированному registry. Не забудьте смонтировать директорию с загруженными образами Deckhouse в контейнер установщика.

1. Загрузите образы Deckhouse Kubernetes Platform с помощью команды `dhctl mirror` в изолированный registry.

   Пример команды для загрузки образов из файла `/tmp/d8-images/d8.tar`:

   ```shell
   DHCTL_CLI_MIRROR_USER="<USERNAME>" DHCTL_CLI_MIRROR_PASS="<PASSWORD>" dhctl mirror --images-bundle-path /tmp/d8-images/d8.tar --registry="your.private.registry.com:5000/deckhouse/ee"
   ```

   > Обратите внимание, образы будут выгружены в registry по пути, указанному в параметре `--registry` (в примере - `/deckhouse/ee`).
   > Перед запуском команды убедитесь, что этот путь существует и у используемой учетной записи есть права на запись.

   Если registry не требует авторизации, флаги `--registry-login`/`--registry-password` или переменные `$DHCTL_CLI_MIRROR_USER`/`$DHCTL_CLI_MIRROR_PASS` указывать не нужно.

1. После загрузки образов в изолированный registry можно переходить к установке Deckhouse Kubernetes Platform. Воспользуйтесь [руководством по быстрому старту](/gs/bm-private/step2.html).

   При запуске установщика используйте его образ из registry, в который ранее были загружены образы Deckhouse Kubernetes Platform, а не из публичного registry. Например, используйте адрес вида `your.private.registry.com:5000/deckhouse/ee/install:stable` вместо `registry.deckhouse.ru/deckhouse/ee/install:stable`.

   В ресурсе `InitConfiguration` при установке также используйте адрес вашего registry и данные авторизации (параметры [imagesRepo](/documentation/v1/installing/configuration.html#initconfiguration-deckhouse-imagesrepo), [registryDockerCfg](/documentation/v1/installing/configuration.html#initconfiguration-deckhouse-registrydockercfg) или [шаг 3](/gs/bm-private/step3.html) руководства по быстрому старту).

   После завершения установки примените сгенерированные во время загрузки манифесты DeckhouseReleases к вашему кластеру используя `kubectl`:

   ```shell
   kubectl apply -f $(pwd)/d8-images/deckhousereleaases.yaml
   ```

### Ручная загрузка образов подключаемых модулей Deckhouse в изолированный приватный registry

Ниже описаны шаги, необходимые для ручной загрузки образов модулей, подключаемых из источника модулей (ресурса [*ModuleSource*](cr.html#modulesource)):

1. Запустите установщик Deckhouse версии 1.58.0 или выше.

   ```shell
   docker run -ti --pull=always -v $(HOME)/d8-modules:/tmp/d8-modules -v $(HOME)/module_source.yml:/tmp/module_source.yml registry.deckhouse.ru/deckhouse/ce/install:v1.58.4 bash
   ```

   > В контейнер установщика монтируется директория с файловой системы хоста, в которую будут загружены образы модулей и YAML-манифест [ModuleSource](cr.html#modulesource), описывающий источник модулей.

1. Скачайте образы модулей из их источника, описанного в виде ресурса `ModuleSource` в выделенную директорию, используя команду `dhctl mirror-modules`.

   `dhctl mirror-modules` скачивает только версии модулей, доступные в каналах обновлений модуля на момент копирования.

   Следующая команда скачает образы модулей из источника, описанного в ресурсе `ModuleSource`, находящемся в файле `$HOME/module_source.yml`:

   ```shell
   dhctl mirror-modules -d /tmp/d8-modules -m /tmp/module_source.yml
   ```

1. Опционально: Скопируйте утилиту `dhctl` из контейнера в директорию со скачанными образами Deckhouse Kubernetes Platform.

   ```shell
   cp /usr/bin/dhctl /tmp/d8-modules/dhctl
   ```

1. Передайте директорию с загруженными образами модулей на хост с доступом к изолированному registry.
   Для продолжения установки используйте скопированную ранее утилиту `dhctl` или запустите установщик Deckhouse Kubernetes Platform аналогично пунктам 1 и 2 на хосте с доступом к изолированному registry. Не забудьте смонтировать директорию с загруженными образами модулей в контейнер установщика.

1. Загрузите образы модулей в изолированный registry с помощью команды `dhctl mirror-modules`.

   Пример команды для загрузки образов из директории `/tmp/d8-modules`:

   ```shell
   DHCTL_CLI_MIRROR_USER="<USERNAME>" DHCTL_CLI_MIRROR_PASS="<PASSWORD>" dhctl mirror-modules -d /tmp/d8-modules --registry="your.private.registry.com:5000/deckhouse-modules"
   ```

   > Обратите внимание, образы будут выгружены в registry по пути, указанному в параметре `--registry` (в примере - `/deckhouse-modules`).
   > Перед запуском команды убедитесь, что этот путь существует и у используемой учетной записи есть права на запись.

   Если registry не требует авторизации, флаги `--registry-login`/`--registry-password` указывать не нужно.

1. После загрузки образов в изолированный registry, отредактируйте YAML-манифест `ModuleSource`:

   * Измените поле `.spec.registry.repo` на адрес, который вы указали в параметре `--registry` при выгрузке образов;
   * Измените поле `.spec.registry.dockerCfg` на Base64-строку с данными для авторизации в вашем registry в формате `dockercfg`. Обратитесь к документации вашего registry для получения информации о том, как сгенерировать этот токен.

1. Примените в кластере полученный на прошлом шаге манифест `ModuleSource`:

   ```shell
   kubectl apply -f $HOME/module_source.yml
   ```

   После применения манифеста модули готовы к использованию. Обратитесь к документации разработчика модулей для получения дальнейших инструкций по их настройке и использованию.



Нужен ли сценарий - Как установить Deckhouse из стороннего registry и проксирование?

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

Deckhouse Kubernetes Platform можно настроить на работу с проксирующим registry внутри закрытого контура, для этого выполните следующие шаги: 

1. Установите следующие параметры в ресурсе `InitConfiguration`:

   * `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/ee` — адрес образа Deckhouse EE в стороннем registry. Пример: `imagesRepo: registry.deckhouse.ru/deckhouse/ee`;
   * `registryDockerCfg: <BASE64>` — права доступа к стороннему registry, зашифрованные в Base64.

2. При разрешенном анонимном доступе к образам Deckhouse Kubernetes Platform в стороннем registry, удостоверьтесь, что `registryDockerCfg` выглядит следующим образом:

   ```json
   {"auths": { "<PROXY_REGISTRY>": {}}}
   ```

   > Приведенное значение должно быть закодировано в Base64.

3. Если для доступа к образам Deckhouse Kubernetes Platform в стороннем registry необходима аутентификация, удостоверьтесь, что `registryDockerCfg` выглядит следующим образом:

   ```json
   {"auths": { "<PROXY_REGISTRY>": {"username":"<PROXY_USERNAME>","password":"<PROXY_PASSWORD>","auth":"<AUTH_BASE64>"}}}
   ```

   где:

   * `<PROXY_USERNAME>` — имя пользователя для аутентификации на `<PROXY_REGISTRY>`;
   * `<PROXY_PASSWORD>` — пароль пользователя для аутентификации на `<PROXY_REGISTRY>`;
   * `<PROXY_REGISTRY>` — адрес стороннего registry в виде `<HOSTNAME>[:PORT]`;
   * `<AUTH_BASE64>` — строка вида `<PROXY_USERNAME>:<PROXY_PASSWORD>`, закодированная в Base64.

   > Итоговое значение для `registryDockerCfg` должно быть также закодировано в Base64.

3. Чтобы настроить нестандартные конфигурации сторонних registry в ресурсе `InitConfiguration`, используйте еще два параметра:

   * `registryCA` — корневой сертификат, которым можно проверить сертификат registry (если registry использует самоподписанные сертификаты);
   * `registryScheme` — протокол доступа к registry (`HTTP` или `HTTPS`). По умолчанию — `HTTPS`.
   
      <div markdown="0" style="height: 0;" id="особенности-настройки-сторонних-registry"></div>
      ```



