---
title: Модуль operator-trivy
permalink: ru/architecture/security/operator-trivy.html
lang: ru
search: operator-trivy, сканирование образов, сканирование на уязвимости
description: Архитектура модуля operator-trivy в Deckhouse Kubernetes Platform.
---

Модуль [`operator-trivy`](/modules/operator-trivy/) обеспечивает сканирование пользовательских образов в рантайм на известные CVE (Common Vulnerabilities and Exposures), включая уязвимости Astra Linux, ALT Linux и РЕД ОС. Базируется на проекте [Trivy](https://github.com/aquasecurity/trivy). Для сканирования используются [публичные базы уязвимостей](https://github.com/aquasecurity/trivy-db/tree/main/pkg/vulnsrc), обогащаемые базами Astra Linux, ALT Linux и РЕД ОС, а также [БДУ ФСТЭК (Банком данных угроз Федеральной службы по техническому и экспортному контролю)](https://bdu.fstec.ru/vul).

Также модуль производит анализ соответствия кластера Kubernetes требованиям [CIS (Center for Internet Security) Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes).

Дополнительно модуль `operator-trivy` обеспечивает возможность собирать поведенческие модели рабочей нагрузки при использовании опционального компонента node-agent.

Модуль `operator-trivy` работает с кастомными ресурсами API-групп `trivy.deckhouse.io` и `spdx.softwarecomposition.kubescape.io`.

В API-группу `trivy.deckhouse.io` входят следующие ресурсы:

- [ClusterComplianceReport](/modules/operator-trivy/cr.html#clustercompliancereport) — хранит сводный отчёт о соответствии кластера требованиям безопасности (например, CIS Kubernetes Benchmark);
- ClusterConfigAuditReport — хранит результаты аудита конфигурации Kubernetes-объектов на уровне кластера;
- ClusterInfraAssessmentReport — хранит результаты проверок безопасности инфраструктуры Kubernetes на уровне кластера;
- ClusterRbacAssessmentReport — хранит результаты проверки настроек управления доступом на основе ролей (RBAC) на уровне кластера;
- ClusterSbomReport — хранит сводный SBOM (Software Bill of Materials) по программным компонентам на уровне кластера;
- ClusterVulnerabilityReport — хранит агрегированные результаты сканирования уязвимостей на уровне кластера;
- [ConfigAuditReport](/modules/operator-trivy/cr.html#configauditreport) — хранит результаты аудита конфигурации Kubernetes-объектов;
- [ExposedSecretReport](/modules/operator-trivy/cr.html#exposedsecretreport) — хранит результаты поиска потенциальных секретов в образе контейнера;
- InfraAssessmentReport — хранит результаты проверок безопасности инфраструктуры Kubernetes-объектов;
- [NodeVulnerabilityReport](/modules/operator-trivy/cr.html#nodevulnerabilityreport) — хранит результаты сканирования уязвимостей в `rootfs` (корневой файловой системе) узла;
- [RbacAssessmentReport](/modules/operator-trivy/cr.html#rbacassessmentreport) — хранит результаты проверки RBAC-настроек на предмет избыточных привилегий и других рисков;
- [RegistryImageVulnerabilityReport](/modules/operator-trivy/cr.html#registryimagevulnerabilityreport) — хранит результаты CVE-сканирования конкретного тега образа из реестра;
- [RegistryScanTarget](/modules/operator-trivy/cr.html#registryscantarget) — задаёт целевой реестр, репозитории и параметры периодического сканирования;
- [SbomReport](/modules/operator-trivy/cr.html#sbomreport) — хранит SBOM (Software Bill of Materials), то есть состав ПО и зависимостей в образе контейнера;
- [VulnerabilityReport](/modules/operator-trivy/cr.html#vulnerabilityreport) — хранит результаты сканирования уязвимостей в образе контейнера.

В API-группу `spdx.softwarecomposition.kubescape.io` входят следующие ресурсы:

- ApplicationProfiles — хранит профиль поведения приложения в рантайме (системные вызовы, запускаемые процессы, доступ к файлам, HTTP-эндпоинты);
- CollapseConfigurations — задаёт параметры объединения динамических путей, эндпоинтов и сетевых адресов для уменьшения объёма рантайм-профилей;
- ConfigurationScanSummaries — хранит сводку результатов проверок конфигурации для группы рабочих нагрузок в заданной области (например, неймспейсе);
- ContainerProfiles — хранит профиль отдельного контейнера, включая рантайм-поведение и сетевые взаимодействия;
- GeneratedNetworkPolicies — хранит сгенерированные на основе наблюдаемого трафика объекты NetworkPolicy;
- KnownServers — хранит справочник известных серверов и диапазонов IP-адресов для обогащения сгенерированных сетевых политик;
- NetworkNeighborhoods — хранит наблюдаемую карту входящих и исходящих сетевых взаимодействий рабочих нагрузок;
- OpenVulnerabilityExchangeContainers — хранит документы формата OpenVEX со статусами уязвимостей для компонентов;
- SbomSyftFiltereds — хранит отфильтрованный Syft SBOM только с релевантными уязвимыми компонентами;
- SbomSyfts — хранит SBOM в формате Syft;
- SeccompProfiles — хранит seccomp-профили контейнеров (разрешённые системные вызовы и правила фильтрации);
- VulnerabilityManifestSummaries — хранит сводку VulnerabilityManifest по уровням критичности с ссылками на связанные объекты;
- VulnerabilityManifests — хранит подробный манифест найденных уязвимостей;
- VulnerabilitySummaries — хранит агрегированную сводку уязвимостей для заданной области видимости;
- WorkloadConfigurationScans — хранит детальные результаты сканирования конфигурации конкретного workload;
- WorkloadConfigurationScanSummaries — хранит сводку результатов WorkloadConfigurationScan.

Подробнее с описанием модуля можно ознакомиться [в разделе документации модуля](/modules/operator-trivy/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`operator-trivy`](/modules/operator-trivy/) на уровне 2 модели C4 и его взаимодействие с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

![Архитектура модуля operator-trivy](../../images/architecture/security/c4-l2-operator-trivy.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Operator** (Deployment) — компонент, реализованный на базе [Trivy Operator](https://github.com/aquasecurity/trivy-operator), следит за ресурсами Pod, ReplicaSet, ReplicationController, StatefulSet, DaemonSet, CronJob и Job и при изменении их спецификаций запускает задачу Job для выполнения сканирования.

   Operator анализирует результаты работы Job и формирует следующие отчёты в виде кастомных ресурсов:

   - ClusterComplianceReport;
   - ClusterConfigAuditReport;
   - ClusterInfraAssessmentReport;
   - ClusterRbacAssessmentReport;
   - ClusterVulnerabilityReport;
   - ConfigAuditReport;
   - ExposedSecretReport;
   - InfraAssessmentReport;
   - NodeVulnerabilityReport;
   - RbacAssessmentReport;
   - SbomReport;
   - VulnerabilityReport.

   Состоит из следующих контейнеров:

   - **operator** — основной контейнер;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищённого доступа к метрикам контроллера.

1. **Trivy-server** (StatefulSet) — компонент реализует сервис сканирования безопасности, базируется на основе Open Source-проекта [Trivy](https://github.com/aquasecurity/trivy).

   Trivy-server при старте и впоследствии регулярно обновляет базу данных уязвимостей из образа формата Open Container Initiative (OCI).

   Trivy-server обрабатывает запросы от других компонентов модуля (operator, registry-scanner, trivy-provider, задачи сканирования), выполняет запрошенное сканирование и возвращает результат.

   Состоит из следующих контейнеров:

   - **server** — основной контейнер;
   - **trivy-db-info** — сайдкар-контейнер, который синхронизирует метаданные кеша Trivy с ресурсом ConfigMap `trivy-db-info` в неймспейсе `d8-operator-trivy`.

1. **Trivy-provider** (StatefulSet) — компонент состоит из одного контейнера **trivy-provider** и предоставляет интерфейс для проверки образов компонентом Gatekeeper модуля [`admission-policy-engine`](/modules/admission-policy-engine/).

   При установке модуля создаётся ресурс Provider, который регистрирует trivy-provider как провайдер в Gatekeeper. Подробнее с описанием интеграции можно ознакомиться в [соответствующем разделе документации Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/externaldata/).

   Deckhouse-контроллер разворачивает этот компонент, если параметр [`.settings.denyVulnerableImages.enabled`](/modules/operator-trivy/stable/configuration.html#parameters-denyvulnerableimages-enabled) кастомного ресурса ModuleConfig принимает значение `true` (по умолчанию — `false`) и включён компонент Gatekeeper модуля [`admission-policy-engine`](/modules/admission-policy-engine/).

1. **Report-updater** (Deployment) — опциональный контроллер, состоящий из одного контейнера **report-updater**, реализует MutatingWebhook и обеспечивает обогащение кастомного ресурса VulnerabilityReport идентификаторами БДУ. Словарь БДУ регулярно (каждые 6 часов) обновляется из OCI-образа.

   Deckhouse-контроллер разворачивает этот компонент, если параметр [`.settings.linkCVEtoBDU`](/modules/operator-trivy/stable/configuration.html#parameters-linkcvetobdu) кастомного ресурса ModuleConfig принимает значение `true` (по умолчанию — `false`).

1. **Registry-scanner** (StatefulSet) — опциональный компонент, состоящий из одного контейнера **registry-scanner**, выполняет регулярное сканирование образов, размещённых в произвольных реестрах контейнеров, указанных пользователем. Сканирование нагрузок в самом кластере выполняется компонентом operator.

   Deckhouse-контроллер разворачивает этот компонент, если параметр [`.settings.registryScanning.enabled`](/modules/operator-trivy/stable/configuration.html#parameters-registryscanning-enabled) кастомного ресурса ModuleConfig принимает значение `true` (по умолчанию — `false`).

   Registry-scanner читает кастомные ресурсы [RegistryScanTarget](/modules/operator-trivy/cr.html#registryscantarget), запускает сканирование образов через компонент trivy-server и сохраняет результаты обработки в кастомном ресурсе [RegistryImageVulnerabilityReport](/modules/operator-trivy/cr.html#registryimagevulnerabilityreport).

1. **Security-storage** (Deployment) — контроллер, состоящий из одного контейнера **apiserver**, является расширением API Kubernetes и выполняет обработку запросов к кастомным ресурсам API-групп `trivy.deckhouse.io` и `spdx.softwarecomposition.kubescape.io`.

   Также security-storage реализует бэкенд для хранения этих ресурсов:
   - метаданные сохраняются в базе данных SQLite;
   - тело объектов сохраняется в gob-формате в каталоге `/data`.

1. **Node-agent** (DaemonSet) — опциональный компонент с одним контейнером **node-agent**, который запускается на всех узлах кластера в привилегированном режиме, и, используя eBPF-подпрограммы, наблюдает за поведением контейнеров и формирует рантайм-профили.

   Node-agent сохраняет рантайм-профили в кастомных ресурсах ApplicationProfile и NetworkNeighborhood (в компоненте security-storage). Подробнее с работой node-agent можно ознакомиться [в разделе документации модуля](/modules/operator-trivy/stable/runtime_map.html).

   {% alert level="warning" %}
   У компонента node-agent есть привилегированный доступ к операционной системе каждого узла. В Linux для этого требуются capabilities:

   - SYS_ADMIN
   - SYS_PTRACE
   - NET_ADMIN
   - SYSLOG
   - SYS_RESOURCE
   - IPC_LOCK
   - NET_RAW

   Это необходимо для наблюдения за поведением контейнеров, используя eBPF-подпрограммы.

   Node-agent работает в профилирующем режиме, не выполняет активное обнаружение атак по правилам.
   {% endalert %}

   Deckhouse-контроллер разворачивает этот компонент, если параметр [`.settings.nodeAgent.enabled`](/modules/operator-trivy/stable/configuration.html#parameters-nodeagent-enabled) кастомного ресурса ModuleConfig принимает значение `true` (по умолчанию — `false`).

1. **Scan-noderootfs-&lt;HASH&gt;** (Job) — компонент, состоящий из одного контейнера **node-rootfs-scanner**, реализует задачу по сканированию корневой файловой системы (scan-noderootfs) узла кластера. Задача создаётся и управляется компонентом operator.

1. **Scan-vulnerabilityreport-&lt;HASH&gt;** (Job) — компонент обеспечивает запуск задач по сканированию безопасности Pod, ReplicaSet, ReplicationController, StatefulSet, DaemonSet, CronJob и Job с использованием компонента trivy-server. Задача создаётся и управляется компонентом operator.

   Состоит из набора контейнеров **&lt;CONTAINER_NAME&gt;**, каждый из которых отвечает за сканирование соответствующего контейнера рабочей нагрузки (workload). В качестве базового образа для них используется образ trivy с добавленным trivy-wrapper, а в аргументах для него передаётся образ, заданный в спецификации целевого контейнера. Trivy-wrapper выполняет авторизацию у хранилищу образов командой `trivy registry login`, а после передаёт управление trivy.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   - авторизация запросов к метрикам operator;
   - работа с кастомными ресурсами API-группы `trivy.deckhouse.io`;
   - работа с кастомными ресурсами RegistryScanTarget и RegistryImageVulnerabilityReport;
   - отслеживание ресурсов Pod, ReplicaSet, ReplicationController, StatefulSet, DaemonSet, CronJob и Job;
   - создание, обновление и удаление ресурсов Secret, ConfigMap и Job.

1. [**Registry**](/modules/registry/) — загрузка OCI-образов с базой уязвимостей и базой БДУ.

1. **Хранилище образов** — сканирование образов

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver**:

   - пересылка API-запросов по кастомным ресурсам из API-групп `trivy.deckhouse.io` и `spdx.softwarecomposition.kubescape.io`;
   - мутация кастомных ресурсов VulnerabilityReport.

1. **Prometheus-main** — сбор метрик модуля.
