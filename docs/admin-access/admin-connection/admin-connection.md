## Термины и определения

## Подключение администратора

Изначально администратор входит под логином сервисного аккаунта. 

Идентификатор администратора - сервисный аккаунт, при установке кластера и с правами администратора кластера. 

Сервисные аккаунты могут объединяться в кластерные роли. 

Администратор создает кластер при помощи файла `kubeconfig` и раздает его пользователям (администратор назначает им роли), которые могут пользоваться только предоставленным кластером. Управление доступом производится на основе ролей (RBAC). RBAC можно использовать со всеми ресурсами, которые поддерживают *CRUD* (Create, Read, Update, Delete).

### Требования к окружению

* Персональный компьютер с которого будет производиться установка. Он нужен только для запуска инсталлятора Deckhouse и не будет частью кластера.
* ОС: Windows 10+, macOS 10.15+, Linux (Ubuntu 18.04+, Fedora 35+).
* Установленный docker для запуска инсталлятора Deckhouse (инструкции для Ubuntu, macOS, Windows).
* HTTPS-доступ до хранилища образов контейнеров `registry.deckhouse.ru`.
* Доступ до API облачного провайдера, учетная запись с правами на создание ресурсов и настроенная утилита [**govc**](ссылка).

### Минимальные рекомендованные ресурсы

* 8 ядер CPU;
* 16 ГБ RAM;
* 100 ГБ дискового пространства;
* HTTPS-доступ к хранилищу образов контейнеров registry.deckhouse.io.

### Необходимые ресурсы

* **User** с необходимым[набором прав](#создание-и-назначение-роли).
* **Network** с DHCP и доступом в интернет.
* **Datacenter** с соответствующим тегом[`k8s-region`](#создание-тегов-и-категорий-тегов).
* **Cluster** с соответствующим тегом[`k8s-zone`](#создание-тегов-и-категорий-тегов).
* **Datastore** в любом количестве, с соответствующими[тегами](#конфигурация-datastore).
* **Template** —[подготовленный](#подготовка-образа-виртуальной-машины) образ виртуальной машины.

Для работы кластера необходим VLAN с DHCP и доступом в интернет с условиями:

* Если VLAN публичный (публичные адреса), нужна вторая сеть, в которой необходимо развернуть сеть узлов кластера (в этой сети DHCP не нужен).
* Если VLAN внутренний (приватные адреса), эта же сеть будет сетью узлов кластера.
* Если имеется внутренний балансировщик запросов, можно направлять трафик напрямую на фронтенд-узлы кластера.
* Если балансировщик отсутствует, для организации отказоустойчивых сервисов *LoadBalancer* рекомендуется использовать MetalLB в режиме BGP. В кластере будут созданы frontend-узлы с двумя интерфейсами. Для этого дополнительно потребуются отдельный VLAN обмена трафиком между BGP-роутерами и MetalLB. В этом VLAN должны быть DHCP, доступ в интернет и IP-адреса BGP-роутеров.
* ASN (номер автономной системы) на BGP-роутере.
* ASN (номер автономной системы) в кластере.
* Диапазона, из которого анонсированы адреса.

### Хранилища данных

В кластере может одновременно использоваться различное количество типов хранилищ. В минимальной конфигурации потребуются:

* Datastore, в котором Kubernetes-кластер будет заказывать способы хранения *PersistentVolume*;
* Datastore, в котором будут заказываться root-диски для виртуальной машины (это может быть тот же Datastore, что и для *PersistentVolume*).

Список необходимых ресурсов выбирается в заивисимости от [провайдера](ссылка).

### Подготовка образа виртуальной машины

Для создания шаблона виртуальной машины (`Template`) рекомендуется использовать готовый cloud-образ/OVA-файл, предоставляемый вендором ОС:
* [РЕД ОС](ссылка);
* [AlterOS](ссылка);
* [Astra Linux Special Edition](ссылка);
* [**Ubuntu**](https://cloud-images.ubuntu.com/);
* [**Debian**](https://cloud.debian.org/images/cloud/);
* [**CentOS**](https://cloud.centos.org/);
* [**Rocky Linux**](https://rockylinux.org/alternative-images/) (секция _Generic Cloud / OpenStack_).

Если необходимо использовать собственный образ, обратитесь к [документации](/documentation/v1/modules/030-cloud-provider-vsphere/environment.html#требования-к-образу-виртуальной-машины).

1. Загрузите ОС и настройте доступ по ключу с основного ПК:

   ```
   $ ssh-copy-id <IP-адрес сервера>
   ```

1. Сгенерируйте ключ командой `ssh-keygen -t rsa`.

1. Подключитесь к серверу, чтобы убедиться в корректных настройках (ниже представлен пример успешной работы на ОС Ubuntu):

   ```
   $ ssh 192.168.2.38
   Welcome to Ubuntu 22.04.2 LTS (GNU/Linux 5.15.0-60-generic x86_64)
   
    * Documentation:  https://help.ubuntu.com
    * Management:     https://landscape.canonical.com
    * Support:        https://ubuntu.com/advantage
   
     System information as of Wed Mar  1 11:23:13 AM UTC 2023
   
     System load:  0.0                Temperature:             46.0 C
     Usage of /:   2.7% of 292.35GB   Processes:               135
     Memory usage: 2%                 Users logged in:         0
     Swap usage:   0%                 IPv4 address for enp3s0: 192.168.2.38
   
   
    * Introducing Expanded Security Maintenance for Applications.
      Receive updates to over 25,000 software packages with your
      Ubuntu Pro subscription. Free for personal use.
   
        https://ubuntu.com/pro
  
   Expanded Security Maintenance for Applications is not enabled.
   
   0 updates can be applied immediately.
   
   Enable ESM Apps to receive additional future security updates.
   See https://ubuntu.com/esm or run: sudo pro status
   
   
   Last login: Wed Mar  1 10:34:01 2023 from 192.168.2.35
   ```

1. Отключитесь от сервера, выполнив команду `exit` или нажав сочетание клавиш **Ctrl** + **D**.

### Установка кластера

Установите кластер. Нажмите кнопку **Устанрвка**, как описано в разделе [Быстрый старт](https://deckhouse.ru/gs/). На странице отобразится сгенерированное содержимое для файла конфигурации `config.yml`. Введенный ранее шаблон доменных имен появится в секции `publicDomainTemplate`:

```
# Cекция с общими параметрами кластера (ClusterConfiguration).
# Используемая версия API Deckhouse Platform.
apiVersion: deckhouse.io/v1
# Тип секции конфигурации.
kind: ClusterConfiguration
# Тип инфраструктуры: bare metal (Static) или облако (Cloud).
clusterType: Static
# Адресное пространство Pod'ов кластера.
podSubnetCIDR: 10.111.0.0/16
# Адресное пространство для Service'ов кластера.
serviceSubnetCIDR: 10.222.0.0/16
# Устанавливаемая версия Kubernetes.
kubernetesVersion: "1.23"
# Домен кластера.
clusterDomain: "cluster.local"
---
# Секция первичной инициализации кластера Deckhouse (InitConfiguration).
# Используемая версия API Deckhouse.
apiVersion: deckhouse.io/v1
# Тип секции конфигурации.
kind: InitConfiguration
# Секция с параметрами Deckhouse.
deckhouse:
  # Используемый канал обновлений.
  releaseChannel: Stable
  configOverrides:
    global:
      modules:
        # Шаблон, который будет использоваться для составления адресов системных приложений в кластере.
        # Например, Grafana для %s.example.com будет доступна на домене grafana.example.com.
        publicDomainTemplate: "%s.example.com"
    # Включить модуль cni-cilium.
    cniCiliumEnabled: true
    # Конфигурация модуля
    cniCilium:
      # Режим работы туннеля.
      tunnelMode: VXLAN
---
# Cекция с параметрами bare metal кластера (StaticClusterConfiguration).
# Используемая версия API Deckhouse.
apiVersion: deckhouse.io/v1
# Тип секции конфигурации.
kind: StaticClusterConfiguration
# Список внутренних сетей узлов кластера (например, '10.0.4.0/24'), который
# используется для связи компонентов Kubernetes (kube-apiserver, kubelet...) между собой.
# Если каждый узел в кластере имеет только один сетевой интерфейс,
# ресурс StaticClusterConfiguration можно не создавать.
internalNetworkCIDRs:
- '192.168.2.0/24'
```

> Обратите внимание: в последнем разделе сгенерированного файла конфигурации *StaticClusterConfiguration* укажите сеть, в которую направлен основной сетевой интерфейс сервера, поскольку на борту имеется несколько интерфейсов. Если в виртуальной машине один сетевой интерфейс — эту секцию можно удалить.
>
> Модуль, отвечающий за реализацию CNI, был заменен с `cni-flannel` на `cni-cilium`. Это требуется для корректной работы виртуальных машин, так как их сетевое взаимодействие основано на Cilium. В настройках также указан параметр, определяющий режим работы туннелей, который установлен в значение `VXLAN`.

Сохраните содержимое в файле `config.yml`, положив его в любой отдельную директорию.

### Развертывание кластера

1. Выберите [инсфраструктуру](https://deckhouse.ru/gs/).

Для развертывания будет использован минимальный набор:

* Кластер из одного мастер-узла и одного рабочего узла.
* Управляющие компоненты Kubernetes-кластера и Deckhouse-контроллер, работающие на мастер-узле.
* Deckhouse с не системными компонентами (Ingress-контроллер, Prometheus, cert-manager и т.д.) на рабочем узле. Поэтому приложения должны работать на рабочем узле.

2. Создайте роль с необходимыми [правами](ссылка на раздел RBAC)

### Подключение к мастер-узлу

1. Подключитесь к мастер-узлу по SSH (IP-адрес мастер-узла выводится инсталлятором по завершении установки, но также можно его найти, используя веб-интерфейс или CLI‑утилиты облачного провайдера):

   ```
   ssh ubuntu@<MASTER_IP>
   ```
2. Проверьте работу `kubectl`, выведя список узлов кластера:

   ```
   sudo /opt/deckhouse/bin/kubectl get nodes
   ```
   Пример вывода:

   ```
   $ sudo /opt/deckhouse/bin/kubectl get nodes
   NAME                                     STATUS   ROLES                  AGE   VERSION
   cloud-demo-master-0                      Ready    control-plane,master   12h   v1.23.9
   cloud-demo-worker-01a5df48-84549-jwxwm   Ready    worker                 12h   v1.23.9
   ```

   > Запуск Ingress-контроллера после завершения установки Deckhouse может занять какое-то время. Прежде чем продолжить убедитесь что Ingress-контроллер запустился:
   >
   > ```
   > sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get po
   > ```

3. Дождитесь перехода подов в статус `Ready`.

   Пример вывода:

   ```
   DNS
   ```

4. Чтобы получить доступ к веб-интерфейсам компонентов Deckhouse, настройте работу DNS и укажите в параметрах Deckhouse шаблон DNS-имен. Шаблон DNS-имен используется для настройки Ingress-ресурсов системных приложений. Например, за интерфейсом Grafana закреплено имя `grafana`. Поэтому для шаблона `%s.kube.company.my Grafana` будет доступен шаблон DNS-имен по адресу `grafana.kube.company.my`.

5. Настройте DNS для сервисов Deckhouse одним из следующих способов:

* С возможностью добавления DNS-записи, используя DNS-сервер. Если ваш шаблон DNS-имен кластера является wildcard DNS-шаблоном (например, `%s.kube.company.my`), то добавьте соответствующую wildcard A-запись со значением IP-адреса мастер-узла. Если ваш шаблон DNS-имен кластера не является wildcard DNS-шаблоном (например, `%s-kube.company.my`), добавьте А или CNAME-записи с адресом мастер-узла, для следующих DNS-имен сервисов согласно шаблону DNS-имен:
  * `api`
  * `argocd`
  * `cdi-uploadproxy`
  * `dashboard`
  * `documentation`
  * `dex`
  * `grafana`
  * `hubble`
  * `istio`
  * `istio-api-proxy`
  * `kubeconfig`
  * `openvpn-admin`
  * `prometheus`
  * `status`
  * `upmeter`

* Без управления DNS-сервером. На компьютере, с которого необходим доступ к сервисам Deckhouse, добавьте статические записи в файл `/etc/hosts` (%SystemRoot%\system32\drivers\etc\hosts для Windows).

Для добавления записей в файл `/etc/hosts` на на Linux-компьютере с которого необходим доступ к сервисам Deckhouse (далее — ПК), выполните [следующие шаги](ссылка).

### Настройка удаленного доступа к кластеру

На персональном компьютере выполните следующие шаги, для того чтобы настроить подключение `kubectl` к кластеру:

1. Откройте веб-интерфейс сервиса *Kubeconfig Generator*. Для него зарезервировано имя `kubeconfig`, и адрес для доступа формируется согласно шаблона DNS-имен (который установили ранее). Например, для шаблона DNS-имен `%s.1.2.3.4.sslip.io`, веб-интерфейс *Kubeconfig Generator* будет доступен по адресу `https://kubeconfig.1.2.3.4.sslip.io`.
2. Авторизуйтесь под пользователем `admin@deckhouse.io`. Пароль пользователя, сгенерированный на предыдущем шаге:
` — rjm1pcgttf` (пароль можно найти в *CustomResource User* в файле `resource.yml`).
3. Выберите вкладку с ОС персонального компьютера.
4. Последовательно скопируйте и выполните команды, приведенные на странице.
5. Проверьте корректную работу `kubectl` (например, выполнив команду  `kubectl get no`).

### Настройка Grafana

Настройте [Grafana](https://grafana.com/docs/grafana/latest/setup-grafana/installation/). <!-- подробнее описать? -->

### Настройка модулей

Deckhouse состоит из оператора Deckhouse и модулей. Модуль — это набор из Helm-чарта, хуков [Addon-operator'а](https://github.com/flant/addon-operator/), правил сборки компонентов модуля (компонентов Deckhouse) и других файлов.

Deckhouse настраивается с помощью:

* **[Глобальных настроек](deckhouse-configure-global.html).** Глобальные настройки хранятся в custom resource `ModuleConfig/global`. Глобальные настройки можно рассматривать как специальный модуль `global`, который нельзя отключить.
* **[Настроек модулей](#настройка-модуля).** Настройки каждого модуля хранятся в custom resource `ModuleConfig`, имя которого совпадает с именем модуля (в kebab-case).
* **Кастомных ресурсов** Некоторые модули настраиваются с помощью дополнительных custom resource'ов.

> При работе с модулями Deckhouse использует проект [addon-operator](https://github.com/flant/addon-operator/). Ознакомьтесь с его документацией, если хотите понять, как Deckhouse работает с [модулями](https://github.com/flant/addon-operator/blob/main/docs/src/MODULES.md), [хуками модулей](https://github.com/flant/addon-operator/blob/main/docs/src/HOOKS.md) и [параметрами модулей](https://github.com/flant/addon-operator/blob/main/docs/src/VALUES.md). Будем признательны, если поставите проекту _звезду_.

Модуль настраивается с помощью custom resource `ModuleConfig`, имя которого совпадает с именем модуля (в kebab-case). Custom resource `ModuleConfig` имеет следующие поля:

* `metadata.name` — название модуля Deckhouse в kebab-case (например, `prometheus`, `node-manager`);
* `spec.version` — версия схемы настроек модуля (целое число, больше нуля). Обязательное поле, если `spec.settings` не пустое. Номер актуальной версии можно увидеть в документации модуля в разделе _Настройки_:
  - Deckhouse поддерживает обратную совместимость версий схемы настроек модуля. Если используется схема настроек устаревшей версии, при редактировании или просмотре custom resource'а будет выведено предупреждение о необходимости обновить схему настроек модуля;
* `spec.settings` — настройки модуля. Необязательное поле, если используется поле `spec.enabled`. Описание возможных настроек можно найти в документации модуля в разделе _Настройки_;
* `spec.enabled` — необязательное поле для явного[включения или отключения модуля](#включение-и-отключение-модуля). Если не задано, модуль может быть включен по умолчанию в одном из[наборов модулей](#наборы-модулей).

> Deckhouse не изменяет custom resource'ы `ModuleConfig`. Это позволяет применять подход Infrastructure as Code (IaC) при хранении конфигурации. Другими словами, можно воспользоваться всеми преимуществами системы контроля версий для хранения настроек Deckhouse, использовать Helm, kubectl и другие привычные инструменты.

Пример custom resource для настройки модуля `kube-dns`:

```
<span class="na">apiVersion</span><span class="pi">:</span> <span class="s">deckhouse.io/v1alpha1</span>
<span class="na">kind</span><span class="pi">:</span> <span class="s">ModuleConfig</span>
<span class="na">metadata</span><span class="pi">:</span>
  <span class="na">name</span><span class="pi">:</span> <span class="s">kube-dns</span>
<span class="na">spec</span><span class="pi">:</span>
  <span class="na">version</span><span class="pi">:</span> <span class="m">1</span>
  <span class="na">settings</span><span class="pi">:</span>
    <span class="na">stubZones</span><span class="pi">:</span>
    <span class="pi">-</span> <span class="na">upstreamNameservers</span><span class="pi">:</span>
      <span class="pi">-</span> <span class="s">192.168.121.55</span>
      <span class="pi">-</span> <span class="s">10.2.7.80</span>
      <span class="na">zone</span><span class="pi">:</span> <span class="s">directory.company.my</span>
    <span class="na">upstreamNameservers</span><span class="pi">:</span>
    <span class="pi">-</span> <span class="s">10.2.100.55</span>
    <span class="pi">-</span> <span class="s">10.2.200.55</span>
```

Некоторые модули настраиваются с помощью дополнительных custom resource'ов. Воспользуйтесь поиском (вверху страницы) или выберите модуль в меню слева, чтобы просмотреть документацию по его настройкам и используемым custom resource'ам.

### Включение и отключение модуля

> Некоторые модули могут быть включены по умолчанию в зависимости от используемого [набора модулей](#наборы-модулей).

Для явного включения или отключения модуля необходимо установить `true` или `false` в поле `.spec.enabled` в соответствующем custom resource `ModuleConfig`. Если для модуля нет такого custom resource `ModuleConfig`, его нужно создать.

Пример явного выключения модуля `user-authn` (модуль будет выключен независимо от используемого набора модулей):

```
<span class="na">apiVersion</span><span class="pi">:</span> <span class="s">deckhouse.io/v1alpha1</span>
<span class="na">kind</span><span class="pi">:</span> <span class="s">ModuleConfig</span>
<span class="na">metadata</span><span class="pi">:</span>
  <span class="na">name</span><span class="pi">:</span> <span class="s">user-authn</span>
<span class="na">spec</span><span class="pi">:</span>
  <span class="na">enabled</span><span class="pi">:</span> <span class="no">false</span>
```

Проверить состояние модуля можно с помощью команды `kubectl get moduleconfig <&#x418;&#x41C;&#x42F;_&#x41C;&#x41E;&#x414;&#x423;&#x41B;&#x42F;>`.

Пример:

```
<span class="nv">$ </span>kubectl get moduleconfig user-authn
NAME                STATE      VERSION    STATUS    AGE
user-authn          Disabled   1                    12h
```

## Наборы модулей

В зависимости от используемого [набора модулей](./modules/002-deckhouse/configuration.html#parameters-bundle) (bundle) модули могут быть включены или выключены по умолчанию.

| Набор модулей (bundle) | Список включенных по умолчанию модулей |
| :--- | :--- |
| **Default** |
| admission-policy-engine
| cert-manager
| chrony
| containerized-data-importer
| control-plane-manager
| dashboard
| external-module-manager
| deckhouse
| documentation
| descheduler
| extended-monitoring
| flow-schema
| helm
| ingress-nginx
| kube-dns
| kube-proxy
| local-path-provisioner
| log-shipper
| monitoring-custom
| monitoring-deckhouse
| monitoring-kubernetes-control-plane
| monitoring-kubernetes
| monitoring-ping
| namespace-configurator
| node-manager
| pod-reloader
| priority-class
| prometheus
| prometheus-metrics-adapter
| secret-copier
| smoke-mini
| snapshot-controller
| terraform-manager
| upmeter
| user-authn
| user-authz
| vertical-pod-autoscaler
| node-local-dns
| flant-integration |
| **Managed** |
| admission-policy-engine
| cert-manager
| containerized-data-importer
| dashboard
| external-module-manager
| deckhouse
| documentation
| descheduler
| extended-monitoring
| flow-schema
| helm
| ingress-nginx
| local-path-provisioner
| log-shipper
| monitoring-custom
| monitoring-deckhouse
| monitoring-kubernetes
| monitoring-ping
| namespace-configurator
| pod-reloader
| prometheus
| prometheus-metrics-adapter
| secret-copier
| snapshot-controller
| upmeter
| user-authz
| vertical-pod-autoscaler
| flant-integration |
| **Minimal** |
| deckhouse |

> **Обратите внимание,** что в наборе модулей `Minimal` не включен ряд базовых модулей (например, модуль работы с CNI). Deckhouse с набором модулей `Minimal` без включения базовых модулей сможет работать только в уже развернутом кластере.

### Выделение узлов под определенный вид нагрузки

Если в параметрах модуля не указаны явные значения `nodeSelector/tolerations`, то для всех модулей используется следующая стратегия:

1. Если параметр `nodeSelector` модуля не указан, то Deckhouse попытается вычислить `nodeSelector` автоматически. В этом случае, если в кластере присутствуют узлы с[лейблами из списка или лейблами определенного формата](#особенности-автоматики-зависящие-от-типа-модуля), Deckhouse укажет их в качестве `nodeSelector` ресурсам модуля.
2. Если параметр `tolerations` модуля не указан, то Pod'ам модуля автоматически устанавливаются все возможные toleration'ы ([подробнее](#особенности-автоматики-зависящие-от-типа-модуля)).
3. Отключить автоматическое вычисление параметров `nodeSelector` или `tolerations` можно, указав значение `false`.

Возможность настройки `nodeSelector` и `tolerations` отключена для модулей:

* которые работают на всех узлах кластера (например, `cni-flannel`, `monitoring-ping`);
* которые работают на всех master-узлах (например, `prometheus-metrics-adapter`, `vertical-pod-autoscaler`).

### Особенности автоматики, зависящие от типа модуля

* Модули _monitoring_ (`operator-prometheus`, `prometheus` и `vertical-pod-autoscaler`):
  - Порядок поиска узлов (для определения[nodeSelector](modules/300-prometheus/configuration.html#parameters-nodeselector)):
    1. Наличие узла с лейблом `node-role.deckhouse.io/MODULE_NAME`.
    2. Наличие узла с лейблом `node-role.deckhouse.io/monitoring`.
    3. Наличие узла с лейблом `node-role.deckhouse.io/system`.
  - Добавляемые toleration'ы (добавляются одновременно все):
    + `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}` (например, `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"operator-prometheus"}`);
    + `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"monitoring"}`;
    + `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}`.
* Модули _frontend_ (исключительно модуль `ingress-nginx`):
  - Порядок поиска узлов (для определения `nodeSelector`):
    1. Наличие узла с лейблом `node-role.deckhouse.io/MODULE_NAME`.
    2. Наличие узла с лейблом `node-role.deckhouse.io/frontend`.
  - Добавляемые toleration'ы (добавляются одновременно все):
    + `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}`;
    + `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"frontend"}`.
* Все остальные модули:
  - Порядок поиска узлов (для определения `nodeSelector`):
    1. Наличие узла с лейблом `node-role.deckhouse.io/MODULE_NAME` (например, `node-role.deckhouse.io/cert-manager`).
    2. Наличие узла с лейблом `node-role.deckhouse.io/system`.
  - Добавляемые toleration'ы (добавляются одновременно все):
    + `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}` (например, `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"network-gateway"}`);
    + `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}`.


### Настройка CI/CD-системы

**Создание ServiceAccount для сервера и предоставление ему доступа**

Создание ServiceAccount с доступом к Kubernetes API может потребоваться, например, при настройке развертывания приложений через CI-системы.  

1. Создайте *ServiceAccount*, например в namespace `d8-service-accounts`:

   ```shell
   kubectl create -f - <<EOF
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: gitlab-runner-deploy
     namespace: d8-service-accounts
   ---
   apiVersion: v1
   kind: Secret
   metadata:
     name: gitlab-runner-deploy-token
     namespace: d8-service-accounts
     annotations:
       kubernetes.io/service-account.name: gitlab-runner-deploy
   type: kubernetes.io/service-account-token
   EOF
   ```

1. Дайте необходимые *ServiceAccount* права (используя [кастомные ресурсы ClusterAuthorizationRule](cr.html#clusterauthorizationrule)):

   ```shell
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: ClusterAuthorizationRule
   metadata:
     name: gitlab-runner-deploy
   spec:
     subjects:
     - kind: ServiceAccount
       name: gitlab-runner-deploy
       namespace: d8-service-accounts
     accessLevel: SuperAdmin
     # Опция доступна только при включенном режиме enableMultiTenancy (версия Enterprise Edition).
     allowAccessToSystemNamespaces: true      
   EOF
   ```

   Если в конфигурации Deckhouse включен режим мультитенантности (параметр [enableMultiTenancy](configuration.html#parameters-enablemultitenancy), доступен только в Enterprise Edition), настройте доступные для ServiceAccount пространства имен (параметр [namespaceSelector](cr.html#clusterauthorizationrule-v1-spec-namespaceselector)).

1. Определите значения переменных (они будут использоваться далее), выполнив следующие команды (**подставьте свои значения**):

   ```shell
   export CLUSTER_NAME=my-cluster
   export USER_NAME=gitlab-runner-deploy.my-cluster
   export CONTEXT_NAME=${CLUSTER_NAME}-${USER_NAME}
   export FILE_NAME=kube.config
   ```

1. Сгенерируйте секцию `cluster` в файле конфигурации kubectl:

   Используйте один из следующих вариантов доступа к API-серверу кластера:

   * Если есть прямой доступ до API-сервера:
     1. Получите сертификат CA кластера Kubernetes:

        ```shell
        kubectl get cm kube-root-ca.crt -o jsonpath='{ .data.ca\.crt }' > /tmp/ca.crt
        ```

     1. Сгенерируйте секцию `cluster` (используется IP-адрес API-сервера для доступа):

        ```shell
        kubectl config set-cluster $CLUSTER_NAME --embed-certs=true \
          --server=https://$(kubectl get ep kubernetes -o json | jq -rc '.subsets[0] | "\(.addresses[0].ip):\(.ports[0].port)"') \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   * Если прямого доступа до API-сервера нет, то используйте один следующих вариантов:
      * включите доступ к API-серверу через Ingress-контроллер (параметр [publishAPI](../150-user-authn/configuration.html#parameters-publishapi)), и укажите адреса с которых будут идти запросы (параметр [whitelistSourceRanges](../150-user-authn/configuration.html#parameters-publishapi-whitelistsourceranges));
      * укажите адреса с которых будут идти запросы в отдельном Ingress-контроллере (параметр [acceptRequestsFrom](../402-ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom)).

   * Если используется непубличный CA:

     1. Получите сертификат CA из Secret'а с сертификатом, который используется для домена `api.%s`:

        ```shell
        kubectl -n d8-user-authn get secrets -o json \
          $(kubectl -n d8-user-authn get ing kubernetes-api -o jsonpath="{.spec.tls[0].secretName}") \
          | jq -rc '.data."ca.crt" // .data."tls.crt"' \
          | base64 -d > /tmp/ca.crt
        ```

     2. Сгенерируйте секцию `cluster` (используется внешний домен и CA для доступа):

        ```shell
        kubectl config set-cluster $CLUSTER_NAME --embed-certs=true \
          --server=https://$(kubectl -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   * Если используется публичный CA. Сгенерируйте секцию `cluster` (используется внешний домен для доступа):

     ```shell
     kubectl config set-cluster $CLUSTER_NAME \
       --server=https://$(kubectl -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
       --kubeconfig=$FILE_NAME
     ```

1. Сгенерируйте секцию `user` с токеном из Secret'а ServiceAccount в файле конфигурации kubectl:

   ```shell
   kubectl config set-credentials $USER_NAME \
     --token=$(kubectl -n d8-service-accounts get secret gitlab-runner-deploy-token -o json |jq -r '.data["token"]' | base64 -d) \
     --kubeconfig=$FILE_NAME
   ```

1. Сгенерируйте контекст в файле конфигурации kubectl:

   ```shell
   kubectl config set-context $CONTEXT_NAME \
     --cluster=$CLUSTER_NAME --user=$USER_NAME \
     --kubeconfig=$FILE_NAME
   ```

1. Установите сгенерированный контекст как используемый по умолчанию в файле конфигурации kubectl:

   ```shell
   kubectl config use-context $CONTEXT_NAME --kubeconfig=$FILE_NAME
   ```

### Направление трафика на приложение 

Создайте Service и [Ingress](https://deckhouse.ru/documentation/v1/modules/402-ingress-nginx/) для вашего приложения.


### Мониторинг приложения

Добавьте аннотации prometheus.deckhouse.io/custom-target: "my-app" и prometheus.deckhouse.io/port: "80" к созданному Service’у.
Настройте [monitoring-custom](https://deckhouse.ru/documentation/v1/modules/340-monitoring-custom/)



