---
title: "Быстрый старт"
permalink: ru/getting_started.html
layout: page-nosidebar
lang: ru
toc: false
---

{::options parse_block_html="false" /}

<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>

<div markdown="1">
Скорее всего вы уже ознакомились с основными [возможностями Deckhouse](/ru/features.html).

В данном руководстве рассмотрен пошаговый процесс установки Deckhouse на примере Community Edition. *(Подробности о вариантах лицензирования и отличиях между версиями CE и EE см. в разделе [продуктов](/ru/products.html#ce-vs-ee).)*

Установка платформы Deckhouse возможна как на железные серверы (bare metal), так и в инфраструктуру одного из поддерживаемых облачных провайдеров. Однако в зависимости от выбранной инфраструктуры процесс может немного отличаться, поэтому ниже приведены примеры установки для разных вариантов.

## Установка

### Требования и подготовка

Установка Deckhouse в общем случае выглядит так:

1.  На локальной машине (с которой будет производиться установка) запускается Docker-контейнер.
2.  Этому контейнеру передаются приватный SSH-ключ с локальной машины и файл конфигурации будущего кластера в формате YAML (например, `config.yml`).
3.  Контейнер подключается по SSH к целевой машине (для bare metal-инсталляций) или облаку, после чего происходит непосредственно установка и настройка кластера Kubernetes.

***Примечание**: при установке Deckhouse в публичное облако для Kubernetes-кластера будут использоваться «обычные» вычислительные ресурсы провайдера, а не managed-решение с Kubernetes, предлагаемое провайдером.*

Ограничения/требования для установки:

-   На машине, с которой будет производиться установка, необходимо наличие Docker runtime.
-   Deckhouse поддерживает разные версии Kubernetes: с 1.16 по 1.21 включительно. Однако обратите внимание, что для установки «с чистого листа» доступны только релизы 1.16, 1.19, 1.20 и 1.21. В примерах конфигурации ниже используется версия 1.19.
-   Рекомендованная минимальная аппаратная конфигурация для будущего кластера:
    -   не менее 4 ядер CPU;
    -   не менее 8  ГБ RAM;
    -   не менее 40 ГБ дискового пространства для кластера и данных etcd;
    -   ОС: Ubuntu Linux 16.04/18.04/20.04 LTS или CentOS 7;
    -   доступ к интернету и стандартным репозиториям используемой ОС для установки дополнительных необходимых пакетов.


## Шаг 1. Конфигурация

Выберите тип инфраструктуры, в которую будет устанавливаться Deckhouse:
</div>

<div class="tabs">
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure active"
  onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_bm');openTab(event, 'tabs__btn_infrastructure', 'tabs__content_installation', 'cluster_bootstrap_bare_metal');">
    Bare Metal
  </a>
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure"
    onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_yc');openTab(event, 'tabs__btn_infrastructure', 'tabs__content_installation', 'cluster_bootstrap_cloud');">
    Yandex.Cloud
  </a>
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure"
    onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_existing');openTab(event, 'tabs__btn_infrastructure', 'tabs__content_installation', 'deckhouse_install');">
    Существующий кластер Kubernetes
  </a>
</div>

<div id="infrastructure_bm" class="tabs__content tabs__content_infrastructure active">
<ul>
<li>
Организуйте SSH-доступ между машиной, с которой будет производиться установка, и будущим master-узлом кластера.
</li>
<li>
Определите 3 секции параметров для будущего кластера, создав новый конфигурационный файл <code>config.yml</code>:

{% offtopic title="config.yml" %}
```yaml
# секция с общими параметрами кластера (ClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: ClusterConfiguration
# тип инфраструктуры: bare-metal (Static) или облако (Cloud)
clusterType: Static
# адресное пространство pod’ов кластера
podSubnetCIDR: 10.111.0.0/16
# адресное пространство для service’ов кластера
serviceSubnetCIDR: 10.222.0.0/16
# устанавливаемая версия Kubernetes
kubernetesVersion: "1.19"
# домен кластера, используется для локальной маршрутизации
clusterDomain: "cluster.local"
---
# секция первичной инициализации кластера Deckhouse (InitConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: InitConfiguration
# конфигурация Deckhouse
deckhouse:
  # адрес реестра с образом инсталлятора; указано значение по умолчанию для официальной CE-сборки Deckhouse
  # подробнее см. в описании следующего шага
  imagesRepo: registry.deckhouse.io/deckhouse/fe
  # строка с ключом для доступа к Docker registry
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
  # используемый канал обновлений
  releaseChannel: Beta
  configOverrides:
    global:
      # имя кластера; используется, например, в лейблах алертов Prometheus
      clusterName: main
      # имя проекта; используется для тех же целей
      project: someproject
      modules:
        # шаблон, который будет использоваться для составления адресов системных приложений в кластере
        # например, Grafana для %s.somedomain.com будет доступна на домене grafana.somedomain.com
        publicDomainTemplate: "%s.somedomain.com"
    cniFlannelEnabled: true
    cniFlannel:
      podNetworkMode: vxlan
    prometheusMadisonIntegrationEnabled: false
    nginxIngressEnabled: false
---
# секция с параметрами bare metal-кластера (StaticClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: StaticClusterConfiguration
# адресное пространство для внутренней сети кластера
internalNetworkCIDRs:
- 10.0.4.0/24
```
{% endofftopic %}
</li>
</ul>
</div>

<div id="infrastructure_yc" class="tabs__content tabs__content_infrastructure">
  <div markdown="1">
  Чтобы Deckhouse смог управлять ресурсами в облаке, необходимо создать сервисный аккаунт в облачном провайдере и выдать ему edit права. Подробная инструкция по созданию сервисного аккаунта в Яндекс.Облаке доступна в [документации провайдера](https://cloud.yandex.com/en/docs/resource-manager/operations/cloud/set-access-bindings). Здесь мы представим краткую последовательность необходимых действий:
  </div>
  <ol>
    <li>
      Создайте пользователя с именем <code>candi</code>. В ответ вернутся параметры пользователя:
<div markdown="1">
```yaml
yc iam service-account create --name candi
id: <userId>
folder_id: <folderId>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: candi
```
</div>
    </li>
    <li>
      Назначьте роль <code>editor</code> вновь созданному пользователю для своего облака:
<div markdown="1">
```yaml
yc resource-manager folder add-access-binding <cloudname> --role editor --subject serviceAccount:<userId>
```
</div>
    </li>
    <li>
      Создайте JSON-файл с параметрами авторизации пользователя в облаке. В дальнейшем с помощью этих данных будем авторизовываться в облаке:
<div markdown="1">
```yaml
yc iam key create --service-account-name candi --output candi-sa-key.json
```
</div>
      <ul>
        <li>
          Сгенерируйте на машине-установщике SSH-ключ для доступа к виртуальным машинам в облаке. В Linux и macOS это можно сделать с помощью консольной утилиты <code>ssh-keygen</code>. Публичную часть ключа необходимо включить в файл конфигурации: она будет использована для доступа к узлам облака.
        </li>
        <li>
          Выберите layout — архитектуру размещения объектов в облаке; для каждого провайдера есть несколько таких предопределённых layouts в Deckhouse. Для примера с Яндекс.Облаком мы возьмем вариант <strong>WithoutNAT</strong>. В данной схеме размещения NAT (любого вида) не используется, а каждому узлу выдаётся публичный IP.
        </li>
        <li>
          Задайте минимальные 3 секции параметров для будущего кластера в файле <code>config.yml</code>:
{% offtopic title="config.yml" %}
```yaml
# секция с общими параметрами кластера (ClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: ClusterConfiguration
# тип инфраструктуры: bare-metal (Static) или облако (Cloud)
clusterType: Cloud
# параметры облачного провайдера
cloud:
  # используемый облачный провайдер
  provider: Yandex
  # префикс для объектов кластера для их отличия (используется, например, при маршрутизации)
  prefix: "yandex-demo"
# адресное пространство pod’ов кластера
podSubnetCIDR: 10.111.0.0/16
# адресное пространство для service’ов кластера
serviceSubnetCIDR: 10.222.0.0/16
# устанавливаемая версия Kubernetes
kubernetesVersion: "1.19"
# домен кластера (используется для локальной маршрутизации)
clusterDomain: "cluster.local"
---
# секция первичной инициализации кластера Deckhouse (InitConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: InitConfiguration
# секция с параметрами Deckhouse
deckhouse:
  # адрес реестра с образом инсталлятора; указано значение по умолчанию для официальной CE-сборки Deckhouse
  # подробнее см. в описании следующего шага
  imagesRepo: registry.deckhouse.io/deckhouse/fe
  # строка с ключом для доступа к Docker registry
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
  # используемый канал обновлений
  releaseChannel: Beta
  configOverrides:
    global:
      # имя кластера; используется, например, в лейблах алертов Prometheus
      clusterName: somecluster
      # имя проекта для кластера; используется для тех же целей
      project: someproject
      modules:
        # шаблон, который будет использоваться для составления адресов системных приложений в кластере
        # например, Grafana для %s.somedomain.com будет доступна на домене grafana.somedomain.com
        publicDomainTemplate: "%s.somedomain.com"
    prometheusMadisonIntegrationEnabled: false
    nginxIngressEnabled: false
---
# секция с параметрами облачного провайдера (YandexClusterConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: YandexClusterConfiguration
# публичная часть SSH-ключа для доступа к узлам облака
sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
# layout — архитектура расположения ресурсов в облаке
layout: WithoutNAT
# адресное пространство узлов кластера
nodeNetworkCIDR: 10.100.0.0/21
# параметры группы master-узла
masterNodeGroup:
# количество реплик
  replicas: 1
  # количество CPU, RAM, HDD, используемый образ виртуальной машины и политика назначения внешних IP-адресов
  instanceClass:
    cores: 4
    memory: 8192
    imageID: fd8vqk0bcfhn31stn2ts
    diskSizeGB: 40
    externalIPAddresses:
    - Auto
# идентификатор облака и каталога Yandex.Cloud
provider:
  cloudID: "***"
  folderID: "***"
  # данные сервисного аккаунта облачного провайдера, имеющего права на создание и управление виртуальными машинами
  # это содержимое файла candi-sa-key.json, который был сгенерирован выше
  serviceAccountJSON: |
    {
      "id": "***",
      "service_account_id": "***",
      "created_at": "2020-08-17T08:56:17Z",
      "key_algorithm": "RSA_2048",
      "public_key": ***,
      "private_key": ***
    }
```
{% endofftopic %}
        </li>
      </ul>
    </li>
  </ol>
  <div markdown="1">
Примечания:
-   Полный список поддерживаемых облачных провайдеров и настроек для них доступен в секции документации [Cloud providers](/ru/documentation/v1/kubernetes.html).
-   Подробнее о каналах обновления Deckhouse (release channels) можно почитать в [документации](/ru/documentation/v1/deckhouse-release-channels.html).
  </div>
</div>

<div id="infrastructure_existing" class="tabs__content tabs__content_infrastructure">
<ul>
<li>
Организуйте SSH-доступ между машиной, с которой будет производиться установка, и существующим master-узлом кластера.
</li>
<li>
Определите параметры для Deckhouse, создав новый конфигурационный файл <code>config.yml</code>:

{% offtopic title="config.yml" %}
```yaml
# секция первичной инициализации Deckhouse (InitConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: InitConfiguration
# конфигурация Deckhouse
deckhouse:
  # адрес реестра с образом инсталлятора; указано значение по умолчанию для официальной CE-сборки Deckhouse
  # подробнее см. в описании следующего шага
  imagesRepo: registry.deckhouse.io/deckhouse/fe
  # строка с ключом для доступа к Docker registry
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
  # используемый канал обновлений
  releaseChannel: Beta
  configOverrides:
    deckhouse:
      bundle: Minimal
    global:
      # имя кластера; используется, например, в лейблах алертов Prometheus
      clusterName: main
      # имя проекта; используется для тех же целей
      project: someproject
      modules:
        # шаблон, который будет использоваться для составления адресов системных приложений в кластере
        # например, Grafana для %s.somedomain.com будет доступна на домене grafana.somedomain.com
        publicDomainTemplate: "%s.somedomain.com"
```
{% endofftopic %}
</li>
</ul>
</div>

<div id="cluster_bootstrap_bare_metal" class="tabs__content tabs__content_installation active" markdown="1">

## Шаг 2. Установка

Для непосредственной установки потребуется Docker-образ установщика Deckhouse. Мы воспользуемся уже готовым официальным образом от проекта. Информацию по самостоятельной сборке образа из исходников можно будет найти в [репозитории проекта](https://github.com/deckhouse/deckhouse).

В результате запуска следующей команды произойдет скачивание Docker-образа установщика Deckhouse, в который будут передана приватная часть SSH-ключа и файл конфигурации, подготовленные на прошлом шаге (пути расположения файлов даны по умолчанию). Будет запущен интерактивный терминал в системе образа:

```yaml
docker login -u demotoken -p <ACCESS_TOKEN> registry.deckhouse.io
docker run -it -v $(pwd)/config.yml:/config.yml -v $HOME/.ssh/:/tmp/.ssh/ registry.deckhouse.io/deckhouse/fe/install:beta bash
```

Далее для запуска установки необходимо выполнить команду:

```yaml
dhctl bootstrap \
  --ssh-user=<username> \
  --ssh-host=<master_ip> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml
```

Здесь переменная `username` — это:
-   имя пользователя, от которого генерировался SSH-ключ для установки в случае bare metal;
-   пользователь по умолчанию для соответствующего образа виртуальной машины для облачных инсталляций (например: `ubuntu`, `user`, `azureuser`).

Примечания:
-   В процессе установки не рекомендуется выходить из контейнера (например, чтобы поменять конфигурацию). Иначе при повторном запуске потребуется предварительно вручную удалить ресурсы, созданные в провайдере. В случае необходимости изменить конфигурацию следует использовать внешний текстовый редактор (например, vim): после сохранения файла обновлённая конфигурация автоматически подгрузится в контейнер.
-   В случае возникновения проблем для остановки процесса установки следует воспользоваться следующей командой (файл конфигурации должен совпадать с тем, с которым производилось разворачивание кластера):

```yaml
dhctl bootstrap-phase abort --config=/config.yml
```

По окончании установки произойдет возврат к командной строке. Кластер готов к работе: управлению дополнительными модулями, разворачиванию ваших приложений и т.п.

## Шаг 3. Проверка статуса

Просмотр состояния Kubernetes-кластера возможен сразу после (или даже во время) установки Deckhouse. По умолчанию `.kube/config`, используемый для доступа к Kubernetes, генерируется на хосте с кластером. Если подключиться к этому хосту по SSH, для взаимодействия с Kubernetes можно воспользоваться стандартными инструментами, такими как `kubectl`.
</div>

<div id="cluster_bootstrap_cloud" class="tabs__content tabs__content_installation" markdown="1">

## Шаг 2. Установка

Для непосредственной установки потребуется Docker-образ установщика Deckhouse. Мы воспользуемся уже готовым официальным образом от проекта. Информацию по самостоятельной сборке образа из исходников можно будет найти в [репозитории проекта](https://github.com/deckhouse/deckhouse).

В результате запуска следующей команды произойдет скачивание Docker-образа установщика Deckhouse, в который будут передана приватная часть SSH-ключа и файл конфигурации, подготовленные на прошлом шаге (пути расположения файлов даны по умолчанию). Будет запущен интерактивный терминал в системе образа:

```yaml
docker login -u demotoken -p <ACCESS_TOKEN> registry.deckhouse.io
docker run -it -v $(pwd)/config.yml:/config.yml -v $HOME/.ssh/:/tmp/.ssh/ registry.deckhouse.io/deckhouse/fe/install:beta bash
```

Далее для запуска установки необходимо выполнить команду:

```yaml
dhctl bootstrap \
  --ssh-user=<username> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml
```

Здесь переменная `username` — это:
-   имя пользователя, от которого генерировался SSH-ключ для установки в случае bare metal;
-   пользователь по умолчанию для соответствующего образа виртуальной машины для облачных инсталляций (например: `ubuntu`, `user`, `azureuser`).

Примечания:
-   В процессе установки не рекомендуется выходить из контейнера (например, чтобы поменять конфигурацию). Иначе при повторном запуске потребуется предварительно вручную удалить ресурсы, созданные в провайдере. В случае необходимости изменить конфигурацию следует использовать внешний текстовый редактор (например, vim): после сохранения файла обновлённая конфигурация автоматически подгрузится в контейнер.
-   В случае возникновения проблем для остановки процесса установки следует воспользоваться следующей командой (файл конфигурации должен совпадать с тем, с которым производилось разворачивание кластера):

```yaml
dhctl bootstrap-phase abort --config=/config.yml
```

По окончании установки произойдет возврат к командной строке. Кластер готов к работе: управлению дополнительными модулями, разворачиванию ваших приложений и т.п.

## Шаг 3. Проверка статуса

Просмотр состояния Kubernetes-кластера возможен сразу после (или даже во время) установки Deckhouse. По умолчанию `.kube/config`, используемый для доступа к Kubernetes, генерируется на хосте с кластером. Если подключиться к этому хосту по SSH, для взаимодействия с Kubernetes можно воспользоваться стандартными инструментами, такими как `kubectl`.
</div>

<div id="deckhouse_install" class="tabs__content tabs__content_installation" markdown="1">

## Шаг 2. Установка

Для непосредственной установки потребуется Docker-образ установщика Deckhouse. Мы воспользуемся уже готовым официальным образом от проекта. Информацию по самостоятельной сборке образа из исходников можно будет найти в [репозитории проекта](https://github.com/deckhouse/deckhouse).

В результате запуска следующей команды произойдет скачивание Docker-образа установщика Deckhouse, в который будут передана приватная часть SSH-ключа и файл конфигурации, подготовленные на прошлом шаге (пути расположения файлов даны по умолчанию). Будет запущен интерактивный терминал в системе образа:

```yaml
docker login -u demotoken -p <ACCESS_TOKEN> registry.deckhouse.io
docker run -it -v $(pwd)/config.yml:/config.yml -v $HOME/.ssh/:/tmp/.ssh/ -v $(pwd)/kubeconfig:/kubeconfig registry.deckhouse.io/deckhouse/fe/install:beta bash
```

Примечания:
-   В kubeconfig необходимо смонтировать kubeconfig с доступом к Kubernetes API.

Далее для запуска установки необходимо выполнить команду:

```yaml
dhctl bootstrap-phase install-deckhouse \
  --kubeconfig=/kubeconfig \
  --config=/config.yml
```

По окончании установки произойдет возврат к командной строке. Deckhouse готов к работе: управлению дополнительными модулями, разворачиванию ваших приложений и т.п.

## Шаг 3. Проверка статуса

Просмотр состояния Kubernetes-кластера возможен сразу после (или даже во время) установки Deckhouse.
</div>

<div markdown="1">
Например, посмотреть на состояние кластера командой:

```yaml
kubectl -n d8-system get deployments/deckhouse
```

В ответе deployment с именем `deckhouse` должен иметь статус `READY 1/1` — это будет свидетельствовать о том, что установка модулей завершена, кластер готов для дальнейшего использования.

Для более удобного контроля за кластером доступен модуль с официальной веб-панелью для Kubernetes — [dashboard](/en/documentation/v1/modules/500-dashboard/). Он активируется по умолчанию после установки и доступен по адресу `https://dashboard<значение параметра publicDomainTemplate>` с уровнем доступа `User`. (Подробнее про уровни доступа см. в документации по [модулю user-authz](/ru/documentation/v1/modules/140-user-authz/).)

Логи ведутся в формате JSON, поэтому для их просмотра логов «на лету» удобнее использовать `jq`:

```yaml
kubectl logs -n d8-system deployments/deckhouse -f --tail=10 | jq -rc .msg
```
А для полноценного мониторинга состояния кластера существует специальный [набор модулей](/en/documentation/v1/modules/300-prometheus/).

## Следующие шаги

### Работа с модулями

Модульная система Deckhouse позволяет «на лету» добавлять и убирать модули из кластера. Для этого необходимо отредактировать конфигурацию кластера — все изменения применятся автоматически.

Например, добавим модуль [user-authn](/en/documentation/v1/modules/150-user-authn/):

1.  Открываем конфигурацию Deckhouse:
    ```yaml
    kubectl -n d8-system edit cm/deckhouse
    ```
2.  Находим секцию `data` и включаем в ней модуль:
    ```yaml
data:
  userAuthnEnabled: "true"
```
3.  Сохраняем конфигурацию. В этот момент Deckhouse понимает, что произошли изменения, и модуль устанавливается автоматически.

Для изменения настроек модуля необходимо повторить пункт 1, т.е. внести изменения в конфигурацию и сохранить их. Изменения автоматически применятся.

Для отключения модуля потребуется аналогичным образом задать параметру значение `false`.

### Куда двигаться дальше?

Все установлено, настроено и работает. Подробная информация о системе в целом и по каждому компоненту Deckhouse расположена в [документации](/en/documentation/v1/).

По всем возникающим вопросам связывайтесь с нашим [онлайн-сообществом](/ru/community.html#online-community).

</div>
