# Моделирование угроз и поверхности атаки модуля `110-istio`

Документ сформирован по Методике моделирования угроз и поверхности атаки на основании локально предоставленных исходных данных из директории `deckhouse_security_module`.

| Параметр | Значение |
| --- | --- |
| **Объект моделирования** | Deckhouse-модуль `110-istio` и EE-расширение `ee/modules/110-istio` |
| **Основные исходные данные** | `110-istio`, `ee/modules/110-istio`, `istio-1.21.6`, `istio-1.25.2`, `operator-1.25.2`, `proxy-1.21.6`, `proxy-1.25.2`, `envoy-5c3dc559371181d5baa4a7533c36f2370fc97581`, `envoy-abc32af61f354196bc8d1a011faf8dacc4dc12d7`, `5.7-threat-modeling.md`, `abbr.md`, `Угрозы-Способы.csv` |
| **Проверенные классы артефактов** | README и документация, Helm/Deckhouse templates, CRD/OpenAPI, RBAC, webhook, Service/Deployment/DaemonSet, Dockerfile/werf-описания, mount-points, patches, VEX/SBOM, Go/Bazel/Rust-зависимости, конфигурация секретов, метрик, логирования и обновления |
| **Локальный источник БДУ** | Использован файл `Угрозы-Способы.csv`; при сопоставлении угроз использованы идентификаторы УБИ и способы из локального файла |
| **Файлы с описанием функций/архитектуры/SBOM** | Отдельные файлы с полным описанием функций, архитектуры и полным SBOM релизных образов не обнаружены. Требует уточнения: наличие внешнего архитектурного описания и полного SBOM/VEX всех образов модуля |

## 1. Определение целей и критичных функций модуля

| Параметр | Описание |
| --- | --- |
| **Наименование модуля** | `110-istio`, Deckhouse-модуль `istio`; EE-расширение `ee/modules/110-istio` для multicluster, federation и alliance-сценариев |
| **Назначение** | Реализация Service Mesh в DKP для централизованного управления сетевым трафиком в Kubernetes-кластере. Модуль обеспечивает mutual TLS, авторизацию, маршрутизацию, балансировку нагрузки, sidecar/ambient dataplane, ingress gateway, наблюдаемость и интеграцию с Kiali. В EE-редакциях дополнительно обеспечивает multicluster/federation/alliance-взаимодействие между кластерами. |
| **Режим эксплуатации** | Сетевой, распределённый, кластерный. Компоненты работают внутри Kubernetes-кластера в namespace `d8-istio` и `d8-ingress-istio`; отдельные dataplane-компоненты разворачиваются как sidecar в пользовательских namespace или как DaemonSet на нодах. При включении multicluster/federation часть интерфейсов публикуется наружу через Ingress/Gateway и межкластерные endpoint. |
| **Среда исполнения** | DKP/Kubernetes, Linux-ноды, Kubernetes API, Admission Webhook, Istio CRD, Gateway API, Envoy proxy, Istio CNI, ztunnel, Operator/Sail Operator, Container Registry, Deckhouse update flow, Prometheus, Kiali, внешние или провайдерские LoadBalancer/Ingress/Gateway, удалённые Kubernetes-кластеры для EE-сценариев. |
| **Основные функции** | Развёртывание и обновление Istio control plane версий `1.21`/`1.25` и дополнительных ревизий; управление `istiod`, operator, sidecar injection, validation webhook; выпуск и распространение CA/сертификатов `cacerts`; xDS/SDS/CA-взаимодействие `istiod` с sidecar, gateway и ztunnel; настройка outbound traffic policy `AllowAny`/`RegistryOnly`; управление CNI-перенаправлением трафика; поддержка ambient dataplane через ztunnel; развёртывание ingress-gateway controller; публикация Kiali; экспорт метрик; EE-механизмы federation/multicluster/alliance, metadata exporter, metrics exporter и API proxy. |
| **Критичные функции** | Защита service-to-service трафика с использованием mTLS и trust domain; корректность sidecar injection и validation webhook; целостность xDS/SDS-конфигурации Envoy; защита CA private key, workload certificates, service account token, remote kubeconfig и remote authn keypair; корректность Istio RBAC/Authn/Authz policy; доступность `istiod` и gateway; корректность CNI/iptables/ambient перенаправления; контроль egress через `outboundTrafficPolicyMode`; безопасное межкластерное доверие и проверка metadata endpoints; целостность сборки `pilot`, `proxyv2`, `operator`, `ztunnel`, `cni`, Kiali и EE-exporter/API образов. |
| **Критичные последствия** | Компрометация CA и доверия mesh; несанкционированный доступ к сервисам внутри mesh; подмена xDS/SDS или маршрутизации; раскрытие TLS-ключей, service account token, remote kubeconfig и межкластерных ключей; обход mTLS/Authz; отказ в обслуживании приложений, `istiod`, admission webhook или gateway; нарушение сетевой конфигурации ноды через CNI/ztunnel capabilities; несанкционированное подключение удалённого кластера; внедрение скомпрометированных зависимостей в runtime-образы. |
| **Объекты защиты** | CA Secret `cacerts`; ConfigMap `istio-ca-root-cert`; projected service account tokens с audience `istio-ca`; workload certificates и UDS-сокеты; `Istio`, `IstioOperator`, `IngressIstioController`, `IstioCNI`, `ZTunnel`, `IstioFederation`, `IstioMulticluster`, Istio CRD (`VirtualService`, `DestinationRule`, `Gateway`, `AuthorizationPolicy`, `PeerAuthentication`, `RequestAuthentication`, `EnvoyFilter`, `WasmPlugin`, `Telemetry`); webhook configurations; RBAC/ClusterRole/ServiceAccount; `d8-istio-sidecar-registry`; Kiali signing key и auth secrets; remote kubeconfig Secret; remote public metadata Secret; remote authn keypair Secret; Envoy access logs and metrics; CNI hostPath `/opt/cni/bin`, `/etc/cni/net.d`, `/var/run/istio-cni`, `/var/run/netns`, `/var/run/ztunnel`; container images and source dependencies. |
| **Категории субъектов** | Внешний клиент/нарушитель, обращающийся к ingress gateway или опубликованным Kiali/API/metadata endpoint (0); пользователь приложения внутри mesh (0/1); пользователь Kubernetes namespace с правом создавать Pods, Services и Istio resources (1); пользователь Kubernetes с правом менять module values, CRD `IngressIstioController`/`IstioFederation`/`IstioMulticluster` или namespace labels (2); оператор/администратор DKP и Kubernetes API (доверенный/привилегированный субъект); компоненты Deckhouse, `istiod`, operator, CNI, ztunnel, Envoy sidecar/gateway, Kiali и EE exporters (ограниченно доверенные субъекты); удалённый кластер federation/multicluster и его metadata endpoint (ограниченно доверенная внешняя система); Container Registry, source repositories, GOPROXY/CargoProxy/SOURCE_REPO и build runners (ограниченно доверенная supply-chain среда). |
| **Реализованные меры защиты, выявленные по исходным данным** | Namespace `d8-istio`; отдельный `d8-ingress-istio` для ingress gateway controller; `failurePolicy: Fail` для sidecar injection и Istio validation webhook; mTLS/CA trust через `cacerts`, `istio-ca-root-cert`, service account token audience `istio-ca`; `kube-rbac-proxy` для Kiali, CNI, operator и EE metrics; read-only root filesystem и drop capabilities для operator/Kiali/части proxy-контейнеров; запуск non-root `1337`/`1000` для operator, Kiali и gateway proxy; `seccompProfile: RuntimeDefault` для injected sidecar template; VPA/PDB/HA anti-affinity; OpenAPI/CRD validation для module values и EE endpoint; HTTPS requirement для federation/multicluster metadata endpoint; использование distroless runtime-образов; закрепление версий Istio/Envoy и локальные VEX для части известных уязвимостей. |
| **Неопределённости** | Требует уточнения: фактические module values в продуктивных кластерах (`ambient.enabled`, `trafficRedirectionSetupMode`, `outboundTrafficPolicyMode`, `enableHTTP10`, tracing, access logs, Kiali exposure, ingress gateway inlet/hostPort/source ranges, federation/multicluster/alliance). Требует уточнения: наличие NetworkPolicy/mesh policy вокруг `d8-istio`, `d8-ingress-istio`, Kiali, API proxy и metadata endpoints. Требует уточнения: полный SBOM и VEX всех релизных образов; в исходных данных обнаружены только отдельные `known_vulnerabilities.vex`. Требует уточнения: фактические правила доступа к созданию Istio CRD, namespace labels `istio-injection`/`istio.io/rev`, `IngressIstioController`, `IstioFederation` и `IstioMulticluster`. |

## 2. Архитектурная модель модуля

**Компоненты модуля и границы доверия:**

| Компонент | Тип | Назначение | Уровень доверия | Граница доверия |
| --- | --- | --- | --- | --- |
| **Deckhouse templates модуля `110-istio`** | Внутренний компонент поставки | Формирование namespace, RBAC, Secrets, ConfigMap, webhook, PodMonitor, Deployment/DaemonSet, Istio/Sail Operator resources и ingress gateway resources | Доверенный субъект при условии контроля релизного артефакта | Да, между релизным артефактом и Kubernetes API |
| **Namespace `d8-istio`** | Внутренняя зона исполнения | Размещение control plane, operator, Kiali, CNI, ztunnel, CA/ConfigMap/Secrets и EE-компонентов | Доверенная зона с привилегированными компонентами | Да, между модулем и пользовательскими namespace |
| **Namespace `d8-ingress-istio`** | Внутренняя зона исполнения | Размещение ingress-gateway controller DaemonSet и связанных ServiceAccount/Service/PodMonitor | Ограниченно доверенная зона, принимает внешний трафик | Да, между внешней сетью и mesh |
| **Istio operator / Sail Operator** | Внутренний компонент управления | Reconcile Istio/Sail resources, управление control plane resources, webhooks и статусами; в версии 1.25 использует `sailoperator.io/v1` `Istio` | Доверенный privileged-controller с широкими RBAC | Да, между Kubernetes API и control plane |
| **`istiod` / Pilot discovery** | Внутренний security-critical компонент | xDS/SDS/CA endpoint, validation `/validate`, sidecar injection `/inject`, обработка Istio CRD и Gateway API, выпуск workload certs | Доверенный компонент, обрабатывающий ограниченно доверенные Kubernetes data | Да, между Kubernetes API, dataplane и admission |
| **MutatingWebhook `d8-istio-sidecar-injector-*`** | Внутренний admission-интерфейс | Inject sidecar по labels `istio-injection`, `istio.io/rev`, `sidecar.istio.io/inject`; `failurePolicy: Fail` | Доверенный механизм изменения Pod spec | Да, между пользователем Kubernetes и создаваемым Pod |
| **ValidatingWebhook `d8-istio-validator-*`** | Внутренний admission-интерфейс | Валидация Istio resources групп `security.istio.io`, `networking.istio.io`, `telemetry.istio.io`, `extensions.istio.io`; `failurePolicy: Fail` | Доверенный механизм контроля целостности конфигурации | Да, между Kubernetes API и Istio CRD |
| **Envoy sidecar / `proxyv2`** | Внутренний dataplane-компонент в пользовательских Pod | Перехват inbound/outbound traffic, mTLS, L7 policy enforcement, telemetry, xDS/SDS interaction, access logging | Ограниченно доверенный субъект: обрабатывает недоверенный пользовательский трафик и конфигурацию | Да, между приложением, сетью и control plane |
| **Ingress gateway controller / Envoy router** | Внутренний dataplane-компонент на границе | Публичный или ограниченный входящий L4/L7 traffic, SNI-DNAT/router mode, связь с `istiod` CA/xDS, expose ports 8080/8443/15021/15090/15012 | Ограниченно доверенный субъект на внешней поверхности атаки | Да, между внешней сетью и mesh |
| **Istio CNI DaemonSet** | Внутренний node-level компонент с повышенными правами | Установка CNI plugin, изменение hostPath `/opt/cni/bin` и `/etc/cni/net.d`, repair pods, iptables/ipset/netns для traffic redirection и ambient | Доверенный привилегированный компонент | Да, между Pod и сетевым стеком ноды |
| **ztunnel DaemonSet** | Внутренний ambient dataplane-компонент | HBONE/ambient dataplane, TPROXY/setns, связь с `istiod` CA/xDS, hostPath `/var/run/ztunnel`, token audience `istio-ca` | Ограниченно доверенный privileged dataplane; требует root и `NET_ADMIN`, `SYS_ADMIN`, `NET_RAW` | Да, между workload network namespace, нодой и control plane |
| **Kiali + kube-rbac-proxy** | Внутренний UI/observability компонент | Визуализация mesh, доступ через Ingress/HTTPRoute, Kubernetes API reads, metrics, signing key, auth через basic/external auth и kube-rbac-proxy | Ограниченно доверенный субъект, доступ к топологии и конфигурации | Да, между пользователем UI и Kubernetes/Istio данными |
| **Prometheus/PodMonitor endpoints** | Внешняя система мониторинга | Scrape метрик operator, CNI, ztunnel, Kiali, ingress gateway, EE exporters через kube-rbac-proxy или PodMonitor | Ограниченно доверенная внешняя система | Да, между observability-системой и модулем |
| **Istio CA Secret `cacerts`** | Внутренний объект данных | Хранение `ca-cert.pem`, `ca-key.pem`, `cert-chain.pem`, `root-cert.pem` | Доверенный объект защиты критического уровня | Да, при доступе `istiod` и Kubernetes Secret consumers |
| **`d8-istio-sidecar-registry` и `deckhouse-registry` Secrets** | Внутренний объект данных | Pull secret для sidecar/dataplane/operator images | Доверенный объект защиты | Да, между runtime и registry |
| **Istio CRD и Gateway API resources** | Внутренние/пользовательские данные | `VirtualService`, `DestinationRule`, `Gateway`, `AuthorizationPolicy`, `PeerAuthentication`, `RequestAuthentication`, `EnvoyFilter`, `WasmPlugin`, `Telemetry`, Gateway API | Недоверенные или ограниченно доверенные данные в зависимости от RBAC | Да, между пользовательскими namespace и control plane |
| **EE metadata exporter** | Внутренний EE-компонент | Публикация публичной/частной metadata для federation/multicluster, metrics на loopback и kube-rbac-proxy | Ограниченно доверенный субъект | Да, между локальным и удалённым кластером |
| **EE API proxy** | Внутренний EE-компонент с опубликованным HTTPS endpoint | Проксирование multicluster API для удалённых кластеров, чтение remote public metadata, порт 4443 | Ограниченно доверенный субъект, принимает запросы от внешнего/удалённого кластера | Да, между удалённым кластером и Kubernetes API/модулем |
| **EE metrics exporter** | Внутренний EE-компонент | Экспорт multicluster metrics через kube-rbac-proxy | Ограниченно доверенный субъект | Да |
| **EE alliance ingressgateway** | Внутренний dataplane-компонент на межкластерной границе | Публикация intercluster traffic на 15443, 15012, 15017, 15090, 15021, использование workload certs | Ограниченно доверенный субъект на внешней поверхности атаки | Да |
| **Remote cluster metadata endpoint** | Внешняя система | Источник root CA, authn public key, private metadata, ingress gateways, remote API host | Ограниченно доверенная внешняя система | Да |
| **Container Registry, source repositories, GOPROXY/CargoProxy/SOURCE_REPO, Bazel cache/deps** | Внешняя supply-chain среда | Источник runtime images, Istio/operator/proxy/envoy/ztunnel source, Go/Rust/Bazel dependencies и build cache | Ограниченно доверенная система поставки | Да |

**Основные интерфейсы и потоки данных:**

| Источник | Получатель | Протокол/формат | Назначение | Доверенность данных |
| --- | --- | --- | --- | --- |
| Kubernetes API | Operator / `istiod` | Kubernetes watch/list/get/update, JSON/YAML | Reconcile Istio resources, CRD, ConfigMap, Secret, Namespace, Webhook, Gateway API | Ограниченно доверенные данные |
| Пользователь Kubernetes | Kubernetes API / admission | Pod, namespace labels, Istio CRD, Gateway API, ModuleConfig/CRD YAML | Включение injection, задание routing/authn/authz/telemetry/egress policy | Недоверенные или ограниченно доверенные данные |
| Kubernetes API admission | `istiod` `/inject` | HTTPS AdmissionReview v1 | Sidecar injection в Pod spec | Недоверенный Pod spec на входе, доверенный результат при корректном webhook |
| Kubernetes API admission | `istiod` `/validate` | HTTPS AdmissionReview v1 | Validation Istio resources | Недоверенный Istio object на входе |
| Envoy sidecar/gateway/ztunnel | `istiod` | xDS/SDS/CA, gRPC/TLS, 15012; metrics/status 15090/15021 | Получение конфигурации, сертификатов, состояния readiness | Ограниченно доверенные workload identity и telemetry |
| `istiod` | Envoy sidecar/gateway/ztunnel | xDS resources, certificates, policy/config | Программирование dataplane и mTLS | Доверенные control-plane данные |
| Внешний клиент/LB | Ingress gateway controller | HTTP/HTTPS/TLS/SNI, TCP, 8080/8443/hostPort/LoadBalancer | Входящий пользовательский трафик в mesh | Недоверенные сетевые данные |
| Внутреннее приложение | Envoy sidecar | HTTP/gRPC/TCP/TLS, loopback/network namespace | Inbound/outbound app traffic через proxy | Недоверенные пользовательские данные |
| Istio CNI | Host filesystem/network stack | hostPath, CNI config JSON, iptables/ipset/netns, UDS | Traffic redirection setup, repair pods, ambient support | Доверенное privileged-действие |
| ztunnel | Workload/network namespace | HBONE/ambient, TPROXY, UDS `/var/run/ztunnel` | Ambient dataplane redirection and mTLS tunnel | Ограниченно доверенные сетевые данные |
| Kiali user | Kiali / kube-rbac-proxy | HTTPS через Ingress/HTTPRoute, basic/external auth | Просмотр topology, workloads, Istio config, metrics | Ограниченно доверенные пользовательские запросы |
| Prometheus | kube-rbac-proxy / metrics endpoints | HTTPS/HTTP scrape | Сбор метрик control plane, dataplane, Kiali, EE exporters | Ограниченно доверенные observability данные |
| EE remote cluster | Metadata exporter/API proxy | HTTPS, JSON metadata, JWT/kubeconfig-derived auth | Federation/multicluster discovery и доступ к remote API | Недоверенные до проверки TLS/authn |
| Metadata discovery hooks | Remote metadata endpoint | HTTPS, JSON, optional CA или `insecureSkipVerify` | Получение публичной/частной metadata удалённого кластера | Ограниченно доверенные внешние данные |
| Build runner | SOURCE_REPO/GOPROXY/CargoProxy/Bazel cache | Git, Go modules, Cargo registry, Bazel archive/cache | Сборка pilot/proxyv2/operator/ztunnel/cni/Kiali/EE images | Ограниченно доверенные supply-chain данные |

**Используемые сторонние компоненты и зависимости с фиксацией версий:**

| Компонент | Версия / источник | Назначение | Замечания безопасности |
| --- | --- | --- | --- |
| Istio | `1.21.6`, `1.25.2`, также в module images присутствуют `v1x27x9` | Control plane, Pilot, CNI, sidecar/gateway templates | Фактическая активная версия определяется module values `globalVersion` и `additionalVersions`; требует уточнения для конкретного контура. |
| Sail Operator / Istio operator | `tag-<istioVersion>` из `deckhouse/network/sail-operator.git`; operator images `v1x21x6`, `v1x25x2` | Reconcile control plane | Используются локальные patches; для `operator-v1x21x6` есть VEX, для `operator-v1x25x2` полного VEX/SBOM не обнаружено. |
| Istio proxy / Envoy | `proxy-1.21.6` с Envoy `5c3dc559371181d5baa4a7533c36f2370fc97581`; `proxy-1.25.2` с Envoy `abc32af61f354196bc8d1a011faf8dacc4dc12d7` | Envoy dataplane, sidecar/gateway/agent | Envoy собирается через Bazel, build cache/deps из `SOURCE_REPO`; полная проверка зависимостей требует SBOM. |
| Go toolchain | Для `proxyv2-v1x25x2` указан Go `1.23.1`; сборка pilot/operator через `builder/golang-alpine` | Сборка Go-бинарей `pilot-agent`, `pilot-discovery`, operator | Используются `GOPROXY` secret и `go mod download/vendor`; требуется контроль mirror/proxy. |
| Bazel/protoc/LLVM | Bazel `6.5.0`, protoc `22.3`, LLVM `14.0.6` | Сборка Envoy/proxy | Загружаются из зеркал/репозиториев через `SOURCE_REPO`; требуется проверка целостности. |
| ztunnel | `istio/ztunnel` tag `istioVersion`; Rust/Cargo build | Ambient dataplane | Использует `CARGO_PROXY`, `SOURCE_REPO`, Rust/Cargo; допускается HTTP sparse mirror при `CARGO_PROXY`; требует уточнения политики доверия к mirror. |
| Kiali | Images `kiali-v1x21x6`, `kiali-v1x25x2`, `kiali-v1x27x9` | UI Service Mesh | Доступ защищается kube-rbac-proxy и auth ingress, но фактическая публикация и auth mode требуют уточнения. |
| kube-rbac-proxy | Common image Deckhouse | Авторизация доступа к Kiali и метрикам | Является security boundary для UI/metrics; требует регрессионной проверки конфигурации SubjectAccessReview. |
| `common/distroless`, `glibc-v2.41`, `iptables 1.8.9`, `d8-curl 8.9.1` | Из Deckhouse/common images и registry packages | Runtime base and utilities | Дистролесс снижает поверхность, но CNI/proxy используют host/network utilities; SBOM runtime образов не обнаружен. |
| EE `api-proxy`, `metadata-exporter`, `metrics-exporter` | Локальные Go images из `ee/modules/110-istio/images/*` | Multicluster/federation/alliance integration | Обрабатывают межкластерные данные и секреты; требуется тестирование authn/authz и обработки metadata. |

**Требует уточнения:**

| Наблюдение | Значение для дальнейшего анализа |
| --- | --- |
| В исходных данных нет отдельного полного архитектурного документа модуля; архитектура восстановлена по templates, CRD, OpenAPI и upstream manifests. | Необходимо подтвердить соответствие продуктивной архитектуре и включённым editions/features. |
| Полный SBOM релизных образов не обнаружен; найдены только `known_vulnerabilities.vex` для `pilot-v1x21x6`, `pilot-v1x25x2`, `operator-v1x21x6`. | Оценка известных уязвимостей и supply-chain риска неполна без SBOM/VEX для всех образов, включая proxyv2, ztunnel, cni, Kiali и EE images. |
| В модуле `110-istio` и `ee/modules/110-istio` NetworkPolicy не обнаружены. | Сетевая сегментация `d8-istio`, `d8-ingress-istio`, Kiali, API proxy, metadata exporter, CNI и ztunnel должна подтверждаться внешними политиками кластера. |
| CNI и ztunnel используют root, hostPath и сетевые capabilities; ztunnel указывает `allowPrivilegeEscalation: true`. | Угроза компрометации ноды зависит от фактического включения CNI/ambient и изоляции нод. |
| Для `ztunnel` в Deckhouse-шаблоне явно описан ServiceAccount, но полномочия могут формироваться upstream-operator/Istio ресурсами. | Требует уточнения фактический ClusterRole/RoleBinding `ztunnel` в отрендеренном кластере. |
| CRD `IstioFederation`/`IstioMulticluster` требуют HTTPS metadata endpoint, но допускают `insecureSkipVerify`. | Риск подмены удалённой metadata зависит от политики использования этого флага и проверки CA. |
| Kiali, API proxy и metadata endpoints могут быть опубликованы через Ingress/HTTPRoute с внешней аутентификацией. | Фактическая поверхность зависит от `publicDomainTemplate`, ingressClass, auth settings, source ranges и внешних LB. |
| В RBAC Kiali обнаружены права на `pods/portforward create`, а EE `api-proxy` имеет read/watch-доступ к части Kubernetes resources, включая Secrets в multicluster-сценарии. | Требуется отдельная проверка минимальности RBAC для UI/API proxy и фильтрации данных, возвращаемых удалённым клиентам. |
| `outboundTrafficPolicyMode` по умолчанию `AllowAny`. | Egress из mesh разрешён по умолчанию, если не переопределено на `RegistryOnly`; влияет на сценарии утечки и command-and-control. |

## 3. Анализ поверхности атаки модуля

| Элемент | Компонент | Версия | Функция безопасности | Тип интерфейса | Уровень доступа | Характер взаимодействия | Недоверенные данные |
| --- | --- | --- | --- | --- | --- | --- | --- |
| **Sidecar injection webhook `/inject`** | `istiod`, MutatingWebhookConfiguration `d8-istio-sidecar-injector-global/revisions` | Istio `1.21.6`/`1.25.2` в зависимости от revision | Автоматическое добавление `istio-proxy`, seccomp template, projected token, traffic interception | Kubernetes admission API | Kubernetes API; доступен при `CREATE pods` | Внутренний | Pod spec, namespace labels, pod labels/annotations |
| **Validation webhook `/validate`** | `istiod`, ValidatingWebhookConfiguration `d8-istio-validator-global/revisions` | Istio `1.21.6`/`1.25.2` | Валидация `security.istio.io`, `networking.istio.io`, `telemetry.istio.io`, `extensions.istio.io` resources; `failurePolicy: Fail` | Kubernetes admission API | Kubernetes API | Внутренний | Istio CRD YAML/JSON, Gateway API objects |
| **xDS/SDS/CA endpoint 15012** | `istiod` | Pilot `1.21.6`/`1.25.2` | Выдача конфигурации Envoy, workload certificates, SDS, mTLS trust | gRPC/TLS программный | Ограниченный workload identity | Внутренний | Workload identity, CSR, node/workload metadata |
| **`istiod` status/metrics 8080/15010/15014/15017** | `istiod` Service/Deployment | Pilot `1.21.6`/`1.25.2` | Readiness, metrics, webhook service, internal debug/service endpoints | HTTP/HTTPS/gRPC | Ограниченный внутри кластера; часть через PodMonitor/kube-rbac-proxy | Внутренний | Метрики, debug/status сведения, webhook requests |
| **Istio CRD: routing and policy resources** | Kubernetes API, `istiod`, Envoy | Istio API `networking/security/telemetry/extensions` | Управление mTLS, authorization, routing, telemetry, Envoy extension | Kubernetes API/configuration | Ограниченный RBAC namespace/cluster | Внутренний | `VirtualService`, `DestinationRule`, `Gateway`, `AuthorizationPolicy`, `PeerAuthentication`, `RequestAuthentication`, `EnvoyFilter`, `WasmPlugin`, `Telemetry` |
| **Namespace labels and pod annotations for injection** | Kubernetes API, `istiod` injector | DKP/Istio templates | Включение/выключение sidecar injection и revision selection | Configuration | Ограниченный RBAC на namespace/pod | Внутренний | `istio-injection`, `istio.io/rev`, `sidecar.istio.io/inject`, `proxy.istio.io/config` |
| **Envoy sidecar inbound/outbound listeners** | `proxyv2`, Envoy | Proxy `1.21.6`/`1.25.2`; Envoy `5c3dc...`/`abc32...` | mTLS, Authn/Authz policy enforcement, routing, egress control, telemetry | Сетевой L4/L7 | Доступен приложениям и сервисам mesh | Внутренний/внешний через workload traffic | HTTP/gRPC/TCP/TLS traffic, headers, body, SNI, ALPN |
| **Envoy admin/status and readiness 15000/15021/15090** | `istio-proxy`, gateway, sidecar | Proxy `1.21.6`/`1.25.2` | Admin drain, readiness, Prometheus metrics | HTTP локальный/Pod | Внутренний; 15090 scrape | Внутренний | Метрики, admin commands, proxy config/status |
| **Ingress gateway public ports 8080/8443/hostPort/LoadBalancer** | `ingress-gateway-controller`, Envoy router | Proxy `1.21.6`/`1.25.2` | Входной L4/L7 traffic, TLS/SNI routing, mTLS to mesh | Пользовательский сетевой | Публичный или ограниченный LB/HostPort | Внешний | HTTP/HTTPS/TCP traffic, SNI, headers, body |
| **Alliance ingressgateway 15443/15012/15017/15090/15021** | EE `alliance-ingressgateway` | Proxy `1.21.6`/`1.25.2` | Межкластерный ingress, remote service access, discovery/CA paths | Сетевой межкластерный | Публичный/ограниченный по inlet `LoadBalancer`/`NodePort` | Внешний/межкластерный | Межкластерный TLS/HBONE/traffic, remote discovery requests |
| **Istio CNI hostPath and netns operations** | `istio-cni-node` DaemonSet | Istio CNI `1.21.6`/`1.25.2` | Безопасная настройка перенаправления без privileged initContainer; repair pods | Privileged host interface | Привилегированный node-level | Host/internal | CNI config JSON, pod netns, iptables/ipset, hostPath files |
| **`proxy_init` / initContainer traffic redirection** | `proxyv2`, Istio sidecar injection | Istio `1.21.6`/`1.25.2` | Настройка iptables для sidecar mode без CNIPlugin | Privileged pod init interface | Ограниченный namespace, но с повышенными Pod capabilities при использовании initContainer mode | Внутренний/Pod | iptables rules, pod network namespace, pod annotations |
| **CNI metrics 15014 через kube-rbac-proxy 9734/4286** | `istio-cni-node`, kube-rbac-proxy | DKP common + Istio CNI | Ограничение доступа к метрикам через SubjectAccessReview | HTTPS metrics | Ограниченный RBAC | Внутренний | Метрики CNI, node/pod identifiers |
| **ztunnel stats 15020 and HBONE/ambient paths** | `ztunnel` DaemonSet | Istio ztunnel `1.25.2` при ambient | Ambient dataplane, mTLS tunnel, TPROXY, workload identity | Сетевой/host namespace/UDS | Внутренний; privileged на ноде | Внутренний/host | HBONE traffic, workload metadata, ztunnel socket data |
| **Kiali HTTPS/UI endpoint через kube-rbac-proxy 8443 и Ingress/HTTPRoute** | Kiali, kube-rbac-proxy | Kiali images `v1x21x6`/`v1x25x2`/`v1x27x9` | UI access control, mesh topology, auth via basic/external auth | Пользовательский web/API | Ограниченный, при публикации внешний | Внешний/внутренний | UI requests, auth headers, topology queries |
| **Kiali Kubernetes API access** | Kiali ServiceAccount | DKP RBAC | Чтение mesh resources, workloads, services, gateways, tokenreviews | Kubernetes API | Привилегированный service access | Внутренний | API responses, namespace/service/topology data |
| **Prometheus scrape endpoints** | operator, Kiali, ztunnel, CNI, gateway, EE exporters | DKP module + kube-rbac-proxy | Observability and access-controlled metrics | HTTPS/HTTP metrics | Ограниченный RBAC | Внутренний | Labels `namespace`, `service`, `workload`, `pod`, `cluster`, traffic metrics |
| **Tracing collector address** | Envoy/istiod telemetry | Config `tracing.collector.zipkin.address` | Trace export | Network egress | Привилегированный configuration | Внешний/внутренний | Trace spans, request metadata |
| **`outboundTrafficPolicyMode` and ServiceEntry** | Envoy sidecar, `istiod` | Module default `AllowAny` | Egress allow/deny policy | Configuration + network egress | Namespace/mesh config | Внешний | Outbound destination, DNS, ServiceEntry data |
| **CA Secret `cacerts` and service account tokens** | `istiod`, Envoy, ztunnel, gateway | Istio CA | Root/signing CA and workload authentication | Secret/projected volume | Привилегированный service/RBAC access | Внутренний | CA private key, cert chain, JWT tokens |
| **Remote kubeconfig Secret `istio-remote-secret-*`** | EE multicluster | EE templates | Доступ `istiod` к удалённому cluster registry/API | Secret | Привилегированный | Межкластерный | Remote API token/kubeconfig, CA |
| **Remote authn keypair Secret `d8-remote-authn-keypair`** | EE federation/multicluster | EE templates | Подпись/проверка удалённой аутентификации metadata/API | Secret | Привилегированный | Межкластерный | Private key, public key |
| **Remote metadata endpoint** | EE hooks, metadata exporter, remote clusters | `IstioFederation`/`IstioMulticluster` CRD | Обмен root CA, public/private metadata, API host, ingress gateway list | HTTPS JSON | Внешний ограниченно доверенный | Внешний/межкластерный | Remote JSON metadata, CA, authn key, cluster UUID |
| **`insecureSkipVerify` for remote metadata** | EE `IstioFederation`/`IstioMulticluster` config | CRD v1alpha1 | Отключение проверки TLS удалённого metadata endpoint | Configuration | Привилегированный | Внешний | TLS peer identity, remote metadata |
| **EE API proxy 4443 and public Ingress/HTTPRoute** | `api-proxy` | EE image `apiProxy` | Authenticated multicluster API endpoint | HTTPS API | Внешний при публикации | Внешний/межкластерный | Remote cluster requests, JWT/auth data |
| **EE metadata exporter 8080 and metrics 4225/4226** | `metadata-exporter`, kube-rbac-proxy | EE image `metadataExporter` | Публикация metadata and metrics | HTTP/HTTPS | Ограниченный; может публиковаться через Ingress/HTTPRoute | Внешний/внутренний | Public/private metadata, metrics |
| **EE metrics exporter 4224** | `metrics-exporter`, kube-rbac-proxy | EE image `metricsExporter` | Multicluster metrics export | HTTPS metrics | Ограниченный RBAC | Внутренний | Cross-cluster metrics and labels |
| **Registry/source clone/build secrets** | werf build, `SOURCE_REPO`, `GOPROXY`, `CARGO_PROXY`, Bazel deps/cache | Istio/proxy/operator/ztunnel image builds | Supply-chain integrity | Build-time network/source | Привилегированный сборочный | Внешний/сборочный | Source code, patches, dependencies, build cache, registry images |

**Требует уточнения:**

| Наблюдение | Значение для дальнейшего анализа |
| --- | --- |
| `ambient.enabled` по умолчанию зависит от values и версии; ztunnel/CNI privileged surface появляется только при включении ambient/CNI mode. | Условность угроз ambient должна быть подтверждена фактическими module values. |
| `outboundTrafficPolicyMode` по умолчанию `AllowAny`. | При `RegistryOnly` часть egress-сценариев становится менее реализуемой; при `AllowAny` egress остаётся широкой поверхностью атаки. |
| `enableHTTP10`, tracing, access log formats и `proxy.istio.io/config` могут расширять обработку недоверенных данных. | Нужна проверка фактических Pod annotations и module values. |
| Ingress/Kiali/API proxy/metadata exporter exposure зависит от `ingressClass`, Gateway API/Ingress, external auth и source ranges. | Без фактической сетевой схемы часть угроз является условно актуальной. |
| NetworkPolicy для `d8-istio` и `d8-ingress-istio` в исходных данных не подтверждены. | Достижимость внутренних endpoint и pprof/debug/status требует уточнения. |

## 4. Идентификация угроз

| Компонент | Элемент поверхности атаки | STRIDE | Идентификатор БДУ/перечня | Название угрозы | Источник угрозы | Потенциал | Нарушаемые свойства (К/Ц/Д) |
| --- | --- | --- | --- | --- | --- | --- | --- |
| **Ingress gateway / Envoy** | Публичные 8080/8443/HostPort/LoadBalancer | Denial of Service | УБИ.6, УБИ.8 | Угроза отказа в обслуживании; угроза нарушения функционирования | Внешний нарушитель | Низкий/Средний | Д |
| **Ingress gateway / Envoy** | SNI/Host/path/routing | Spoofing / Tampering | УБИ.4, УБИ.3 | Угроза несанкционированной подмены; угроза несанкционированной модификации | Внешний нарушитель или пользователь namespace | Средний | Ц, Д |
| **Ingress gateway / Envoy** | TLS/mTLS termination and passthrough | Information Disclosure | УБИ.1 | Угроза утечки информации | Внешний нарушитель | Средний | К |
| **Envoy sidecar** | Inbound/outbound application traffic | Tampering | УБИ.3 | Угроза несанкционированной модификации (искажения) | Пользователь приложения или скомпрометированный workload | Средний | Ц |
| **Envoy sidecar** | AuthorizationPolicy/PeerAuthentication/RequestAuthentication | Elevation of Privilege | УБИ.2 | Угроза несанкционированного доступа | Пользователь Kubernetes namespace | Средний | К, Ц |
| **Envoy sidecar** | `outboundTrafficPolicyMode=AllowAny`, ServiceEntry | Misuse / Information Disclosure | УБИ.7, УБИ.1 | Угроза ненадлежащего использования; угроза утечки информации | Скомпрометированный workload | Средний | К, Д |
| **Envoy sidecar / proxy admin** | Admin/status/readiness/metrics endpoints | Repudiation / Information Disclosure | УБИ.11, УБИ.1 | Угроза несанкционированного массового сбора информации; угроза утечки информации | Внутренний нарушитель | Низкий/Средний | К |
| **`istiod`** | xDS/SDS/CA 15012 | Spoofing / Tampering | УБИ.4, УБИ.3 | Угроза несанкционированной подмены; угроза несанкционированной модификации | Внутренний нарушитель, скомпрометированный workload | Средний/Высокий | К, Ц |
| **`istiod`** | CA issuance / CSR processing | Elevation of Privilege | УБИ.2 | Угроза несанкционированного доступа | Внутренний нарушитель | Высокий | К, Ц |
| **`istiod`** | Validation webhook `/validate` | Denial of Service | УБИ.6 | Угроза отказа в обслуживании | Пользователь Kubernetes namespace | Низкий/Средний | Д |
| **`istiod`** | Validation webhook bypass/unavailability | Tampering | УБИ.3 | Угроза несанкционированной модификации | Внутренний нарушитель | Средний | Ц |
| **Sidecar injector** | Mutating webhook `/inject` | Tampering / Elevation of Privilege | УБИ.3, УБИ.2 | Угроза несанкционированной модификации; угроза несанкционированного доступа | Пользователь Kubernetes namespace | Средний | Ц, К |
| **Sidecar injector** | Namespace labels and pod annotations | Spoofing | УБИ.4 | Угроза несанкционированной подмены | Пользователь Kubernetes namespace | Низкий/Средний | Ц |
| **Istio CRD** | `EnvoyFilter`, `WasmPlugin`, `Telemetry` | Elevation of Privilege / Execution | УБИ.2, УБИ.3 | Угроза несанкционированного доступа; угроза несанкционированной модификации | Пользователь Kubernetes с правом менять Istio resources | Средний/Высокий | К, Ц, Д |
| **Istio CRD** | `VirtualService`, `DestinationRule`, `Gateway`, ServiceEntry | Tampering / Spoofing | УБИ.3, УБИ.4 | Угроза несанкционированной модификации; угроза несанкционированной подмены | Пользователь Kubernetes namespace | Низкий/Средний | Ц, Д |
| **Istio CRD** | Authn/Authz policy | Repudiation / Elevation of Privilege | УБИ.2, УБИ.3 | Угроза несанкционированного доступа; угроза несанкционированной модификации | Внутренний нарушитель | Средний | К, Ц |
| **CNI DaemonSet** | hostPath `/opt/cni/bin`, `/etc/cni/net.d` | Tampering | УБИ.3, УБИ.5 | Угроза несанкционированной модификации; угроза удаления информационных ресурсов | Внутренний нарушитель | Высокий | Ц, Д |
| **CNI DaemonSet** | netns/iptables/ipset operations, repair pods | Elevation of Privilege | УБИ.2 | Угроза несанкционированного доступа | Внутренний нарушитель | Высокий | К, Ц, Д |
| **CNI DaemonSet** | CNI metrics and logs | Information Disclosure | УБИ.11 | Угроза несанкционированного массового сбора информации | Внутренний нарушитель | Низкий/Средний | К |
| **ztunnel** | Ambient HBONE/TPROXY and `/var/run/ztunnel` | Tampering / Elevation of Privilege | УБИ.3, УБИ.2 | Угроза несанкционированной модификации; угроза несанкционированного доступа | Внутренний нарушитель | Высокий | К, Ц, Д |
| **ztunnel** | ztunnel traffic processing | Denial of Service | УБИ.6, УБИ.8 | Угроза отказа в обслуживании; угроза нарушения функционирования | Внутренний/внешний нарушитель через workload traffic | Средний | Д |
| **CA Secret `cacerts`** | Kubernetes Secret and mounted CA materials | Information Disclosure / Elevation of Privilege | УБИ.1, УБИ.2 | Угроза утечки информации; угроза несанкционированного доступа | Внутренний нарушитель | Средний/Высокий | К, Ц |
| **ServiceAccount tokens** | Projected token audience `istio-ca`, pod tokens | Spoofing / Elevation of Privilege | УБИ.4, УБИ.2 | Угроза несанкционированной подмены; угроза несанкционированного доступа | Внутренний нарушитель | Средний | К, Ц |
| **Kiali** | UI/API via Ingress/HTTPRoute/kube-rbac-proxy | Spoofing / Information Disclosure | УБИ.2, УБИ.1, УБИ.11 | Угроза несанкционированного доступа; угроза утечки информации; угроза массового сбора информации | Внешний/внутренний нарушитель | Низкий/Средний | К |
| **Kiali** | Signing key and auth secrets | Information Disclosure | УБИ.1 | Угроза утечки информации | Внутренний нарушитель | Средний | К |
| **Metrics/observability** | Prometheus scrape labels, traffic metrics, logs/traces | Information Disclosure | УБИ.11, УБИ.1 | Угроза несанкционированного массового сбора информации; угроза утечки информации | Внутренний нарушитель | Низкий/Средний | К |
| **Tracing collector** | Zipkin address and trace export | Information Disclosure / Tampering | УБИ.1, УБИ.9 | Угроза утечки информации; угроза получения информационных ресурсов из недоверенного источника | Внутренний/внешний нарушитель | Средний | К, Ц |
| **EE metadata exporter** | Public/private metadata endpoint | Information Disclosure / Spoofing | УБИ.1, УБИ.4 | Угроза утечки информации; угроза несанкционированной подмены | Внешний или удалённый нарушитель | Средний | К, Ц |
| **EE API proxy** | HTTPS 4443 and public Ingress/HTTPRoute | Elevation of Privilege / Denial of Service | УБИ.2, УБИ.6 | Угроза несанкционированного доступа; угроза отказа в обслуживании | Удалённый кластер/внешний нарушитель | Средний/Высокий | К, Ц, Д |
| **EE remote metadata discovery** | `metadataEndpoint`, `ca`, `insecureSkipVerify` | Spoofing / Tampering | УБИ.4, УБИ.9, УБИ.3 | Угроза подмены; угроза получения ресурсов из недоверенного источника; угроза модификации | Внешний нарушитель, скомпрометированный remote cluster | Средний | Ц |
| **EE remote kubeconfig** | Secret `istio-remote-secret-*` | Information Disclosure / Elevation of Privilege | УБИ.1, УБИ.2 | Угроза утечки информации; угроза несанкционированного доступа | Внутренний нарушитель | Высокий | К, Ц |
| **EE authn keypair** | Secret `d8-remote-authn-keypair` | Spoofing / Information Disclosure | УБИ.4, УБИ.1 | Угроза подмены; угроза утечки информации | Внутренний нарушитель | Высокий | К, Ц |
| **EE alliance ingressgateway** | Межкластерные ports 15443/15012/15017 | Denial of Service / Spoofing | УБИ.6, УБИ.4 | Угроза отказа в обслуживании; угроза подмены | Внешний/удалённый нарушитель | Средний | Ц, Д |
| **Operator** | Reconcile Istio/Sail resources and webhooks | Tampering / Repudiation | УБИ.3 | Угроза несанкционированной модификации | Внутренний нарушитель | Средний | Ц, Д |
| **Registry/update flow** | Deckhouse images, pull secrets, release channel | Tampering | УБИ.9, УБИ.3 | Угроза получения ресурсов из недоверенного или скомпрометированного источника; угроза модификации | Внешний поставщик/внутренний нарушитель сборки | Высокий | К, Ц, Д |
| **Build flow** | `SOURCE_REPO`, `GOPROXY`, `CARGO_PROXY`, Bazel deps/cache, patches | Tampering / Elevation of Privilege | УБИ.9, УБИ.3, УБИ.2 | Угроза получения ресурсов из недоверенного источника; угроза модификации; угроза несанкционированного доступа | Внешний поставщик, скомпрометированный mirror, внутренний нарушитель | Высокий | К, Ц, Д |
| **Build flow** | Отсутствие полного SBOM/VEX, VEX подключён не для всех образов, Bazel `external.tar.gz` как black-box dependency set | Repudiation | УБИ.3, УБИ.6 | Угроза модификации; угроза отказа в обслуживании при эксплуатации известных уязвимостей | Внутренний нарушитель/внешний поставщик | Средний | Ц, Д |
| **Runtime provenance metadata** | `ISTIO_META_ISTIO_PROXY_SHA` в `proxyv2` image spec | Repudiation / Tampering | УБИ.3 | Угроза несанкционированной модификации и затруднение расследования | Внутренний нарушитель supply chain | Средний | Ц |

**Покрытие STRIDE:**

| STRIDE | Покрытые элементы | Вывод |
| --- | --- | --- |
| Spoofing | xDS/SDS identity, service account tokens, remote metadata, Kiali/API auth, Gateway/SNI routing | Риск связан с подменой workload identity, remote cluster identity, client identity и маршрута. |
| Tampering | Istio CRD, EnvoyFilter/WasmPlugin, CNI hostPath, xDS config, registry/build artifacts, remote metadata | Риск связан с изменением mesh policy/routing, сетевых правил ноды и supply-chain артефактов. |
| Repudiation | Kubernetes API действия, operator reconcile, build/update flow, audit of Secret access and metrics/logs | Требует уточнения полноты Kubernetes audit, CI/CD provenance и журналирования доступа к секретам. |
| Information Disclosure | CA private key, service account tokens, remote kubeconfigs, Kiali topology, metrics, logs, traces | Риск связан с раскрытием trust material, топологии mesh и межкластерных данных. |
| Denial of Service | Public gateways, `istiod`, webhooks, CNI/ztunnel, API proxy, Prometheus/telemetry paths | Риск связан с публичным трафиком, блокирующими webhook и privileged dataplane на нодах. |
| Elevation of Privilege | EnvoyFilter/WasmPlugin, CNI/ztunnel capabilities, CA issuance, remote kubeconfig, operator RBAC | Риск связан с преобразованием namespace-level или workload-level доступа в control-plane/node-level воздействие. |

## 5. Моделирование сценариев атак

### Сценарий AS-01. Отказ в обслуживании публичного Istio ingress gateway

| Параметр | Значение |
| --- | --- |
| **ID сценария** | AS-01 |
| **Связанная угроза** | УБИ.6, УБИ.8 |
| **Элемент поверхности атаки** | Ingress gateway public ports 8080/8443/HostPort/LoadBalancer, Envoy listeners, readiness 15021 |
| **Источник угрозы** | Внешний нарушитель |
| **Начальный уровень доступа** | Неаутентифицированный сетевой клиент с доступом к published gateway |
| **Вектор атаки** | Множество TLS/HTTP/gRPC соединений, большие headers/body, удержание HTTP/2 streams, SNI/path enumeration, overload upstream services |
| **Используемая уязвимость** | Архитектурная открытость gateway к внешнему трафику; фактические rate limits, WAF/LB protections и resource limits требуют уточнения |
| **Краткая последовательность действий** | 1. Нарушитель определяет публичные host/SNI/gateway endpoint. 2. Генерирует ресурсоёмкие запросы или множество соединений. 3. Envoy/gateway и upstream workloads потребляют CPU, memory, connection pools или worker capacity. 4. Пользователи получают ошибки, timeout или degraded mesh traffic. |
| **Последствия** | Нарушение доступности приложений за gateway, частичная деградация mesh и возможная нагрузка на `istiod` через retries/config updates. |

### Сценарий AS-02. Подмена или несанкционированное изменение маршрутизации через Istio CRD

| Параметр | Значение |
| --- | --- |
| **ID сценария** | AS-02 |
| **Связанная угроза** | УБИ.3, УБИ.4 |
| **Элемент поверхности атаки** | `VirtualService`, `DestinationRule`, `Gateway`, `ServiceEntry`, Gateway API resources |
| **Источник угрозы** | Пользователь Kubernetes namespace или внутренний нарушитель |
| **Начальный уровень доступа** | RBAC на создание/изменение routing resources в namespace или cluster-scoped gateway resources |
| **Вектор атаки** | Создание маршрута на недоверенный backend, изменение weights, SNI/host/path match, добавление ServiceEntry для внешнего адреса |
| **Используемая уязвимость** | Доверие `istiod` к Kubernetes API как источнику конфигурации; недостаточная сегментация RBAC или policy review на Istio CRD |
| **Краткая последовательность действий** | 1. Нарушитель создаёт или изменяет Istio routing object. 2. `istiod` валидирует и распространяет конфигурацию через xDS. 3. Envoy sidecar/gateway применяет изменённые routes/clusters. 4. Трафик направляется в нежелательный backend или за пределы mesh. |
| **Последствия** | Нарушение целостности маршрутизации, обход intended policy, утечка данных через внешний сервис, отказ отдельных приложений. |

### Сценарий AS-03. Обход mTLS/Authz через некорректную Authn/Authz policy

| Параметр | Значение |
| --- | --- |
| **ID сценария** | AS-03 |
| **Связанная угроза** | УБИ.2, УБИ.3 |
| **Элемент поверхности атаки** | `PeerAuthentication`, `AuthorizationPolicy`, `RequestAuthentication`, namespace labels, workload selectors |
| **Источник угрозы** | Пользователь Kubernetes namespace или внутренний нарушитель |
| **Начальный уровень доступа** | Право изменять Istio security policy в namespace или cluster/root namespace |
| **Вектор атаки** | Ослабление mTLS mode, удаление/изменение deny/allow policy, расширение selector, некорректные JWT issuer/audience rules |
| **Используемая уязвимость** | Ошибка RBAC/ownership модели для security policy или недостаточная validation business semantics |
| **Краткая последовательность действий** | 1. Нарушитель меняет policy, влияющую на workload или gateway. 2. `istiod` распространяет новую policy. 3. Envoy начинает принимать трафик без ожидаемой проверки identity/JWT/mTLS. 4. Нарушитель обращается к сервису с расширенными полномочиями. |
| **Последствия** | Несанкционированный доступ к внутренним сервисам, нарушение конфиденциальности и целостности данных. |

### Сценарий AS-04. Опасное расширение Envoy через `EnvoyFilter` или `WasmPlugin`

| Параметр | Значение |
| --- | --- |
| **ID сценария** | AS-04 |
| **Связанная угроза** | УБИ.2, УБИ.3, УБИ.9 |
| **Элемент поверхности атаки** | `EnvoyFilter`, `WasmPlugin`, xDS extension configuration |
| **Источник угрозы** | Пользователь Kubernetes с правом менять Istio extension resources |
| **Начальный уровень доступа** | Ограниченный или привилегированный Kubernetes RBAC на Istio extensions |
| **Вектор атаки** | Внедрение фильтра, меняющего headers/body/auth decision; загрузка недоверенного Wasm artifact; некорректное изменение listener/filter chain |
| **Используемая уязвимость** | Высокие полномочия extension resources над dataplane; отсутствие полного allowlist/review для EnvoyFilter/WasmPlugin требует уточнения |
| **Краткая последовательность действий** | 1. Нарушитель создаёт extension resource. 2. `istiod` транслирует его в xDS. 3. Envoy загружает или применяет фильтр. 4. Фильтр изменяет трафик, раскрывает данные или нарушает доступность. |
| **Последствия** | Компрометация трафика и policy enforcement в затронутом workload/gateway; возможный отказ Envoy. |

### Сценарий AS-05. Компрометация Istio CA и workload identity

| Параметр | Значение |
| --- | --- |
| **ID сценария** | AS-05 |
| **Связанная угроза** | УБИ.1, УБИ.2, УБИ.4 |
| **Элемент поверхности атаки** | Secret `cacerts`, `istio-ca-root-cert`, projected tokens, `istiod` CA issuance |
| **Источник угрозы** | Внутренний нарушитель |
| **Начальный уровень доступа** | Доступ к Secret, Pod/ServiceAccount token, backup, node filesystem или privileged component |
| **Вектор атаки** | Чтение CA private key, cert chain, root CA или service account token; выпуск поддельного workload certificate |
| **Используемая уязвимость** | Высокая ценность CA Secret и service account tokens; эксплуатация требует дополнительного доступа к Kubernetes Secret/Pod/node |
| **Краткая последовательность действий** | 1. Нарушитель получает доступ к Secret или Pod с trust material. 2. Извлекает CA key/token/cert. 3. Выпускает или имитирует workload identity. 4. Подключается к mTLS-сервисам как доверенный workload. |
| **Последствия** | Полная или частичная компрометация доверия mesh, подмена workload, раскрытие и модификация межсервисного трафика. |

### Сценарий AS-06. Нарушение доступности или целостности через sidecar injection/validation webhook

| Параметр | Значение |
| --- | --- |
| **ID сценария** | AS-06 |
| **Связанная угроза** | УБИ.3, УБИ.6 |
| **Элемент поверхности атаки** | MutatingWebhook `/inject`, ValidatingWebhook `/validate`, namespace/pod selectors, `failurePolicy: Fail` |
| **Источник угрозы** | Пользователь Kubernetes namespace или внутренний нарушитель |
| **Начальный уровень доступа** | Право создавать Pods/Istio resources или воздействовать на webhook availability |
| **Вектор атаки** | Создание объектов, вызывающих длительную обработку; изменение labels для injection bypass; недоступность `istiod` service; некорректная revision selection |
| **Используемая уязвимость** | Безопасность и availability зависят от доступности `istiod`; `failurePolicy: Fail` защищает от silent bypass, но может блокировать workload deployment |
| **Краткая последовательность действий** | 1. Нарушитель создаёт Pod/Istio object, попадающий под webhook. 2. Добивается ошибки или таймаута webhook. 3. Kubernetes API блокирует операции либо пропускает объект в непредусмотренной revision-схеме. 4. Workload не разворачивается или разворачивается вне ожидаемой mesh-policy. |
| **Последствия** | Нарушение доступности deployment pipeline, неполная защита workload sidecar/mTLS, нарушение целостности конфигурации. |

### Сценарий AS-07. Компрометация ноды через Istio CNI

| Параметр | Значение |
| --- | --- |
| **ID сценария** | AS-07 |
| **Связанная угроза** | УБИ.2, УБИ.3, УБИ.5 |
| **Элемент поверхности атаки** | `istio-cni-node`, hostPath `/opt/cni/bin`, `/etc/cni/net.d`, `/var/run/netns`, capabilities `NET_ADMIN`, `NET_RAW`, `SYS_PTRACE`, `SYS_ADMIN`, `DAC_OVERRIDE` |
| **Источник угрозы** | Внутренний нарушитель |
| **Начальный уровень доступа** | Возможность выполнить код в CNI Pod, изменить CNI image/config или получить privileged workload на ноде |
| **Вектор атаки** | Модификация CNI binary/config, iptables/ipset/netns, pod repair behavior, hostPath content |
| **Используемая уязвимость** | CNI по назначению имеет node-level hostPath и сетевые capabilities; эксплуатация требует предварительной компрометации или supply-chain воздействия |
| **Краткая последовательность действий** | 1. Нарушитель получает исполнение в CNI context или подменяет artifact. 2. Меняет CNI config/binary или сетевые правила. 3. Новые Pod получают некорректное перенаправление трафика. 4. Трафик перехватывается, блокируется или выводится из mesh. |
| **Последствия** | Компрометация ноды и workload traffic, обход mTLS/policy, отказ workloads на ноде. |

### Сценарий AS-08. Компрометация ambient dataplane через ztunnel

| Параметр | Значение |
| --- | --- |
| **ID сценария** | AS-08 |
| **Связанная угроза** | УБИ.2, УБИ.3, УБИ.6 |
| **Элемент поверхности атаки** | ztunnel DaemonSet, HBONE/TPROXY, `/var/run/ztunnel`, root + `NET_ADMIN`/`SYS_ADMIN`/`NET_RAW` |
| **Источник угрозы** | Внутренний нарушитель или скомпрометированный workload при включённом ambient |
| **Начальный уровень доступа** | Доступ к workload traffic, ztunnel Pod, hostPath socket или node-level privileges |
| **Вектор атаки** | Воздействие на ztunnel socket, overload HBONE path, эксплуатация ошибки обработки traffic/identity, подмена token/workload metadata |
| **Используемая уязвимость** | ztunnel является privileged dataplane-компонентом; фактическая применимость зависит от `ambient.enabled` |
| **Краткая последовательность действий** | 1. Нарушитель определяет workload в ambient mode. 2. Воздействует на ztunnel traffic path или socket. 3. ztunnel неверно маршрутизирует, блокирует или раскрывает трафик. 4. Нарушается mTLS/HBONE dataplane или availability. |
| **Последствия** | Нарушение конфиденциальности, целостности или доступности ambient workloads и межсервисного трафика. |

### Сценарий AS-09. Раскрытие topology, traffic и policy через Kiali, метрики, логи и трассировки

| Параметр | Значение |
| --- | --- |
| **ID сценария** | AS-09 |
| **Связанная угроза** | УБИ.1, УБИ.11 |
| **Элемент поверхности атаки** | Kiali UI/API, Prometheus metrics, access logs, Zipkin tracing, Envoy metrics 15090, PodMonitor endpoints |
| **Источник угрозы** | Внешний нарушитель при опубликованном UI или внутренний нарушитель |
| **Начальный уровень доступа** | Доступ к Kiali, Prometheus, logs/traces или kube-rbac-proxy-authorized endpoint |
| **Вектор атаки** | Массовый сбор namespace/workload/service labels, route names, policy status, traffic graph, trace metadata, access log fields |
| **Используемая уязвимость** | Observability data содержит топологию и traffic metadata; фактические auth/source ranges и log redaction требуют уточнения |
| **Краткая последовательность действий** | 1. Нарушитель получает доступ к UI/metrics/logs/traces. 2. Собирает карту сервисов, маршрутов, политик и traffic flows. 3. Использует сведения для выбора целей и дальнейших атак. |
| **Последствия** | Раскрытие внутренней архитектуры mesh, идентификаторов сервисов и traffic metadata; повышение эффективности последующих атак. |

### Сценарий AS-10. Подмена remote metadata и доверия в federation/multicluster

| Параметр | Значение |
| --- | --- |
| **ID сценария** | AS-10 |
| **Связанная угроза** | УБИ.4, УБИ.9, УБИ.3 |
| **Элемент поверхности атаки** | `IstioFederation`, `IstioMulticluster`, `metadataEndpoint`, `ca`, `insecureSkipVerify`, metadata exporter |
| **Источник угрозы** | Внешний нарушитель, скомпрометированный remote cluster или внутренний нарушитель |
| **Начальный уровень доступа** | Возможность управлять remote metadata endpoint, MITM-сетью или CRD config |
| **Вектор атаки** | Подмена root CA, authn public key, private metadata, ingress gateway address/API host; использование `insecureSkipVerify` |
| **Используемая уязвимость** | Доверие к удалённому HTTPS endpoint; CRD допускает отключение проверки TLS; фактическое использование требует уточнения |
| **Краткая последовательность действий** | 1. Нарушитель подменяет metadata endpoint или TLS identity. 2. Локальный кластер загружает ложную metadata. 3. Модуль формирует meshNetworks/remote secrets/trust config на основании ложных данных. 4. Межкластерный трафик направляется на недоверенный endpoint или доверяет неверному CA/key. |
| **Последствия** | Нарушение целостности межкластерного доверия, перехват/подмена межкластерного трафика, отказ federation/multicluster. |

### Сценарий AS-11. Компрометация remote kubeconfig или remote authn keypair

| Параметр | Значение |
| --- | --- |
| **ID сценария** | AS-11 |
| **Связанная угроза** | УБИ.1, УБИ.2, УБИ.4 |
| **Элемент поверхности атаки** | Secret `istio-remote-secret-*`, Secret `d8-remote-authn-keypair`, API proxy secret volumes |
| **Источник угрозы** | Внутренний нарушитель |
| **Начальный уровень доступа** | Доступ к Kubernetes Secret, Pod volume, backup или service account с правами чтения |
| **Вектор атаки** | Чтение kubeconfig token/CA или private key, использование для remote API access или подписи/имитации remote identity |
| **Используемая уязвимость** | Высокая чувствительность межкластерных секретов; эксплуатация требует доступа к `d8-istio` Secret/Pod |
| **Краткая последовательность действий** | 1. Нарушитель получает доступ к секрету. 2. Извлекает remote API token или private key. 3. Использует его для обращения к remote API или подмены межкластерной аутентификации. 4. Расширяет доступ за пределы локального кластера. |
| **Последствия** | Компрометация remote cluster trust, несанкционированный межкластерный доступ, раскрытие или изменение данных. |

### Сценарий AS-12. Компрометация supply chain образов Istio/Envoy/ztunnel/operator

| Параметр | Значение |
| --- | --- |
| **ID сценария** | AS-12 |
| **Связанная угроза** | УБИ.9, УБИ.3, УБИ.2 |
| **Элемент поверхности атаки** | `werf.inc.yaml`, `SOURCE_REPO`, `GOPROXY`, `CARGO_PROXY`, Bazel cache/deps, patches, registry images, missing full SBOM |
| **Источник угрозы** | Внешний поставщик, скомпрометированный mirror/repository, внутренний нарушитель сборочной среды |
| **Начальный уровень доступа** | Доступ к source mirror, build cache, dependency proxy, patch directory, registry или CI secrets |
| **Вектор атаки** | Подмена Go/Rust/Bazel dependency, Envoy source archive/cache, local patch, base image, registry artifact или build secret |
| **Используемая уязвимость** | Сложная цепочка C++/Go/Rust/Bazel зависимостей; неполный SBOM/VEX; сборка использует внешние секреты и зеркала |
| **Краткая последовательность действий** | 1. Нарушитель внедряет изменение в dependency/source/cache/patch. 2. Werf build собирает изменённый бинарь или image. 3. Образ публикуется и разворачивается DKP. 4. Вредоносный код выполняется в `istiod`, Envoy, CNI, ztunnel или operator context. |
| **Последствия** | Потенциальная полная компрометация control plane, dataplane, нод, секретов и межкластерного доверия. |

## 6. Оценка актуальности и формирование модели угроз

| ID угрозы | ID сценария | Наличие уязвимости | Реализуемость сценария | Потенциальный ущерб | Итоговая категория | Решение | Обоснование |
| --- | --- | --- | --- | --- | --- | --- | --- |
| **УБИ.6, УБИ.8** | AS-01 | Да | Высокая | Высокий | Высокий | **Актуальна** | Public gateway по назначению принимает недоверенный трафик; DoS возможен без специальных прав при отсутствии достаточных LB/rate limits. |
| **УБИ.3, УБИ.4** | AS-02 | Да | Средняя | Высокий | Высокий | **Актуальна** | Routing resources являются штатным управляющим интерфейсом mesh; ущерб зависит от RBAC и ownership политики. |
| **УБИ.2, УБИ.3** | AS-03 | Да | Средняя | Высокий | Высокий | **Актуальна** | Authn/Authz policy напрямую влияет на доступ к сервисам; validation синтаксиса не исключает логически опасную policy. |
| **УБИ.2, УБИ.3, УБИ.9** | AS-04 | Требует уточнения | Средняя | Высокий/Критический | Высокий | **Условно актуальна** | Реализуемость зависит от выдачи RBAC на `EnvoyFilter`/`WasmPlugin` и политики загрузки Wasm artifacts; при наличии прав последствия значительны. |
| **УБИ.1, УБИ.2, УБИ.4** | AS-05 | Да | Средняя | Критический | Критический | **Актуальна** | `cacerts` содержит CA private key; компрометация нарушает доверие всего mesh. |
| **УБИ.3, УБИ.6** | AS-06 | Да | Средняя | Средний/Высокий | Высокий | **Актуальна** | Webhook с `failurePolicy: Fail` защищает целостность, но его отказ блокирует deployment и validation operations. |
| **УБИ.2, УБИ.3, УБИ.5** | AS-07 | Да | Низкая/Средняя | Критический | Критический | **Актуальна** | CNI имеет hostPath и capabilities для изменения сетевой конфигурации ноды; эксплуатация требует внутреннего или supply-chain доступа, но последствия критичны. |
| **УБИ.2, УБИ.3, УБИ.6** | AS-08 | Требует уточнения | Средняя при ambient | Критический | Критический | **Условно актуальна** | Применимость зависит от `ambient.enabled`; ztunnel требует root и сетевые capabilities, поэтому последствия компрометации высокие. |
| **УБИ.1, УБИ.11** | AS-09 | Да | Средняя | Средний | Средний | **Актуальна** | Kiali, metrics, logs and traces реально содержат topology и traffic metadata; фактическая внешняя достижимость требует уточнения. |
| **УБИ.4, УБИ.9, УБИ.3** | AS-10 | Да | Средняя | Высокий | Высокий | **Условно актуальна** | Реализуемость зависит от включения EE federation/multicluster и `insecureSkipVerify`; CRD допускает этот режим. |
| **УБИ.1, УБИ.2, УБИ.4** | AS-11 | Да | Низкая/Средняя | Критический | Критический | **Условно актуальна** | Актуально при включении EE federation/multicluster; remote kubeconfig/keypair являются критичными секретами. |
| **УБИ.9, УБИ.3, УБИ.2** | AS-12 | Да | Средняя | Критический | Критический | **Актуальна** | Сборка включает SOURCE_REPO, GOPROXY, CARGO_PROXY, Bazel cache/deps, локальные patches и множество native dependencies; полный SBOM не обнаружен. |

**Итоговая модель актуальных угроз:**

| ID | Угроза | Актуальность | Основные компоненты | Приоритет нейтрализации |
| --- | --- | --- | --- | --- |
| TM-01 | Отказ в обслуживании публичного Istio ingress gateway | Актуальна | Ingress gateway, Envoy, LB, backend workloads | Высокий |
| TM-02 | Подмена или несанкционированное изменение маршрутизации mesh | Актуальна | Istio CRD, `istiod`, Envoy sidecar/gateway | Высокий |
| TM-03 | Обход mTLS/Authz через некорректную security policy | Актуальна | `AuthorizationPolicy`, `PeerAuthentication`, `RequestAuthentication`, Envoy | Высокий |
| TM-04 | Опасное расширение dataplane через `EnvoyFilter`/`WasmPlugin` | Условно актуальна | `istiod`, Envoy, extension resources | Высокий |
| TM-05 | Компрометация Istio CA и workload identity | Актуальна | Secret `cacerts`, `istiod`, service account tokens, workload certs | Критический |
| TM-06 | Отказ или обход sidecar injection/validation webhook | Актуальна | MutatingWebhook, ValidatingWebhook, `istiod`, Kubernetes API | Высокий |
| TM-07 | Компрометация ноды и traffic redirection через Istio CNI | Актуальна | `istio-cni-node`, hostPath, iptables/ipset/netns | Критический |
| TM-08 | Компрометация ambient dataplane через ztunnel | Условно актуальна | ztunnel, CNI, HBONE, `/var/run/ztunnel` | Критический при ambient |
| TM-09 | Раскрытие topology/traffic/policy через observability | Актуальна | Kiali, Prometheus, metrics, logs, tracing | Средний |
| TM-10 | Подмена remote metadata и межкластерного доверия | Условно актуальна | EE federation/multicluster, metadata exporter, remote endpoint | Высокий при EE |
| TM-11 | Компрометация remote kubeconfig и remote authn keypair | Условно актуальна | EE Secrets, API proxy, metadata hooks | Критический при EE |
| TM-12 | Компрометация цепочки поставки образов Istio/Envoy/ztunnel/operator | Актуальна | werf build, SOURCE_REPO, GOPROXY, CARGO_PROXY, Bazel, registry | Критический |

**Меры по нейтрализации:**

| Угроза | Приоритет | Рекомендуемые меры |
| --- | --- | --- |
| TM-01 | Высокий | Настроить rate limiting/WAF/LB limits для gateway; контролировать Envoy circuit breakers, request body/header limits, HTTP/2 concurrency, resource limits и autoscaling; проводить DoS-safe нагрузочные тесты gateway. |
| TM-02 | Высокий | Ограничить RBAC на Istio routing CRD по namespace ownership; применять policy review/OPA/Kyverno для `VirtualService`, `DestinationRule`, `Gateway`, `ServiceEntry`; включить тесты на недопустимые host/SNI/egress routes. |
| TM-03 | Высокий | Ограничить изменение Authn/Authz policy; задать baseline `PeerAuthentication` STRICT, где применимо; ревью `AuthorizationPolicy` deny/allow; тестировать negative access cases. |
| TM-04 | Высокий | Ограничить или запретить `EnvoyFilter`/`WasmPlugin` для обычных namespace; использовать allowlist trusted Wasm artifacts и registry; проводить экспертный review extension resources и fuzz/regression tests для Envoy config. |
| TM-05 | Критический | Минимизировать доступ к Secret `cacerts`; включить audit Secret access; защищать backups; ограничить exec/debug в `d8-istio`; обеспечить ротацию CA/workload certs и процедуру отзыва при компрометации. |
| TM-06 | Высокий | Мониторить availability и latency webhooks; проверять `failurePolicy`, `timeoutSeconds`, HA replicas; тестировать injection/validation для разных revision labels; алертить ошибки admission. |
| TM-07 | Критический | Ограничить доступ к CNI DaemonSet и hostPath; контролировать целостность CNI binary/config; мониторить изменения `/etc/cni/net.d`, iptables/ipset; проверять supply-chain CNI image; ограничить privileged workloads на нодах. |
| TM-08 | Критический при ambient | Включать ambient только при необходимости; ограничить доступ к ztunnel Pod/socket; мониторить ztunnel health/error rates; тестировать HBONE/TPROXY failure cases; ревью `SYS_ADMIN`/`NET_ADMIN` blast radius. |
| TM-09 | Средний | Ограничить доступ к Kiali, Prometheus, logs and traces по RBAC; проверить external auth/source ranges; минимизировать чувствительные access log fields; установить retention и audit доступа к observability. |
| TM-10 | Высокий при EE | Запрещать `insecureSkipVerify` без documented risk acceptance; указывать CA для remote metadata; проверять подпись/схему metadata; ограничить изменение `IstioFederation`/`IstioMulticluster`; мониторить metadata readiness conditions. |
| TM-11 | Критический при EE | Ограничить чтение `istio-remote-secret-*` и `d8-remote-authn-keypair`; шифровать Secrets at rest; audit Secret access; обеспечить ротацию remote tokens/keypair; отделить права API proxy/exporters; пересмотреть необходимость `secrets` get/list/watch у EE `api-proxy`. |
| TM-12 | Критический | Сформировать полный SBOM/VEX для всех образов; подключить VEX к соответствующим runtime images; закреплять versions/commits/checksums; защищать SOURCE_REPO/GOPROXY/CARGO_PROXY/Bazel cache and `external.tar.gz`; проверять подписи/provenance образов; актуализировать `ISTIO_META_ISTIO_PROXY_SHA`; выполнять SCA/SAST и review patches. |

**Компоненты, подлежащие тестированию:**

| Компонент | Виды тестирования | Связанные угрозы | Цель тестирования |
| --- | --- | --- | --- |
| Ingress gateway / Envoy router | DAST, нагрузочные и регрессионные тесты | TM-01, TM-02 | Устойчивость HTTP/HTTPS/gRPC/TLS/SNI processing, rate/connection limits, корректность routing. |
| Envoy sidecar / `proxyv2` | Fuzzing, regression, config validation tests | TM-02, TM-03, TM-04 | Проверка обработки xDS, Authn/Authz, EnvoyFilter/WasmPlugin и egress behavior. |
| `istiod` / Pilot | Fuzzing, unit/regression, admission tests | TM-02, TM-03, TM-05, TM-06 | Проверка validation, injection, CA issuance, xDS/SDS authorization и отказоустойчивости. |
| Mutating/Validating webhooks | DAST/admission regression | TM-06 | Проверка `failurePolicy: Fail`, timeout, revision labels, bypass через namespace/pod selectors. |
| Istio CRD/RBAC policies | Configuration review, policy tests | TM-02, TM-03, TM-04 | Проверка разделения прав на routing/security/extensions resources. |
| Istio CNI | Privileged-container tests, regression, host integration tests | TM-07 | Проверка CNI config, hostPath writes, iptables/ipset/netns, repair behavior и recovery. |
| ztunnel / ambient | DAST/internal traffic tests, regression, fuzzing where applicable | TM-08 | Проверка HBONE/TPROXY, socket access, workload identity, DoS and failover behavior. |
| Kiali / kube-rbac-proxy | DAST, authz regression | TM-09 | Проверка auth, RBAC SubjectAccessReview, exposure, sensitive topology disclosure. |
| Prometheus/logging/tracing | Configuration review, privacy tests | TM-09 | Проверка labels/log fields/traces на раскрытие sensitive topology and request metadata. |
| EE metadata exporter/API proxy/metrics exporter | DAST, authn/authz regression, fuzzing JSON metadata | TM-10, TM-11 | Проверка remote metadata validation, JWT/auth, HTTPS CA, `insecureSkipVerify`, API proxy behavior. |
| EE remote Secrets/keypair hooks | Unit/integration, Secret access audit review | TM-10, TM-11 | Проверка генерации, хранения, ротации и использования remote kubeconfig/keypair. |
| Build/update flow | SCA/SAST, supply-chain review, reproducibility/provenance tests | TM-12 | Проверка SOURCE_REPO/GOPROXY/CARGO_PROXY/Bazel deps, patches, checksums, SBOM/VEX and image signatures. |

**Требует уточнения:**

| Вопрос | Влияние на модель |
| --- | --- |
| Фактические module values: `ambient.enabled`, `trafficRedirectionSetupMode`, `outboundTrafficPolicyMode`, tracing, access logs, Kiali/public exposure, ingress gateway inlet/hostPort. | Определяет перевод условных угроз TM-04, TM-08, TM-10, TM-11 и части TM-09 в актуальные или неактуальные для конкретного контура. |
| Наличие NetworkPolicy, внешних LB/WAF/rate limits/source ranges и ingress auth configuration. | Влияет на реализуемость DoS, UI/API exposure и доступность внутренних endpoint. |
| Фактический RBAC пользователей namespace и администраторов на Istio CRD, Gateway API, namespace labels и EE CRD. | Влияет на реализуемость TM-02, TM-03, TM-04, TM-10. |
| Полный SBOM/VEX всех образов, подключение VEX к werf/runtime images, manifest Bazel `external.tar.gz` и provenance сборки. | Необходимы для точной оценки известных CVE и supply-chain риска TM-12. |
| Audit Kubernetes API, Secret access, CI/CD, Registry and observability access. | Нужен для оценки Repudiation, расследования инцидентов и остаточного риска TM-05/TM-09/TM-11/TM-12. |
| Политика использования `insecureSkipVerify` в federation/multicluster. | Критична для оценки подмены remote metadata и межкластерного доверия. |
| Отрендеренные RBAC и webhook policies в конкретном кластере, включая стартовое состояние upstream validation webhook `Ignore -> Fail`. | Влияет на окно обхода/ослабления validation и минимальность прав Kiali, ztunnel, api-proxy. |

**План проверки безопасности:**

| Направление | Проверки |
| --- | --- |
| DAST/penetration testing | Gateway DoS-safe tests, SNI/Host/path routing, Authz bypass negative tests, Kiali/API proxy auth, remote metadata endpoint validation. |
| Fuzzing | AdmissionReview objects, Istio CRD parsing, Envoy config generation, metadata JSON, proxy/ztunnel protocol paths where supported. |
| Code review | Deckhouse templates, RBAC, webhook configs, CNI/ztunnel securityContext, EE API proxy/exporters, local patches, werf build scripts. |
| Configuration review | Module values, namespace labels, Istio CRD ownership, `outboundTrafficPolicyMode`, NetworkPolicy, Kiali exposure, tracing/logging, `insecureSkipVerify`. |
| Supply-chain review | SBOM/VEX completeness, dependency pinning, SOURCE_REPO/GOPROXY/CARGO_PROXY trust, Bazel cache/deps, image signing/provenance and patch integrity. |

при моделировании угроз использовался ИИ агент GPT-5.5
