## Версия 1.70

### Обратите внимание

- Модуль `ceph-csi` удален. Вместо него используйте `csi-ceph`. Deckhouse не будет обновляться, если в кластере включен `ceph-csi`. Шаги по миграции с модуля ceph-csi приведены [в документации модуля csi-ceph](https://deckhouse.ru/products/kubernetes-platform/modules/csi-ceph/stable/).

- Добавлена версия 1.12 `контроллера Ingress NGINX`. Версия контроллера используемая по умолчанию изменена на 1.10. Все Ingress-контроллеры, версия которых не задана явно (параметр [`controllerVersion`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-controllerversion) ресурса IngressNginxController или параметр [`defaultControllerVersion`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/modules/ingress-nginx/configuration.html#parameters-defaultcontrollerversion) модуля `ingress-nginx`), будут перезапущены.

- Удалена метрика `falco_events` (модуль `runtime-audit-engine`). Начиная с DKP 1.68, метрика `falco_events` считалась устаревшей. Используйте метрику [falcosecurity_falcosidekick_falco_events_total](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/modules/runtime-audit-engine/faq.html#%D0%BA%D0%B0%D0%BA-%D0%BE%D0%BF%D0%BE%D0%B2%D0%B5%D1%89%D0%B0%D1%82%D1%8C-%D0%BE-%D0%BA%D1%80%D0%B8%D1%82%D0%B8%D1%87%D0%B5%D1%81%D0%BA%D0%B8%D1%85-%D1%81%D0%BE%D0%B1%D1%8B%D1%82%D0%B8%D1%8F%D1%85). Дашборды и оповещения, основанные на метрике `falco_events`, могут не работать.

- Все компоненты DKP будут перезапущены в процессе обновления.

### Основные изменения

- Теперь, в [режиме обновления](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/modules/deckhouse/configuration.html#parameters-update-mode) `Auto`, обновление патч-версий (например, с `v1.70.1` на `v1.70.2`) применяется с учетом окон обновлений, если они заданы. Ранее, в этом режиме с учетом окон обновлений применялись только обновления минорных версий (например с `1.69.х` на `1.70.х`), а обновления патч-версий применялись по мере их появления на канале обновлений.

- Добавлена возможность перезагрузки узла, если на соответствующем объекте Node установлена аннотация `update.node.deckhouse.io/reboot`.

- При очистке статического узла теперь удаляются и созданные Deckhouse Kubernetes Platform локальные пользователи.

- Добавлен мониторинг состояния синхронизации Istio в мультикластерной конфигурации. Добавлен алерт мониторинга [`D8IstioRemoteClusterNotSynced`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/alerts.html#istio-d8istioremoteclusternotsynced), который появляется в следующих случаях:
  - удаленный кластер находится в автономном режиме;
  - удаленная конечная точка API недоступна;
  - токен удаленного `ServiceAccount` недействителен или истек;
  - между кластерами существует проблема с TLS или сертификатом.

- Команда `deckhouse-controller collect-debug-info` теперь собирает и [отладочную информацию](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/modules/deckhouse/faq.html#%D0%BA%D0%B0%D0%BA-%D1%81%D0%BE%D0%B1%D1%80%D0%B0%D1%82%D1%8C-%D0%B8%D0%BD%D1%84%D0%BE%D1%80%D0%BC%D0%B0%D1%86%D0%B8%D1%8E-%D0%B4%D0%BB%D1%8F-%D0%BE%D1%82%D0%BB%D0%B0%D0%B4%D0%BA%D0%B8) про `Istio:`
  - ресурсы в пространстве имен `d8-istio`;
  - CRD групп `istio.io` и `gateway.networking.k8s.io`;
  - журналы `Istio`;
  - журналы `Sidecar` одного случайно выбранного пользовательского приложения.

- Добавлен дашборд мониторинга с информацией о состоянии сертификатов OpenVPN. По истечении срока действия теперь сертификаты сервера будут перевыпущены, а сертификаты клиентов удалены. Добавлены алерты:
  - [`OpenVPNClientCertificateExpired`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnclientcertificateexpired) — о наличии просроченных клиентских сертификатов,
  - [`OpenVPNServerCACertificateExpired`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercacertificateexpired) — об истечении сертификата OpenVPN CA,
  - [`OpenVPNServerCACertificateExpiringSoon`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercacertificateexpiringsoon) и [`OpenVPNServerCACertificateExpiringInAWeek`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercacertificateexpiringinaweek) — о скором окончании срока действия сертификата OpenVPN CA (менее 30 дней или 7 дней до окончания срока действия сертификата соответственно),
  - [`OpenVPNServerCertificateExpired`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercertificateexpired) — появляется, если срок действия сертификата сервера OpenVPN истек.
  - [`OpenVPNServerCertificateExpiringSoon`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercertificateexpiringsoon) и [`OpenVPNServerCertificateExpiringInAWeek`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercertificateexpiringinaweek) — о скором окончании срока действия сертификата сервера OpenVPN (менее 30 дней или 7 дней до окончания срока действия сертификата соответственно).

- Переименованы и изменены дашборды мониторинга:
  - «L2LoadBalancer» переименован в «MetalLB L2». Добавлена фильтрация пулов и колонок;
  - «Metallb» переименован в «MetalLB BGP». Добавлена фильтрация пулов и колонок. Удалена панель, отвечавшая за отображение ARP-запросов;
  - «L2LoadBalancer / Pools» переименован в «MetalLB / Pools».

- Для модуля `upmeter` увеличен размер PVC, чтобы вместить данные за период хранения (13 месяцев). В некоторых случаях предыдущего размера PVC было недостаточно.

- В статусе ресурса [ModuleSource](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/cr.html#modulesource) добавлен вывод информации о версии модулей в источнике.

- В статусе ресурса [Module](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/cr.html#module) добавлен вывод информации о стадии жизненного цикла модуля. Модуль в процессе своего жизненного цикла может проходить следующие стадии: `Experimental` (экспериментальная версия), `Preview` (предварительная версия), `General Availability` (общедоступная версия) и `Deprecated` (модуль устарел). Подробнее о жизненном цикле модуля и о том, как понять насколько модуль стабилен, можно узнать [в документации](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/module-development/versioning/#%D0%B6%D0%B8%D0%B7%D0%BD%D0%B5%D0%BD%D0%BD%D1%8B%D0%B9-%D1%86%D0%B8%D0%BA%D0%BB-%D0%BC%D0%BE%D0%B4%D1%83%D0%BB%D1%8F).

- Теперь можно выбирать более сильные или современные алгоритмы шифрования (такие как `RSA-3072`, `RSA-4096` или `ECDSA-P256`)  для сертификатов control plane кластера вместо стандартного `RSA-2048`. Для выбора алгоритма используется параметр [`encryptionAlgorithm`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/installing/configuration.html#clusterconfiguration-encryptionalgorithm) ресурса ClusterConfiguration.

- Для модуля `descheduler` теперь можно настроить вытеснение подов, использующих локальное хранилище. Для этого используется параметр [`evictLocalStoragePods`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/modules/descheduler/cr.html#descheduler-v1alpha2-spec-evictlocalstoragepods) конфигурации модуля.

- Добавлена возможность управлять уровнем логирования Ingress-контроллера (параметр [`controllerLogLevel`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.70/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-controllerloglevel) ресурса  IngressNginxController). По умолчанию установлен уровень логирования `Info`. Управление уровнем логирования позволяет, например, предотвратить переполнение сборщика логов при перезапуске Ingress-контроллера.

### Безопасность

- Уровень критичности (severity) алертов, сигнализирующих о нарушении политик безопасности, повышен с 7 до 3.

- В провайдерах для `Yandex Cloud`, `Zvirt` и `Dynamix` вместо Terraform теперь используется `OpenTofu`. Это позволит приносить изменения в провайдер, например, чтобы устранять известные уязвимости (CVE).

- Исправлены CVE-уязвимости в модулях: `chrony`, `descheduler`, `dhctl`, `node-manager`, `registry-packages-proxy`, `falco`, `cni-cilium`, `vertical-pod-autoscaler`.

### Обновление версий компонентов

Обновлены следующие компоненты DKP:

- `containerd`: 1.7.27
- `runc`: 1.2.5
- `go`: 1.24.2, 1.23.8
- `golang.org/x/net`: v0.38.0
- `mcm`: v0.36.0-flant.23
- `ingress-nginx`: 1.12.1
- `terraform-provider-aws`: 5.83.1
- `Deckhouse CLI`: 0.12.1
- `etcd`: v3.5.21

## Версия 1.69

### Обратите внимание

- Добавлена поддержка Kubernetes 1.32 и прекращена поддержка Kubernetes 1.27.
  Версия Kubernetes используемая по умолчанию изменена на [1.30](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.69/supported_versions.html#kubernetes).
  В будущих релизах DKP поддержка Kubernetes 1.28 будет прекращена.

- Все компоненты DKP будут перезапущены в процессе обновления.

### Основные изменения

- Модуль `ceph-csi` считается устаревшим.
  Запланируйте переход на модуль [`csi-ceph`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.69/reference/mc/csi-ceph/).
  Подробнее о работе с Ceph читайте [в документации](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.69/storage/admin/external/ceph.html).

- Теперь можно выдавать доступ к веб-интерфейсам Deckhouse по именам пользователей с помощью поля `auth.allowedUserEmails`.
  Разграничение доступа настраивается одновременно с параметром `auth.allowedUserGroups`
  в конфигурации модулей с веб-интерфейсами: `cilium-hubble`, `dashboard`, `deckhouse-tools`, `documentation`,
  `istio`, `openvpn`, `prometheus` и `upmeter` ([пример для модуля `prometheus`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.69/modules/prometheus/configuration.html#parameters-auth-alloweduseremails)).

- Для модуля `cni-cilium` добавлен дашборд **Cilium Nodes Connectivity Status & Latency** в Grafana,
  который позволяет отслеживать проблемы с сетевой связностью между узлами.
  Дашборд отображает матрицу доступности, аналогичную выводу команды `cilium-health status`.
  Данные собираются из метрик, которые уже доступны в Prometheus.

- Для модуля `control-plane-manager` добавлен [алерт `D8KubernetesStaleTokensDetected`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.69/alerts.html#control-plane-manager-d8kubernetesstaletokensdetected),
  который срабатывает при обнаружении в кластере устаревших токенов сервисных аккаунтов.

- Появилась возможность создавать проект на основе существующего пространства имен,
  а также включать существующие объекты в проект.
  Для этого на пространство имен и расположенные в нем объекты необходимо установить аннотацию `projects.deckhouse.io/adopt`.
  Это позволяет перейти на использование проектов без пересоздания ресурсов в кластере.

- Добавлен статус `Terminating` для ресурсов ModuleSource и ModuleRelease,
  который будет отображаться в случае неудачной попытки удаления этих ресурсов.

- В контейнере установщика теперь автоматически настраивается доступ к управлению кластером
  после успешного развёртывания (генерируется `kubeconfig` в `~/.kube/config`,
  а также настраивается локальный TCP-прокси через SSH-туннель).
  Это позволяет сразу использовать `kubectl` локально,
  без необходимости вручную подключаться к узлу с control plane по SSH.

- Теперь изменения в ресурсах Kubernetes при мультикластерных и федеративных конфигурациях
  отслеживаются напрямую через API Kubernetes.
  Это ускоряет синхронизацию данных между кластерами и исключает использование устаревших сертификатов.
  Также теперь полностью исключено монтирование ConfigMap и Secret в подах,
  что устраняет риски, связанные с компрометацией файловой системы внутри контейнеров.

- В CoreDNS добавлен новый плагин [dynamicforward](https://github.com/coredns/coredns/pull/7105),
  который улучшает процесс обработки DNS-запросов в кластере.
  Плагин интегрируется с модулем `node-local-dns` и постоянно отслеживает изменения на эндпоинтах `kube-dns`,
  автоматически обновляя список DNS-переадресаторов.
  Если узел с control plane становится недоступен,
  DNS-запросы продолжают перенаправляться к доступным эндпоинтам, что повышает стабильность работы кластера.

- В модуле `loki` реализована новая стратегия ротации логов.
  Теперь старые записи удаляются автоматически при достижении порога использования диска.
  Порог рассчитывается как минимум из двух значений: 95% размера PVC или размер PVC за вычетом объёма,
  необходимого для хранения двух минут логов при заданной скорости приёма ([`ingestionRateMB`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.69/modules/loki/configuration.html#parameters-lokiconfig-ingestionratemb)).
  Параметр [`retentionPeriodHours`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.69/modules/loki/configuration.html#parameters-retentionperiodhours) больше не управляет сроком хранения данных и используется только для алертов мониторинга.
  Если `loki` начнёт удалять записи до истечения заданного периода,
  пользователю будет отправлен алерт [`LokiRetentionPerionViolation`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.69/alerts.html#loki-lokiretentionperionviolation)
  и будет необходимо уменьшить значение `retentionPeriodHours`, либо увеличить размер PVC.

- Добавлен параметр [`nodeDrainTimeoutSecond`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.69/modules/node-manager/cr.html#nodegroup-v1-spec-nodedraintimeoutsecond), который позволяет задать максимальную продолжительность попыток выполнения
  операции drain на узле (в секундах) для каждого ресурса NodeGroup.
  Ранее можно было использовать либо вариант по умолчанию (10 минут),
  либо уменьшить время до 5 минут (параметр `quickShutdown`, который теперь следует считать устаревшим).

- В настройках модуля `openvpn` теперь можно указать значение нового параметра [`defaultClientCertExpirationDays`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.69/modules/openvpn/configuration.html#parameters-clientcertexpirationdays),
  который задаёт срок действия клиентских сертификатов.

### Безопасность

Закрыты известные уязвимости в модулях `ingress-nginx`, `istio`, `prometheus` и `local-path-provisioner`.

### Обновление версий компонентов

Обновлены следующие компоненты DKP:

- `cert-manager`: 1.17.1
- `dashboard`: 1.6.1
- `dex`: 2.42.0
- `go-vcloud-director`: 2.26.1
- Grafana: 10.4.15
- Kubernetes control plane: 1.29.14, 1.30.1, 1.31.6, 1.32.2
- `kube-state-metrics` (`monitoring-kubernetes`): 2.15.0
- `local-path-provisioner`: 0.0.31
- `machine-controller-manager`: v0.36.0-flant.19
- `pod-reloader`: 1.2.1
- `prometheus`: 2.55.1
- Terraform-провайдеры:
  - OpenStack: 1.54.1
  - vCD: 3.14.1

## Версия 1.68

### Обратите внимание

- После обновления у всех источников данных (DataSource) Grafana,
  созданных с помощью ресурса GrafanaAdditionalDatasource, изменится UID.
  Если на источник данных ссылались по UID, то такая связь нарушится.

### Основные изменения

- Новый параметр [`iamNodeRole`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.68/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-iamnoderole) для провайдера AWS.
  Параметр позволяет задать имя IAM-роли, которая будет привязана ко всем AWS-инстансам узлов кластера.
  Это может потребоваться, если в IAM-роль узла нужно добавить больше прав (например, доступ к ECR и т.п.)

- Ускорено создание узлов [с типом CloudPermanent](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.68/modules/node-manager/cr.html#nodegroup-v1-spec-nodetype).
  Теперь все такие узлы создаются параллельно.
  Ранее параллельно создавались CloudPermanent-узлы только в рамках одной группы.

- Изменения в мониторинге:
  - добавлена возможность мониторинга сертификатов в секретах типа `Opaque`;
  - добавлена возможность мониторинга образов в Amazon ECR;
  - исправлена ошибка, из-за которой при перезапуске экземпляров Prometheus могла потеряться часть метрик.

- При использовании мультикластерной конфигурации Istio или федерации,
  теперь можно явно указать список адресов ingressgateway,
  которые нужно использовать для организации межкластерных запросов.
  Ранее эти адреса вычислялись только автоматически, но в некоторых конфигурациях их определить невозможно.

- У аутентификатора (ресурс DexAuthenticator) появился [параметр `highAvailability`](https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/user-authn/cr.html#dexauthenticator-v1-spec-highavailability),
  который управляет режимом высокой доступности.
  В режиме высокой доступности запускается несколько реплик аутентификатора.
  Ранее режим высокой доступности всех аутентификаторов определялся [настройками глобального параметра](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.68/deckhouse-configure-global.html#parameters-highavailability)
  или настройками модуля `user-authn`.
  Все аутентификаторы, развёрнутые самим DKP, теперь наследуют режим высокой доступности соответствующего модуля.

- Лейблы узлов теперь можно добавлять, удалять и изменять,
  используя файлы, хранящиеся на узле в директории `/var/lib/node_labels` и её поддиректориях.
  Полный набор применённых лейблов хранится в аннотации `node.deckhouse.io/last-applied-local-labels`.

- Добавлена поддержка [облачного провайдера Huawei Cloud](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.68/modules/cloud-provider-huaweicloud/).

- Новый параметр [`keepDeletedFilesOpenedFor`](https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/log-shipper/cr.html#clusterloggingconfig-v1alpha2-spec-kubernetespods-keepdeletedfilesopenedfor) в модуле `log-shipper` позволяет настроить период,
  в течение которого будут храниться открытыми удалённые файлы логов.
  Опция позволит какое-то время читать логи удалённых подов в случае недоступности хранилища логов.

- TLS-шифрование для сборщиков логов (Elasticsearch, Vector, Loki, Splunk, Logstash, Socket, Kafka)
  теперь можно настроить, используя секреты, вместо хранения сертификатов в ресурсах ClusterLogDestination.
  Секрет должен находиться в пространстве имён `d8-log-shipper` и иметь лейбл `log-shipper.deckhouse.io/watch-secret: true`.

- В статусе [проекта](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.68/modules/multitenancy-manager/cr.html#project) в разделе `resources` теперь можно увидеть, какие ресурсы проекта были установлены.
  Такие ресурсы будут отмечены флагом `installed: true`.

- В инсталлятор добавлен параметр `--tf-resource-management-timeout`,
  позволяющий управлять таймаутом создания ресурсов в облаках.
  По умолчанию таймаут составляет 10 минут.
  Параметр имеет влияние только для следующих облаков: AWS, Azure, GCP, Yandex Cloud, OpenStack, Базис.DynamiX.

### Безопасность

Закрыты известные уязвимости в следующих модулях:

- `admission-policy-engine`
- `chrony`
- `cloud-provider-azure`
- `cloud-provider-gcp`
- `cloud-provider-openstack`
- `cloud-provider-yandex`
- `cloud-provider-zvirt`
- `cni-cilium`
- `control-plane-manager`
- `extended-monitoring`
- `descheduler`
- `documentation`
- `ingress-nginx`
- `istio`
- `loki`
- `metallb`
- `monitoring-kubernetes`
- `monitoring-ping`
- `node-manager`
- `operator-trivy`
- `pod-reloader`
- `prometheus`
- `prometheus-metrics-adapter`
- `registrypackages`
- `runtime-audit-engine`
- `terraform-manager`
- `user-authn`
- `vertical-pod-autoscaler`
- `static-routing-manager`

### Обновление версий компонентов

Обновлены следующие компоненты DKP:

- Kubernetes Control Plane: 1.29.14, 1.30.10, 1.31.6
- `aws-node-termination-handler`: 1.22.1
- `capcd-controller-manager`: 1.3.2
- `cert-manager`: 1.16.2
- `chrony`: 4.6.1
- `cni-flannel`: 0.26.2
- `docker_auth`: 1.13.0
- `flannel-cni`: 1.6.0-flannel1
- `gatekeeper`: 3.18.1
- `jq`: 1.7.1
- `kubernetes-cni`: 1.6.2
- `kube-state-metrics`: 2.14.0
- `vector` (`log-shipper`): 0.44.0
- `prometheus`: 2.55.1
- `snapshot-controller`: 8.2.0
- `yq4`: 3.45.1

### Перезапуск компонентов

После обновления DKP до версии 1.68 будут перезапущены следующие компоненты:

- Kubernetes Control Plane
- Ingress controller
- Prometheus, Grafana
- `admission-policy-engine`
- `chrony`
- `cloud-provider-azure`
- `cloud-provider-gcp`
- `cloud-provider-openstack`
- `cloud-provider-yandex`
- `cloud-provider-zvirt`
- `cni-cilium`
- `control-plane-manager`
- `descheduler`
- `documentation`
- `extended-monitoring`
- `ingress-nginx`
- `istio`
- `kube-state-metrics`
- `log-shipper`
- `loki`
- `metallb`
- `monitoring-kubernetes`
- `monitoring-ping`
- `node-manager`
- `openvpn`
- `operator-trivy`
- `prometheus`
- `prometheus-metrics-adapter`
- `pod-reloader`
- `registrypackages`
- `runtime-audit-engine`
- `service-with-healthchecks`
- `static-routing-manager`
- `terraform-manager`
- `user-authn`
- `vertical-pod-autoscaler`
