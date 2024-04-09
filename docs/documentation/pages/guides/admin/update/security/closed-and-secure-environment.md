
---
title: Обновление в закрытом контуре
permalink: ru/update/security/
lang: ru
---

Deckhouse Kubernetes Platform использует актуальные версии компонентов для обеспечения стабильности и безопасности системы. Обновления могут включать исправления уязвимостей, улучшение производительности и добавление новых функций.
Закрытый контур может требовать использования специфических версий компонентов или патчей, которые не доступны в стандартных репозиториях. Поэтому можно настроить Deckhouse Kubernetes Platform на работу со сторонним реестром, который содержит необходимые образы. Также обновления в закрытом контуре необходимы для обеспечения совместимости с другими компонентами в системе или для поддержки новых функций, так как Deckhouse Kubernetes Platform отвечает за то, чтобы кластер одинаково работал на любой поддерживаемой инфраструктуре из следующих:

* в [облаках](смотри информацию по соответствующему cloud провайдеру - добавить ссылку);
* на виртуальных машинах или bare metall (включая on-premises);
* в гибридной инфраструктуре.

## Доставка образов поставки в закрытое окружение

Для установки обновлений DKP в закрытом окружении необходимо наличие образов последних патч-версий для каждой минорной версии платформы.

Доставка образов платформы в закрытое окружение осуществляется в виде готовой поставки платформы на USB-носителе или с помощью утилиты `dhctl mirror` (требуется доступ в Интернет).

Поставка на USB-носителе включает в себя все необходимые данные для установки обновлений в закрытых окружениях. В состав поставки входят:

- архив с образами контейнеров платформы `d8.tar`, содержащий все необходимые промежуточные версии, начиная от заданной минимальной версии и заканчивая последней доступной;
- манифесты релизов DKP, соответствующие версиям образов поставки, в файле `deckhousereleases.yaml`;
- исполняемый файл `dhctl`.

При использовании `dhctl mirror`, указанные выше артефакты будут созданы в процессе работы утилиты.

1. Выполните аутентификацию на `registry.deckhouse.ru`:

   ```bash
   docker login -u license-token registry.deckhouse.ru
   ```

2. Запустите образ установщика версии 1.58.3, указав подходящий каталог рабочей станции для проброса в контейнер:
   
   ```bash
   docker run -ti --pull=always -v $(pwd)/d8-images:/tmp/d8-images registry.deckhouse.ru/deckhouse/ee/install:v1.58.3 bash
   ```

   Подробнее об использовании `dhctl mirror` для выгрузки образов читайте в [разделе Обновление в закрытом контуре](ссылка на раздел).

## Подготовка к установке обновлений в закрытый контур

1. Убедитесь, что все обновляемые кластеры не имеют заданного канала обновлений `ReleaseChannel`. Чтобы проверить, выполните команду ниже:

   ```bash
   kubectl get mc deckhouse -o yaml | grep releaseChannel
   ```

1. В случае, если канал обновлений указан, удалите его, отредактировав конфигурацию модуля Deckhouse:

   ```bash
   kubectl edit mc deckhouse -o yaml
   ```

1. После внесения изменений, дождитесь завершения обработки очереди Deckhouse Kubernetes Platform, проверьте, что измеени внесены, командой:

   ```bash
   kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller queue list
   ```

1. Переведите установку обновлений платформы в ручной режим. Для этого отредактируйте конфигурацию модуля Deckhouse Kubernetes Platform командой:

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

1. После внесения изменений, дождитесь завершения обработки очереди Deckhouse Kubernetes Platform, проверьте, что обработка очереди произошла, командой:

   ```bash
   kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller queue list
   ```

1. Загрузите все образы поставки DKP в реестр образов контейнеров, находящийся в закрытом окружении. Для этого перейдите в каталог с содержимым поставки и выполните команду:

   ```bash
   ./dhctl mirror -i ./d8.tar -r "REGISTRY.EXAMPLE.COM:5000/path/to/deckhouse/ee" -u "ПОЛЬЗОВАТЕЛЬ" -p "ПАРОЛЬ"
   ```

1. В случае использования самоподписанных сертификатов для реестра образов контейнеров используйте переменные окружения `SSL_CERT_FILE` и `SSL_CERT_DIR`, чтобы задать пути к СА сертификату и сертификатам реестра образов контейнеров, как представлено на примере:

   ```bash
   export SSL_CERT_FILE="/etc/docker/certs.d/REGISTRY.EXAMPLE.COM/registry.example.com.cert"
   export SSL_CERT_DIR="/etc/docker/certs.d/REGISTRY.EXAMPLE.COM"
   ```

Подробнее об использовании `dhctl mirror` для загрузки образов в закрытый реестр образов контейнеров читайте в [разделе Обновление в закрытом контуре](ссылка на раздел).

1. Установите канал обновлений, например, `Stable`. Для этого отредактируйте конфигурацию модуля Deckhouse Kubernetes Platform командой:

   ```bash
   kubectl edit mc deckhouse -o yaml
   ```

1. Добавьте `releaseChannel: Stable` в блок `settings`.

   Пример корректной конфигурации модуля Deckhouse Kubernetes Platform:

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

1. После внесения изменений, дождитесь завершения обработки очереди Deckhouse Kubernetes Platform, проверьте, что обработка очереди произошла, командой:

   ```bash
   kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller queue list
   ```

1. Загрузите манифесты `DeckhouseReleases` из файла `deckhousereleases.yaml` командой:

   ```bash
   kubectl apply -f deckhousereleases.yaml
   ```

1. Проверьте наличие релизов Deckhouse Kubernetes Platform командой:

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

1. В случае обнаружения в списке релиза с нестандартным названием без точек (из примера выше: `v1-57-5`) удалите его командой:

   ```bash
   kubectl delete deckhousereleases v1-57-5
   ```

## Установка обновлений

Так как установка обновлений осуществляется в основном в ручном режиме, необходимо вручную одобрять каждый устанавливаемый релиз.

В среднем установка каждого релиза занимает около 30 минут для кластера с 3 мастер-узлами и 2 воркер-узлами.

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

2. Найдите в списке релиз с сообщением `Waiting for manual approval` или зайдите в раздел **Alerts** сервиса Prometheus и найдите алерт `DeckhouseReleaselsWaitingManualApproval`. Разверните этот алерт, чтобы узнать ожидаемый релиз для одобрения.

3. Проверьте, что ваш кластер соответствует требованиям для выполнения обновлений. Для этого выполните команду ниже и ознакомьтесь с секцией `Requirements`:

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

- в разделе **Alerts** сервиса Prometheus погас алерт `DeckhouseUpdating`;
- в Grafana отображается желаемая версия Deckhouse Kubernetes Platform;
- в очереди Deckhouse Kubernetes Platform нет задач для обработки;
- релиз перешел из статуса `Pending` в статус `Deployed`.

Пример вывода для установленного релиза `v1.45.11`:

```text
$ kubectl get deckhousereleases
NAME       PHASE      TRANSITIONTIME   MESSAGE
v1.45.11   Deployed   55s              
v1.46.12   Pending    10s              Waiting for manual approval
v1.47.5    Pending    5m55s            
v1.48.9    Pending    5m...
```
### Загрузка образов в изолированный приватный registry

{% alert level="warning" %}
Доступно только в Enterprise Edition.
{% endalert %}

1. При необходимости авторизуйтесь в container registry `registry.deckhouse.ru` или `registry.deckhouse.io` с помощью вашего лицензионного ключа.

   ```shell
   docker login -u license-token registry.deckhouse.ru
   ```

1. Запустите установщик Deckhouse Kubernetes Platform версии 1.58.6 или выше.

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

1. После загрузки образов в изолированный registry переходите к установке Deckhouse Kubernetes Platform. Воспользуйтесь [руководством по быстрому старту](/gs/bm-private/step2.html).

   При запуске установщика используйте его образ из registry, в который ранее были загружены образы Deckhouse Kubernetes Platform, а не из публичного registry. Например, используйте адрес вида `your.private.registry.com:5000/deckhouse/ee/install:stable` вместо `registry.deckhouse.ru/deckhouse/ee/install:stable`.

   При установке в ресурсе `InitConfiguration` также используйте адрес вашего registry и данные авторизации с параметрами [imagesRepo](/documentation/v1/installing/configuration.html#initconfiguration-deckhouse-imagesrepo), [registryDockerCfg](/documentation/v1/installing/configuration.html#initconfiguration-deckhouse-registrydockercfg) или с [параметрами доступа к хранилищу образов контейнеров (или проксирующему registry)](/gs/bm-private/step3.html).

   После завершения установки примените сгенерированные во время загрузки манифесты DeckhouseReleases к вашему кластеру, используя `kubectl`:

   ```shell
   kubectl apply -f $(pwd)/d8-images/deckhousereleaases.yaml
   ```

### Загрузка образов подключаемых модулей Deckhouse в изолированный приватный registry

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

1. Опционально: скопируйте утилиту `dhctl` из контейнера в директорию со скачанными образами Deckhouse Kubernetes Platform.

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

   После применения манифеста модули готовы к использованию.

## Использование proxy-сервера в закрытом контуре

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


