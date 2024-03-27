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
* 100 ГБ дискового пространства.

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
* Диапазон, из которого анонсировать адреса.

### Хранилища данных и список необходимых ресурсов

В кластере может одновременно использоваться различное количество типов хранилищ. В минимальной конфигурации потребуются:

* Datastore, в котором Kubernetes-кластер будет заказывать способы хранения *PersistentVolume*;
* Datastore, в котором будут заказываться root-диски для виртуальной машины (это может быть тот же Datastore, что и для *PersistentVolume*).

Список необходимых ресурсов выбирается в заивисмотси от [провайдера](ссылка).

### Подготовка образа виртуальной машины

Для создания шаблона виртуальной машины (`Template`) рекомендуется использовать готовый cloud-образ/OVA-файл, предоставляемый вендором ОС:

* [**Ubuntu**](https://cloud-images.ubuntu.com/)
* [**Debian**](https://cloud.debian.org/images/cloud/)
* [**CentOS**](https://cloud.centos.org/)
* [**Rocky Linux**](https://rockylinux.org/alternative-images/) (секция _Generic Cloud / OpenStack_)

Если необходимо использовать собственный образ, обратитесь к [документации](/documentation/v1/modules/030-cloud-provider-vsphere/environment.html#требования-к-образу-виртуальной-машины).

### Развертывание кластера

1. Выберите [инсфраструктуру](https://deckhouse.ru/gs/).

Для развертывания будет использован минимальный набор:

* Кластер из одного мастер-узла и одного рабочего узла.
* Управляющие компоненты Kubernetes-кластера и Deckhouse-контроллер, работающие на мастер-узле.
* Deckhouse с не системными компонентами (Ingress-контроллер, Prometheus, cert-manager и т.д.) на рабочем узле. Поэтому приложения должны работать на рабочем узле.

2. Создайте роль с необходимыми [правами](ссылка на раздел RBAC)

### Выбор способа конфигурации

## Подключение к мастер-узлу

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

4. Чтобы получить доступ к веб-интерфейсам компонентов Deckhouse настройте работу DNS и укажите в параметрах Deckhouse шаблон DNS-имен. Шаблон DNS-имен используется для настройки Ingress-ресурсов системных приложений. Например, за интерфейсом Grafana закреплено имя `grafana`. Поэтому для шаблона `%s.kube.company.my Grafana` будет доступен шаблон DNS-имен по адресу `grafana.kube.company.my`.

4. Настройте DNS для сервисов Deckhouse одним из следующих способов:

Если у вас есть возможность добавить DNS-запись используя DNS-сервер:
Если ваш шаблон DNS-имен кластера является wildcard DNS-шаблоном (например, %s.kube.company.my), то добавьте соответствующую wildcard A-запись со значением IP-адреса master-узла.
Если ваш шаблон DNS-имен кластера НЕ является wildcard DNS-шаблоном (например, %s-kube.company.my), то добавьте А или CNAME-записис адресом master-узла, для следующих DNS-имен сервисов согласно шаблону DNS-имен:
api
argocd
cdi-uploadproxy
dashboard
documentation
dex
grafana
hubble
istio
istio-api-proxy
kubeconfig
openvpn-admin
prometheus
status
upmeter
Если вы не имеете под управлением DNS-сервер, то на компьютере, с которого необходим доступ к сервисам Deckhouse, добавьте статические записи в файл /etc/hosts (%SystemRoot%\system32\drivers\etc\hosts для Windows).

Для добавления записей в файл /etc/hosts на на Linux-компьютере с которого необходим доступ к сервисам Deckhouse (далее — ПК), выполните следующие шаги:

[Выполните на ПК] Укажите используемый шаблон DNS-имен в переменной DOMAIN_TEMPLATE (например, %s.kube.company.my):

DOMAIN_TEMPLATE='<DOMAIN_TEMPLATE>'
[Выполните на ПК] Укажите IP-адрес балансировщика в переменной BALANCER_IP:

BALANCER_IP='<BALANCER_IP>'
[Выполните на ПК] Добавьте записи в файл /etc/hosts:

for i in api argocd cdi-uploadproxy dashboard documentation dex grafana hubble istio istio-api-proxy kubeconfig openvpn-admin prometheus status upmeter; do echo "${BALANCER_IP}  ${DOMAIN_TEMPLATE} "| sed "s/%s/$i/"; done  | sudo bash -c "cat >>/etc/hosts"
Затем, на master-узле выполните следующую команду (укажите используемый шаблон DNS-имен в переменной DOMAIN_TEMPLATE):

DOMAIN_TEMPLATE='<DOMAIN_TEMPLATE>'
sudo /opt/deckhouse/bin/kubectl patch mc global --type merge -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"${DOMAIN_TEMPLATE}\"}}}}"

## Настройте удаленный доступ к кластеру
На персональном компьютере выполните следующие шаги, для того чтобы настроить подключение kubectl к кластеру:

Откройте веб-интерфейс сервиса Kubeconfig Generator. Для него зарезервировано имя kubeconfig, и адрес для доступа формируется согласно шаблона DNS-имен (который вы установили ранее). Например, для шаблона DNS-имен %s.1.2.3.4.sslip.io, веб-интерфейс Kubeconfig Generator будет доступен по адресу https://kubeconfig.1.2.3.4.sslip.io.
Авторизуйтесь под пользователем admin@deckhouse.io. Пароль пользователя, сгенерированный на предыдущем шаге, — rjm1pcgttf (вы также можете найти его в CustomResource User в файле resource.yml).
Выберите вкладку с ОС персонального компьютера.
Последовательно скопируйте и выполните команды, приведенные на странице.
Проверьте корректную работу kubectl (например, выполнив команду kubectl get no).


Инструкция по установке [Grafana](https://grafana.com/docs/grafana/latest/setup-grafana/installation/)


* [Каналы обновлений](/ru/platform/deckhouse-release-channels.html)

[En](/en/)

1. Документация

1. Платформа
2. v1
3. Как настроить?

[Платформа](#)

* [Модули](/ru/modules/)

* [Введение в документацию](./deckhouse-overview.html)
* [Deckhouse](#)
  - [Как установить?](#)
    + [Описание](./installing/)
    + [Настройки](./installing/configuration.html)
  - [Как настроить?](#)
    + [Описание](./)
    + [Глобальные настройки](./deckhouse-configure-global.html)
    + [Custom Resources](./cr.html)
  - [Каналы обновлений](./deckhouse-release-channels.html)
  - [Поддерживаемые версии K8s и ОС](./supported_versions.html)
  - [Сравнение редакций](./revision-comparison.html)
  - [Настройка ПО безопасности](./security_software_setup.html)
  - [FAQ](./deckhouse-faq.html)
  - [Модули](#)
    + [deckhouse](#)
      * [Описание](./modules/002-deckhouse/)
      * [Примеры](./modules/002-deckhouse/usage.html)
      * [Справка](#)
        - [Настройки](./modules/002-deckhouse/configuration.html)
        - [Custom Resources](./modules/002-deckhouse/cr.html)
      * [FAQ](./modules/002-deckhouse/faq.html)
    + [documentation](#)
      * [Описание](./modules/810-documentation/)
      * [Примеры](./modules/810-documentation/examples.html)
      * [Настройки](./modules/810-documentation/configuration.html)
* [Кластер Kubernetes](#)
  - [Описание](./kubernetes.html)
  - [Cloud providers](#)
    + [Amazon Web Services](#)
      * [Описание](./modules/030-cloud-provider-aws/)
      * [Установка](#)
        - [Подготовка окружения](./modules/030-cloud-provider-aws/environment.html)
        - [Схемы размещения](./modules/030-cloud-provider-aws/layouts.html)
        - [Настройка провайдера](./modules/030-cloud-provider-aws/cluster_configuration.html)
      * [Справка](#)
        - [Настройки](./modules/030-cloud-provider-aws/configuration.html)
        - [Custom Resources](./modules/030-cloud-provider-aws/cr.html)
      * [Примеры](./modules/030-cloud-provider-aws/examples.html)
      * [FAQ](./modules/030-cloud-provider-aws/faq.html)
    + [Google Cloud Platform](#)
      * [Описание](./modules/030-cloud-provider-gcp/)
      * [Установка](#)
        - [Подготовка окружения](./modules/030-cloud-provider-gcp/environment.html)
        - [Схемы размещения](./modules/030-cloud-provider-gcp/layouts.html)
        - [Настройка провайдера](./modules/030-cloud-provider-gcp/cluster_configuration.html)
      * [Справка](#)
        - [Настройки](./modules/030-cloud-provider-gcp/configuration.html)
        - [Custom Resources](./modules/030-cloud-provider-gcp/cr.html)
      * [Примеры](./modules/030-cloud-provider-gcp/examples.html)
      * [FAQ](./modules/030-cloud-provider-gcp/faq.html)
    + [Microsoft Azure](#)
      * [Описание](./modules/030-cloud-provider-azure/)
      * [Установка](#)
        - [Подготовка окружения](./modules/030-cloud-provider-azure/environment.html)
        - [Схемы размещения](./modules/030-cloud-provider-azure/layouts.html)
        - [Настройка провайдера](./modules/030-cloud-provider-azure/cluster_configuration.html)
      * [Справка](#)
        - [Настройки](./modules/030-cloud-provider-azure/configuration.html)
        - [Custom Resources](./modules/030-cloud-provider-azure/cr.html)
      * [Примеры](./modules/030-cloud-provider-azure/examples.html)
    + [OpenStack EE Only](#)
      * [Описание](./modules/030-cloud-provider-openstack/)
      * [Установка](#)
        - [Подготовка окружения](./modules/030-cloud-provider-openstack/environment.html)
        - [Схемы размещения](./modules/030-cloud-provider-openstack/layouts.html)
        - [Настройка провайдера](./modules/030-cloud-provider-openstack/cluster_configuration.html)
      * [Справка](#)
        - [Настройки](./modules/030-cloud-provider-openstack/configuration.html)
        - [Custom Resources](./modules/030-cloud-provider-openstack/cr.html)
      * [Примеры](./modules/030-cloud-provider-openstack/examples.html)
      * [FAQ](./modules/030-cloud-provider-openstack/faq.html)
    + [VMware vSphere EE Only](#)
      * [Описание](./modules/030-cloud-provider-vsphere/)
      * [Установка](#)
        - [Подготовка окружения](./modules/030-cloud-provider-vsphere/environment.html)
        - [Схемы размещения](./modules/030-cloud-provider-vsphere/layouts.html)
        - [Настройка провайдера](./modules/030-cloud-provider-vsphere/cluster_configuration.html)
      * [Справка](#)
        - [Настройки](./modules/030-cloud-provider-vsphere/configuration.html)
        - [Custom Resources](./modules/030-cloud-provider-vsphere/cr.html)
      * [Примеры](./modules/030-cloud-provider-vsphere/examples.html)
      * [FAQ](./modules/030-cloud-provider-vsphere/faq.html)
    + [VMware Cloud Director  Experimental EE Only](#)
      * [Описание](./modules/030-cloud-provider-vcd/)
      * [Установка](#)
        - [Подготовка окружения](./modules/030-cloud-provider-vcd/environment.html)
        - [Схемы размещения](./modules/030-cloud-provider-vcd/layouts.html)
        - [Настройка провайдера](./modules/030-cloud-provider-vcd/cluster_configuration.html)
      * [Справка](#)
        - [Настройки](./modules/030-cloud-provider-vcd/configuration.html)
        - [Custom Resources](./modules/030-cloud-provider-vcd/cr.html)
      * [Примеры](./modules/030-cloud-provider-vcd/examples.html)
      * [FAQ](./modules/030-cloud-provider-vcd/faq.html)
    + [Yandex Cloud](#)
      * [Описание](./modules/030-cloud-provider-yandex/)
      * [Установка](#)
        - [Подготовка окружения](./modules/030-cloud-provider-yandex/environment.html)
        - [Схемы размещения](./modules/030-cloud-provider-yandex/layouts.html)
        - [Настройка провайдера](./modules/030-cloud-provider-yandex/cluster_configuration.html)
      * [Справка](#)
        - [Настройки](./modules/030-cloud-provider-yandex/configuration.html)
        - [Custom Resources](./modules/030-cloud-provider-yandex/cr.html)
      * [Примеры](./modules/030-cloud-provider-yandex/examples.html)
      * [FAQ](./modules/030-cloud-provider-yandex/faq.html)
  - [Управление control plane](#)
    + [Описание](./modules/040-control-plane-manager/)
    + [Примеры](./modules/040-control-plane-manager/examples.html)
    + [Настройки](./modules/040-control-plane-manager/configuration.html)
    + [FAQ](./modules/040-control-plane-manager/faq.html)
  - [Управление узлами](#)
    + [Описание](./modules/040-node-manager/)
    + [Примеры](./modules/040-node-manager/examples.html)
    + [Справка](#)
      * [Настройки](./modules/040-node-manager/configuration.html)
      * [Custom Resources](./modules/040-node-manager/cr.html)
    + [FAQ](./modules/040-node-manager/faq.html)
  - [Другие модули](#)
    + [cni-cilium](#)
      * [Описание](./modules/021-cni-cilium/)
      * [Настройки](./modules/021-cni-cilium/configuration.html)
    + [cilium-hubble](#)
      * [Описание](./modules/500-cilium-hubble/)
      * [Настройки](./modules/500-cilium-hubble/configuration.html)
    + [cni-flannel](#)
      * [Описание](./modules/035-cni-flannel/)
      * [Настройки](./modules/035-cni-flannel/configuration.html)
    + [cni-simple-bridge](#)
      * [Описание](./modules/035-cni-simple-bridge/)
    + [kube-proxy](#)
      * [Описание](./modules/021-kube-proxy/)
    + [kube-dns](#)
      * [Описание](./modules/042-kube-dns/)
      * [Настройки](./modules/042-kube-dns/configuration.html)
      * [Примеры](./modules/042-kube-dns/examples.html)
      * [FAQ](./modules/042-kube-dns/faq.html)
    + [node-local-dns EE Only](#)
      * [Описание](./modules/350-node-local-dns/)
      * [Примеры](./modules/350-node-local-dns/examples.html)
      * [Настройки](./modules/350-node-local-dns/configuration.html)
    + [terraform-manager](#)
      * [Описание](./modules/040-terraform-manager/)
      * [Настройки](./modules/040-terraform-manager/configuration.html)
* [Доступ к кластеру](#)
  - [dashboard](#)
    + [Описание](./modules/500-dashboard/)
    + [Примеры](./modules/500-dashboard/examples.html)
    + [Настройки](./modules/500-dashboard/configuration.html)
  - [openvpn  Experimental](#)
    + [Описание](./modules/500-openvpn/)
    + [Примеры](./modules/500-openvpn/examples.html)
    + [Настройки](./modules/500-openvpn/configuration.html)
* [Доставка Experimental EE Only](#)
  - [Описание](./modules/502-delivery/)
  - [Примеры](./modules/502-delivery/usage.html)
  - [Справка](#)
    + [Настройки](./modules/502-delivery/configuration.html)
    + [Custom Resources](./modules/502-delivery/cr.html)
* [Балансировка трафика](#)
  - [ingress-nginx](#)
    + [Описание](./modules/402-ingress-nginx/)
    + [Примеры](./modules/402-ingress-nginx/examples.html)
    + [Справка](#)
      * [Настройки](./modules/402-ingress-nginx/configuration.html)
      * [Custom Resources](./modules/402-ingress-nginx/cr.html)
    + [FAQ](./modules/402-ingress-nginx/faq.html)
  - [istio](#)
    + [Описание](./modules/110-istio/)
    + [Справка](#)
      * [Настройки](./modules/110-istio/configuration.html)
      * [Custom Resources](./modules/110-istio/cr.html)
      * [Custom Resources (от istio.io)](./modules/110-istio/istio-cr.html)
    + [Примеры](./modules/110-istio/examples.html)
* [Мониторинг](#)
  - [Prometheus & Grafana](#)
    + [Описание](./modules/300-prometheus/)
    + [Примеры](./modules/300-prometheus/usage.html)
    + [Справка](#)
      * [Настройки](./modules/300-prometheus/configuration.html)
      * [Custom Resources](./modules/300-prometheus/cr.html)
    + [FAQ](./modules/300-prometheus/faq.html)
  - [Мониторинг вашего приложения](#)
    + [Описание](./modules/340-monitoring-custom/)
    + [Настройки](./modules/340-monitoring-custom/configuration.html)
  - [Мониторинг сети](#)
    + [Описание](./modules/340-monitoring-ping/)
    + [Настройки](./modules/340-monitoring-ping/configuration.html)
    + [Примеры](./modules/340-monitoring-ping/usage.html)
  - [Мониторинг кластера](#)
    + [Описание](./modules/340-monitoring-kubernetes/)
    + [Настройки](./modules/340-monitoring-kubernetes/configuration.html)
  - [Мониторинг control plane](#)
    + [Описание](./modules/340-monitoring-kubernetes-control-plane/)
    + [Настройки](./modules/340-monitoring-kubernetes-control-plane/configuration.html)
  - [Расширенный мониторинг](#)
    + [Описание](./modules/340-extended-monitoring/)
    + [Настройки](./modules/340-extended-monitoring/configuration.html)
  - [Логи](#)
    + [Доставка логов](#)
      * [Описание](./modules/460-log-shipper/)
      * [Примеры](./modules/460-log-shipper/examples.html)
      * [Расширенная конфигурация](./modules/460-log-shipper/advanced_usage.html)
      * [Справка](#)
        - [Настройки](./modules/460-log-shipper/configuration.html)
        - [Custom Resources](./modules/460-log-shipper/cr.html)
    + [Кратковременное хранение Experimental](#)
      * [Описание](./modules/462-loki/)
      * [Примеры](./modules/462-loki/examples.html)
      * [Настройки](./modules/462-loki/configuration.html)
  - [Другие модули](#)
    + [flant-integration EE Only](#)
      * [Описание](./modules/600-flant-integration/)
      * [Настройки](./modules/600-flant-integration/configuration.html)
    + [okmeter  Proprietary](#)
      * [Описание](./modules/500-okmeter/)
      * [Примеры](./modules/500-okmeter/examples.html)
      * [Настройки](./modules/500-okmeter/configuration.html)
    + [prometheus-pushgateway](#)
      * [Описание](./modules/303-prometheus-pushgateway/)
      * [Примеры](./modules/303-prometheus-pushgateway/examples.html)
      * [Настройки](./modules/303-prometheus-pushgateway/configuration.html)
    + [upmeter](#)
      * [Описание](./modules/500-upmeter/)
      * [Примеры](./modules/500-upmeter/examples.html)
      * [Настройки](#)
        - [Настройки](./modules/500-upmeter/configuration.html)
        - [Custom Resources](./modules/500-upmeter/cr.html)
* [Масштабирование и управление ресурсами](#)
  - [Priority classes](#)
    + [Описание](./modules/001-priority-class/)
    + [Настройки](./modules/001-priority-class/configuration.html)
  - [Масштабирование по метрикам](#)
    + [Описание](./modules/301-prometheus-metrics-adapter/)
    + [Примеры](./modules/301-prometheus-metrics-adapter/usage.html)
    + [Справка](#)
      * [Настройки](./modules/301-prometheus-metrics-adapter/configuration.html)
      * [Custom Resources](./modules/301-prometheus-metrics-adapter/cr.html)
  - [Вертикальное масштабирование](#)
    + [Описание](./modules/302-vertical-pod-autoscaler/)
    + [Примеры](./modules/302-vertical-pod-autoscaler/examples.html)
    + [Справка](#)
      * [Настройки](./modules/302-vertical-pod-autoscaler/configuration.html)
      * [Custom Resources](./modules/302-vertical-pod-autoscaler/cr.html)
    + [FAQ](./modules/302-vertical-pod-autoscaler/faq.html)
  - [Другие модули](#)
    + [descheduler](#)
      * [Описание](./modules/400-descheduler/)
      * [Примеры](./modules/400-descheduler/examples.html)
      * [Справка](#)
        - [Настройки](./modules/400-descheduler/configuration.html)
        - [Custom Resources](./modules/400-descheduler/cr.html)
    + [flow-schema](#)
      * [Описание](./modules/011-flow-schema/)
      * [Настройки](./modules/011-flow-schema/configuration.html)
      * [FAQ](./modules/011-flow-schema/faq.html)
    + [pod-reloader](#)
      * [Описание](./modules/465-pod-reloader/)
      * [Примеры](./modules/465-pod-reloader/examples.html)
      * [Настройки](./modules/465-pod-reloader/configuration.html)
* [Безопасность](#)
  - [User authentication](#)
    + [Описание](./modules/150-user-authn/)
    + [Примеры](./modules/150-user-authn/usage.html)
    + [Справка](#)
      * [Настройки](./modules/150-user-authn/configuration.html)
      * [Custom Resources](./modules/150-user-authn/cr.html)
    + [FAQ](./modules/150-user-authn/faq.html)
  - [User authorization](#)
    + [Описание](./modules/140-user-authz/)
    + [Примеры](./modules/140-user-authz/usage.html)
    + [Справка](#)
      * [Настройки](./modules/140-user-authz/configuration.html)
      * [Custom Resources](./modules/140-user-authz/cr.html)
    + [FAQ](./modules/140-user-authz/faq.html)
  - [Multitenancy  Experimental EE Only](#)
    + [Описание](./modules/160-multitenancy-manager/)
    + [Примеры](./modules/160-multitenancy-manager/usage.html)
    + [Справка](#)
      * [Настройки](./modules/160-multitenancy-manager/configuration.html)
      * [Custom Resources](./modules/160-multitenancy-manager/cr.html)
  - [Политики безопасности](#)
    + [Описание](./modules/015-admission-policy-engine/)
    + [Справка](#)
      * [Настройки](./modules/015-admission-policy-engine/configuration.html)
      * [Custom Resources](./modules/015-admission-policy-engine/cr.html)
    + [FAQ](./modules/015-admission-policy-engine/faq.html)
  - [Аудит Experimental EE Only](#)
    + [Описание](./modules/650-runtime-audit-engine/)
    + [Справка](#)
      * [Настройки](./modules/650-runtime-audit-engine/configuration.html)
      * [Custom Resources](./modules/650-runtime-audit-engine/cr.html)
    + [Расширенная конфигурация](./modules/650-runtime-audit-engine/advanced_usage.html)
    + [Примеры](./modules/650-runtime-audit-engine/examples.html)
    + [FAQ](./modules/650-runtime-audit-engine/faq.html)
  - [Другие модули](#)
    + [network-policy-engine](#)
      * [Описание](./modules/050-network-policy-engine/)
      * [Примеры](./modules/050-network-policy-engine/examples.html)
      * [Настройки](./modules/050-network-policy-engine/configuration.html)
    + [cert-manager](#)
      * [Описание](./modules/101-cert-manager/)
      * [Примеры](./modules/101-cert-manager/usage.html)
      * [Справка](#)
        - [Настройки](./modules/101-cert-manager/configuration.html)
        - [Custom Resources](./modules/101-cert-manager/cr.html)
      * [FAQ](./modules/101-cert-manager/faq.html)
    + [operator-trivy  Experimental EE Only](#)
      * [Описание](./modules/500-operator-trivy/)
      * [Настройки](./modules/500-operator-trivy/configuration.html)
      * [FAQ](./modules/500-operator-trivy/faq.html)
* [Хранилище](#)
  - [ceph-csi](#)
    + [Описание](./modules/031-ceph-csi/)
    + [Примеры](./modules/031-ceph-csi/examples.html)
    + [Справка](#)
      * [Настройки](./modules/031-ceph-csi/configuration.html)
      * [Custom Resources](./modules/031-ceph-csi/cr.html)
    + [FAQ](./modules/031-ceph-csi/faq.html)
  - [local-path-provisioner](#)
    + [Описание](./modules/031-local-path-provisioner/)
    + [Примеры](./modules/031-local-path-provisioner/examples.html)
    + [Справка](#)
      * [Настройки](./modules/031-local-path-provisioner/configuration.html)
      * [Custom Resources](./modules/031-local-path-provisioner/cr.html)
    + [FAQ](./modules/031-local-path-provisioner/faq.html)
  - [snapshot-controller](#)
    + [Описание](./modules/045-snapshot-controller/)
    + [Примеры](./modules/045-snapshot-controller/usage.html)
    + [Настройки](./modules/045-snapshot-controller/configuration.html)
* [Приятные мелочи](#)
  - [chrony](#)
    + [Описание](./modules/470-chrony/)
    + [Настройки](./modules/470-chrony/configuration.html)
  - [namespace-configurator](#)
    + [Описание](./modules/600-namespace-configurator/)
    + [Настройки](./modules/600-namespace-configurator/configuration.html)
    + [Примеры](./modules/600-namespace-configurator/examples.html)
  - [secret-copier](#)
    + [Описание](./modules/600-secret-copier/)
* [Bare Metal](#)
  - [keepalived EE Only](#)
    + [Описание](./modules/450-keepalived/)
    + [Примеры](./modules/450-keepalived/examples.html)
    + [Справка](#)
      * [Настройки](./modules/450-keepalived/configuration.html)
      * [Custom Resources](./modules/450-keepalived/cr.html)
    + [FAQ](./modules/450-keepalived/faq.html)
  - [metallb EE Only](#)
    + [Описание](./modules/380-metallb/)
    + [Примеры](./modules/380-metallb/examples.html)
    + [Настройки](./modules/380-metallb/configuration.html)
  - [network-gateway EE Only](#)
    + [Описание](./modules/450-network-gateway/)
    + [Настройки](./modules/450-network-gateway/configuration.html)
* [Используемое ПО](./oss_info.html)

# Как настроить?

 Вы просматриваете документацию еще не вышедшей версии Deckhouse. Вы можете выбрать необходимый канал обновлений в меню версий или [перейти к последней стабильной версии Deckhouse](/documentation/v1/).

Deckhouse состоит из оператора Deckhouse и модулей. Модуль — это набор из Helm-чарта, хуков [Addon-operator'а](https://github.com/flant/addon-operator/), правил сборки компонентов модуля (компонентов Deckhouse) и других файлов.

Deckhouse настраивается с помощью:

* **[Глобальных настроек](deckhouse-configure-global.html).** Глобальные настройки хранятся в custom resource `ModuleConfig/global`. Глобальные настройки можно рассматривать как специальный модуль `global`, который нельзя отключить.
* **[Настроек модулей](#настройка-модуля).** Настройки каждого модуля хранятся в custom resource `ModuleConfig`, имя которого совпадает с именем модуля (в kebab-case).
* **Custom resource'ов.** Некоторые модули настраиваются с помощью дополнительных custom resource'ов.

Пример набора custom resource'ов конфигурации Deckhouse:

```
<span class="c1"># &#x413;&#x43B;&#x43E;&#x431;&#x430;&#x43B;&#x44C;&#x43D;&#x44B;&#x435; &#x43D;&#x430;&#x441;&#x442;&#x440;&#x43E;&#x439;&#x43A;&#x438;.</span>
<span class="na">apiVersion</span><span class="pi">:</span> <span class="s">deckhouse.io/v1alpha1</span>
<span class="na">kind</span><span class="pi">:</span> <span class="s">ModuleConfig</span>
<span class="na">metadata</span><span class="pi">:</span>
  <span class="na">name</span><span class="pi">:</span> <span class="s">global</span>
<span class="na">spec</span><span class="pi">:</span>
  <span class="na">version</span><span class="pi">:</span> <span class="m">1</span>
  <span class="na">settings</span><span class="pi">:</span>
    <span class="na">modules</span><span class="pi">:</span>
      <span class="na">publicDomainTemplate</span><span class="pi">:</span> <span class="s2">"</span><span class="s">%s.kube.company.my"</span>
<span class="nn">---</span>
<span class="c1"># &#x41D;&#x430;&#x441;&#x442;&#x440;&#x43E;&#x439;&#x43A;&#x438; &#x43C;&#x43E;&#x434;&#x443;&#x43B;&#x44F; monitoring-ping.</span>
<span class="na">apiVersion</span><span class="pi">:</span> <span class="s">deckhouse.io/v1alpha1</span>
<span class="na">kind</span><span class="pi">:</span> <span class="s">ModuleConfig</span>
<span class="na">metadata</span><span class="pi">:</span>
  <span class="na">name</span><span class="pi">:</span> <span class="s">monitoring-ping</span>
<span class="na">spec</span><span class="pi">:</span>
  <span class="na">version</span><span class="pi">:</span> <span class="m">1</span>
  <span class="na">settings</span><span class="pi">:</span>
    <span class="na">externalTargets</span><span class="pi">:</span>
    <span class="pi">-</span> <span class="na">host</span><span class="pi">:</span> <span class="s">8.8.8.8</span>
<span class="nn">---</span>
<span class="c1"># &#x41E;&#x442;&#x43A;&#x43B;&#x44E;&#x447;&#x438;&#x442;&#x44C; &#x43C;&#x43E;&#x434;&#x443;&#x43B;&#x44C; dashboard.</span>
<span class="na">apiVersion</span><span class="pi">:</span> <span class="s">deckhouse.io/v1alpha1</span>
<span class="na">kind</span><span class="pi">:</span> <span class="s">ModuleConfig</span>
<span class="na">metadata</span><span class="pi">:</span>
  <span class="na">name</span><span class="pi">:</span> <span class="s">dashboard</span>
<span class="na">spec</span><span class="pi">:</span>
  <span class="na">enabled</span><span class="pi">:</span> <span class="no">false</span>
```

Посмотреть список custom resource'ов `ModuleConfig`, состояние модуля (включен/выключен) и его статус можно с помощью команды `kubectl get moduleconfigs`:

```
<span class="nv">$ </span>kubectl get moduleconfigs
NAME                STATE      VERSION    STATUS    AGE
deckhouse           Enabled    1                    12h
documentation       Enabled    2                    12h
global              Enabled    1                    12h
prometheus          Enabled    2                    12h
upmeter             Disabled   2                    12h
```

Чтобы изменить глобальную конфигурацию Deckhouse или конфигурацию модуля, нужно создать или отредактировать соответствующий ресурс `ModuleConfig`.

Например, чтобы отредактировать конфигурацию модуля `upmeter`, выполните следующую команду:

```
kubectl <span class="nt">-n</span> d8-system edit moduleconfig/upmeter
```

После завершения редактирования изменения применяются автоматически.

## Настройка модуля

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

|  Набор модулей (bundle) |  Список включенных по умолчанию модулей |
| --- | --- |
| **Default** |
* admission-policy-engine
* cert-manager
* chrony
* containerized-data-importer
* control-plane-manager
* dashboard
* external-module-manager
* deckhouse
* documentation
* descheduler
* extended-monitoring
* flow-schema
* helm
* ingress-nginx
* kube-dns
* kube-proxy
* local-path-provisioner
* log-shipper
* monitoring-custom
* monitoring-deckhouse
* monitoring-kubernetes-control-plane
* monitoring-kubernetes
* monitoring-ping
* namespace-configurator
* node-manager
* pod-reloader
* priority-class
* prometheus
* prometheus-metrics-adapter
* secret-copier
* smoke-mini
* snapshot-controller
* terraform-manager
* upmeter
* user-authn
* user-authz
* vertical-pod-autoscaler
* node-local-dns
* flant-integration |
| **Managed** |
* admission-policy-engine
* cert-manager
* containerized-data-importer
* dashboard
* external-module-manager
* deckhouse
* documentation
* descheduler
* extended-monitoring
* flow-schema
* helm
* ingress-nginx
* local-path-provisioner
* log-shipper
* monitoring-custom
* monitoring-deckhouse
* monitoring-kubernetes
* monitoring-ping
* namespace-configurator
* pod-reloader
* prometheus
* prometheus-metrics-adapter
* secret-copier
* snapshot-controller
* upmeter
* user-authz
* vertical-pod-autoscaler
* flant-integration |
| **Minimal** |
* deckhouse |

> **Обратите внимание,** что в наборе модулей `Minimal` не включен ряд базовых модулей (например, модуль работы с CNI). Deckhouse с набором модулей `Minimal` без включения базовых модулей сможет работать только в уже развернутом кластере.

## Управление размещением компонентов Deckhouse

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

[✕]()

### Запросить пробный доступ

Заполните форму и в течение ближайшего рабочего дня мы свяжемся с вами для уточнения деталей - будьте на связи!

### Запрос получен

Спасибо за ваш интерес к платформе Deckhouse Kubernetes Platform!<br>
В течение ближайшего рабочего дня мы свяжемся с вами для уточнения деталей запроса - будьте на связи!

### Ошибка

Что-то пошло не так

[✕]()

### Запросить обратный звонок

Оставьте ваши данные, и мы свяжемся с вами.

### Заявка отправлена

Мы свяжемся с вами в течение рабочего дня

### Возникла ошибка отправки формы

Попробуйте еще раз

[✕]()

### Запросить обучение

Заполните форму, и мы свяжемся с вами, чтобы обсудить детали.

### Запрос получен

Мы отправили инструкцию с дальнейшими шагами вам на почту.

### Ошибка

Что-то пошло не так

[✕]()

### Запросить демо

Оставьте ваши данные, и мы свяжемся с вами.

### Запрос получен

Спасибо, мы скоро свяжемся с вами.

### Ошибка

Что-то пошло не так

[✕]()

### Получите отчет о соответствии рекомендациям PCI SSC

Документ ориентирован на компании, которые используют платежные системы на базе контейнеров, и помогает защитить эти системы от актуальных угроз. Мы выяснили, насколько Deckhouse соответствует критериям PCI SSC.

### Спасибо

Файл загрузится автоматически. Если этого не произошло, нажмите кнопку <<Скачать>>.

### Ошибка

Что-то пошло не так

[✕]()

### Запросить подробности партнёрской программы

Оставьте ваши данные, и мы свяжемся с вами.

### Запрос получен

Мы свяжемся с вами в течение рабочего дня

### Ошибка

Что-то пошло не так

## Настройка CI/CD-системы

### Создание ServiceAccount для сервера и предоставление ему доступа

Создание ServiceAccount с доступом к Kubernetes API может потребоваться, например, при настройке развертывания приложений через CI-системы.  

1. Создайте ServiceAccount, например в namespace `d8-service-accounts`:

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

1. Дайте необходимые ServiceAccount права (используя custom resource [ClusterAuthorizationRule](cr.html#clusterauthorizationrule)):

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
## Направьте трафик на приложение 

Создайте Service и [Ingress](https://deckhouse.ru/documentation/v1/modules/402-ingress-nginx/) для вашего приложения.

## Мониторинг приложения 
Добавьте аннотации prometheus.deckhouse.io/custom-target: "my-app" и prometheus.deckhouse.io/port: "80" к созданному Service’у.
Настройте [monitoring-custom](https://deckhouse.ru/documentation/v1/modules/340-monitoring-custom/)


