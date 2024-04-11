---
title: Обновление в закрытом контуре
permalink: ru/update/security/
lang: ru
---

Обновления Deckhouse Kubernetes Platform могут включать исправления уязвимостей, улучшение производительности и добавление новых функций. Deckhouse Kubernetes Platform отвечает за то, чтобы кластер одинаково работал на любой поддерживаемой инфраструктуре.


<!--Всё начинается с настройки реджистри.

биррер-токен в нексусе.

В Нексусе - не включено то, что надо.
Эт нулевой шаг перед тем, чтобы забрать образы и прежде чем положить образы куда-то.


2 в-та: хост и прокси - там не надо образы загружать.

На машине, на которй поднимается реджистри, прокси корпоравтиный поднимается и она видит, что можно пропустить. Это завсит от прокси заказчика.


Что делать с реджистри.

!!!!Это самое начало. После тгого как засетапили реджистри - нужно разобраться как провести обновления.

1. Выбрать канал обновлений.
2. Выбор режим.
3. Формировать поставку обновлений. Задат минимальную версию. Если кластер на последней врсии и забрать то, что есть.

4. Закидываем образы - автоматич. режим - само.


5. Если ручной режим - каждую последнюю патч-версию - вручную обновленять.

Дистрапшен аппрув: это тоже нужно.

(поискать команду) - кубктл патч....

В ручном нужно следить, что это обновилось и прменялось. Светится деплой - какие команды нужно смотреть постоянно.


Что если идёт не так:

разные сценарии.


Сетапы: кластеры мульти - от 30 секунд до 1 минуты прерывания  и одноголовый - прерывание сервиса.

Мы обновились, у нас поменялись конфиги - у них они не отображены - кластер засетапил, манифесты положил - может не получиться, надо отправлять в доку (версия в 1.54 был случай. проблема с переменными).



Автоматич, по расписанию, какой канал выбран. Если канал не выбран - то обновление не будет. Ест старые кластеры,  в котрых пинг версий сделан в устаревшие 

Прежде, чем пушить в реджистри:
Если реджистри в норме, скачат образы, затащить в реджистри. Нюанс: в дхтл миррор нет возможности выкачать  релиз. Можно задать минимальную версию. И он докачает до максимальной версии. Клиент может не одобрить. И дхтл миррор не поможет, так как руками вытаскивать образы релизов. Лучшне обновиться до того, как загрузить в реджистри. Есть опасность по версиям - надо обнавляться на последнюю рпатч-версию минорной версии.


Ручная загрузка модулей после реджистри-->

## Требования к окружению

- ОС Linux / Unix / MacOS Х;
- установленный Docker или Docker Desktop;
- наличие установленных пакетов jq, readlink;
- не менее 100 Гб свободного пространства;
- сетевой доступ к репозиторию образов контейнеров (container registry) в закрытом контуре, который используется для размещения образов DKP.

## Доставка образов поставки в закрытое окружение

Доставка образов платформы в закрытое окружение осуществляется в виде готовой поставки платформы:
* на USB-носителе;
* с помощью утилиты `dhctl mirror` -- для этого варианта требуется доступ в интернет.

Поставка на USB-носителе включает в себя все необходимые артефакты для установки обновлений в закрытых окружениях. При использовании `dhctl mirror` эти артефакты будут созданы в процессе работы утилиты. В состав поставки свходят следующие артефакты:

- архив с образами контейнеров платформы `d8.tar` со всеми необходимыми промежуточными версиями: от заданной минимальной версии до последней доступной. (Перед загрузкой в репозиторий образов контейнеров (container registry) архив необходимо распаковать. Архив может разбиваться на части размером до 2 ГБ, а затем собирается и загружается);
- манифесты релизов Deckhouse Kubernetes Platform, соответствующие версиям образов поставки в файле `deckhousereleases.yaml`;
- исполняемый файл `dhctl`.

## Загрузка образов DKP

by КОСТЯ!

1. Создайте каталог на рабочей машине и перейдите в него.

2. В созданном каталоге создайте каталог `d8-images`.

3. Выполните аутентификацию в репозитории образов контейнеров DKP на `registry.deckhouse.ru`, используя в качестве пароля ваш лицензионный ключ:

   ```bash
   docker login -u license-token registry.deckhouse.ru
   ```

4. Выполните команду ниже для запуска в интерактивном режиме контейнера установщика DKP, указав подходящий каталог рабочей станции для проброса в контейнер:

   ```bash
   docker run -ti --pull=always -v $(pwd)/d8-images:/tmp/d8-images registry.deckhouse.ru/deckhouse/ee/install:stable bash
   ```

5. Из запущенного контейнера выполните команду ниже, предварительно заменив `YOUR_LICENSE_KEY` на ваш лицензионный ключ:

   ```bash
   dhctl mirror -l "YOUR_LICENSE_KEY" -i /tmp/d8-images/d8.tar --source="registry.deckhouse.ru/deckhouse/ee" --gost-digest
   ```

6. После завершения загрузки образов в каталог `d8-images` скопируйте утилиту `dhctl` в каталог `d8-images`:

   ```bash
   cp /usr/bin/dhctl /tmp/d8-images/dhctl
   ```

7. Выйдите из запущенного контейнера. --- КАК ЭТО СДЕЛАТЬ?

8. Перенесите содержимое каталога `d8-images` в закрытое окружение, где доступен приватный репозиторий образов контейнеров (container registry).

### Загрузка и выгрузка образов в изолированный репозитрий образов контейнеров

{% alert level="warning" %}
Доступно только в Enterprise Edition.
{% endalert %}

качаем + запихиваем

1. При необходимости авторизуйтесь в container registry `registry.deckhouse.ru` с помощью вашего лицензионного ключа.

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
   {% alert level="warning" %}
   Если загрузка образов будет прервана, повторный вызов команды проверит загруженные образы и продолжит загрузку с момента ее остановки. Продолжение загрузки возможно только если с момента остановки прошло не более суток.
   Используйте параметр `--no-pull-resume`, чтобы принудительно начать загрузку сначала.
   {% endalert %}

1. Чтобы скачать все версии Deckhouse начиная с конкретной версии, укажите ее в параметре `--min-version` в формате `X.Y`. Например, для загрузки всех версий Deckhouse, начиная с версии 1.45, используйте команду:

   ```shell
   DHCTL_CLI_MIRROR_LICENSE="<DECKHOUSE_LICENSE_KEY>" dhctl mirror --source="registry.deckhouse.ru/deckhouse/ee" --images-bundle-path /tmp/d8-images/d8.tar --min-version=1.45
   ```

   {% alert level="warning" %}
   Доступно только в Enterprise Edition.Обратите внимание, параметр `--min-version` будет проигнорирован если вы укажете версию выше находящейся в канале обновлений rock-solid.
   {% endalert %}

   Чтобы загрузить образы Deckhouse Kubernetes Platform из определенного репозитория registry, вы можете указать этот репозиторий с помощью флага `--source`.
   Существуют также дополнительные флаги `--source-login` и `--source-password`, используемые для аутентификации в предоставленном registry.
   Если они не указаны, `dhctl mirror` будет обращаться к registry анонимно.

   Например, вот как можно загрузить образы из стороннего registry:

   ```shell
   DHCTL_CLI_MIRROR_SOURCE_LOGIN="user" DHCTL_CLI_MIRROR_SOURCE_PASSWORD="password" dhctl mirror --source="corp.company.ru/sys/deckhouse" --images-bundle-path /tmp/d8-images/d8.tar
   ```

   {% alert level="warning" %}
   Параметр `--license` действует как сокращение для параметров `--source-login ($DHCTL_CLI_MIRROR_SOURCE_LOGIN)` и `--source-password ($DHCTL_CLI_MIRROR_SOURCE_PASSWORD)` и предназначен для использования с официальным registry Deckhouse.
   Если вы укажете и параметр `--license`, и пару логин + пароль одновременно, будет использована последняя.
   {% endalert %}


   `dhctl mirror` поддерживает расчет контрольных сумм итогового набора образов Deckhouse в формате ГОСТ Р 34.11-2012 (Стрибог) (параметр `--gost-digest`).
   Контрольная сумма будет выведена в лог и записана в файл с расширением `.tar.gostsum` рядом с tar-архивом, содержащим образы Deckhouse.

1. Опционально: Скопируйте утилиту `dhctl` из контейнера в директорию со скачанными образами Deckhouse Kubernetes Platform.

   ```shell
   cp /usr/bin/dhctl /tmp/d8-images/dhctl
   ```

ВЫГРУЗКА!

1. Передайте директорию с загруженными образами Deckhouse Kubernetes Platform на хост с доступом к изолированному registry.
   Для продолжения установки используйте скопированную ранее утилиту `dhctl` или запустите установщик Deckhouse аналогично пунктам 1 и 2 на хосте с доступом к изолированному registry. Не забудьте смонтировать директорию с загруженными образами Deckhouse в контейнер установщика.

1. Загрузите образы Deckhouse Kubernetes Platform с помощью команды `dhctl mirror` в изолированный registry.

   Пример команды для загрузки образов из файла `/tmp/d8-images/d8.tar`:

   ```shell
   DHCTL_CLI_MIRROR_USER="<USERNAME>" DHCTL_CLI_MIRROR_PASS="<PASSWORD>" dhctl mirror --images-bundle-path /tmp/d8-images/d8.tar --registry="your.private.registry.com:5000/deckhouse/ee"
   ```
   {% alert level="warning" %}
   Обратите внимание, образы будут выгружены в registry по пути, указанному в параметре `--registry` (в примере - `/deckhouse/ee`).
   Перед запуском команды убедитесь, что этот путь существует и у используемой учетной записи есть права на запись.
   {% endalert %}


   Если registry не требует авторизации, флаги `--registry-login`/`--registry-password` или переменные `$DHCTL_CLI_MIRROR_USER`/`$DHCTL_CLI_MIRROR_PASS` указывать не нужно.

1. После загрузки образов в изолированный registry переходите к установке Deckhouse Kubernetes Platform. Воспользуйтесь [руководством по быстрому старту](/gs/bm-private/step2.html).

   При запуске установщика используйте его образ из registry, в который ранее были загружены образы Deckhouse Kubernetes Platform, а не из публичного registry. Например, используйте адрес вида `your.private.registry.com:5000/deckhouse/ee/install:stable` вместо `registry.deckhouse.ru/deckhouse/ee/install:stable`.

   При установке в ресурсе `InitConfiguration` также используйте адрес вашего registry и данные авторизации с параметрами [imagesRepo](/documentation/v1/installing/configuration.html#initconfiguration-deckhouse-imagesrepo), [registryDockerCfg](/documentation/v1/installing/configuration.html#initconfiguration-deckhouse-registrydockercfg) или с [параметрами доступа к хранилищу образов контейнеров (или проксирующему registry)](/gs/bm-private/step3.html).

   После завершения установки примените сгенерированные во время загрузки манифесты DeckhouseReleases к вашему кластеру, используя `kubectl`:

   ```shell
   kubectl apply -f $(pwd)/d8-images/deckhousereleaases.yaml
   ```

## Выгрузка образов DKP

ВЫГРУЗКА - нюансы: сертификаты
есть кейсы - не может взять 28 Гб tar. ПОЯВИТСЯ ФИЧА ДЛЯ РАЗБИВКИ!!!

ЕСТЬ АРХИВ → upload в приватный registry - ПРИМЕНЕНИЕ СЕРТИФИКАТОВ!!!

trusted CA!!!

1. Перейдите в каталог `d8-images`, размещенный в закрытом окружении.

2. Задайте следующие переменные окружения, указав пути до сертификатов от закрытого репозитория образов контейнеров (container registry), а также путь до каталога с CA сертификатами:

```bash
export SSL_CERT_FILE="/etc/docker/certs.d/REGISTRY.EXAMPLE.COM/registry.example.com.cert"
export SSL_CERT_DIR="/etc/docker/certs.d/REGISTRY.EXAMPLE.COM"
```

3. Выполните команду для выгрузки образов Deckhouse в ваш репозиторий образов контейнеров, предварительно задав корректный путь и данные доступа YOUR_USERNAME и YOUR_PASSWORD:

```bash
./dhctl mirror -i ./d8.tar -r "REGISTRY.EXAMPLE.COM/deckhouse/ee" -u "YOUR_USERNAME" -p "YOUR_PASSWORD"
```

4. Дождитесь завершения процесса.

5. Убедитесь, что образы появились в вашем репозитории образов контейнеров. --- КАК ЭТО ПРОВЕРИТЬ?

## Подготовка к установке обновлений в закрытый контур

КРИТИЧНО ВАЖНО НЕ НАРУШИТЬ РАБОТУ КЛАСТЕРА!!!!

Чтобы подготовить установку обновлений в закрытый контур:

1. Убедитесь, что все обновляемые кластеры не имеют заданного канала обновлений `ReleaseChannel` с помощью команды:

   ```bash
   kubectl get mc deckhouse -o yaml | grep releaseChannel
   ```

1. В случае, если канал обновлений указан, удалите его, отредактировав конфигурацию модуля Deckhouse Kubernetes Platform:

   ```bash
   kubectl edit mc deckhouse -o yaml
   ```

1. После внесения изменений, дождитесь завершения обработки очереди Deckhouse Kubernetes Platform и проверьте, что изменения внесены, командой:

   ```bash
   kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller queue list
   ```

1. Переведите установку обновлений платформы в ручной режим. Для этого отредактируйте конфигурацию модуля Deckhouse Kubernetes Platform командой:

   ```bash
   kubectl edit mc deckhouse -o yaml
   ```

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

   ПОДПУНКТ 

   В случае использования самоподписанных сертификатов для registry образов контейнеров? используйте переменные окружения `SSL_CERT_FILE` и `SSL_CERT_DIR`, чтобы задать пути к СА сертификату и сертификатам registry образов контейнеров, как представлено на примере:

   ```bash
   export SSL_CERT_FILE="/etc/docker/certs.d/REGISTRY.EXAMPLE.COM/registry.example.com.cert"
   export SSL_CERT_DIR="/etc/docker/certs.d/REGISTRY.EXAMPLE.COM"
   ```

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
   kubectl get deckhousereleases.deckhouse.ru
   ```

   Пример вывода команды:

   ```text
   $ kubectl get deckhousereleases.deckhouse.ru 
   NAME       PHASE     TRANSITIONTIME   MESSAGE           
   v1.57.5    Pending   34s              
   v1.58.3    Pending   34s
   ```

1. В случае обнаружения в списке релиза с нестандартным названием без точек (из примера выше: `v1-57-5`) удалите его командой:

   ```bash
   kubectl delete deckhousereleases v1-57-5
   ```

## Установка обновлений

Дэкхаус релизес применят отдельно. Если релиза нет - обновиться на него не получится, даже при наличии образов. В этом файле отпилить придется - нюанс.

доступ по SSH - ТАЧКА КОНСОЛЬНЫЙ ДОСТУП.
На хос попадают и перевходят на мастер-узел и попадают в кластер (терминальный доступ) +веб-мордами.
Ремоут десктоп не работает в некоторых случаях.

если миррором поставку - архив может быть любого размера.

нюанс: архив перед тем как залить в реджистри - распаковать.
миррор может бить на части - архив. файлами по 2 гига, а внутрии собирать и в погружать в реджистри.

ГОСТ САМ - чексумма. описать. Когда суммы собираются - нужно отвалидировать.


Никаких дэкхаус io, ru - только для вариантов в РФ - в реджистри.
ФСТЭК своя редакция и свой сценарий - ЕГО ПОКА НЕ УКАЗЫВАЕМ!

### Установка в закрытый контур в ручном режиме

Необходимо вручную одобрять каждый устанавливаемый релиз.

1. Получите список доступных релизов Deckhouse командой:

   ```bash
   kubectl get deckhousereleases.deckhouse.ru
   ```

   Пример вывода команды:

   ```text
   $ kubectl get deckhousereleases.deckhouse.ru 
   NAME       PHASE     TRANSITIONTIME   MESSAGE          
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
       k8s:                              1.25.0
       Nodes Minimal OS Version Ubuntu:  18.04
     Version:                            v1.45.11
   Status:
     Approved:         false
     Message:          Waiting for manual approval
     Phase:            Pending
     Transition Time:  2024-03-11T13:10:00.117064727Z
   Events:             <none>
   ```

ССЫЛКА НА ОБНОВЛЕНИЕ КУБЕРА, ИНГРЕССА, УБУНТУ - команда чтобы проверить версии: uname -a - версию ядра линукса

4. Если все требования соблюдены, одобрите установку обновлений, выполнив команду:

   ```bash
   kubectl patch DeckhouseRelease ВЕРСИЯ_РЕЛИЗА --type=merge -p='{"approved": true}'
   ```

5. Дождитесь завершения установки релиза. Определить успешность операции можно по следующим признакам:

- в разделе **Alerts** сервиса Prometheus погас алерт `DeckhouseUpdating`;
- в Grafana отображается желаемая версия Deckhouse Kubernetes Platform;
- в очереди Deckhouse Kubernetes Platform нет задач для обработки; ВЫШЕ КОМАНДА queue
- релиз перешел из статуса `Pending` в статус `Deployed`. kubectl get deckhousereleases

НЮАНС: релиз становится DEPLOYED сразу. как только начнется - ПО ОДНОЙ ПРОВЕРКЕ НЕЛЬЗЯ ПОНЯТЬ.

Пример вывода для установленного релиза `v1.45.11`:

```text
$ kubectl get deckhousereleases
NAME       PHASE      TRANSITIONTIME   MESSAGE         
v1.46.12   Pending    10s              Waiting for manual approval
v1.47.5    Pending    5m55s            
v1.48.9    Pending    5m...
```

как резолвить. Кернел узла может быть слишком старым. → вручную обновляем ядро NodeGroupConfiguration - написать скрипт  
От ОС - кернел в доках. дать ссылку
По Куберу - поправить версию в конфигах.  
Есть блок по манифестам - если в кластере задпелоен софты, которые триггерят обновления в кубере - эти обновления перестанут работать. Как их резолвить. Для сисадминов - нужно написать - новый сценарий.  

------------------------------------------
ОСТАНОВИЛИСЬ

### Загрузка образов подключаемых модулей Deckhouse в изолированный приватный registry - ЭТО УЖЕ СЦЕНАРИЙ ОБНОВЛЕНИЙ ОТДЕЛЬНЫХ МОДУЛЕЙ

{% alert level="warning" %}
Релизный циклы модулей и DKP не совпадают - модуль может обновляться чаще, чем версия платформы.
{% endalert %}

Ниже описаны шаги, необходимые для ручной загрузки образов модулей, подключаемых из источника модулей (ресурса [*ModuleSource*](cr.html#modulesource)):

ИНСТРУКЦИЯ КОСТИ

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: deckhouse
spec:
  registry:
    # Укажите строку, полученную в п.1 вместо CHANGE
    dockerCfg: CHANGE
    repo: registry.deckhouse.ru/deckhouse/ee/modules
    scheme: HTTPS
  # Выберите подходящий канал обновлений: Alpha, Beta, EarlyAccess, Stable, RockSolid
  releaseChannel: "Stable"
```

1. Запустите установщик Deckhouse версии 1.58.0 или выше.

   ```shell
   docker run -ti --pull=always -v $(HOME)/d8-modules:/tmp/d8-modules -v $(HOME)/module_source.yml:/tmp/module_source.yml registry.deckhouse.ru/deckhouse/ce/install:v1.58.4 bash
   ```

   {% alert level="warning" %}
   В контейнер установщика монтируется директория с файловой системы хоста, в которую будут загружены образы модулей и YAML-манифест [ModuleSource](cr.html#modulesource), описывающий источник модулей.
   {% endalert %}

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
   {% alert level="warning" %}
   Обратите внимание, образы будут выгружены в registry по пути, указанному в параметре `--registry` (в примере - `/deckhouse-modules`).
   Перед запуском команды убедитесь, что этот путь существует и у используемой учетной записи есть права на запись.
   {% endalert %}


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

прокси для кластера  это одно. для реджистри - другая.

настройка прокси, чтобы ходить.

Можно спросить у Андрея Сидорова.

{% alert level="warning" %}
Доступно только в Enterprise Edition.
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
   
   # Choose the port you want. Below we set it to default 3128.
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

Для настройки Deckhouse Kubernetes Platform на использование proxy используйте параметр [proxy](installing/configuration.html#clusterconfiguration-proxy) ресурса `ClusterConfiguration`.

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

СЛЕДУЮЩИЕ СЦЕНАРИИ НУЖНЫ?

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

