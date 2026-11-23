## Версия 1.78

### Основные изменения

Ключевые изменения версии:

- [Deckhouse Engine](#release-1-78-engine) — гипервизор первого типа для узлов Deckhouse Platform с неизменяемой корневой файловой системой и декларативным управлением.
- [Управление bare-metal-узлами через BMC](#release-1-78-cloud-providers) — новый модуль [`infra-provider-baremetal`](https://deckhouse.ru/modules/infra-provider-baremetal/v1.78/) поддерживает создание узлов на физических серверах.
- [Новая реализация registry](#release-1-78-registry) — источники образов и ключи доступа проверяются до применения изменений. Добавлены восстановление загрузки образов при недоступном Deckhouse Platform и отказоустойчивый внутрикластерный кеш с бесшовным переключением при работе в закрытых окружениях.
- [Автоматический подбор resources requests компонентов control plane](#release-1-78-control-plane) — запросы CPU и памяти для etcd, `kube-apiserver`, `kube-controller-manager` и `kube-scheduler` рассчитываются по фактическому потреблению.
- [Переработанный fencing](#release-1-78-fencing) — нагрузка переносится с отказавшего узла только после подтверждения его остановки. Это защищает приложения StatefulSet и поды с томами ReadWriteOnce от одновременной работы на двух узлах.
- [Изменение resources requests без пересоздания подов](#release-1-78-vpa) — режим по умолчанию для новых объектов VPA. Если изменить resources requests без пересоздания невозможно, под пересоздаётся.
- [Обновлённая ролевая модель](#release-1-78-iam) — новый состав подсистем и изменения прав менеджеров платформы.
- [Управление сессиями пользователей](#release-1-78-sessions) — просмотр активных сессий и завершение отдельных сессий или всех сессий пользователя.
- [Управление проектами](#release-1-78-projects) — структурные шаблоны, а в коммерческих редакциях — несколько неймспейсов в одном проекте с общей квотой.
- [TokenExchangeProvider](#release-1-78-token-exchange) — доступ внешних систем, например CI/CD, к API Kubernetes через обмен токенов доверенного издателя без статических учётных данных.
- [ClusterMesh](#release-1-78-clustermesh) — экспериментальная поддержка прямого сетевого взаимодействия подов между кластерами.
- [Kubernetes 1.37 (preview)](#release-1-78-kubernetes) и [Istio 1.29](#release-1-78-istio-versions) — новые поддерживаемые версии. Прекращена поддержка Kubernetes 1.32. Версия Kubernetes по умолчанию изменена с 1.34 на 1.35.

### Обновление до 1.78

Проверьте условия обновления и выполните подготовку для используемых компонентов и интеграций.
При планировании работ учтите перезапуски и временные ограничения, а после обновления выполните миграции и проверки, перечисленные ниже.

Оглавление раздела:

- [Влияние на кластер](#release-1-78-upgrade-impact)
- [Условия и ограничения обновления](#release-1-78-upgrade-requirements)
- [Подготовка к обновлению](#release-1-78-before-upgrade)
- [Проверки и миграции после обновления](#release-1-78-after-upgrade)

#### Влияние на кластер {#release-1-78-upgrade-impact}

При выборе времени обновления учтите следующие изменения:

- При обновлении перезапустятся все компоненты Deckhouse Platform.

- Из-за перехода на профили Topology Manager kubelet перезапустится на всех узлах, а доступная подам память (allocatable) на них немного изменится. На узлах с политикой, отличной от `None`, на которых работают Guaranteed-поды, будет выполнен drain по правилам `disruptions` своей NodeGroup. В режиме `Manual` drain начнётся только после подтверждения. Подробнее — [о профилях Topology Manager](#release-1-78-topology-manager).

- На узлах с containerd v1 при миграции registry возможны перезапуски containerd и kubelet. Число перезапусков зависит от исходного режима и последующего включения `Managed`.
  Миграция из `Direct` и миграция на узлах с containerd v2 не перезапускают containerd. Подробнее — [о перезапусках при миграции registry](https://deckhouse.ru/modules/registry/v1.78/faq.html#перезапускается-ли-containerd-при-миграции).

- Валидационные вебхуки модулей [`admission-policy-engine`](https://deckhouse.ru/modules/admission-policy-engine/v1.78/), [`user-authz`](https://deckhouse.ru/modules/user-authz/v1.78/) и [`user-authn`](https://deckhouse.ru/modules/user-authn/v1.78/) на 3–4 минуты будут недоступны.
  В это время не выполняются проверки этих вебхуков для политик безопасности, RBAC-объектов (включая защиту от эскалации привилегий), пользователей и групп. Запланируйте обновление на время, когда можно не менять эти объекты.

- Пользователям приложений за DexAuthenticator потребуется один раз войти заново. Подробнее — [об идентификаторах клиентов DexAuthenticator](#release-1-78-dexauthenticator-ids).

#### Условия и ограничения обновления {#release-1-78-upgrade-requirements}

Проверьте версии компонентов и готовность конфигурации к обновлению:

- Обновляйте Deckhouse Platform до 1.78 только с версии 1.77.1 или новее. В версии 1.77.1 появилась проверка готовности registry к миграции.

- Если кластер работает на Kubernetes 1.32, переведите его на Kubernetes 1.33 или новее. Подробнее — [о поддерживаемых версиях Kubernetes](#release-1-78-kubernetes).

- <a id="release-1-78-registry-migration"></a>Если кластер использует старую реализацию registry (режимы `Direct`, `Proxy` или `Local` в секции [`registry` ModuleConfig `deckhouse`](https://deckhouse.ru/modules/deckhouse/v1.78/configuration.html#parameters-registry)), подготовьте его к передаче управления модулю `registry` согласно [инструкции по миграции](https://deckhouse.ru/modules/registry/v1.78/faq.html#как-устроена-миграция-на-модуль-registry). Подготовку выполняйте на версии 1.77 до обновления. Модуль примет управление автоматически после обновления до 1.78. Для неподготовленного кластера обновление будет заблокировано. Подробнее — [о новой реализации registry](#release-1-78-registry).

- Если в кластере используются версии Istio ниже 1.25, переведите их на версию 1.25 или новее. Подробнее — [о поддерживаемых версиях Istio](#release-1-78-istio-versions).

- Если в кластере используется модуль [`sds-replicated-volume`](https://deckhouse.ru/modules/sds-replicated-volume/), обновите его до версии не ниже 0.9.5.

- Если в кластере используется модуль [`cloud-provider-dvp`](https://deckhouse.ru/modules/cloud-provider-dvp/), проверьте, что он доступен из [источника модулей (ModuleSource)](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/api/cr.html#modulesource). Иначе обновление невозможно. Подробнее — [о модулях из внешних источников](#release-1-78-external-modules).

- Если кластер развёрнут на версии Deckhouse Platform ниже 1.55, задайте в ModuleConfig [`admission-policy-engine`](https://deckhouse.ru/modules/admission-policy-engine/v1.78/) параметр [`settings.podSecurityStandards.defaultPolicy`](https://deckhouse.ru/modules/admission-policy-engine/v1.78/configuration.html#parameters-podsecuritystandards-defaultpolicy). Без него обновление будет заблокировано. Чтобы сохранить текущее поведение, укажите `Privileged`. Подробнее — [о политике Pod Security Standards по умолчанию](#release-1-78-pss-default).

- Если кластер развёрнут на Basis Dynamix, запланируйте его переустановку. Интеграция поддерживает только Basis Dynamix [версии 4.6 и выше](https://deckhouse.ru/modules/cloud-provider-dynamix/v1.78/environment.html#версия-платформы). В версии Deckhouse Platform 1.78 при удалении кластера на Basis Dynamix ресурсы, включая диски постоянных томов, удаляются безвозвратно, без перемещения в корзину. Сохраните нужные данные вне удаляемой инфраструктуры. Подробнее — [об изменениях интеграции с Basis Dynamix](#release-1-78-dynamix).

#### Подготовка к обновлению {#release-1-78-before-upgrade}

Выполните указанные проверки и действия для своей конфигурации, чтобы не нарушить работу компонентов или интеграций после обновления.

##### Сеть

Проверьте правила файрволов, NetworkPolicy и настройки прокси:

- Если трафик к master-узлам ограничен файрволом, откройте для хранилища образов registry порты `4281/TCP` и `4283/TCP` вместо `5001` и `5003`. Подробнее — [о портах хранилища образов](#release-1-78-registry-ports).

- Если в неймспейсе DexAuthenticator трафик ограничен NetworkPolicy, разрешите трафик от DexAuthenticator к сервису `revocation-feed` (поды `user-authn-controller`, порт `8443/TCP`) в неймспейсе `d8-user-authn`. Подробнее — [об обращении DexAuthenticator к `revocation-feed`](#release-1-78-revocation-feed).

- Если в ClusterConfiguration задан [`proxy`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/api/cr.html#clusterconfiguration-proxy) и у прокси нет доступа к адресу центра сертификатов или публичному адресу кластера, добавьте их в `noProxy`. Подробнее — [о работе `cert-manager` через прокси](#release-1-78-cert-manager-proxy).

##### Узлы

Проверьте настройки NodeGroup и зависимости узлов:

- Если в NodeGroup `master` есть taint с ключом из [`global.modules.placement.customTolerationKeys`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/api/global.html#parameters-modules-placement-customtolerationkeys), удалите этот ключ до обновления. Подробнее — [о размещении компонентов registry](#release-1-78-registry-tolerations).

- Если в AWS или Azure есть статические узлы (NodeGroup с `nodeType: Static`), проверьте, не зависят ли они от настроек облачного провайдера: `ecr-credential-provider`, модуля ядра EROFS или правил udev для дисков Azure NVMe. Подробнее — [о статических узлах в облаке](#release-1-78-static-cloud-nodes).

- Если master-узлы не имеют доступа к NTP-серверам, например в изолированных кластерах, укажите доступные `ntpServers` или отключите проверку параметром `earlyTimeSync.enabled: false`. Подробнее — [о синхронизации времени до запуска kubelet](#release-1-78-early-time-sync).

- Если в NodeGroup задан [`kubelet.memorySwap.swapBehavior: LimitedSwap`](https://deckhouse.ru/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-kubelet-memoryswap-swapbehavior), а на узлах группы нет cgroup v2, удалите `swapBehavior: LimitedSwap` из такой группы. Подробнее — [о валидации LimitedSwap](#release-1-78-limited-swap).

- Если в кластере включён fencing, откройте порты `8500/TCP` и `8500/UDP` между InternalIP-адресами всех узлов кластера.
  Для групп с режимом `Watchdog` проверьте, подходит ли время реакции режима `Slow`, на который они перейдут после обновления. Подробнее — [о fencing](#release-1-78-fencing).

- Если в NodeGroup задан параметр `spec.kubelet.topologyManager`, проверьте, как на группу повлияет переход на профили Topology Manager:
  - чтобы сохранить прежнее поведение групп с `enabled: false` и политикой, отличной от `None`, задайте `policy: None`;
  - чтобы сохранить закрепление CPU в группах с `enabled: true` и `policy: None`, задайте `policy: BestEffort`;
  - для политик `Restricted` и `SingleNumaNode` на узлах с SMT (Hyper-Threading) исправьте запросы Guaranteed-контейнеров с нечётным числом CPU.

  Подробнее — [о профилях Topology Manager](#release-1-78-topology-manager).

##### Политики безопасности и сертификаты

Проверьте нагрузки, политики и сертификаты, которые будут проходить изменённые проверки:

- Если в кластере есть нагрузки с `seLinuxOptions`, проверьте их и при необходимости оформите для них SecurityPolicyException. Подробнее — [об изменении проверки SELinux](#release-1-78-selinux).

- Если в кластере есть констрейнты без `match.kinds` или с `kinds: ["*"]`, которые не проверяют `input.review.operation`, проверьте, что они не запрещают exec и attach пользователям, которым эти операции нужны. Подробнее — [о политиках для `exec` и `attach`](#release-1-78-exec-attach).

- Если в настройках модуля [`istio`](https://deckhouse.ru/modules/istio/v1.78/) заданы сертификаты и ключ CA, проверьте их корректность. При некорректных данных модуль не сможет применить конфигурацию. Подробнее — [о проверке CA в Istio](#release-1-78-istio-ca).

##### Доступ пользователей и приложений

Проверьте права пользователей и совместимость интеграций:

- Если пользователям назначены роли `d8:system:manager`, `d8:subsystem:<SUBSYSTEM>:manager` или устаревшие `d8:manage:*:manager`, пересмотрите сценарии работы в неймспейсах модулей.
  После обновления эти роли дают там только уровень viewer: пользователи потеряют возможность читать секреты, выполнять `exec`, `attach`, `port-forward`, удалять и масштабировать поды.
  Для операций, выходящих за уровень viewer, потребуется роль superadmin подсистемы. Эта роль не снимает ограничения на операции с защищёнными объектами `heritage: deckhouse`, описанные в следующем пункте. Менеджеры системы и подсистемы `iam` также больше не смогут назначать уровни Admin, ClusterAdmin и SuperAdmin через ClusterAuthorizationRule.
  Подробнее — [об изменениях прав менеджеров](#release-1-78-iam).

- Если сценарии сопровождения используют `exec`, `attach`, `port-forward`, `debug` в поды с лейблом `heritage: deckhouse` или масштабируют рабочие нагрузки с этим лейблом, адаптируйте их.
  После обновления эти операции доступны только системным пользователям. Подробнее — [об ограничениях доступа](https://deckhouse.ru/modules/deckhouse/v1.78/faq.html#кто-может-выполнять-exec-в-поды-deckhouse-platform-и-масштабировать-объекты-deckhouse-platform).

- Если у DexClient есть аннотация `dexclient.deckhouse.io/allow-access-to-kubernetes`, удалите её там, где приложению не нужен доступ к API Kubernetes (кроме DexClient `cluster-<UUID>`, которые создаёт Deckhouse Commander). Найти такие DexClient можно командой:

  ```bash
  d8 k get dexclients.deckhouse.io -A -o json | jq -r '.items[] | select((.metadata.annotations["dexclient.deckhouse.io/allow-access-to-kubernetes"] // "") | ascii_downcase | IN("1","t","true")) | "\(.metadata.namespace)/\(.metadata.name)"'
  ```

  Подробнее — [об аннотации `allow-access-to-kubernetes`](#release-1-78-dexclient-annotation).

- Предупредите пользователей приложений за DexAuthenticator, что после обновления им нужно один раз войти заново. Если приложение проверяет claim `aud` ID-токенов DexAuthenticator, подготовьте его к приёму нового значения `dex-authenticator-<NAME>@<NAMESPACE>`. Подробнее — [об идентификаторах клиентов DexAuthenticator](#release-1-78-dexauthenticator-ids).

- Если интеграции обращаются к `user-api`, адаптируйте их. После обновления API принимает только ID-токены веб-интерфейса Deckhouse Platform, а эндпоинт сброса пароля удалён. Подробнее — [об изменениях `user-api`](#release-1-78-user-api).

- Если клиенты kubeconfig обменивают токены внешнего провайдера на токены Dex, перенастройте их. После обновления такой обмен недоступен. Подробнее — [об обмене токенов в kubeconfig](#release-1-78-kubeconfig-token-exchange).

##### Проекты

Проверьте шаблоны, права администраторов и состояние проектов:

- Если вы используете ProjectTemplate с Helm-шаблоном в `resourcesTemplate`, подготовьте сценарии восстановления. После обновления восстановление такого шаблона из резервной копии, GitOps-бутстрап и пересоздание будут отклонены.
  Существующие шаблоны продолжают работать и редактироваться. Для восстановления и пересоздания потребуется перевод шаблона на структурные поля и `manifests`.
  Подробнее — [о переводе шаблона с Helm-текста](https://deckhouse.ru/modules/multitenancy-manager/v1.78/usage.html#перевод-шаблона-с-helm-текста).

- Если права администраторов проектов расширены кастомными ClusterRole с аннотацией `user-authz.deckhouse.io/access-level`, подготовьте замену этих расширений.
  При автоматическом переводе проектов на встроенных шаблонах `default`, `secure` и `secure-with-dedicated-nodes` роль `d8:project:admin` заменяет AuthorizationRule уровня Admin и не включает эти расширения.
  Для сохранения доступа потребуется кастомная роль, привязанная через ProjectRoleBinding. Подробнее — [о переходе проекта с Helm-шаблонов](https://deckhouse.ru/modules/multitenancy-manager/v1.78/usage.html#переход-проекта-с-helm-текста).

- Если вы переводите манифесты ClusterResourceGrantPolicy с v1alpha1 на v1alpha2, переименуйте `projectSelector` в `namespaceSelector`.
  Простая замена `apiVersion` изменит выбор неймспейсов. Если манифесты остаются на v1alpha1, действие не требуется. Подробнее — [о совместимости версий ClusterResourceGrantPolicy](#release-1-78-grant-policy).

- Если в кластере есть проекты, потерявшие основной неймспейс, найдите их и удалите ненужные. При обновлении такие проекты восстановятся. Найти их можно командой:

  ```bash
  d8 k get projects.deckhouse.io,namespaces -o json | jq -r '(INDEX(.items[] | select(.kind == "Namespace"); .metadata.name)) as $ns | .items[] | select(.kind == "Project" and .metadata.labels["projects.deckhouse.io/virtual-project"] != "true" and .spec.projectTemplateName != "virtual" and .metadata.deletionTimestamp == null) | .metadata.name | select($ns[.] == null or $ns[.].metadata.deletionTimestamp != null)'
  ```

  Подробнее — [о восстановлении основного неймспейса проекта](#release-1-78-project-namespace-restore).

##### Установщик dhctl

Проверьте автоматизацию, которая использует CLI или gRPC API установщика:

- Если вы используете `dhctl bootstrap-phase abort` или вызываете подкоманды `execute-bashible-bundle` и `exec-post-bootstrap` в автоматизации, обновите сценарии. Подробнее — [об изменениях `dhctl bootstrap-phase abort`](#release-1-78-dhctl-abort).

- Если вы разрабатываете или сопровождаете клиента `dhctl` или интеграцию, которая читает список фаз, обновите потребителей фаз `dhctl`. Подробнее — [об изменении состава фаз](#release-1-78-dhctl-phases).

- Если вы задаёте `--download-dir` вручную и переиспользуете эту директорию между версиями `dhctl`, выделите отдельную директорию для каждой версии или очистите старую перед обновлением. Подробнее — [о директории загрузок `dhctl`](#release-1-78-dhctl-download-dir).

##### Облачные провайдеры

Проверьте условия создания и пересоздания облачных узлов:

- Если кластер развёрнут в vSphere, [включите DRS](https://deckhouse.ru/modules/cloud-provider-vsphere/v1.78/#требование-к-vsphere) для узлов на CAPI, поскольку новые группы узлов по умолчанию создаются на CAPI. Подробнее — [о переходе vSphere на CAPI](#release-1-78-vsphere-capi).

- Если в Yandex Cloud есть группы узлов на CAPI с [параметром `preemptible: true`](https://deckhouse.ru/modules/cloud-provider-yandex/v1.78/cr.html#yandexinstanceclass-v1-spec-preemptible), учтите, что их узлы станут прерываемыми после следующего пересоздания. Подробнее — [о прерываемых узлах в Yandex Cloud](#release-1-78-yandex-preemptible).

#### Проверки и миграции после обновления {#release-1-78-after-upgrade}

Выполните миграции конфигурации и проверьте результат автоматических изменений для используемых компонентов.

##### Миграции конфигурации

Перенесите настройки интеграции с Yandex Cloud и zVirt в ModuleConfig [`cloud-provider-yandex`](https://deckhouse.ru/modules/cloud-provider-yandex/v1.78/configuration.html) и [`cloud-provider-zvirt`](https://deckhouse.ru/modules/cloud-provider-zvirt/v1.78/) согласно [инструкции по миграции](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/faq.html#как-мигрировать-облачный-провайдер-на-конфигурацию-через-modulec).
Для кластеров, установленных через [Deckhouse Commander](https://deckhouse.ru/modules/commander/), миграция не требуется до версии Deckhouse Commander 1.22. Во время миграции виртуальные машины и диски не пересоздаются.
До её выполнения срабатывают алерты [D8CloudProviderYandexCloudMigrationPending](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#cloud-provider-yandex-d8cloudprovideryandexcloudmigrationpending) и [D8CloudProviderZvirtMigrationPending](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#cloud-provider-zvirt-d8cloudproviderzvirtmigrationpending).
Подробнее — [о едином контракте ModuleConfig](#release-1-78-cloud-moduleconfig).

##### Проверки результата

Проверьте результат изменений:

- Убедитесь, что модуль `registry` принял управление путями загрузки образов. Порядок проверки приведён [в разделе «На какой реализации работает мой кластер?»](https://deckhouse.ru/modules/registry/v1.78/faq.html#на-какой-реализации-работает-мой-кластер).

- Проверьте предупреждения о нарушениях Pod Security Standards в системных неймспейсах (`d8-*`, `kube-*`). Эти неймспейсы проверяются на соответствие политике `Restricted` в режиме `warn` независимо от лейблов.
  В Deckhouse Platform 1.79 для этих неймспейсов будет выполнен переход в режим `deny`.

- Проверьте завершение автоматического перехода проектов на встроенных шаблонах `default`, `secure` и `secure-with-dedicated-nodes` на структурные поля.
  Администраторы проекта переходят с AuthorizationRule уровня Admin на роль `d8:project:admin` (ProjectRoleBinding `d8-administrators`), квота `all-pods` заменяется на `d8-project-quota`.
  Передача доступа выполняется без перерыва. Для сохранения кастомных расширений доступа создайте кастомную роль и привяжите её через ProjectRoleBinding.
  Если переход не завершился за 30 минут, срабатывает алерт [MultitenancyManagerProjectHandoverStuck](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#multitenancy-manager-multitenancymanagerprojecthandoverstuck).
  Подробнее — [о переходе проекта с Helm-шаблонов](https://deckhouse.ru/modules/multitenancy-manager/v1.78/usage.html#переход-проекта-с-helm-текста).

### Устаревшие и удалённые интерфейсы

Статус устаревших настроек, ролей, команд и API, а также замены для удалённых интерфейсов.

Продолжают работать, но устарели:

- Поля версии Kubernetes, сети и домена в ClusterConfiguration. Перенесите настройки в [ModuleConfig `control-plane-manager`](#release-1-78-cluster-config).
- Обмен токенов через DexProvider. Перейдите на [TokenExchangeProvider](#release-1-78-legacy-token-exchange) до удаления прежнего механизма в будущих версиях.
- Helm-шаблоны проектов в `resourcesTemplate`. Перейдите на [структурные поля и `manifests`](#release-1-78-helm-project-templates). Создание и восстановление старых шаблонов уже ограничены.
- Прежние имена ролей подсистем RBACv2, включая `d8:manage:*`. Они будут удалены в 1.79. Используйте [роли `d8:subsystem`](#release-1-78-rbac-subsystems), предварительно проверив объём прав.
- Режим `Watchdog` в `spec.fencing.mode` NodeGroup. В 1.78 он работает как `Slow`. После обновления замените `Watchdog` на `Slow`, чтобы сохранить поведение версии 1.78. Для выбора другого времени реакции используйте поддерживаемые режимы согласно [инструкции по настройке fencing](https://deckhouse.ru/modules/node-manager/v1.78/faq.html#как-настроить-fencing-для-группы-узлов).

Принимаются, но игнорируются:

- `spec.fencing.watchdog.timeout`. После обновления удалите параметр из NodeGroup при [настройке fencing](#release-1-78-fencing).
- `spec.kubelet.topologyManager.enabled`. Задайте профиль через [параметр `policy`](#release-1-78-topology-manager).
- Параметр `--force-abort-from-cache` команды `dhctl bootstrap-phase abort`. Проверьте [сценарии abort](#release-1-78-dhctl-abort).
- Параметр `--download-cache-dir` установщика `dhctl`. Используйте [параметр `--download-dir`](#release-1-78-dhctl-download-dir).

Удалены или больше не поддерживаются:

- Поддержка Kubernetes 1.32 и Istio 1.21 прекращена. Переведите компоненты на поддерживаемые версии по [условиям обновления](#release-1-78-upgrade-requirements).
- Прежняя реализация registry в ModuleConfig `deckhouse` удалена. Выполните [подготовку на 1.77](#release-1-78-registry-migration). После обновления передача управления произойдёт автоматически.
- Команды `dhctl execute-bashible-bundle` и `dhctl exec-post-bootstrap` удалены. Адаптируйте автоматизацию с учётом [изменений установщика](#release-1-78-dhctl-abort).
- Эндпоинт сброса пароля в `user-api` удалён. Проверьте интеграции с учётом [изменений `user-api`](#release-1-78-user-api).
- Обмен токенов внешнего провайдера через клиентов kubeconfig недоступен. Адаптируйте интеграции с учётом [изменений доступа к API Kubernetes](#release-1-78-kubeconfig-token-exchange).

### Изменения по направлениям

Оглавление раздела:

- [Инфраструктура и узлы](#release-1-78-infrastructure)
- [Kubernetes, размещение и масштабирование нагрузки](#release-1-78-kubernetes-scheduling)
- [Пользователи, доступ и проекты](#release-1-78-access-projects)
- [Безопасность и сертификаты](#release-1-78-security)
- [Сеть и межкластерное взаимодействие](#release-1-78-network)
- [Управление платформой и модулями](#release-1-78-platform)

#### Инфраструктура и узлы {#release-1-78-infrastructure}

Изменения в установке кластера, инфраструктурных интеграциях и управлении узлами.

##### Узлы на Deckhouse Engine {#release-1-78-engine}

Deckhouse Engine — гипервизор первого типа для узлов Deckhouse Platform с неизменяемой корневой файловой системой. Узлы Deckhouse Engine объединяются в группы так же, как остальные узлы кластера, и работают в одном кластере с группами на обычных дистрибутивах Linux (Generic OS). Способ управления узлами задаётся для каждой NodeGroup отдельно. Узлом управляет агент `nodelet`: он приводит узел к состоянию, описанному в [NodeConfig](https://deckhouse.ru/modules/node-manager/v1.78/cr.html#nodeconfig), и сообщает о результате в его статусе.

Состояние такого узла нельзя изменить вручную в обход NodeConfig, поэтому конфигурация узлов группы не расходится со временем. Отсутствие командной оболочки, SSH-сервера и пакетного менеджера сокращает поверхность атаки, а неудачное обновление операционной системы откатывается автоматически.

Отличия узлов Deckhouse Engine от узлов Generic OS:

- Корневая файловая система доступна только для чтения. Её целостность проверяет dm-verity, а подпись корневого хеша образа проверяет ядро. Образ с неверной подписью узел не применяет.
- В составе операционной системы нет командной оболочки, SSH-сервера и пакетного менеджера. Состояние узла определяют образ операционной системы и NodeConfig, поэтому узлы одной группы настроены одинаково.
- Узел настраивается декларативно, SSH-доступ и bashible для этого не используются. Агент узла `nodelet` приводит узел в состояние, описанное в NodeConfig: сеть, диски, параметры и модули ядра, расширения и статические поды.
- Компоненты узла, такие как containerd и kubelet, поставляются в виде подписанных системных расширений (systemd-sysext). Дополнительные расширения, например с модулями ядра, устанавливаются с помощью NodeExtensionRequest.
- Операционная система обновляется целиком. Узел загружает новый образ из хранилища образов и перезагружается по правилам обновления своей NodeGroup. Если загрузиться с новым образом не удалось, узел автоматически возвращается к предыдущему образу.

Подробнее о Deckhouse Engine — [в документации Deckhouse Platform](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/deckhouse-engine.html).

##### Ресурсы узлов

Изменения в учёте и резервировании ресурсов узлов:

- Ёмкость узла (capacity) обновляется после горячего добавления CPU и памяти автоматически. Раз в 5 минут kubelet сравнивает ресурсы узла с теми, что были при его запуске. Если добавился хотя бы один CPU или не меньше 100 МиБ памяти, kubelet сам перезапускается. Ранее kubelet считывал CPU и память узла только при запуске, поэтому после горячего добавления ресурсов в виртуальную машину ёмкость узла не менялась до ручного перезапуска kubelet.

- В режиме `Auto` параметра [`spec.kubelet.resourceReservation`](https://deckhouse.ru/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-kubelet-resourcereservation) резерв CPU для системы рассчитывается по числу ядер узла вместо фиксированных `70m`. Резерв составляет 6 % первого ядра, 1 % второго, по 0,5 % третьего и четвёртого и по 0,25 % каждого следующего. Новое значение применяется, только если поды узла помещаются в новый объём доступных ресурсов (allocatable).

- <a id="release-1-78-topology-manager"></a>**Профили Topology Manager и расчёт NUMA-настроек kubelet на каждом узле.** Параметр [`spec.kubelet.topologyManager.policy`](https://deckhouse.ru/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-kubelet-topologymanager-policy) в NodeGroup задаёт полный профиль kubelet: CPU Manager, Memory Manager и Topology Manager. Параметр [`enabled`](https://deckhouse.ru/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-kubelet-topologymanager-enabled) объявлен устаревшим и игнорируется. NUMA-зависимые настройки kubelet рассчитываются на каждом узле отдельно. Если checkpoint CPU Manager или Memory Manager устарел, например после изменения CPU или памяти узла, kubelet сбрасывает его и запускается, не останавливая работающие контейнеры. Контейнеры, у которых память могла потерять привязку к NUMA-узлу, видны по событию NUMACheckpointReset и алерту [D8KubeletCheckpointStateUnpinnedContainers](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#node-manager-d8kubeletcheckpointstateunpinnedcontainers). Привязка восстанавливается после пересоздания пода.

  Переход на профили Topology Manager меняет поведение групп узлов:
  - В группах с `enabled: false` (явно или по умолчанию) и политикой, отличной от `None`, политика начнёт действовать: CPU и память Guaranteed-подов будут закрепляться за NUMA-узлами. kubelet начнёт отклонять поды, которые не удаётся разместить. При любой политике под отклоняется с `UnexpectedAdmissionError`, если не помещается память. При `Restricted` и `SingleNumaNode` под отклоняется ещё и с `TopologyAffinityError`, если не удаётся выровнять CPU. Планировщик NUMA-узлы не учитывает, поэтому пересозданный под может снова попасть на тот же узел. Чтобы сохранить прежнее поведение, задайте `policy: None`.
  - Группы с `enabled: true` и `policy: None` перестанут закреплять CPU. Чтобы сохранить закрепление, задайте `policy: BestEffort`.
  - Для политик `Restricted` и `SingleNumaNode` включается опция CPU Manager full-pcpus-only. Контейнер получает только целые физические ядра, поэтому на узлах с SMT (Hyper-Threading) Guaranteed-контейнеры с нечётным числом CPU отклоняются с `SMTAlignmentError`.

  Подробнее — [в разделе «Как выделить подам отдельные CPU и выровнять их по NUMA?»](https://deckhouse.ru/modules/node-manager/v1.78/faq.html#как-выделить-подам-отдельные-cpu-и-выровнять-их-по-numa).

- <a id="release-1-78-limited-swap"></a>Исправлена валидация [`kubelet.memorySwap.swapBehavior: LimitedSwap`](https://deckhouse.ru/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-kubelet-memoryswap-swapbehavior) в NodeGroup. Валидация требует cgroup v2. NodeGroup, созданная с `LimitedSwap` на узлах без cgroup v2, может зависнуть при удалении, поскольку обновление и снятие финалайзера отклоняются.

##### Обслуживание и отказоустойчивость узлов

Изменения в fencing, drain и обслуживании узлов:

- <a id="release-1-78-fencing"></a>Переработанный [fencing](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/glossary.html#fencing) переносит нагрузку с отказавшего узла только после подтверждения, что узел остановлен.
  Это защищает приложения StatefulSet и поды с томами ReadWriteOnce от одновременной работы прежнего и нового экземпляров на разных узлах.
  Ранее поды удалялись через 60 секунд после последнего обновления Lease без подтверждения остановки узла.

  Режимы `Critical`, `Normal` и `Slow` позволяют выбрать скорость реакции на отказ для каждой NodeGroup через параметр [`spec.fencing.mode`](https://deckhouse.ru/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-fencing-mode).
  Целевое время реакции fencing составляет около 12, 30 и 60 секунд соответственно с допуском ±20 %.
  Более быстрая реакция повышает риск ложной остановки узла при задержках сети или control plane. Режим `Critical` доступен только в коммерческих редакциях.

  По умолчанию fencing выключен. Для его работы требуются открытые порты `8500/TCP` и `8500/UDP` между InternalIP-адресами всех узлов кластера.
  Режим `Watchdog` устарел и работает как `Slow`, а параметр [`spec.fencing.watchdog.timeout`](https://deckhouse.ru/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-fencing-watchdog-timeout) игнорируется.
  Подробнее — [в разделе «Как настроить fencing для группы узлов?»](https://deckhouse.ru/modules/node-manager/v1.78/faq.html#как-настроить-fencing-для-группы-узлов).

- В кластере с одним master-узлом включить fencing у существующей NodeGroup нельзя. При создании NodeGroup с fencing Deckhouse Platform выдаёт предупреждение. Подробнее — [в разделе «Fencing в кластере с одним master-узлом»](https://deckhouse.ru/modules/node-manager/v1.78/faq.html#fencing-в-кластере-с-одним-master-узлом).

- **Корректный статус drain.** Ошибка или тайм-аут drain больше не записываются как успешное завершение. Узел сохраняет аннотацию draining, получает `update.node.deckhouse.io/drain-failed` с текстом ошибки и повторяет попытку. Обновление NodeGroup останавливается до успешного drain, а NodeStuckInDraining срабатывает через пять минут. Удаление аннотации draining отменяет вытеснение подов и возвращает узел в планировщик. В `status.eviction` NodeOperation видно, каких подов ещё ждёт операция.

- **Защита NodeGroup и объектов CAPI.** Группу master нельзя удалить, а при удалении в обход проверки она восстанавливается с прежними параметрами, лейблами и аннотациями. Удаление CAPI Cluster в `d8-cloud-instance-manager` запрещено.

- `node-controller` сохраняет taints других компонентов при первой обработке Static- и CloudPermanent-узлов.

- Статические поды при обновлении kubelet перезапускаются только при изменении их манифеста. Ранее kubelet принудительно завершал все статические поды узла, даже если манифест не менялся.

- **Диагностика ошибок bashible.** В сообщении [Instance](https://deckhouse.ru/modules/node-manager/v1.78/cr.html#instance) выводится stderr неудачного шага вместо общего кода выхода оболочки. Причину ошибки можно определить без подключения к узлу по SSH.

- Прерванный бутстрап статического узла продолжается с сохранённого состояния.

- <a id="release-1-78-early-time-sync"></a>**Синхронизация времени до запуска kubelet.** На master-узлах `nodelet` проверяет время по NTP при каждой загрузке и исправляет его до запуска kubelet. Настройка `earlyTimeSync.enabled` включена по умолчанию. Если NTP-серверы недоступны, загрузка master-узла может занять дополнительно 30 секунд. Тайм-аут по умолчанию составляет 30 секунд, на bashible-узлах его можно увеличить до 60 секунд параметром `earlyTimeSync.timeoutSeconds`. После неуспешной проверки kubelet запускается, а алерт [NodeTimeNotSynchronizedAtBoot](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#chrony-nodetimenotsynchronizedatboot) сообщает о проблеме. Подробнее — [в разделе «Синхронизация времени при загрузке master-узлов»](https://deckhouse.ru/modules/chrony/v1.78/#синхронизация-времени-при-загрузке-master-узлов).

##### Установка кластера

Изменения установщика `dhctl` и процесса установки:

- **Декларативные фазы и работа CLI-установщика (`dhctl`) без SSH.** Параметр `--skip-phase` пропускает указанные фазы бутстрапа, например при повторном запуске прерванной установки. Операции gRPC принимают kubeconfig и могут выполняться без доступных master-узлов и без SSH-подключения. Фазы bootstrap, destroy и converge описаны единым деревом, поэтому выполняются и отображаются одинаково в CLI и gRPC. Доступны отдельные кеши состояния (state cache) для статических и управляемых кластеров и конфигурация без ClusterConfiguration.

- <a id="release-1-78-dhctl-phases"></a>Состав объявляемых фаз `dhctl` изменился во всех операциях. `FirstMaster` и `WaitForControlPlaneManagerReadiness` стали верхнеуровневыми фазами. Формат gRPC-сообщений не изменился, но клиенты с фиксированным списком или числом фаз требуют доработки.

- <a id="release-1-78-dhctl-abort"></a>Изменилось поведение `dhctl bootstrap-phase abort`. Команда откатывает установку только из кеша состояния и не удаляет ресурсы работающего кластера. Для удаления кластера используйте `dhctl destroy`. Параметр `--force-abort-from-cache` принимается и игнорируется. Команды `execute-bashible-bundle` и `exec-post-bootstrap` удалены. При abort статического кластера передайте тот же набор `--ssh-host`, что при бутстрапе.

- <a id="release-1-78-dhctl-download-dir"></a>Не используйте общую директорию загрузок для разных версий `dhctl`. `dhctl` распаковывает candi, OpenTofu, terraform-manager и provider bundle в директории, имена которых содержат дайджест образа. Параметр `--download-dir` действует на все команды, а `--download-cache-dir` принимается и игнорируется.

- **SSH-сертификаты пользователей в `dhctl`.** Для каждого ключа из `--ssh-agent-private-keys` `dhctl` читает сертификат `<KEY>-cert.pub` из той же директории. Аутентификация по SSH CA работает и для целевых серверов, и для bastion. Сначала предлагается обычный ключ, затем его сертификат и остальные ключи из `ssh-agent`.

- NodeGroup и InstanceClass для CloudPermanent-узлов применяются до их создания, поэтому прерванная установка больше не оставляет группы с нулевым числом реплик. Сеть провайдера настраивается до установки bootstrap-пакетов, что позволяет загрузить их с master-узла из другой зоны.

- Перед запуском `rpp-get` bashible проверяет бинарный файл по дайджесту образа из bundle. `bashible-apiserver` принимает только TLS 1.2 и новее с современными шифрами.

##### Облачные и инфраструктурные провайдеры {#release-1-78-cloud-providers}

Изменения в настройке и модулях облачных и инфраструктурных провайдеров:

- **Префикс облачных ресурсов в ModuleConfig `global`.** Параметр `cloud.prefix` ClusterConfiguration  перенесен в [`spec.settings.prefix`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/api/global.html#parameters-prefix). При обновлении выполняется одноразовая автоматическая миграция. После неё cloud-controller-manager, имена CAPI/MCM MachineDeployment и приоритеты cluster-autoscaler читают префикс из ModuleConfig. Изменение префикса в работающем кластере запрещено, поскольку оно привело бы к пересозданию узлов и переименованию базовой инфраструктуры.

- **Выбор провайдера на уровне NodeGroup.** Параметр [`spec.providerType`](https://deckhouse.ru/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-providertype) определяет тип провайдера для каждой группы узлов. `bashible-apiserver` уже выбирает конфигурацию провайдера по NodeGroup. Группы с `nodeType: Static` больше не получают шаги настройки облачного провайдера.

- <a id="release-1-78-static-cloud-nodes"></a>Для статических узлов (NodeGroup с `nodeType: Static`), размещённых в AWS и Azure, не применяется часть настроек от облачного провайдера:
  - AWS: не устанавливается `ecr-credential-provider` и не включается в настройки kubelet. Если это был единственный способ аутентификации, уже настроенные узлы перестанут загружать приватные образы из Amazon ECR. На Ubuntu также больше не устанавливается модуль ядра EROFS, если он нужен для containerd v2.
  - Azure: не устанавливаются правила udev для обнаружения дисков Azure NVMe. Уже установленные правила не удалятся.

- <a id="release-1-78-cloud-moduleconfig"></a>Конфигурация модулей [`cloud-provider-yandex`](https://deckhouse.ru/modules/cloud-provider-yandex/v1.78/) и [`cloud-provider-zvirt`](https://deckhouse.ru/modules/cloud-provider-zvirt/v1.78/) перешла на единый контракт ModuleConfig. Узлы всех типов настраиваются одинаково через InstanceClass и NodeGroup, отдельный ProviderClusterConfiguration не нужен. Подробнее — [в разделе «Как мигрировать облачный провайдер на конфигурацию через ModuleConfig?»](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/faq.html#как-мигрировать-облачный-провайдер-на-конфигурацию-через-modulec).

- Изменения в модуле [`cloud-provider-yandex`](https://deckhouse.ru/modules/cloud-provider-yandex/v1.78/):
  - [Размер блока дисков](https://deckhouse.ru/modules/cloud-provider-yandex/v1.78/configuration.html#parameters-storage-parameters-provisionedstorageclasses) задаётся в StorageClass, что позволяет создавать диски объёмом до 256 ТиБ.
  - Размер дисков типов network-ssd-nonreplicated и network-ssd-io-m3 проверяется заранее, и ошибка обнаруживается до создания инфраструктуры.
  - Зона доступности `ru-central1-f` поддерживается. Чтобы использовать зону, укажите `ru-central1-f` в параметре [`zones`](https://deckhouse.ru/modules/cloud-provider-yandex/v1.78/configuration.html#parameters-nodes-parameters-zones) вместо одной из зон по умолчанию. Порядок настройки описан [в разделе «Использование зоны ru-central1-f»](https://deckhouse.ru/modules/cloud-provider-yandex/v1.78/examples.html#использование-зоны-ru-central1-f).
  - <a id="release-1-78-yandex-preemptible"></a>Существующие узлы групп на CAPI в Yandex Cloud с [параметром `preemptible: true`](https://deckhouse.ru/modules/cloud-provider-yandex/v1.78/cr.html#yandexinstanceclass-v1-spec-preemptible) станут прерываемыми после [следующего пересоздания узлов группы](https://deckhouse.ru/modules/node-manager/v1.78/faq.html#как-пересоздать-эфемерные-машины-в-облаке-с-новой-конфигурацией). Ранее такие узлы создавались как обычные виртуальные машины.

- В модуле [`cloud-provider-zvirt`](https://deckhouse.ru/modules/cloud-provider-zvirt/v1.78/) в ZvirtInstanceClass доступны параметры [`storageDomainID`](https://deckhouse.ru/modules/cloud-provider-zvirt/v1.78/cr.html#zvirtinstanceclass-v1-spec-storagedomainid) и [`etcdDiskSizeGb`](https://deckhouse.ru/modules/cloud-provider-zvirt/v1.78/cr.html#zvirtinstanceclass-v1-spec-etcddisksizegb), позволяющие выбрать домен хранения и размер диска etcd на master-узлах.

- Изменения в модуле [`cloud-provider-vsphere`](https://deckhouse.ru/modules/cloud-provider-vsphere/v1.78/):
  - <a id="release-1-78-vsphere-capi"></a>Провайдер vSphere работает на базе Cluster API (CAPI) вместо Machine Controller Manager (MCM). Существующие группы узлов остаются на MCM, новые создаются на CAPI. Для узлов на CAPI в кластере vSphere [включите DRS](https://deckhouse.ru/modules/cloud-provider-vsphere/v1.78/#требование-к-vsphere). Порядок перевода существующей группы приведён [в разделе «Как мигрировать группы узлов на Cluster API (CAPI)»](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/faq.html#как-мигрировать-группы-узлов-на-cluster-api-capi).
  - Сертификат корпоративного центра сертификации для vCenter и NSX-T задаётся параметрами ([`provider.caBundle`](https://deckhouse.ru/modules/cloud-provider-vsphere/v1.78/cluster_configuration.html#vsphereclusterconfiguration-provider-cabundle), [`nsxt.caBundle`](https://deckhouse.ru/modules/cloud-provider-vsphere/v1.78/cluster_configuration.html#vsphereclusterconfiguration-nsxt-cabundle)). Поэтому проверку TLS-сертификатов не нужно отключать.

- Изменения в модуле [`cloud-provider-openstack`](https://deckhouse.ru/modules/cloud-provider-openstack/v1.78/):
  - Для Ingress можно использовать заранее созданный балансировщик Octavia или floating IP. Это делает адрес Ingress предсказуемым. Подробнее — [в разделе «Как настроить LoadBalancer?»](https://deckhouse.ru/modules/cloud-provider-openstack/v1.78/faq.html#как-настроить-loadbalancer).
  - Поддержка [прерываемых узлов](https://deckhouse.ru/modules/cloud-provider-openstack/v1.78/cr.html#openstackinstanceclass-v1-spec-preemptible) в облаке Selectel для групп узлов на Cluster API.
  - Для схемы размещения SimpleWithInternalNetwork внутреннюю подсеть можно указать [по идентификатору](https://deckhouse.ru/modules/cloud-provider-openstack/v1.78/cluster_configuration.html#openstackclusterconfiguration-simplewithinternalnetwork-internalsubnetid), например если у подсети нет имени.
  - Одновременно указывать `tenantName` и `tenantID` запрещено.
  - В кластерах OpenStack со схемой размещения [SimpleWithInternalNetwork](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/admin/integrations/private/openstack/configuration-and-layout-scheme.html#simplewithinternalnetwork) первый запуск `dhctl converge` после обновления покажет в плане изменение параметров поиска внутренней подсети. Узлы при этом не пересоздаются.

- <a id="release-1-78-dynamix"></a>Изменения в модуле [`cloud-provider-dynamix`](https://deckhouse.ru/modules/cloud-provider-dynamix/v1.78/):
  - Интеграция поддерживает только Basis Dynamix [версии 4.6 и выше](https://deckhouse.ru/modules/cloud-provider-dynamix/v1.78/environment.html#версия-платформы). Кластеры на Basis Dynamix, созданные в предыдущих версиях Deckhouse Platform, необходимо переустановить.
  - При удалении кластера на Basis Dynamix ресурсы, включая диски постоянных томов, удаляются безвозвратно, без перемещения в корзину.
  - Диски размещаются по [политике хранения](https://deckhouse.ru/modules/cloud-provider-dynamix/v1.78/cluster_configuration.html#dynamixclusterconfiguration-storagepolicy) Basis Dynamix, которую можно переопределить для отдельной группы узлов. StorageClass создаются для каждой доступной политики хранения. Учётной записи кластера требуется [доступ к группе API storage_policy](https://deckhouse.ru/modules/cloud-provider-dynamix/v1.78/environment.html#права-учётной-записи).
  - Для схемы размещения Standard DNS-серверы из [параметра `nameservers`](https://deckhouse.ru/modules/cloud-provider-dynamix/v1.78/cluster_configuration.html#dynamixclusterconfiguration-nameservers) прописываются на всех узлах.
  - Объекты Basis Dynamix ищутся по точному совпадению имени, что исключает выбор не того объекта при похожих именах. Значения `imageName` и `externalNetwork` нужно указывать полностью.
  - Исправлены синхронизация балансировщика нагрузки с узлами, удаление сервисов типа LoadBalancer и доступ по SSH к узлам, подключённым одновременно к ViNS и внешней сети.

- Изменения в модуле [`cloud-provider-aws`](https://deckhouse.ru/modules/cloud-provider-aws/v1.78/):
  - Обязательное использование IMDSv2 на всех инстансах EC2 кластера включается [параметром `imdsv2`](https://deckhouse.ru/modules/cloud-provider-aws/v1.78/cluster_configuration.html#awsclusterconfiguration-imdsv2). Это защищает метаданные инстансов от SSRF-атак. После включения сервис метаданных становится недоступен подам, не использующим сеть узла.
  - terraform-provider-aws обновлён до версии 6.69.1, что добавляет поддержку новых регионов AWS, например ap-southeast-6.

- В модуле [`cloud-provider-huaweicloud`](https://deckhouse.ru/modules/cloud-provider-huaweicloud/v1.78/) единичная ошибка API Huawei Cloud больше не останавливает масштабирование групп узлов.

- Новый модуль [`infra-provider-baremetal`](https://deckhouse.ru/modules/infra-provider-baremetal/v1.78/) управляет узлами кластера на bare-metal-серверах через BMC.

- В модулях облачных провайдеров AWS, Azure, GCP, Yandex Cloud, OpenStack, VCD и vSphere DaemonSet [`csi-node`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/architecture/storage/csi-drivers/csi-driver.html#компоненты-драйвера) использует отдельный ServiceAccount для доступа к API Kubernetes. Это устраняет ошибки авторизации CSI-драйверов при чтении сведений об узлах.

- Исправлено зависание установки кластеров в OpenStack и VCD на этапе запуска CAPI-контроллеров.

#### Kubernetes, размещение и масштабирование нагрузки {#release-1-78-kubernetes-scheduling}

Изменения в работе control plane, размещении подов и управлении их ресурсами.

##### Работа и обновление control plane {#release-1-78-control-plane}

Изменения в версиях Kubernetes, ресурсах и обновлении компонентов control plane:

- <a id="release-1-78-kubernetes"></a>Kubernetes 1.37 поддерживается в статусе preview. Поддержка Kubernetes 1.32 прекращена. Версия по умолчанию изменена с 1.34 на 1.35 (значение `Default` параметра [`kubernetesVersion`](https://deckhouse.ru/modules/control-plane-manager/v1.78/configuration.html#parameters-kubernetesversion)). Поддерживаемые версии перечислены [в разделе «Поддерживаемые версии Kubernetes и ОС»](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/supported_versions.html). Управление версиями описано [в разделе «Управление версиями control plane»](https://deckhouse.ru/modules/control-plane-manager/v1.78/#управление-версиями).

- **Автоподбор ресурсных запросов компонентов control plane.** Запросы CPU и памяти для etcd, `kube-apiserver`, `kube-controller-manager` и `kube-scheduler` подбираются по фактическому потреблению за 7 дней с запасом 10 % и пересчитываются раз в сутки. Планировщик учитывает реальную нагрузку control plane на master-узлах, а подбирать `resourcesRequests` вручную не требуется. Если новые запросы не помещаются на самый маленький master-узел, они не применяются и срабатывает алерт [D8ControlPlaneComponentsResourcesInsufficientCapacity](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#control-plane-manager-d8controlplanecomponentsresourcesinsufficientcapacity). Если в ModuleConfig `control-plane-manager` задан [`resourcesRequests.cpu`](https://deckhouse.ru/modules/control-plane-manager/v1.78/configuration.html#parameters-resourcesrequests-cpu) или [`resourcesRequests.memory`](https://deckhouse.ru/modules/control-plane-manager/v1.78/configuration.html#parameters-resourcesrequests-memory), автоподбор для этого ресурса отключается. Подробнее — [в разделе «Настройка ресурсных запросов control plane»](https://deckhouse.ru/modules/control-plane-manager/v1.78/#настройка-ресурсных-запросов-control-plane).

- **Плавное обновление control plane.** `kube-apiserver` обновляются поочерёдно, так что большинство master-узлов всё время обслуживает запросы. Останавливающийся `kube-apiserver` успевает передать клиентов на другие master-узлы, и запросы к API не обрываются. `kube-controller-manager` и `kube-scheduler` подключаются к API через `kubernetes-api-proxy`, поэтому перезапуск `kube-apiserver` на их master-узле больше не приводит к смене их лидеров. Лидеры etcd, `kube-controller-manager` и `kube-scheduler` меняются один раз за обновление и сразу переходят на обновлённый экземпляр. В кластерах с арбитром etcd лидером становится master-узел, арбитр лидером не становится.

- Резервные копии etcd на master-узлах создаются по очереди со сдвигом в 15 минут. Ранее копии создавались на всех master-узлах одновременно, и нагрузка на диски могла приводить к потере lease и перезапускам `kube-controller-manager` и `kube-scheduler`. Самая свежая копия хранится на master-узле с наименьшим именем, например master-0. Подробнее — [в разделе «Резервное копирование и восстановление etcd»](https://deckhouse.ru/modules/control-plane-manager/v1.78/faq.html#резервное-копирование-и-восстановление-etcd).

- **Проверки и завершение работы `kubernetes-api-proxy`.** Клиенты, которые обращаются к API Kubernetes через `kubernetes-api-proxy`, быстрее переключаются на работающий `kube-apiserver` при его перезапуске или зависании. Медленные, но работающие `kube-apiserver` остаются в ротации. Останавливающиеся, перезапускающиеся и зависшие экземпляры перестают получать новые соединения, а зависшие клиентские соединения закрываются для переподключения к работающему экземпляру. Если ни один `kube-apiserver` не готов, прокси проверяет каждый экземпляр каждые 500 мс в течение 30 секунд. Состояние доступно в `/upstreams`, проверки работоспособности по умолчанию доступны на `127.0.0.1:6480`. После SIGTERM прокси ещё 3 секунды обслуживает `127.0.0.1:6445`. При остановке master-узла kubelet даёт node-critical подам 25 секунд на завершение вместо 5 секунд.

- Изменения в аудите (модуль [`control-plane-manager`](https://deckhouse.ru/modules/control-plane-manager/v1.78/)):
  - Базовая политика аудита снова записывает pods/exec во всех неймспейсах, как до Deckhouse Platform 1.77.
  - Базовая политика аудита записывает обновления статуса подов от kubelet с телом запроса, поэтому завершения контейнеров и их причины попадают в аудит-лог.

- <a id="release-1-78-cluster-config"></a>Следующие параметры кластера перенесены из ClusterConfiguration [в ModuleConfig модуля `control-plane-manager`](https://deckhouse.ru/modules/control-plane-manager/v1.78/configuration.html#parameters-kubernetesversion):
  - Версия Kubernetes. Задаётся в [`settings.kubernetesVersion`](https://deckhouse.ru/modules/control-plane-manager/v1.78/configuration.html#parameters-kubernetesversion): конкретная версия или `Default`. Значение `Automatic` допускается только в устаревшем поле в ClusterConfiguration.
  - Адресное пространство подов ([`podSubnetCIDR`](https://deckhouse.ru/modules/control-plane-manager/v1.78/configuration.html#parameters-network-podsubnetcidr)).
  - Адресное пространство сервисов кластера ([`serviceSubnetCIDR`](https://deckhouse.ru/modules/control-plane-manager/v1.78/configuration.html#parameters-network-servicesubnetcidr)).
  - Размер префикса подсети подов, выделяемой каждому узлу ([`podSubnetNodeCIDRPrefix`](https://deckhouse.ru/modules/control-plane-manager/v1.78/configuration.html#parameters-network-podsubnetnodecidrprefix)).
  - Домен кластера ([`clusterDomain`](https://deckhouse.ru/modules/control-plane-manager/v1.78/configuration.html#parameters-network-clusterdomain)).

  Соответствующие поля ClusterConfiguration объявлены устаревшими, но продолжают работать. Пока они заполнены, срабатывают алерты [D8ObsoleteNetworkFieldsInClusterConfiguration](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#control-plane-manager-d8obsoletenetworkfieldsinclusterconfiguration) и [D8ObsoleteClusterDomainInClusterConfiguration](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#control-plane-manager-d8obsoleteclusterdomaininclusterconfiguration) с инструкциями по миграции. Перенесите параметры в ModuleConfig `control-plane-manager` по этим инструкциям. После установки кластера валидация не даёт изменить сетевые параметры и домен кластера. Сетевые параметры можно изменить только с аннотацией `deckhouse.io/allow-unsafe=true`.

##### Распределение подов {#release-1-78-scheduling}

Изменения планировщика `kube-scheduler` в модуле [`control-plane-manager`](https://deckhouse.ru/modules/control-plane-manager/v1.78/):

- **Симуляция размещения подов в `kube-scheduler`.** Запрос `POST /scheduler/simulate` к Service `control-plane-proxy` показывает, на каких узлах может быть размещён под или виртуальная машина и почему остальные узлы не подошли. Доступ даёт ClusterRole `d8:control-plane-manager:scheduler-simulate`. Подробнее — [о симуляции размещения пода](https://deckhouse.ru/modules/control-plane-manager/v1.78/faq.html#как-проверить-на-какой-узел-будет-размещён-под).

- Метрики `kube-scheduler` и новый дашборд «Scheduling» показывают запросы к extender и решения о выборе узла.

- **Приостановка недоступного extender.** Если 10 запросов подряд к extender с `failurePolicy: Ignore` завершились сбоем, `kube-scheduler` приостанавливает этот extender на 30 секунд, и нагрузка размещается без его участия.

- Параметр [`sendPlacements`](https://deckhouse.ru/modules/control-plane-manager/v1.78/cr.html#kubeschedulerwebhookconfiguration-v1alpha1-webhooks-sendplacements) в KubeSchedulerWebhookConfiguration передаёт extender поды, которые `kube-scheduler` только что разместил. Подробнее — [о расширении логики планировщика](https://deckhouse.ru/modules/control-plane-manager/v1.78/faq.html#как-изменить-или-расширить-логику-работы-планировщика).

##### Перераспределение подов

Изменения модуля [`descheduler`](https://deckhouse.ru/modules/descheduler/v1.78/):

- **Проверка узла назначения перед вытеснением.** Стратегия LowNodeUtilization использует [симуляцию размещения](#release-1-78-scheduling) и вытесняет нагрузку, только если `kube-scheduler` сможет разместить её на недогруженном узле. Подробнее — [о проверке узла назначения](https://deckhouse.ru/modules/descheduler/v1.78/#проверка-узла-назначения).

- **Оценка загрузки узлов по средней загрузке CPU за 15 минут из Prometheus.** Для её использования задайте `metricsSource: Prometheus`. Подробнее — [о загрузке CPU за 15 минут](https://deckhouse.ru/modules/descheduler/v1.78/#загрузка-cpu-усредненная-за-15-минут).

- **Балансировка относительно средней загрузки кластера.** Параметр [`useDeviationThresholds`](https://deckhouse.ru/modules/descheduler/v1.78/cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-usedeviationthresholds) стратегии [LowNodeUtilization](https://deckhouse.ru/modules/descheduler/v1.78/cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization) задаёт пороги `thresholds` и `targetThresholds` как отклонение от средней загрузки узлов кластера. Параметр [`evictionLimits.node`](https://deckhouse.ru/modules/descheduler/v1.78/cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-evictionlimits-node) ограничивает число подов, вытесняемых с одного узла. При [`metricsSource: Prometheus`](https://deckhouse.ru/modules/descheduler/v1.78/cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-metricssource) он задаёт, сколько подов вытеснять с узла за цикл. Если параметр не задан, вытесняется один под.

- **Защита подов с локальными дисками от вытеснения.** Поды с PersistentVolumeClaim можно защитить от вытеснения, перечислив их StorageClass в [`spec.protectedStorageClasses`](https://deckhouse.ru/modules/descheduler/v1.78/cr.html#descheduler-v1alpha2-spec-protectedstorageclasses).

- Параметр [`logLevel`](https://deckhouse.ru/modules/descheduler/v1.78/configuration.html#parameters-loglevel) задаёт уровень логирования.

##### Вертикальное масштабирование {#release-1-78-vpa}

Изменения модуля [`vertical-pod-autoscaler`](https://deckhouse.ru/modules/vertical-pod-autoscaler/v1.78/):

- VPA по умолчанию меняет resources requests подов без перезапуска. Если не задан [параметр `updateMode`](https://deckhouse.ru/modules/vertical-pod-autoscaler/v1.78/cr.html#verticalpodautoscaler-v1-spec-updatepolicy-updatemode), объекты VerticalPodAutoscaler при создании получают режим `InPlaceOrRecreate` вместо `Recreate`. В этом режиме VPA меняет resources requests подов без пересоздания, в том числе у приложений с одной репликой, и пересоздаёт под, только если изменить их таким способом не удалось. Существующие объекты VPA сохраняют свой режим и не меняются.

- **Коэффициент для рекомендаций VPA.** Параметр [`recommendationFactor`](https://deckhouse.ru/modules/vertical-pod-autoscaler/v1.78/cr.html#verticalpodautoscaler-v1-spec-resourcepolicy-containerpolicies-recommendationfactor) в [`spec.resourcePolicy.containerPolicies[]`](https://deckhouse.ru/modules/vertical-pod-autoscaler/v1.78/cr.html#verticalpodautoscaler-v1-spec-resourcepolicy-containerpolicies) VerticalPodAutoscaler умножает рекомендации VPA для CPU и памяти на указанный коэффициент (не больше 10).

- Параметр [`logLevel`](https://deckhouse.ru/modules/vertical-pod-autoscaler/v1.78/configuration.html#parameters-loglevel) задаёт уровень логирования VPA.

#### Пользователи, доступ и проекты {#release-1-78-access-projects}

Изменения в правах доступа, аутентификации пользователей и приложений и управлении проектами.

##### Авторизация {#release-1-78-iam}

Изменения ролевой модели и прав доступа:

- <a id="release-1-78-rbac-subsystems"></a>Новый список [подсистем ролевой модели](https://deckhouse.ru/modules/user-authz/v1.78/#подсистемы-ролевой-модели) RBACv2. Роли подсистем имеют вид `d8:subsystem:<SUBSYSTEM>:<LEVEL>`. Подсистемы: `iam`, `security`, `cluster`, `delivery`, `network`, `storage`, `observability`, `managed-services`. Подсистемы `deckhouse`, `infrastructure` и `kubernetes` объединены в `cluster`, а `networking` переименована в `network`. Модули [`user-authn`](https://deckhouse.ru/modules/user-authn/v1.78/), [`user-authz`](https://deckhouse.ru/modules/user-authz/v1.78/) и [`multitenancy-manager`](https://deckhouse.ru/modules/multitenancy-manager/v1.78/) перенесены в `iam`. Менеджер подсистемы `iam` также назначает роли. Устаревшие роли `d8:manage:*` продолжают работать, но будут удалены в 1.79. Подробнее — [в разделе «Устаревшие имена ролей»](https://deckhouse.ru/modules/user-authz/v1.78/#устаревшие-имена-ролей). Роль-замена `d8:subsystem:cluster:*` даёт больше прав, чем прежние роли, поэтому перед пересозданием привязок проверьте, какие права нужны субъектам. Кастомные capability и модули со старыми лейблами агрегации показывают алерты [D8UserAuthzRetiredAggregationLabel](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#user-authz-d8userauthzretiredaggregationlabel) и [D8UserAuthzLegacyUseCapability](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#user-authz-d8userauthzlegacyusecapability).

- Права менеджеров систем и подсистем сокращены:
  - Пользователи с ролями `d8:system:manager`, `d8:subsystem:<SUBSYSTEM>:manager` и устаревшими `d8:manage:*:manager` получают в неймспейсах своих модулей уровень viewer. Они больше не могут читать секреты, выполнять exec, attach, port-forward, удалять и масштабировать поды. Для операций выше viewer потребуется роль superadmin подсистемы. Она не снимает [ограничения на операции с защищёнными объектами платформы](https://deckhouse.ru/modules/deckhouse/v1.78/faq.html#кто-может-выполнять-exec-в-поды-deckhouse-platform-и-масштабировать-объекты-deckhouse-platform), включая поды с лейблом `heritage: deckhouse`.
  - Кастомные системные и подсистемные роли проецируются в неймспейсы модулей не выше viewer. Роли, которых это коснулось, показывает метрика `d8_user_authz_projection_capped`.
  - Менеджеры `d8:system:manager` и `d8:subsystem:iam:manager` назначают роли до уровня manager и ClusterEditor. Они не могут создавать ClusterAuthorizationRule уровней Admin, ClusterAdmin и SuperAdmin или открывающие доступ к системным неймспейсам. Существующие правила сохраняются. Такие изменения выполняет ClusterAdmin или SuperAdmin.

- Вывод списка объектов командой `d8 k get` с параметром `--scope` поддерживает агрегированные API, которые объявили это аннотацией `scope.deckhouse.io/resources` на APIService, а также ресурсы `roles` и `rolebindings`.

- Роли `d8:namespace:*` и `d8:project:*` дают доступ к namespaced-ресурсам Gateway API: чтение с уровня viewer, запись с уровня manager.

- На master-узлах (host network) заняты порты `4233/TCP` (метрики вебхука авторизации) и `4243/TCP` (upstream `kube-rbac-proxy` на `127.0.0.1`, правило файрвола не требуется).

- Создавать и изменять ClusterRole с префиксом `d8:commander:` может только агент Deckhouse Commander.

- **Скрытие чувствительных полей в API.** Пароли и auth в RegistryConfig и RegistryUpstream, а также NodeUser `spec.passwordHash` скрыты от пользователей без доступа к субресурсам `/sensitive`. Для остальных читателей API возвращает `<omitted>`, значения также маскируются в аудите.

##### Пользователи и сессии {#release-1-78-sessions}

Изменения в управлении пользователями и их сессиями:

- **Управление сессиями пользователей.** Блокировка, сброс пароля или 2FA, удаление пользователя и новая операция Logout завершают сессии пользователя в Dex, DexAuthenticator и API Kubernetes. Basic Authentication к API Kubernetes через LDAP проверяет пароль в Dex при каждом запросе, поэтому `Logout` её не завершает. Для этого в [UserOperation](https://deckhouse.ru/modules/user-authn/v1.78/cr.html#useroperation) доступны типы `Logout` и `RequirePasswordChange`. В режиме мультитенантности на Kubernetes 1.34 и новее API-сервер перестаёт принимать отозванные ID-токены в течение нескольких секунд.

- **Устройства пользователя.** [UserAccount.status.sessions](https://deckhouse.ru/modules/user-authn/v1.78/cr.html#useraccount-v1alpha1-status-sessions) показывает активные сессии. Операция `Logout` завершает одну сессию или все, кроме текущей.

- **Браузерные сессии Dex.** Параметры [`sessions`](https://deckhouse.ru/modules/user-authn/v1.78/configuration.html#parameters-sessions) включают единый вход во все веб-интерфейсы платформы, выход из сессии браузера во всех DexAuthenticator и back-channel logout. По умолчанию браузерные сессии выключены.

- В DexClient доступны параметры [`postLogoutRedirectURIs`](https://deckhouse.ru/modules/user-authn/v1.78/cr.html#dexclient-v1-spec-postlogoutredirecturis), [`backchannelLogoutURI`](https://deckhouse.ru/modules/user-authn/v1.78/cr.html#dexclient-v1-spec-backchannellogouturi) и [`refreshTokenLifetime`](https://deckhouse.ru/modules/user-authn/v1.78/cr.html#dexclient-v1-spec-refreshtokenlifetime), в OIDC DexProvider — [`upstreamLogout`](https://deckhouse.ru/modules/user-authn/v1.78/cr.html#dexprovider-v1-spec-oidc-upstreamlogout).

- Блокировка ([Lock](https://deckhouse.ru/modules/user-authn/v1.78/cr.html#useroperation-v1-spec-lock) с [`spec.target`](https://deckhouse.ru/modules/user-authn/v1.78/cr.html#useroperation-v1-spec-target)) работает для пользователей любых провайдеров, включая OIDC, SAML, GitHub, GitLab и Bitbucket Cloud. Вход через браузер больше не снимает блокировку, установленную администратором.

- Секреты 2FA и блокировки пользователей Dex хранятся в UserIdentity. UserAccount ведётся для пользователей всех провайдеров.

- Операции Lock и ResetPassword больше не сбрасывают секрет 2FA. Чтобы пользователь заново зарегистрировал приложение-аутентификатор, дополнительно [выполните Reset2FA (d8 iam user reset2fa)](https://deckhouse.ru/modules/user-authn/v1.78/usage.html#второй-фактор-учётной-записи).

- Ручное удаление UserIdentity запрещено. Чтобы удалить локального пользователя, удалите User, а для сброса 2FA или блокировки используйте UserOperation. Подробнее — [в разделе «Можно ли удалить объект UserIdentity?»](https://deckhouse.ru/modules/user-authn/v1.78/faq.html#можно-ли-удалить-объект-useridentity).

- Ограничение частоты запросов (`rateLimit`) распространяется на шаги 2FA и страницу смены пароля. При включённой 2FA Dex запрашивает код до смены пароля. Подробнее — [в разделе «Как Dex защищён от подбора логина и пароля?»](https://deckhouse.ru/modules/user-authn/v1.78/faq.html#как-dex-защищен-от-подбора-логина-и-пароля).

- Пользователи Active Directory с [`idAttr: objectGUID`](https://deckhouse.ru/modules/user-authn/v1.78/cr.html#dexprovider-v1-spec-ldap-usersearch-idattr) могут входить в систему.

- <a id="release-1-78-user-api"></a>`user-api` принимает только ID-токены веб-интерфейса Deckhouse. Эндпоинт сброса пароля удалён.

##### Доступ приложений к API Kubernetes

Изменения в доступе приложений и внешних систем к API Kubernetes:

- <a id="release-1-78-token-exchange"></a>Кастомный ресурс [TokenExchangeProvider](https://deckhouse.ru/modules/user-authn/v1.78/cr.html#tokenexchangeprovider) описывает доверенного издателя, токены которого Dex обменивает на токены API Kubernetes. С его помощью можно настроить доступ из CI/CD (например, GitLab CI) без статических учётных данных.

- <a id="release-1-78-dexclient-annotation"></a>Аннотация `dexclient.deckhouse.io/allow-access-to-kubernetes` у DexClient действует всегда. Приложение такого DexClient получает ID-токены, которые принимает `kube-apiserver`, и может действовать в API Kubernetes от имени вошедших пользователей. Раньше без DexAuthenticator с доступом к Kubernetes, `publishAPI` или `kubeconfigGenerator` аннотация не действовала. Подробнее об использовании аннотации — [в разделе «Предоставление приложению доступа к Kubernetes API»](https://deckhouse.ru/modules/user-authn/v1.78/usage.html#предоставление-приложению-доступа-к-kubernetes-api).

- <a id="release-1-78-kubeconfig-token-exchange"></a>Клиенты kubeconfig больше не могут обменивать токены внешнего провайдера на токены Dex.

- <a id="release-1-78-legacy-token-exchange"></a>Обмен токенов через DexProvider устарел. В существующих кластерах с DexClient с аннотацией `dexclient.deckhouse.io/allow-access-to-kubernetes` обмен через OIDC- и GitLab-DexProvider остаётся включённым параметром `spec.tokenExchange.enabled`, его можно только выключить. Если за последние 24 часа токен обменивался через DexProvider, срабатывает алерт [D8UserAuthnDexProviderTokenExchangeDeprecated](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#user-authn-d8userauthndexprovidertokenexchangedeprecated). Переведите обмен на TokenExchangeProvider до удаления этой возможности после следующего LTS-релиза. Ограничение клиентов kubeconfig действует уже в 1.78 независимо от настройки DexProvider. Подробнее — [о TokenExchangeProvider](#release-1-78-token-exchange).

##### Аутентификация для приложений

Изменения в работе DexAuthenticator и Dex:

- DexAuthenticator переведён на `oauth2-proxy` 7.15.5. Он проверяет TLS-сертификат Dex, использует PKCE, не завершает сессии пользователей при кратковременной недоступности Dex и поддерживает вход в нескольких вкладках. Выход из приложения отзывает refresh-токен в Dex. Принцип работы DexAuthenticator описан [в разделе «Как работает аутентификация с помощью DexAuthenticator»](https://deckhouse.ru/modules/user-authn/v1.78/faq.html#как-работает-аутентификация-с-помощью-dexauthenticator).

- <a id="release-1-78-dexauthenticator-ids"></a>Клиенты DexAuthenticator получили однозначные идентификаторы. После обновления пользователям приложений за DexAuthenticator нужно один раз войти заново. Claim `aud` ID-токенов DexAuthenticator меняется на `dex-authenticator-<NAME>@<NAMESPACE>`. Приложение, которое проверяет `aud`, должно принимать новое значение. Подробнее — [в разделе «Содержимое токена JWT и особенности его обработки»](https://deckhouse.ru/modules/user-authn/v1.78/faq.html#содержимое-токена-jwt-и-особенности-его-обработки).

- <a id="release-1-78-revocation-feed"></a>DexAuthenticator обращается к сервису `revocation-feed` (поды `user-authn-controller`, порт `8443/TCP`) в неймспейсе `d8-user-authn`. Если в неймспейсе DexAuthenticator исходящий трафик ограничен NetworkPolicy, разрешите этот трафик.

- События DexAuthenticator (входы, выходы, неудачные попытки, ошибки CSRF) передаются как события безопасности при включённом [модуле `security-events-manager`](https://deckhouse.ru/modules/security-events-manager/).

- Повышена отказоустойчивость Dex и защищённых приложений:
  - Dex не перезапускается в цикле при недоступности издателя OIDC-провайдера и стартует с остальными провайдерами.
  - Dex не перезапускается при нехватке CPU на узле.
  - Защищённые приложения продолжают работать при кратковременной недоступности Dex и во время его перезапуска.

- Реплики DexAuthenticator с [`spec.highAvailability`](https://deckhouse.ru/modules/user-authn/v1.78/cr.html#dexauthenticator-v1-spec-highavailability) распределяются по разным узлам.

- DexAuthenticator работает в изолированных проектах и проектах с политиками, требующими указания ресурсных запросов (requests).

##### Управление проектами {#release-1-78-projects}

Изменения в квотах, шаблонах и неймспейсах проектов:

- Несколько неймспейсов в одном проекте. В коммерческих редакциях дополнительные неймспейсы создаются через [ProjectNamespace](https://deckhouse.ru/modules/multitenancy-manager/v1.78/cr.html#projectnamespace). Права, выданные через ProjectRoleBinding, действуют во всех неймспейсах проекта. В Deckhouse Platform Open (CE) у проекта остаётся только один основной неймспейс.

- Квота проекта, заданная в [Project.spec.quota](https://deckhouse.ru/modules/multitenancy-manager/v1.78/cr.html#project-v1alpha3-spec-quota), — одна на весь проект.
  В коммерческих редакциях квота ограничивает суммарное потребление всех неймспейсов проекта. В Deckhouse Platform Open (CE) она ограничивает потребление единственного основного неймспейса.
  Потребление отображается в объекте [ProjectQuotaStatus](https://deckhouse.ru/modules/multitenancy-manager/v1.78/cr.html#projectquotastatus), доступном только для чтения.
  Создание объекта, превышающего квоту проекта, отклоняется. Сумма по неймспейсам проверяется, только если включён модуль [`control-plane-manager`](https://deckhouse.ru/modules/control-plane-manager/v1.78/).
  Квота может ограничивать виртуальные машины проекта (ядра, гарантированный CPU, память, GPU) и балансировщики нагрузки (по классу, пулу и внешнему адресу).

- Изменение [квоты проекта](https://deckhouse.ru/modules/multitenancy-manager/v1.78/cr.html#project-v1alpha3-spec-quota) требует capability `manage_quotas`. Использование квоты проверяет вебхук `quota-ledger.multitenancy-manager.deckhouse.io`. Пока ни одна реплика `multitenancy-manager` не отвечает, создание учитываемых объектов в неймспейсах проектов с квотой завершается ошибкой 500 (`failed calling webhook`); ошибка 503 означает кратковременную недоступность учёта квот, например при смене лидера, и запрос можно повторить. При включённом модуле [`control-plane-manager`](https://deckhouse.ru/modules/control-plane-manager/v1.78/) модуль работает в двух репликах.

- Через [Project.spec.grants](https://deckhouse.ru/modules/multitenancy-manager/v1.78/cr.html#project-v1alpha3-spec-grants) проекту можно выдать cluster-wide-ресурсы в дополнение к выданным его шаблоном. Механизм выдачи масштабируется до тысяч объектов.

- Шаблон проекта версии v1alpha2 описывается структурными полями: профиль безопасности подов, сетевая изоляция, сбор логов, мониторинг, сканирование образов на уязвимости. Контроллер сам создаёт по этим полям объекты в каждом неймспейсе проекта и поддерживает их в актуальном состоянии, поэтому писать и сопровождать Helm-шаблон не требуется. Подробнее — [в разделе «Структурированные шаблоны»](https://deckhouse.ru/modules/multitenancy-manager/v1.78/usage.html#структурированные-шаблоны).

- <a id="release-1-78-project-manifests"></a>Поле [`manifests`](https://deckhouse.ru/modules/multitenancy-manager/v1.78/cr.html#projecttemplate-v1alpha2-spec-manifests) в ProjectTemplate v1alpha2 задаёт шаблонизируемые namespaced-объекты, которые контроллер применяет от имени пользователя проекта с ролью `d8:project:admin`, а не со своими правами. Через `manifests` можно создавать дополнительные неймспейсы проекта (ProjectNamespace) и размещать в них объекты. Порядок перевода шаблона на `manifests` описан [в разделе «Перевод шаблона с Helm-текста»](https://deckhouse.ru/modules/multitenancy-manager/v1.78/usage.html#перевод-шаблона-с-helm-текста).

- При включённом модуле [`multitenancy-manager`](https://deckhouse.ru/modules/multitenancy-manager/v1.78/) закрыты способы создать неймспейс в обход проектов: создавать неймспейсы с именами длиннее 61 символа, с префиксом `upmeter-` или с именем `deckhouse` могут только системные компоненты и администраторы кластера. Неймспейсы, которые остались без проекта, показывает алерт [D8NamespaceNotAdopted](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#multitenancy-manager-d8namespacenotadopted).

- Удаление проекта требует права на удаление его неймспейсов. Подробнее — [в разделе «Дополнительные неймспейсы проекта»](https://deckhouse.ru/modules/multitenancy-manager/v1.78/usage.html#дополнительные-неймспейсы-проекта).

- Лейблы и аннотации неймспейса, принятого в проект, больше не копируются в параметры Project, и релиз проекта их не откатывает. Дополнительные неймспейсы проекта их не наследуют. Нужные лейблы задайте в `spec.parameters.namespace.labels` проекта. Неймспейс, принятый в проект, сохраняет свой Pod Security Standard.

- Параметр [`allowNamespacesWithoutProjects`](https://deckhouse.ru/modules/multitenancy-manager/v1.78/configuration.html#parameters-allownamespaceswithoutprojects) больше не считается устаревшим. Ограничение реализовано через ValidatingAdmissionPolicy.

- <a id="release-1-78-project-namespace-restore"></a>Основной неймспейс проекта, удалённый в обход политики (например, от `system:sudouser`), восстанавливается контроллером вместе с объектами шаблона. При обновлении восстановятся все проекты, потерявшие основной неймспейс.

- <a id="release-1-78-helm-project-templates"></a>Helm-шаблоны проектов (`resourcesTemplate`) устарели. Создать [ProjectTemplate](https://deckhouse.ru/modules/multitenancy-manager/v1.78/cr.html#projecttemplate) через `deckhouse.io/v1alpha1` больше нельзя, задать новый `resourcesTemplate` тоже. Существующие шаблоны продолжают работать и редактироваться. Но восстановление такого шаблона из резервной копии, GitOps-бутстрап или пересоздание будут отклонены, пока шаблон не переведён на структурные поля и `manifests`. Такие шаблоны показывает алерт [MultitenancyManagerHelmProjectTemplate](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#multitenancy-manager-multitenancymanagerhelmprojecttemplate). AuthorizationRule уровня Admin в `manifests` запрещены, администраторов проекта задавайте в `spec.administrators`. Подробнее — [о поле `manifests`](#release-1-78-project-manifests).

- <a id="release-1-78-grant-policy"></a>[ClusterResourceGrantPolicy](https://deckhouse.ru/modules/multitenancy-manager/v1.78/cr.html#clusterresourcegrantpolicy) хранится в версии v1alpha2. Версия v1alpha2 выбирает проекты по лейблам Project, а неймспейсы — отдельным `namespaceSelector`. Политики, записанные через v1alpha1, продолжают выбирать те же неймспейсы. При переводе манифеста на v1alpha2 переименуйте `projectSelector` в `namespaceSelector`. Если поменять только `apiVersion`, селектор начнёт сопоставляться с лейблами Project и политика перестанет применяться к нужным неймспейсам. Дополнительные неймспейсы наследуют лейблы шаблона. Подробнее — [в разделе «Как политика выбирает неймспейсы»](https://deckhouse.ru/modules/multitenancy-manager/v1.78/usage.html#как-политика-выбирает-неймспейсы).

#### Безопасность и сертификаты {#release-1-78-security}

Изменения в политиках безопасности и управлении сертификатами.

##### Политики безопасности

Изменения модуля [`admission-policy-engine`](https://deckhouse.ru/modules/admission-policy-engine/v1.78/):

- <a id="release-1-78-pss-default"></a>Политика Pod Security Standards по умолчанию — всегда [Baseline](https://deckhouse.ru/modules/admission-policy-engine/v1.78/configuration.html#parameters-podsecuritystandards-defaultpolicy).
  В кластерах, развёрнутых на версии Deckhouse Platform ниже 1.55, по умолчанию действовала политика `Privileged`.

  В таких кластерах обновление блокируется, пока в ModuleConfig [`admission-policy-engine`](https://deckhouse.ru/modules/admission-policy-engine/v1.78/) не задан параметр [`settings.podSecurityStandards.defaultPolicy`](https://deckhouse.ru/modules/admission-policy-engine/v1.78/configuration.html#parameters-podsecuritystandards-defaultpolicy).

  Чтобы сохранить текущее поведение, укажите `Privileged`. Перед переходом на `Baseline` или `Restricted` убедитесь, что нагрузки в несистемных неймспейсах соответствуют выбранной политике.

  Если требованиям не соответствуют только отдельные нагрузки, не понижайте политику для всего кластера. Оформите для них исключения [SecurityPolicyException](https://deckhouse.ru/modules/admission-policy-engine/v1.78/#исключения-из-политик-безопасности) на уровне пода или отдельного контейнера.

- <a id="release-1-78-selinux"></a>Проверка SELinux в Pod Security Standards изменена. Политики `Baseline` и `Restricted` отклоняют поды, у которых SELinux type отличается от `container_t`, `container_init_t`, `container_kvm_t` и `container_engine_t`, а также поды с заданными SELinux user или role. До обновления такие поды проходят проверку. Пример политики с допустимыми SELinux type приведён [в разделе «Как включить одну или несколько политик Pod Security Standards, не отключая весь набор?»](https://deckhouse.ru/modules/admission-policy-engine/v1.78/faq.html#как-включить-одну-или-несколько-политик-pod-security-standards-не-отключая-весь-набор).

- Правила [Pod Security Standards](https://deckhouse.ru/modules/admission-policy-engine/v1.78/#pod-security-standards) опубликованы как объекты SecurityPolicy `d8-pod-security-baseline` и `d8-pod-security-restricted`. Префиксы имён `d8-pod-security-`, `d8-system-default-` и `d8-controllers-` для SecurityPolicy и OperationPolicy зарезервированы за модулем.

- **Валидация контроллеров.** Проверку контроллеров можно настроить для каждого правила: [`podSecurityStandards.controllerValidationExcludedPolicies`](https://deckhouse.ru/modules/admission-policy-engine/v1.78/configuration.html#parameters-podsecuritystandards-controllervalidationexcludedpolicies) для Pod Security Standards и `spec.controllerValidation` для [OperationPolicy](https://deckhouse.ru/modules/admission-policy-engine/v1.78/cr.html#operationpolicy-v1alpha1-spec-controllervalidation) и [SecurityPolicy](https://deckhouse.ru/modules/admission-policy-engine/v1.78/cr.html#securitypolicy-v1alpha1-spec-controllervalidation). На переходный период контроллер, который создаёт поды и нарушает политику с `enforcementAction: Deny`, получает предупреждение вместо отказа. Сами поды по-прежнему отклоняются. Чтобы отклонять и контроллер, задайте [`controllerValidationEnforcement: Policy`](https://deckhouse.ru/modules/admission-policy-engine/v1.78/configuration.html#parameters-podsecuritystandards-controllervalidationenforcement) или `spec.controllerValidation.mode: Enforce`.

- В исключениях [SecurityPolicyException](https://deckhouse.ru/modules/admission-policy-engine/v1.78/cr.html#securitypolicyexception) для hostPath можно разрешить вложенные пути параметром `pathType` со значением `Prefix`. Также в SecurityPolicyException можно задавать исключения для sysctl, профилей AppArmor Localhost и общих hostPath-томов.

- <a id="release-1-78-exec-attach"></a>Политики для `exec` и `attach` снова применяются в пользовательских неймспейсах, как в Deckhouse Platform 1.73–1.74. Подробнее — [в разделе «Как запретить операции kubectl exec и kubectl attach в определённые поды?»](https://deckhouse.ru/modules/admission-policy-engine/v1.78/faq.html#как-запретить-операции-kubectl-exec-и-kubectl-attach-в-определённые-поды).

- Учётные данные хранилищ образов для проверки подписей образов кешируются на 5 минут вместо 12 часов. Время кеширования задаёт параметр [`imageSignatureVerification.registryCredentialsCacheTTLSeconds`](https://deckhouse.ru/modules/admission-policy-engine/v1.78/configuration.html#parameters-imagesignatureverification-registrycredentialscachettlseconds).

- Алерты о нарушениях политик содержат более подробную информацию и срабатывают отдельно по каждой политике и неймспейсу.

- Алерты сообщают о недоступности компонентов модуля `admission-policy-engine` и admission-вебхука. Подробнее — [в разделе «Что делать, если validating-вебхук недоступен?»](https://deckhouse.ru/modules/admission-policy-engine/v1.78/faq.html#что-делать-если-validating-вебхук-недоступен).

- Политики продолжают действовать во время обновления, которое добавляет шаблон констрейнта. Вебхук Gatekeeper перестаёт направлять запросы на недоступный узел через 30 секунд вместо 5 минут.

- При выключении модуля `admission-policy-engine` больше не блокируются exec в поды и очередь Deckhouse Platform.

##### Управление сертификатами

Изменения модуля [`cert-manager`](https://deckhouse.ru/modules/cert-manager/v1.78/):

- <a id="release-1-78-cert-manager-proxy"></a>`cert-manager` использует прокси из параметра [`proxy`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/api/cr.html#clusterconfiguration-proxy) ClusterConfiguration. Запросы к ACME-серверу, HashiCorp Vault, API DNS-провайдеров и self-check HTTP-01 идут через прокси. Подробнее — [в разделе «Как cert-manager работает через прокси-сервер?»](https://deckhouse.ru/modules/cert-manager/v1.78/faq.html#как-cert-manager-работает-через-прокси-сервер).

- Параметр [`highAvailability`](https://deckhouse.ru/modules/cert-manager/v1.78/configuration.html#parameters-highavailability) управляет режимом высокой доступности модуля `cert-manager` независимо от глобальной настройки.

- Новые параметры модуля `cert-manager`:
  - [`enableACMERenewalInfo`](https://deckhouse.ru/modules/cert-manager/v1.78/configuration.html#parameters-enableacmerenewalinfo) — включает ACME Renewal Information (ARI). `cert-manager` перевыпускает сертификат в окне, которое назначил ACME-сервер, например Let's Encrypt.
  - [`http01WaitInsteadOfSelfCheck`](https://deckhouse.ru/modules/cert-manager/v1.78/configuration.html#parameters-http01waitinsteadofselfcheck) — заменяет self-check HTTP-01 у ClusterIssuer Let's Encrypt ожиданием. Сертификаты выпускаются и в кластерах, которые не могут обратиться к своему публичному адресу, например из-за NAT без hairpinning или split-horizon DNS.
  - [`certificateRequestMaximumBackoffDuration`](https://deckhouse.ru/modules/cert-manager/v1.78/configuration.html#parameters-certificaterequestmaximumbackoffduration) — ограничивает задержку между попытками выпуска после неудачного CertificateRequest. С меньшим значением `cert-manager` быстрее повторяет выпуск, когда издатель снова доступен, например после обслуживания CA.

- Вебхук `cert-manager` продолжает обслуживать запросы во время обновления и ротации своего CA. Мутирующий вебхук вызывается только для новых CertificateRequest.

- Поле `spec.certificateOwnerRef` стандартного кастомного ресурса `cert-manager` Certificate больше не вызывает бесконечный цикл обновления Secret.

#### Сеть и межкластерное взаимодействие {#release-1-78-network}

Изменения в публикации системных интерфейсов, Istio и взаимодействии подов между кластерами.

##### Публикация системных интерфейсов

Глобальный параметр Deckhouse Platform [`settings.modules.ingress`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/api/global.html#parameters-modules-ingress) включает или отключает создание Ingress-ресурсов сразу для всех модулей, которые это поддерживают. Отдельный модуль может переопределить его собственным параметром `ingress.enabled`.

##### Service mesh на базе Istio

Изменения модуля [`istio`](https://deckhouse.ru/modules/istio/v1.78/):

- <a id="release-1-78-istio-versions"></a>Istio 1.29 поддерживается. Поддержка Istio 1.21 прекращена. Минимальная поддерживаемая версия — 1.25.

- <a id="release-1-78-istio-ca"></a>Перед применением настроек Istio проверяет сертификаты и ключ CA. Если ранее заданные данные некорректны, модуль не сможет применить конфигурацию до их исправления.

- Для Istio 1.27 и новее по умолчанию используется native sidecar.

- Сертификат и ключ CA для Istio можно хранить в Secret, а в настройках модуля `istio` указывать ссылку на него вместо самих значений.

- В редакции Ultimate/EE выполняется регулярная проверка конфигурации Istio с оповещениями об ошибках.

- Для авторизации запросов в Istio можно использовать внешний HTTP-сервис.

- [OpenTelemetry и Zipkin](https://deckhouse.ru/modules/istio/v1.78/examples.html#трассировка-через-telemetry-api) доступны как провайдеры трассировки для отправки трассировок отдельных приложений в соответствующие коллекторы.

- В статусах IstioMulticluster причины ошибок обмена метаданными описываются точнее.

- Исправлены шаблоны внедрения прокси Istio. В некоторых конфигурациях они мешали запуску подов с native sidecar или не добавляли прокси шлюза необходимые настройки безопасности и проверку работоспособности.

- Исправлено определение версии Istio для подов. В некоторых конфигурациях это устраняет ложные предупреждения о несовпадении версий и позволяет корректно обновлять sidecar автоматически.

- Оператор Istio 1.25 запускается на узлах с containerd v2 при файловой системе контейнера в режиме только чтения.

- Исправлена ошибка, из-за которой после изменения версии Istio или отключения модуля мог оставаться запущенным istiod версии 1.25.

- Если Helm-релиз Istio 1.25 застрял при установке, обновлении или удалении, оператор восстанавливает его и продолжает применять конфигурацию.

- Добавление или удаление версии Istio больше не вызывает перезапуск `kube-apiserver`.

- При проверке совместимости перед обновлением Deckhouse Platform учитываются и версии Istio, удаление которых ещё не завершилось.

- В Istio 1.25 реплики istiod размещаются на разных узлах.

- Роль User может просматривать [WaypointInstance](https://deckhouse.ru/modules/istio/v1.78/cr.html#waypointinstance), роль Editor — управлять ими.

- **Защита обмена метаданными между кластерами Istio.** Если идентификатор соседнего кластера изменится, Deckhouse Platform остановит обмен приватными данными и сообщит об этом. При намеренной замене кластера подключение потребуется создать заново.

##### Межкластерное взаимодействие подов через ClusterMesh {#release-1-78-clustermesh}

ClusterMesh в модуле `cni-cilium` (доступен в редакциях Ultimate/EE) — экспериментальная функция, обеспечивающая прямое взаимодействие подов между кластерами, объединёнными с помощью модуля peering. Подробнее — [в разделе «Пиринг кластеров»](https://deckhouse.ru/modules/cni-cilium/v1.78/advanced_usage.html#пиринг-кластеров).

#### Управление платформой и модулями {#release-1-78-platform}

Изменения в хранении образов компонентов платформы, поставке модулей и доступе к их документации.

##### Управление хранением образов Deckhouse Platform {#release-1-78-registry}

Изменения модуля [`registry`](https://deckhouse.ru/modules/registry/v1.78/):

- **Управление путями загрузки образов контейнеров компонентов Deckhouse Platform.** Адрес upstream и использование внутрикластерного кеша меняются без перезаписи ссылок на образы и без обновления всех узлов, поскольку `registry-agent` на каждом узле настраивает containerd на постоянный внутрикластерный адрес. Оба параметра задаются независимо [в ModuleConfig `registry`](https://deckhouse.ru/modules/registry/v1.78/configuration.html). С кешем все узлы получают образы из хранилища на master-узлах. Без upstream кластер работает изолированно, и единственным источником образов становится кеш. Перед удалением upstream модуль проверяет, что кеш содержит ожидаемый набор образов. Модуль принимает управление автоматически после обновления до 1.78, если кластер [подготовлен на версии 1.77](#release-1-78-registry-migration).

- <a id="release-1-78-registry-ports"></a>Хранилище образов на master-узлах использует порт `4281/TCP` для загрузки агентами и репликации и `4283/TCP` для публикации через registry-push Ingress вместо `5001` и `5003`. `registry-agent` остаётся на `127.0.0.1:5001`, ссылки `registry.d8-system.svc:5001` не меняются. Публикация и загрузка через кеш разрешены только в репозиториях под `system/deckhouse`. Подробнее — [в разделе «Внутрикластерный кеш»](https://deckhouse.ru/modules/registry/v1.78/#внутрикластерный-кеш).

- <a id="release-1-78-registry-tolerations"></a>`registry-controller`, `registry-storage` и `registry-packages-proxy` больше не учитывают ключи из [`global.modules.placement.customTolerationKeys`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/api/global.html#parameters-modules-placement-customtolerationkeys).

- **Обслуживание внутрикластерного кеша модуля `registry`.** Сборка мусора запускается после обновления набора образов и при отказе записи в хранилище. Её можно запустить вручную через RegistryOperation, а реплики хранилища сообщают ожидаемый объём освобождаемого места. Повреждённые blob-объекты проверяются и восстанавливаются в фоновом режиме.

- **Диагностика кеша registry.** Объект RegistryImage показывает содержимое кеша. Метрики `registry-agent` собираются через textfile collector `node-exporter`. Подробнее — [в разделе «Кеш растёт. Что его чистит?»](https://deckhouse.ru/modules/registry/v1.78/faq.html#кеш-растёт-что-его-чистит).

- **RegistryUpstream без `registry-agent`.** Дополнительные хранилища образов контейнеров, например хранилища вендоров или собственные, объявленные в RegistryUpstream, доступны узлам и при выключенном модуле `registry`, и в режиме `Unmanaged`. Настройки применяются на узлах Generic OS и Deckhouse Engine. `registry-agent` учитывает параметр `proxy` ClusterConfiguration при обращении к upstream registry. Подробнее — [в разделе «Дополнительные хранилища образов контейнеров»](https://deckhouse.ru/modules/registry/v1.78/#дополнительные-хранилища-образов-контейнеров).

- **Обновление и переключение хранилища registry.** Хранилище выбирает нового лидера при потере master-узла, отказе registry или диска. Адрес кеша на узлах не переключается преждевременно во время смены лидера. Агент не ждёт недоступного лидера при каждой загрузке образа. Обновление кеша работает в изолированном кластере с одним master-узлом, а причину блокировки показывает алерт [D8RegistryStorageUpdateBlocked](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#registry-d8registrystorageupdateblocked). Подробнее — [в разделе «Как обновляется кеш?»](https://deckhouse.ru/modules/registry/v1.78/faq.html#как-обновляется-кеш).

- Устранено зависание `registry-packages-proxy` после ошибки кеша и выдача повреждённых пакетов, которые блокировали обновление узлов.

##### Модули из внешних источников {#release-1-78-external-modules}

Следующие модули загружаются из внешних источников и развиваются в рамках отдельного релизного цикла:

- Модуль [`cloud-provider-dvp`](https://deckhouse.ru/modules/cloud-provider-dvp/). Обновление Deckhouse Platform при использовании этого модуля возможно, только если модуль доступен из [источника модулей (ModuleSource)](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.78/reference/api/cr.html#modulesource).
- Модуль [`service-with-healthchecks`](https://deckhouse.ru/modules/service-with-healthchecks/). Модуль будет установлен автоматически, если он включён в кластере.

##### Документация модулей

Документацию модуля можно открыть для любой версии, независимо от версии, установленной в кластере. Для просмотра нужной версии выберите «Посмотреть другие версии» в меню «Канал/версия» на странице документации модуля.

### Исправления уязвимостей

Исправления, связанные с безопасностью:

- Настройки прокси в ClusterConfiguration больше не позволяют выполнять произвольные shell-команды при бутстрапе.

- DexAuthenticator больше не пишет в лог cookie запроса, включая refresh-токен, при ошибке CSRF (GHSA-hhqp-vx7f-5c6m).

- В компонентах Istio и Kiali устранены уязвимости.

- Исправлены уязвимости в `golang.org/x/net`, `golang.org/x/text`, `go-jose/v4`, `go-chi/chi/v5` и `klauspost/compress` в модулях [`user-authn`](https://deckhouse.ru/modules/user-authn/v1.78/), [`user-authz`](https://deckhouse.ru/modules/user-authz/v1.78/), [`multitenancy-manager`](https://deckhouse.ru/modules/multitenancy-manager/v1.78/) и модулях подсистемы Security.

### Обновление версий компонентов

Для компонентов с несколькими вариантами поставки указаны обновлённые версии каждого варианта. Поддерживаемые версии Kubernetes и версия по умолчанию приведены [в описании изменений control plane](#release-1-78-kubernetes).

Новые версии компонентов Deckhouse Platform:

- Amazon EC2 Utils: 2.3.0
- Amazon ECR credential provider: v1.33.4, v1.34.3, v1.35.2, v1.36.0, v1.37.0
- Amazon Web Services Cloud provider: v1.33.4, v1.34.3, v1.35.2, v1.36.0, v1.37.0
- Amazon Web Services CSI driver: v1.54.0, v1.66.0
- Amazon Web Services Terraform provider: 6.69.1
- cert-manager: 1.21.2
- cfssl: 1.7.1
- Cluster API: 1.12.11
- Cluster Autoscaler: v1.33.1, v1.34.4, v1.35.2, v1.36.1
- containerd: 1.7.36, 2.2.10
- Descheduler: 0.37.0
- Dex: 2.46.0
- Dynamix Terraform provider: 4.12.2
- etcd: 3.6.15
- Gatekeeper: 3.23.1
- Google Cloud Platform Cloud provider: ccm/v33.4.0, ccm/v34.2.0, ccm/v35.0.5
- Google Cloud Platform CSI driver: v1.15.4, v1.27.0
- Istio: 1.29.6
- Kiali: 2.21.0
- Kubernetes: 1.33.13, 1.34.13, 1.35.10, 1.36.6, 1.37.2
- Kubernetes Autoscaler: 1.8.1
- Microsoft Azure Cloud provider: v1.33.6, v1.34.3, v1.36.1, v1.37.0
- Microsoft Azure CSI driver: v1.33.8, v1.34.2, v1.36.0
- OAuth2 Proxy: 7.15.5
- OpenStack Cloud provider: v1.33.1, v1.34.1, v1.35.0, v1.36.0
- OpenStack CSI driver: v1.33.1, v1.34.1, v1.35.0, v1.36.0
- QEMU guest agent: 9.2.4
- Ratify: 1.4.6
- runc: 1.3.6
- VMware vSphere Cloud provider: v1.33.1, v1.34.0, v1.35.1, v1.37.0
- VMware vSphere Cluster API provider: v1.15.3
- VMware vSphere CSI driver: v3.7.1, v3.7.3
- VPA: 1.8.1

Модули [`user-authn`](https://deckhouse.ru/modules/user-authn/v1.78/), [`user-authz`](https://deckhouse.ru/modules/user-authz/v1.78/), [`multitenancy-manager`](https://deckhouse.ru/modules/multitenancy-manager/v1.78/) и [`admission-policy-engine`](https://deckhouse.ru/modules/admission-policy-engine/v1.78/) используют базовый distroless-образ.

Полный перечень изменений доступен [в changelog версии 1.78](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.78.md).

## Версия 1.77

### Обратите внимание

- Добавлена поддержка Kubernetes 1.36 и прекращена поддержка Kubernetes 1.31.
  Используемая по умолчанию версия Kubernetes изменена на 1.34 ([параметр `kubernetesVersion`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.77/reference/api/cr.html#clusterconfiguration-kubernetesversion) установлен в `Automatic`).

- Во время обновления будут перезапущены компоненты control plane.

- Будут перезапущены все поды Ingress NGINX Controller.

- При включённой настройке [`enableMultiTenancy`](https://deckhouse.ru/modules/user-authz/v1.77/configuration.html#parameters-enablemultitenancy) модуля `user-authz`
  после обновления начинают действовать существующие RoleBinding и ClusterRoleBinding,
  которые ранее не учитывались из-за ограничений ClusterAuthorizationRule.
  Перед обновлением проверьте назначенные пользователям привязки RoleBinding и ClusterRoleBinding
  и скорректируйте или удалите те из них, которые не должны предоставлять доступ.

- Если в кластере DP используются кастомные роли устаревшей схемы RBACv2,
  после обновления сработает [алерт `D8UserAuthzLegacyRBACv2CustomRoleFound`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.77/reference/alerts.html#user-authz-d8userauthzlegacyrbacv2customrolefound).
  Такие роли необходимо перевести на новую схему до обновления на DP 1.78, иначе оно будет заблокировано.
  Порядок миграции описан [в FAQ модуля `user-authz`](https://deckhouse.ru/modules/user-authz/v1.77/faq.html#как-перевести-кастомные-роли-на-новую-схему-в-dp-178).

- Если в кластере используется реализация v1 управления хранилищем образов (секция `registry` ModuleConfig `deckhouse`) в режиме `Proxy` или `Local`
  либо в режиме `Direct` без настроенного ModuleConfig `registry`,
  кластер необходимо перевести на модуль [`registry`](https://deckhouse.ru/modules/registry/v1.77/) до обновления на DP 1.78, иначе оно будет заблокировано.
  Порядок миграции описан [в FAQ модуля `registry`](https://deckhouse.ru/modules/registry/v1.78/faq.html#как-устроена-миграция-на-модуль-registry).

- Если конфигурация [модуля `metallb`](https://deckhouse.ru/modules/metallb/v1.77/) управляется через GitOps,
  перед обновлением временно отключите синхронизацию из вашего репозитория,
  а после обновления замените в репозитории конфигурацию BGP из ModuleConfig на кастомные ресурсы [MetalLoadBalancerConfiguration](https://deckhouse.ru/modules/metallb/v1.77/cr.html#metalloadbalancerconfiguration), [MetalLoadBalancerBGPPeer](https://deckhouse.ru/modules/metallb/v1.77/cr.html#metalloadbalancerbgppeer) и [MetalLoadBalancerPool](https://deckhouse.ru/modules/metallb/v1.77/cr.html#metalloadbalancerpool)
  и снова включите синхронизацию.

- Если в кластере используются политики NetworkPolicy с правилом `ipBlock: 0.0.0.0/0`, после обновления они начнут распространяться и на внутрикластерный трафик. Перед обновлением рекомендуется проверить и при необходимости скорректировать такие политики.

- В провайдерах AWS, Azure и GCP CNI по умолчанию изменён на Cilium.
  Перед созданием или обновлением кластера убедитесь, что используется ядро Linux версии не ниже 5.8,
  а сетевые политики и межсетевые экраны не блокируют UDP-порты, необходимые для работы Cilium в режиме VXLAN.
  Несоблюдение этих требований может привести к ошибкам при создании или обновлении кластера
  или к нарушению сетевой связности между узлами.
  Подробные требования приведены [в разделе «Сетевое взаимодействие»](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.77/reference/network_interaction.html).

- Провайдеры vSphere, Huawei Cloud, OpenStack и VCD переведены с Terraform на OpenTofu.
  Миграция выполняется автоматически и не требует ручных действий.

- Провайдеры Yandex Cloud и OpenStack переведены с Machine Controller Manager (MCM) на Cluster API (CAPI).
  Механизм управления определяется для каждой группы узлов CloudEphemeral отдельно. Существующие группы продолжают работать на MCM,
  а новые по умолчанию создаются на CAPI, поэтому в одном кластере могут быть группы обоих видов.
  Порядок перевода существующей группы приведён [в разделе «Как мигрировать группы узлов на Cluster API (CAPI)»](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.77/faq.html#как-мигрировать-группы-узлов-на-cluster-api-capi).

- Следующие модули теперь загружаются из внешних источников и развиваются в рамках отдельного релизного цикла
  (никаких ручных действий для миграции не требуется):

  - модули системы мониторинга:

    - `extended-monitoring`;
    - `log-shipper`;
    - `loki`;
    - `monitoring-custom`;
    - `monitoring-deckhouse`;
    - `monitoring-kubernetes`;
    - `monitoring-ping`;
    - `okmeter`;
    - `operator-prometheus`;
    - `prometheus`;
    - `prometheus-metrics-adapter`;
    - `prometheus-pushgateway`;
    - `upmeter`;

  - модуль `ingress-nginx`.

- Модуль `monitoring-kubernetes-control-plane` был объединён с модулем `control-plane-manager`.
  Мониторинг компонентов control plane теперь осуществляется модулем `control-plane-manager`.

### Подсистема Deckhouse

- [В модуль `deckhouse`](https://deckhouse.ru/modules/deckhouse/v1.77/) добавлена политика,
  запрещающая несистемным пользователям устанавливать и изменять лейбл `heritage`.
  Это предотвращает случайное создание ресурсов, которые DP может принять за собственные.

- Повышена надёжность обновления кластеров, использующих внешние модули (ModuleSource).
  Обновление начинается только после предварительной загрузки модулей,
  что исключает их временную недоступность в процессе обновления.

- В модуль `registry` добавлена поддержка установки кластера с хранилищем образов [в режиме `Local`](https://deckhouse.ru/modules/deckhouse/v1.77/configuration.html#parameters-registry-mode).
  Это позволяет разворачивать статические кластеры без доступа к внешнему хранилищу, в том числе в изолированных окружениях.

### Подсистема Kubernetes & Scheduling

- Изменения [в модуле `descheduler`](https://deckhouse.ru/modules/descheduler/v1.77/):

  - Добавлена метрика `descheduler_pod_evictions_total` с информацией о нагрузке, к которой относится вытесняемый под.

  - Добавлен дашборд Grafana «Workload Happiness», отражающий показатель happiness score у нагрузок,
    а также баланс нагрузки по узлам и историю активности модуля (вытеснения, применение стратегий, таймлайн).

- Изменения [в модуле `vertical-pod-autoscaler`](https://deckhouse.ru/modules/vertical-pod-autoscaler/v1.77/):

  - Добавлены [алерт `VerticalPodAutoscalerDeprecatedUpdateModeAuto`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.77/reference/alerts.html#vertical-pod-autoscaler-verticalpodautoscalerdeprecatedupdatemodeauto)
    и дашборд Grafana «VPA Deprecated Update Mode», позволяющие обнаружить объекты VerticalPodAutoscaler,
    использующие устаревший режим `updateMode: Auto`.

  - Повышена надёжность развёртывания вебхука `vertical-pod-autoscaler`.
    Исправлена проблема с порядком запуска компонентов, которая могла приводить к ошибкам во время обновления.

- Изменения [в модуле `control-plane-manager`](https://deckhouse.ru/modules/control-plane-manager/v1.77/):

  - Модуль переведён на новую внутреннюю реализацию управления компонентами control plane.
    Добавлены новые кастомные ресурсы [ControlPlaneNode](https://deckhouse.ru/modules/control-plane-manager/v1.77/cr.html#controlplanenode) и [ControlPlaneOperation](https://deckhouse.ru/modules/control-plane-manager/v1.77/cr.html#controlplaneoperation)
    для отслеживания состояния узлов и операций обновления компонентов control plane.

  - Добавлена автоматическая дефрагментация etcd,
    которая по умолчанию включена для мультимастерных кластеров и запускается при фрагментации выше 40%.
    [В настройках модуля](https://deckhouse.ru/modules/control-plane-manager/v1.77/configuration.html#parameters-etcd-defrag) можно задать расписание дефрагментации.

  - В ModuleConfig модуля перенесён ряд настроек и добавлена автоматическая миграция и алерты,
    уведомляющие о необходимости обновить конфигурацию:

    - настройки публикации Kubernetes API ([`publishAPI`](https://deckhouse.ru/modules/control-plane-manager/v1.77/configuration.html#parameters-apiserver-publishapi)) из модуля `user-authn`;

    - [параметр `encryptionAlgorithm`](https://deckhouse.ru/modules/control-plane-manager/v1.77/configuration.html#parameters-encryptionalgorithm) из ClusterConfiguration;

    - [параметр `resourcesRequests`](https://deckhouse.ru/modules/control-plane-manager/v1.77/configuration.html#parameters-resourcesrequests) для компонентов control plane из ModuleConfig `global`.

- В `kube-scheduler` добавлен патч, предотвращающий назначение новых подов на узлы,
  находящиеся в процессе плавного завершения работы (graceful shutdown).

- Ускорено обновление etcd в одномастерных кластерах.
  Устранены простои компонентов control plane, возникавшие во время обновления.

- Снижен риск троттлинга `kube-controller-manager` при интенсивной работе контроллеров.

- Ускорена подготовка подов к запуску за счёт параллельной загрузки нескольких образов контейнеров.

- В конфигурации kubelet включён по умолчанию feature gate `KubeletEnsureSecretPulledImages`
  для соответствия требованиям CIS Benchmark.

### Подсистема IAM

- Изменения [в модуле `user-authn`](https://deckhouse.ru/modules/user-authn/v1.77/):

  - Усилена защита аутентификации в Dex: добавлено [ограничение частоты запросов](https://deckhouse.ru/modules/user-authn/v1.77/configuration.html#parameters-ratelimit) для эндпоинтов аутентификации,
    [автоматическая блокировка](https://deckhouse.ru/modules/user-authn/v1.77/configuration.html#parameters-passwordpolicy-lockout) учётных записей LDAP/Crowd после серии неудачных попыток входа
    и [возможность вручную блокировать или разблокировать](https://deckhouse.ru/modules/user-authn/v1.77/cr.html#useroperation-v1-spec-target) внешние учётные записи.

  - Ресурс UserOperation теперь поддерживает постоянную блокировку пользователей ([`spec.lock.for: permanent`](https://deckhouse.ru/modules/user-authn/v1.77/cr.html#useroperation-v1-spec-lock-for)).
    Также расширены аудит-логи операций над учётными записями: теперь они содержат информацию об инициаторе операции,
    её типе и целевом пользователе.

  - Добавлен [ресурс DexProviderCheck](https://deckhouse.ru/modules/user-authn/v1.77/cr.html#dexprovidercheck) для проверки работоспособности провайдеров аутентификации (LDAP, OIDC и других).
    Он позволяет диагностировать проблемы с подключением и учётными данными до того, как они повлияют на вход пользователей.

  - В настройки парольной политики добавлен [уровень сложности `Custom`](https://deckhouse.ru/modules/user-authn/v1.77/configuration.html#parameters-passwordpolicy-custom),
    позволяющий гибко настраивать требования к паролям в соответствии с корпоративными стандартами.

  - [В новом поле `spec.podMetadata`](https://deckhouse.ru/modules/user-authn/v1.77/cr.html#dexauthenticator-v1-spec-podmetadata) кастомного ресурса DexAuthenticator добавлена возможность
    задавать пользовательские лейблы и аннотации, например, аннотации для управления сайдкар-контейнером Istio.

  - DexAuthenticator теперь корректно работает в неймспейсах с ResourceQuota и политиками,
    требующими лимиты ресурсов для всех контейнеров.
    Исправлена проблема, из-за которой под аутентификатора мог не запускаться.

- Изменения [в модуле `user-authz`](https://deckhouse.ru/modules/user-authz/v1.77/):

  - В DP 1.78 меняется схема лейблов агрегации ролевой модели RBACv2.
    Добавлен [алерт `D8UserAuthzLegacyRBACv2CustomRoleFound`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.77/reference/alerts.html#user-authz-d8userauthzlegacyrbacv2customrolefound), который обнаруживает кастомные роли устаревшей схемы.
    Такие роли необходимо перевести на новую схему до обновления на DP 1.78, иначе они перестанут работать,
    а обновление будет заблокировано. Порядок миграции описан [в FAQ модуля](https://deckhouse.ru/modules/user-authz/v1.77/faq.html#как-перевести-кастомные-роли-на-новую-схему-в-dp-178).

  - При включённой [настройке `enableMultiTenancy`](https://deckhouse.ru/modules/user-authz/v1.77/configuration.html#parameters-enablemultitenancy) права доступа пользователя
    теперь определяются объединением ClusterAuthorizationRule, AuthorizationRule и стандартных RBAC-правил (RoleBinding/ClusterRoleBinding).
    После обновления могут начать действовать существующие RBAC-привязки,
    которые раньше не учитывались из-за ограничений ClusterAuthorizationRule.
    Рекомендуется проверить такие привязки перед обновлением.
    Подробнее — [в подразделе «Совместное использование ClusterAuthorizationRule, AuthorizationRule и RBAC»](https://deckhouse.ru/modules/user-authz/v1.77/#совместное-использование-clusterauthorizationrule-authorizationrule-и-rbac).

  - Добавлен [алерт `D8UserAuthzPermissionBrowserUnavailable`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.77/reference/alerts.html#user-authz-d8userauthzpermissionbrowserunavailable),
    который уведомляет о недоступности сервиса просмотра прав пользователей (`permission-browser-apiserver`).

### Подсистема Security

- Изменения [в модуле `admission-policy-engine`](https://deckhouse.ru/modules/admission-policy-engine/v1.77/):

  - Исключения из SecurityPolicy (SecurityPolicyException) теперь можно задавать на уровне отдельного контейнера,
    а не только целиком на под.
    Это позволяет ослаблять требования безопасности для отдельного сайдкар- или init-контейнера,
    не снимая ограничения со всего пода.
    Подробнее — [в документации модуля](https://deckhouse.ru/modules/admission-policy-engine/v1.77/#исключения-из-политик-безопасности).

  - Добавлена встроенная политика ValidatingAdmissionPolicy [`deny-deckhouse-finalizers.deckhouse.io`](https://deckhouse.ru/modules/admission-policy-engine/v1.77/faq.html#встроенная-политика-для-финалайзеров),
    запрещающая вручную удалять финалайзеры `deckhouse.io` с ресурсов.
    Это предотвращает некорректное удаление объектов и появление orphan-ресурсов.

- В модуле `multitenancy-manager` добавлен [механизм предоставления cluster-wide-ресурсов проектам](https://deckhouse.ru/modules/multitenancy-manager/v1.77/usage.html#выдача-кластерных-ресурсов-проектам).
  Администратор DP теперь может определять, какие кластерные ресурсы доступны каждому проекту,
  а также задавать для них значения по умолчанию.
  Механизм может использоваться любыми модулями DP для публикации собственных кластерных ресурсов.

- Добавлена возможность получать список ресурсов сразу из всех доступных неймспейсов без cluster-wide прав.
  Команда `d8 k get <resource> -A --scope=<accessible|projects|system|project:NAME>` возвращает только те объекты,
  к которым у пользователя есть доступ, вместо ошибки `403 Forbidden`.
  Подробнее — [в разделе «Листинг ресурсов с фильтрацией по правам доступа»](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.77/admin/configuration/access/authorization/filtered-listing.html).

- ACL-фильтрация списка неймспейсов теперь поддерживает запросы с режимом `watch`.
  Команда `d8 k get ns -w` отображает изменения в реальном времени без необходимости повторного выполнения запроса.

- Компонент cainjector модуля `cert-manager` теперь [включается автоматически](https://deckhouse.ru/modules/cert-manager/v1.77/configuration.html#parameters-enablecainjector) при обнаружении ресурсов,
  использующих CA-инъекцию `cert-manager`.
  Больше не требуется включать его вручную или постоянно держать запущенным.

### Подсистема Cluster & Infrastructure

- Изменения в `dhctl` (CLI-установщик):

  - Добавлена предварительная проверка длины имени диска для облачных провайдеров.
    Если имя превышает допустимый лимит, установка прерывается ещё до создания инфраструктуры.

  - Улучшен интерактивный режим `dhctl`: добавлена область с логами текущих операций, а вывод стал компактнее,
    благодаря отображению прогресса вместо потока служебных сообщений.
    Полные логи по-прежнему доступны в режиме отладки и при неинтерактивном запуске.

  - Доработана предварительная проверка пользователя `deckhouse`.
    Повторный бутстрап корректно обрабатывает уже существующего пользователя,
    а дополнительные проверки помогают избежать проблем с правами доступа.

  - Исправлены проблемы с очисткой временных процессов и SSH-соединений после выполнения команд `converge` и `destroy`.

  - Исправлена работа команды `dhctl lock release` в интерактивном режиме.

  - Исправлены ошибки команды `dhctl converge-migration`, которые в отдельных сценариях могли приводить к сбою миграции.

  - Повышена надёжность операций `converge` для control plane.
    Проверки выполняются в более корректном порядке, а сообщения о состоянии etcd стали информативнее.

- Изменения [в модуле `node-manager`](https://deckhouse.ru/modules/node-manager/v1.77/):

  - Добавлен [Instance API версии v1alpha2](https://deckhouse.ru/modules/node-manager/v1.77/cr.html#instance).
    Информация о состоянии и ошибках инстансов теперь собрана в одном ресурсе, что упрощает диагностику.

  - Для AWS Spot-инстансов сокращено время ожидания создания MachineDeployment.
    Если AWS не может выделить Spot-инстансы из-за нехватки доступной ёмкости, повторные попытки создания начинаются быстрее.

  - В ресурс NodeGroup добавлен [параметр `spec.kubelet.seccompDefault`](https://deckhouse.ru/modules/node-manager/v1.77/cr.html#nodegroup-v1-spec-kubelet-seccompdefault),
    позволяющий включить seccomp-профиль RuntimeDefault по умолчанию для всех нагрузок без явных seccomp-настроек.

- Для CSI Controller в модулях всех облачных провайдеров уменьшена вероятность ложных перезапусков
  при кратковременных задержках за счёт увеличения таймаута liveness probe.

- Изменения [в модуле `cloud-provider-dvp`](https://deckhouse.ru/modules/cloud-provider-dvp/v1.77/):

  - Реализована поддержка live-миграции дисков между StorageClass без остановки виртуальной машины.
    Миграция выполняется автоматически при изменении StorageClass.

  - Конфигурация модуля перешла на единый контракт ModuleConfig.
    Permanent- и Ephemeral-узлы настраиваются одинаково через InstanceClass и NodeGroup, без отдельного ProviderClusterConfiguration.
    Это упрощает настройку кластера, позволяет версионировать и конвертировать схемы без нарушения совместимости
    и готовит облачные провайдеры к независимому релизному циклу вне ядра DP.
    Подробнее — [в подразделе «Кластер и инфраструктура»](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.77/faq.html#subsystem-cluster-infrastructure) FAQ DP.

  - Исправлена проблема, из-за которой сервисы типа LoadBalancer могли оставаться в состоянии `Pending`,
    а родительский балансировщик не создавался.
    Улучшена обработка конфликтов при обновлении ServiceWithHealthchecks и синхронизация статуса балансировщика.

  - Для StorageClass, созданных через Cluster API кластеров, теперь всегда используется режим `WaitForFirstConsumer`
    независимо от конфигурации родительского кластера.
    Также при обновлении автоматически пересоздаются управляемые StorageClass с несовместимым `volumeBindingMode`.
    Это устраняет проблему, которая могла блокировать обновление и синхронизацию кластеров
    из-за неизменяемого поля `volumeBindingMode`.

- В модулях `cloud-provider-openstack`, `cloud-provider-huaweicloud`, `cloud-provider-vsphere` и `cloud-provider-vcd`
  вместо Terraform теперь используется OpenTofu.
  Это позволяет использовать открытую инфраструктурную платформу, совместимую с Terraform,
  снижает зависимость от изменений лицензирования Terraform
  и унифицирует работу провайдера с остальными cloud-provider-модулями, использующими OpenTofu.

- Провайдеры OpenStack и Yandex Cloud теперь работают на базе Cluster API,
  отраслевого стандарта управления машинными ресурсами в Kubernetes.
  Для плавного перехода сохранена поддержка legacy-режима на базе Machine Controller Manager (MCM).
  Существующие группы узлов продолжают работать на MCM, а новые создаются на Cluster API.

- [В модуле `cloud-provider-huaweicloud`](https://deckhouse.ru/modules/cloud-provider-huaweicloud/v1.77/) исправлена обработка удаления виртуальных машин,
  что повышает надёжность очистки ресурсов и предотвращает ложные ошибки при удалении.
  Виртуальные машины, находящиеся в состоянии удаления более 24 часов, теперь помечаются соответствующим статусом,
  что упрощает диагностику проблем.

- [В модуле `cloud-provider-vsphere`](https://deckhouse.ru/modules/cloud-provider-vsphere/v1.77/) обновлены версии CSI plugin и CSI Syncer,
  а также устранены известные уязвимости в Cloud Controller Manager, CSI plugin и Terraform Manager.
  Это повышает безопасность интеграции и обеспечивает использование актуальных версий компонентов.

- [В модуле `cloud-provider-vcd`](https://deckhouse.ru/modules/cloud-provider-vcd/v1.77/) реализована поддержка аннотации `vcd.cpi.flant.com/load-balancer-ip`
  для сервисов типа LoadBalancer.
  Она позволяет назначить заранее определённый внешний IP-адрес балансировщику.
  Если одновременно указаны аннотация и поле `spec.loadBalancerIP`, приоритет имеет аннотация.

### Подсистема Network

- Изменения [в модуле `istio`](https://deckhouse.ru/modules/istio/v1.77/):

  - Добавлена поддержка Istio 1.27.
    Пользователи получают доступ к новым возможностям последней версии Istio в безоператорном режиме.

  - Расширена поддержка ambient mesh.
    [Новые настройки](https://deckhouse.ru/modules/istio/v1.77/configuration.html#parameters-ambient) позволяют управлять ambient mesh более явно, использовать waypoint-прокси для L7-обработки трафика,
    а также применять протокол HBONE для шифрования трафика между подами
    без необходимости запускать сайдкар-контейнер в каждом поде.

- Изменения [в модуле `cni-cilium`](https://deckhouse.ru/modules/cni-cilium/v1.77/):

  - В новой версии модуля изменено поведение NetworkPolicy с правилом `ipBlock: 0.0.0.0/0`.
    Теперь оно распространяется в том числе на внутрикластерный трафик.

  - Снижено потребление CPU агентами Cilium.
    Высвобождённые ресурсы становятся доступны пользовательским нагрузкам.

- Изменения [в модуле `ingress-nginx`](https://deckhouse.ru/modules/ingress-nginx/):

  - Добавлена поддержка IngressNginxController v1.15.

  - Обновлена формула расчёта нагрузки подов Ingress NGINX Controller
    для горизонтального масштабирования инлетов LoadBalancer, LoadBalancerWithProxyProtocol и LoadBalancerWithSSLPassthrough.
    Контроллер теперь своевременно добавляет и удаляет реплики — поведение под нагрузкой стало стабильнее.

  - API IngressNginxController обновлён до v2.
    Настройки контроля потребления ресурсов `spec.resourcesRequests` заменены
    на [расширенные настройки `spec.resourcesManagement`](https://deckhouse.ru/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-resourcesmanagement), добавлена поддержка resource limits.
    Совместимость с API v1 сохраняется автоматически.

  - Реализована изолированная валидация конфигурации,
    что позволило восстановить полноценную проверку конфигурации NGINX без снижения уровня безопасности.

- [В модуле `cilium-hubble`](https://deckhouse.ru/modules/cilium-hubble/v1.77/) добавлена авторизация для веб-интерфейса Hubble.
  Доступ к сетевой телеметрии кластера теперь можно ограничить по группам пользователей для повышения безопасности.

- [В модуле `metallb`](https://deckhouse.ru/modules/metallb/v1.77/) конфигурация режима BGP перенесена из ModuleConfig
  в отдельные кастомные ресурсы [MetalLoadBalancerPool](https://deckhouse.ru/modules/metallb/v1.77/cr.html#metalloadbalancerpool), [MetalLoadBalancerBGPPeer](https://deckhouse.ru/modules/metallb/v1.77/cr.html#metalloadbalancerbgppeer), и [MetalLoadBalancerConfiguration](https://deckhouse.ru/modules/metallb/v1.77/cr.html#metalloadbalancerconfiguration).
  Существующая конфигурация будет мигрирована автоматически — ручных изменений в кластере не потребуется.
  Все новые настройки BGP необходимо вносить через соответствующие ресурсы.

Полный список изменений, включая перечень обновлённых компонентов,
доступен в [журнале изменений (changelog)](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.77.md) на GitHub.

## Версия 1.76

### Обратите внимание

- [Компонент fencing-agent](https://deckhouse.ru/modules/node-manager/v1.76/cr.html#nodegroup-v1-spec-fencing-mode) модуля `node-manager` переведён на gossip-протокол ([библиотека memberlist](https://github.com/hashicorp/memberlist))
  для распределённого мониторинга состояния узлов.
  Это снижает риск ложных перезагрузок worker-узлов при недоступности control plane или высокой нагрузке на API-сервер.

- Исправлена проблема, при которой `cluster-autoscaler` впадал в тупиковое состояние
  после неудачного создания машины в облаке при ошибках создания машин, отличных от `ResourceExhausted`.
  Теперь кластер автоматически восстанавливает способность к масштабированию при временных сбоях облачного провайдера.

### Подсистема Deckhouse

- Добавлена возможность развертывания кластера DP с использованием [режима `Proxy`](https://deckhouse.ru/modules/deckhouse/v1.76/configuration.html#parameters-registry-proxy) для доступа к хранилищу образов.

- `Kubectl` на узлах кластера заменен на алиас `d8 k` ([утилита Deckhouse CLI](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.76/cli/d8/)).

- Изменения в `dhctl` (CLI-установщик):

  - Добавлена возможность запускать `dhctl` как отдельный бинарный файл
    без предварительной загрузки полного образа со всеми зависимостями.
    Необходимые компоненты загружаются из хранилища образов по ходу выполнения операции,
    сокращая время загрузки и инициализации.

  - Добавлена валидация публичных SSH-ключей.
    Некорректные ключи теперь выявляются до этапа, на котором они могут привести к ошибкам подключения к созданным узлам.

  - Исправлена обработка TLS-сертификатов для хранилищ образов.
    Теперь вместе с пользовательским центром сертификации (CA) используется системный пул сертификатов.

  - Добавлена проверка InstanceClass перед установкой на соответствие выбранному облачному провайдеру.
    Это позволяет заранее обнаруживать некорректную конфигурацию
    и останавливать установку до создания объектов в облачной инфраструктуре.

  - Исправлена настройка неймспейсов во время бутстрапа.
    Теперь `dhctl` обновляет настройки даже если неймспейс уже существует.

- Повышена отказоустойчивость мультимастерного режима.
  Версии Kubernetes теперь корректно определяются при частичной недоступности master-узлов,
  а kube-apiserver больше не направляет запросы на etcd-узлы в статусе `learner`.

### Подсистема Kubernetes & Scheduling

- [В ConfigMap `d8-cluster-kubernetes`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.76/admin/configuration/platform-scaling/control-plane/updating-and-versioning.html#мониторинг-процесса-обновления-kubernetes) добавлена информация о поддерживаемых версиях Kubernetes (`supportedVersions`),
  доступных для обновления и понижения версиях (`availableVersions`),
  а также о текущей автоматически обновляемой версии (`automaticVersion`).

- Изменения [в модуле `descheduler`](https://deckhouse.ru/modules/descheduler/v1.76/):

  - Модуль обновлён до версии 0.35.1.
    Добавлена поддержка фильтрации неймспейсов с помощью [механизма label selector](https://deckhouse.ru/modules/descheduler/v1.76/cr.html#descheduler-v1alpha2-spec-namespacelabelselector) и дополнительные улучшения.
    Полный список доработок доступен [в истории изменений модуля](https://github.com/kubernetes-sigs/descheduler/releases/tag/v0.35.1).

  - В настройки модуля добавлен [параметр `deschedulingInterval`](https://deckhouse.ru/modules/descheduler/v1.76/configuration.html#parameters-deschedulinginterval).
    Он позволяет задать частоту запуска цикла перераспределения подов: `Frequent`, `Moderate` или `Rare`.

  - Добавлена [стратегия `RemovePodsHavingTooManyRestarts`](https://deckhouse.ru/modules/descheduler/v1.76/cr.html#descheduler-v1alpha2-spec-strategies-removepodshavingtoomanyrestarts).
    Она вытесняет поды, у которых суммарное число перезапусков контейнеров превышает заданный порог,
    освобождая ресурсы узлов от подов в состоянии `CrashLoopBackOff`.

  - Модуль теперь [автоматически использует Kubernetes Metrics API](https://deckhouse.ru/modules/descheduler/v1.76/#поставщик-метрик), если в кластере доступна API-группа `metrics.k8s.io`.
    Это позволяет стратегиям учитывать реальные данные потребления ресурсов.

- [Модуль `vertical-pod-autoscaler`](https://deckhouse.ru/modules/vertical-pod-autoscaler/v1.76/) обновлён до версии 1.6.0. Ключевые изменения:

  - [Режим `InPlaceOrRecreate`](https://deckhouse.ru/modules/vertical-pod-autoscaler/v1.76/cr.html#verticalpodautoscaler-v1-spec-updatepolicy-updatemode) теперь используется по умолчанию. В этом режиме VPA пытается изменить `requests` и `limits` пода без его перезагрузки (in-place-обновления).

  - Уменьшена вероятность перезапуска пода при невозможности in-place-обновления (флаг `--in-place-skip-disruption-budget` контроллера), а также, при in-place-обновлениях не учитывается [значение `minReplicas`](https://deckhouse.ru/modules/vertical-pod-autoscaler/v1.76/cr.html#verticalpodautoscaler-v1-spec-updatepolicy-minreplicas).

- Рекомендации VPA по памяти теперь округляются до 64 MiB. Это делает значения более читаемыми и снижает число перезапусков подов, вызванных небольшими колебаниями рекомендаций.

- Для Kubernetes 1.34 и выше по умолчанию включён [feature gate `DRAExtendedResource`](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/#DRAExtendedResource), необходимый для работы модулей.

- Добавлен [алерт `D8EtcdHighRequestsLatency`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.76/reference/alerts.html#control-plane-manager-d8etcdhighrequestslatency), который сообщает о задержке запросов к etcd. Подобные задержки могут приводить к таймаутам API-сервера. Также добавлена соответствующая метрика для дашборда «Control Plane Status».

### Подсистема IAM

- Изменения [в модуле `user-authn`](https://deckhouse.ru/modules/user-authn/v1.76/):

  - Отключён небезопасный [OAuth 2.0 Implicit Flow](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-security-topics#name-implicit-grant).
    В этом потоке access-токен передаётся в URL-фрагменте, что повышает риск его утечки
    (например, через заголовок `Referer`, историю браузера или логи).
    Интеграции, использующие Authorization Code Flow, не затрагиваются.

  - Добавлена поддержка стандарта аутентификации SAML 2.0 [через ресурс DexProvider](https://deckhouse.ru/modules/user-authn/v1.76/cr.html#dexprovider-v1-spec-saml).
    Кластер DP теперь можно подключать к таким провайдерам, как AD FS, Okta, Keycloak, OneLogin и Shibboleth.

  - Для SAML-провайдера реализована поддержка refresh-токенов на стороне Dex.
    Это позволяет DexAuthenticator и kubeconfig-generator обновлять токены без повторного входа пользователя вручную.

  - Добавлена поддержка механизма Single Logout (SLO) для SAML.
    Провайдер может завершить сессию пользователя через `/saml/slo/{connector}`.

  - Исправлена проблема с устаревшим кешем на странице входа.
    В URL-адрес CSS-файла добавлен параметр `cache` с идентификатором сборки Dex.

- При [включенном режиме мультитенантности](https://deckhouse.ru/modules/user-authz/v1.76/configuration.html#parameters-enablemultitenancy) пользователи без прав доступа, настроенных через ClusterAuthorizationRule,
  больше не видят все неймспейсы кластера в веб-интерфейсе Deckhouse.
  Теперь применяется поведение deny-by-default: без явно заданных правил пользователь получает пустой список неймспейсов.
  Ограничение не касается привилегированных групп (`system:masters`, `kubeadm:cluster-admins`).
  В документацию модуля `user-authz` добавлен [подраздел с примером конфигурации](https://deckhouse.ru/modules/user-authz/v1.76/usage.html#пример-выдачи-прав-на-все-неймспейсы) для выдачи прав на все неймспейсы.

- В изолированных проектах добавлена поддержка [модуля `service-with-health-checks`](https://deckhouse.ru/modules/service-with-healthchecks/v1.76/).
  В шаблон проекта добавлено исключение NetworkPolicy для неймспейса `d8-service-with-healthchecks`,
  что позволяет агентам модуля выполнять проверки доступности сервисов и корректно работать в условиях сетевой изоляции.

### Подсистема Security

- Gatekeeper обновлён до версии 3.22.0, Ratify — до версии 1.4.0.
  Унифицировано поведение CEL- и Rego-политик, добавлен доступ к неймспейс-контексту в политиках,
  а также CLI-инструмент для бенчмаркинга политик.

- [Настройка `denyVulnerableImages`](https://deckhouse.ru/modules/operator-trivy/configuration.html#parameters-denyvulnerableimages) перенесена из модуля `admission-policy-engine` в `operator-trivy`.
  Теперь `operator-trivy` является единым владельцем логики запрета запуска уязвимых образов.

- [Модуль `cert-manager`](https://deckhouse.ru/modules/cert-manager/v1.76/) обновлён до версии 1.20.0.
  В новой версии добавлена поддержка Azure DNS Private Zones в DNS01 challenge
  и новая аннотация `acme.cert-manager.io/http01-ingress-ingressclassname` для переопределения IngressClass в HTTP-01 solvers,
  а также учтены исправления безопасности.

- [В модуле `multitenancy-manager`](https://deckhouse.ru/modules/multitenancy-manager/v1.76/) обновлены уязвимые Go-зависимости `go-jose`, `golang.org/x/crypto`, `golang.org/x/net`, `grpc`, `protobuf` и ряд других.

### Подсистема Cluster & Infrastructure

- Изменения [в модуле `node-manager`](https://deckhouse.ru/modules/node-manager/v1.76/):

  - Исправлена генерация событий draining hook. Исправление улучшает наблюдаемость процесса удаления узлов.

  - Исправлено состояние StaticInstance, из-за которого CAPI мог бесконечно ожидать удаления инфраструктуры.
    Исправление предотвращает бесконечные циклы очистки и повторной постановки в очередь
    и позволяет продолжить восстановление или пересоздание машины в обычном режиме.

  - Apiserver-proxy переведён с NGINX на нативную Go-реализацию.
    Это повышает стабильность распределения трафика и ускоряет переподключение при обрыве соединения.

- Изменения [в модуле `cloud-provider-dvp`](https://deckhouse.ru/modules/cloud-provider-dvp/v1.76/):

  - Добавлена поддержка гибридных кластеров.
    Статические master-узлы и облачные worker-узлы теперь можно использовать в одном кластере.

  - Повышена надёжность установки и выполнения `converge`.
    Переработан механизм ожидания готовности ресурсов и добавлен механизм fail-fast для части ошибок при создании инфраструктуры.

  - Для дочерних кластеров, создаваемых через Cluster API,
    добавлено автоматическое наследование StorageClass по умолчанию из родительского кластера.

  - Исправлена логика обработки жизненного цикла виртуальных машин.
    Это снижает вероятность возникновения проблем при удалении виртуальных машин и связанных объектов.

  - Добавлена поддержка [параметра `masterNodeGroup.instanceClass.additionalDisks`](https://deckhouse.ru/modules/cloud-provider-dvp/v1.76/cluster_configuration.html#dvpclusterconfiguration-masternodegroup-instanceclass-additionaldisks),
    позволяющего настраивать дополнительные диски для master-узлов.

- Изменения [в модуле `cloud-provider-aws`](https://deckhouse.ru/modules/cloud-provider-aws/v1.76/):

  - Логика освобождения узла от нагрузок при завершении Spot Instance перенесена в DP.
    Это делает обработку spot-узлов более единообразной.
  - Исправлена обработка регионов, где ещё не поддерживается API DescribeInstanceTopology.
    Это снижает число ложных ошибок IAM.

- Изменения [в модуле `cloud-provider-openstack`](https://deckhouse.ru/modules/cloud-provider-openstack/v1.76/):

  - Добавлена поддержка режима `ipMode: Proxy` для `LoadBalancer` в Kubernetes версии 1.32 и старше.

  - Добавлен [параметр `csiDriver.fsGroupPolicy`](https://deckhouse.ru/modules/cloud-provider-openstack/v1.76/configuration.html#parameters-csidriver-fsgrouppolicy) для CSI-драйвера, определяющий,
    поддерживает ли том изменение владельца и прав доступа перед монтированием.

- Изменения [в модуле `cloud-provider-vsphere`](https://deckhouse.ru/modules/cloud-provider-vsphere/v1.76/):

  - В модуле включён CSI snapshotter, который добавляет возможность создания снимков томов.

  - Исправлено обнаружение зон и datastore. Теперь учитываются только зоны, указанные в конфигурации провайдера.

- [В модуле `cloud-provider-azure`](https://deckhouse.ru/modules/cloud-provider-azure/v1.76/) добавлена поддержка NVMe-дисков для виртуальных машин Gen2.

- В ресурс GCPInstanceClass модуля `cloud-provider-gcp` добавлены параметры [`enableNestedVirtualization`](https://deckhouse.ru/modules/cloud-provider-gcp/v1.76/cr.html#gcpinstanceclass-v1-spec-enablenestedvirtualization) и [`additionalDisks`](https://deckhouse.ru/modules/cloud-provider-gcp/v1.76/cr.html#gcpinstanceclass-v1-spec-additionaldisks),
  позволяющие настраивать узлы для нагрузок, использующих виртуализацию, и сценарии хранения данных.

- [В модуле `cloud-provider-yandex`](https://deckhouse.ru/modules/cloud-provider-yandex/v1.76/) исправлена проблема,
  из-за которой удаление `externalIPAddresses` могло приводить к ошибке выполнения `converge`.

- Провайдеры DVP, Huawei Cloud и zVirt переведены на контракт Cluster API `v1beta2`.

- Для облачных провайдеров Huawei Cloud и VCD включены проверки политик безопасности
  и добавлены необходимые ресурсы SecurityPolicyException для инфраструктурных компонентов.

### Подсистема Network

- Добавлена поддержка обработки входящего трафика через Gateway API с помощью [модуля `alb`](https://deckhouse.ru/modules/alb/).

- Изменения в модуле [`cni-cilium`](https://deckhouse.ru/modules/cni-cilium/v1.76/):

  - Добавлена поддержка синхронизации conntrack-таблиц между узлами для обеспечения бесшовной живой миграции виртуальных машин.

  - Добавлена поддержка ответов на ICMP-запросы (`ping`) по ExternalIP для сервисов с `type: LoadBalancer` при использовании MetalLB.

- Реализован [механизм переключения между поддерживаемыми CNI-плагинами](guides/cni-migration.html) в кластере DP.

- В модуле `node-local-dns` добавлена возможность отключения резолвинга DNS-запросов по IPv6 с помощью [параметра `disableIPv6`](https://deckhouse.ru/modules/node-local-dns/v1.76/configuration.html#parameters-disableipv6).

Полный список изменений, включая перечень обновлённых компонентов,
доступен в [журнале изменений (changelog)](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.76.md) на GitHub.

## Версия 1.75

### Обратите внимание

- Добавлена поддержка Kubernetes 1.35 и прекращена поддержка Kubernetes 1.30.
  В будущих релизах DKP будет прекращена поддержка Kubernetes 1.31.
  Версия Kubernetes используемая по умолчанию изменена на 1.33 (параметр [`kubernetesVersion`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.75/reference/api/cr.html#clusterconfiguration-kubernetesversion) установлен в `Automatic`).

- Обновление до версии DKP 1.75 будет заблокировано,
  если в кластере используется одна из следующих устаревших версий компонентов (их поддержка прекращена):
  - IngressNginxController v1.9;
  - Istio v1.19.

- В DKP 1.75.0–1.75.3 сразу после установки кластера может срабатывать алерт `PodSecurityStandardsViolation`.
  Это происходит, если у неймспейса `d8-system` отсутствует лейбл `heritage=deckhouse`,
  из-за чего политики Pod Security Standards могут ошибочно применяться к системным подам DKP.
  Проблема исправлена в DKP 1.75.4.

  В кластерах, установленных на версиях DKP 1.75.0–1.75.3, добавьте необходимый лейбл вручную с помощью следующей команды:

  ```shell
  d8 k --as=system:sudouser label namespace d8-system heritage=deckhouse
  ```

- Изменения в поддержке версий Istio:
  - прекращена поддержка Istio 1.19;
  - версия Istio 1.21 признана устаревшей, и её поддержка будет прекращена в будущих версиях DKP.
    Чтобы обновить Istio, воспользуйтесь [инструкцией](https://deckhouse.ru/modules/istio/v1.75/examples.html#обновление-istio).

- Начиная с Kubernetes 1.35, на узлах кластера DKP должен использоваться механизм cgroup v2.
  Предыдущая версия (cgroup v1) [объявлена устаревшей](https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.35.md#no-really-you-must-read-this-before-you-upgrade-1).
  В случае, если на узле не поддерживается cgroup v2, появится алерт [D8NodeCgroupV2NotSupported](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.75/reference/alerts.html#node-manager-d8nodecgroupv2notsupported).
  Подробнее о переходе на cgroup v2 можно почитать в [документации Kubernetes](https://kubernetes.io/docs/concepts/architecture/cgroups/#migrating-cgroupv2).

- Начиная с Kubernetes 1.36, на узлах кластера DKP должен использоваться container runtime containerd v2.
  Поддержка containerd v1 [будет прекращена](https://kubernetes.io/blog/2025/08/27/kubernetes-v1-34-release/#kubernetes-to-end-containerd-1-x-support-in-v1-36).
  В случае, если на узле не поддерживается containerd v2, появится алерт [D8NodeContainerdV2NotSupported](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.75/reference/alerts.html#node-manager-d8nodecontainerdv2notsupported).

- Модуль [`vertical-pod-autoscaler`](https://deckhouse.ru/modules/vertical-pod-autoscaler/v1.75/) обновлен с версии 1.4.1 до [1.5.1](https://github.com/kubernetes/autoscaler/releases/tag/vertical-pod-autoscaler-1.5.1).
  Начиная с версии 1.5.0, режим обновления `updateMode: Auto` [объявлен устаревшим](https://github.com/kubernetes/autoscaler/pull/8426).
  Для пользовательских VPA рекомендуется заменить `updateMode: Auto` на [`updateMode: InPlaceOrRecreate`](https://deckhouse.ru/modules/vertical-pod-autoscaler/v1.75/cr.html#verticalpodautoscaler-v1-spec-updatepolicy-updatemode).
  Подробнее об этом режиме обновления можно почитать в [разделе документации про вертикальное масштабирование](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.75/architecture/kubernetes-and-scheduling/vpa.html#режимы-работы-vpa).

- При выпуске новых сертификатов с использованием модуля [`cert-manager`](https://deckhouse.ru/modules/cert-manager/v1.75/) также происходит перевыпуск приватного ключа сертификата. Это необходимо для предотвращения потенциальных рисков безопасности.

- Для подов, обрабатывающих HTTP-01 challenge через ресурсы Issuer и ClusterIssuer, заданы resource requests по умолчанию.
  Теперь при создании такие поды потребляют часть общей квоты ресурсов кластера и проекта.

- Модуль [`operator-trivy`](https://deckhouse.ru/modules/operator-trivy/) теперь загружается из внешнего источника и развивается в рамках отдельного релизного цикла.
  Никаких ручных действий для миграции не требуется.

### Подсистема Deckhouse

- В ресурс ModuleSource добавлен [параметр `scanInterval`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.75/reference/api/cr.html#modulesource-v1alpha1-spec-scaninterval), позволяющий управлять частотой проверки хранилища образов контейнеров на наличие новых модулей и их версий.

- Добавлен режим выделенного узла с лейблом `node.deckhouse.io/etcd-arbiter`, на котором запускается только etcd, без компонентов control plane.
  Это позволяет развернуть отказоустойчивый кластер по схеме 2 master-узла + выделенный etcd-узел
  со сниженными требованиями к ресурсам по сравнению с классической схемой из трёх полноценных master-узлов.
  Подробнее о настройке режима можно почитать в [разделе документации про управление режимом высокой доступности](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.75/admin/configuration/high-reliability-and-availability/enable.html#настройка-режима-ha-c-двумя-master-узлами-и-arbiter-узлом).

- Добавлены новые [режимы работы](https://deckhouse.ru/modules/registry/v1.75/#описание) модуля `registry`:
  **Proxy** (с кешированием образов) и **Local** (для изолированных сред).
  Новые режимы поддерживаются только для уже развёрнутых статичных кластеров и пока не доступны при развертывании кластера DKP.

- Параметры доступа к хранилищу образов теперь можно задавать через [настройки модуля `deckhouse`](https://deckhouse.ru/modules/deckhouse/v1.75/configuration.html#parameters-registry) при установке кластера DKP.
  Добавлена возможность развертывания кластера DKP с использованием режимов [Direct](https://deckhouse.ru/modules/deckhouse/v1.75/configuration.html#parameters-registry-direct) и [Unmanaged](https://deckhouse.ru/modules/deckhouse/v1.75/configuration.html#parameters-registry-unmanaged).
  Сохранён режим совместимости Legacy Unmanaged (через [InitConfiguration](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.75/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo)).
  При отсутствии явной конфигурации режим выбирается автоматически.

- Добавлена проверка используемой оболочки пользователя:
  если используется не bash, утилита `dhctl` теперь выводит понятное сообщение об ошибке.

- Добавлена проверка StaticInstance перед установкой DKP по соответствующим SSHCredentials,
  чтобы утилита `dhctl` не сообщала о готовности до фактического завершения развертывания кластера.

- Расширено тестовое покрытие проверок перед установкой (preflight checks) для повышения предсказуемости развертывания кластера.

- В bashible частично сокращено использование netcat, что снижает количество срабатываний со стороны систем безопасности.

- Исправлена проблема, из-за которой невозможно было продолжить установку кластера после прерывания процесса установки.

- Исправлена логика удаления объекта Node при выполнении `converge`.
  Теперь узел корректно удаляется из кластера после уничтожения соответствующей виртуальной машины.

- Исправлена работа флага `--skip-resources` при выполнении `dhctl destroy`.Теперь указанные ресурсы корректно пропускаются.

### Подсистема Kubernetes & Scheduling

- Новый компонент `update-observer` модуля [`control-plane-manager`](https://deckhouse.ru/modules/control-plane-manager/v1.75/) позволяет отслеживать в реальном времени
  статус обновления компонентов control plane (kube-apiserver, kube-scheduler, kube-controller-manager) и kubelet на узлах.
  Это позволяет оперативно выявлять расхождения и устраняет «слепые зоны» при обновлении Kubernetes.

- Для всех ресурсов VPA, управляемых DKP, режим обновления `updateMode: Auto` автоматически заменён на [`updateMode: InPlaceOrRecreate`](https://deckhouse.ru/modules/vertical-pod-autoscaler/v1.75/cr.html#verticalpodautoscaler-v1-spec-updatepolicy-updatemode).
  Для кластеров с Kubernetes версии 1.32 и ниже автоматически включается [экспериментальная функция (feature gate)](https://deckhouse.ru/modules/control-plane-manager/v1.75/#feature-gates) `InPlacePodVerticalScaling`
  в компонентах control plane (kube-apiserver, kube-scheduler, kube-controller-manager) и kubelet.
  Это позволяет изменять ресурсы подов без их пересоздания.

- Модуль [`descheduler`](https://deckhouse.ru/modules/descheduler/v1.75/) обновлен до версии 0.34. Основные изменения:
  - приоритетное выселение подов — теперь модуль в первую очередь выселяет поды с большим количеством перезапусков,
    что повышает стабильность кластера при перераспределении нагрузки;
  - поддержка DRA — модуль теперь корректно учитывает и работает с ресурсами,
    управляемыми через механизм динамического распределения (Dynamic Resource Allocation);
  - из стратегий [`lowNodeUtilization`](https://deckhouse.ru/modules/descheduler/v1.75/cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization) и [`highNodeUtilization`](https://deckhouse.ru/modules/descheduler/v1.75/cr.html#descheduler-v1alpha2-spec-strategies-highnodeutilization) удалены пороговые значения по умолчанию для `cpu`, `memory` и `pods`.
    Поведение модуля приведено в соответствие с upstream-реализацией.

### Подсистема IAM

- Изменено поведение при получении списка неймспейсов: при запросе всех неймспейсов кластера пользователь увидит только те,
  к которым у него есть доступ, даже если у него нет прав на просмотр полного списка.
  Изменение направлено на корректную работу мультитенантности и пользовательских интерфейсов кластера.

- В ресурс ProjectTemplate модуля `multitenancy-manager` добавлены опции `unmanaged` и `skip-heritage` (задаются аннотациями на ресурсах),
  позволяющие администраторам управлять степенью контроля DKP над ресурсами:
  - при включении [`unmanaged`](https://deckhouse.ru/modules/multitenancy-manager/v1.75/usage.html#исключение-ресурсов-из-управления-multitenancy-manager) модуль `multitenancy-manager` не управляет жизненным циклом объектов;
  - при включении [`skip-heritage`](https://deckhouse.ru/modules/multitenancy-manager/v1.75/usage.html#пропуск-создания-лейбла-heritage-multitenancy-manager) управление сохраняется, но допускается изменение ресурсов сторонними контроллерами.

- Добавлена возможность выполнять запросы к Kubernetes API, аутентифицируясь через LDAP
  с использованием Basic-аутентификации вместо OIDC-токенов (параметр [`ldap.enableBasicAuth`](https://deckhouse.ru/modules/user-authn/v1.75/cr.html#dexprovider-v1-spec-ldap-enablebasicauth) в ресурсе DexProvider).

- Авторизационный вебхук модуля [`user-authz`](https://deckhouse.ru/modules/user-authz/v1.75/) теперь работает по принципу fail-closed: при его недоступности авторизация запрещена.
  Для повышения надежности взаимодействие вебхука с apiserver осуществляется через сеть узлов кластера.

- В модуле `user-authn` добавлено глобальное ограничение времени жизни пользовательской сессии (параметр [`refreshTokenAbsoluteLifetime`](https://deckhouse.ru/modules/user-authn/v1.75/configuration.html#parameters-refreshtokenabsolutelifetime)).
  По истечении указанного срока требуется повторная аутентификация.

- В модуле `user-authn` добавлена поддержка аутентификации в DKP с использованием [Kerberos (SPNEGO)](https://deckhouse.ru/modules/user-authn/v1.75/cr.html#dexprovider-v1-spec-ldap-kerberos).

- Обновлён список TLS-шифров kube-apiserver. Из флага `--tls-cipher-suites` удалены небезопасные шифры TLS_RSA_WITH_AES_256_GCM_SHA384 и TLS_RSA_WITH_AES_128_GCM_SHA256 (считаются небезопасными из-за обмена ключами на основе RSA).
  Добавлены шифры TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256 и TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256.

- Изменён механизм ограничения анонимного доступа к API-серверу.
  Вместо использования флага `--anonymous-auth=false` и сайдкар-прокси, теперь применяется ресурс AuthenticationConfiguration,
  разрешающий анонимные запросы только к `/livez`, `/readyz` и `/healthz`. Это позволяет эффективнее использовать ресурсы кластера.
  При использовании публичного доступа к API-серверу через параметр [`publishAPI.enabled`](https://deckhouse.ru/modules/user-authn/v1.75/configuration.html#parameters-publishapi-enabled) анонимный доступ к probe-запросам по-прежнему ограничен.

### Подсистема Security

- Исправлена сериализация пустых массивов и объектов в ресурсах [SecurityPolicy](https://deckhouse.ru/modules/admission-policy-engine/v1.75/cr.html#securitypolicy) и [OperationPolicy](https://deckhouse.ru/modules/admission-policy-engine/v1.75/cr.html#operationpolicy) модуля `admission-policy-engine`.
  Ранее часть настроек в этих ресурсах могла применяться некорректно.

- Пересмотрена реализация поддержки расширения ModSecurity, в результате чего функциональность [OWASP Core Rule Set (CRS) для ModSecurity](https://owasp.org/www-project-modsecurity-core-rule-set/) теперь доступна во всех поддерживаемых версиях IngressNginxController.

### Подсистема Cluster & Infrastructure

- Изменена [стадия жизненного цикла](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.75/architecture/module-development/versioning/#жизненный-цикл-модуля) следующих модулей интеграции с облачными провайдерами:
  - модули [`cloud-provider-dvp`](https://deckhouse.ru/modules/cloud-provider-dvp/) (DVP), [`cloud-provider-vcd`](https://deckhouse.ru/modules/cloud-provider-vcd/) (VMware Cloud Director), [`cloud-provider-huaweicloud`](https://deckhouse.ru/modules/cloud-provider-huaweicloud/) (HuaweiCloud) и [`cloud-provider-zvirt`](https://deckhouse.ru/modules/cloud-provider-zvirt/) (zVirt) теперь в статусе General Availability (был — Experimental);
  - модуль [`cloud-provider-dynamix`](https://deckhouse.ru/modules/cloud-provider-dynamix/) (провайдер Basis Dynamix) теперь в статусе Preview (был — Experimental).

- Добавлена поддержка использования swap-памяти для подов.
  Вместо немедленного OOM-kill под может временно использовать swap (при наличии выделенных swap-ресурсов на узле)
  и продолжить работу после пикового потребления ресурсов. Требуется поддержка cgroup v2 и достаточные ресурсы на узлах.
  Подробнее про настройку использования swap-памяти можно почитать в описании параметра [`kubelet.memorySwap`](https://deckhouse.ru/modules/node-manager/v1.75/cr.html#nodegroup-v1-spec-kubelet-memoryswap) ресурса NodeGroup.

- Для облачного провайдера DVP:
  - Добавлен параметр [`networkPolicy`](https://deckhouse.ru/modules/cloud-provider-dvp/v1.75/cluster_configuration.html#dvpclusterconfiguration-provider-networkpolicy), позволяющий задать правила управления сетевым трафиком к нагрузкам и от нагрузок, запущенных в рамках ресурса Project.
  - Добавлены метки `managed-by`, `cluster-uuid` и `vm_name` для всех инфраструктурных объектов.
  - Улучшена диагностика ошибок в CSI-драйвере. CSI-контроллер теперь отображает диагностические сообщения непосредственно в статусе PersistentVolumeClaim.
  - Реализован механизм автоматической очистки объектов VMBDA при удалении виртуальной машины.
  - Исправлена проблема, при которой объекты DeckhouseMachine могли оставаться в состоянии удаления
    после физического удаления соответствующих виртуальных машин.
  - Устранена проблема, из-за которой Cloud Controller Manager (CCM) мог инициировать повторное создание
    внешних балансировщиков нагрузки после удаления связанного сервиса Kubernetes.

- В облачном провайдере Yandex Cloud исправлена ошибка отображения,
  при которой во время установки выводились ложные сообщения о проблемах с NAT в конфигурациях, где NAT фактически не используется.

- В модуле [`node-manager`](https://deckhouse.ru/modules/node-manager/v1.75/):
  - Cluster API (CAPI) обновлен с версии 1.10.6 до 1.11.3. Улучшена стабильность работы и совместимость с новыми провайдерами.
  - Реализован отдельный контроллер для NodeGroup-хуков, вынесенный из DKP.
    Это снижает нагрузку на master-узлы и повышает отзывчивость и информативность интерфейса.
  - Добавлена поддержка покарточной настройки MIG: теперь для каждой физической GPU-карты на узле можно [задавать свой набор MIG-партиций](https://deckhouse.ru/modules/node-manager/v1.75/cr.html#nodegroup-v1-spec-gpu-mig-partedconfig).
    Ранее MIG-конфигурация применялась на уровне всего узла и была одинаковой для всех GPU.
    Это обеспечивает более гибкое распределение ресурсов под разные типы нагрузок на одном узле.

### Подсистема Network

- В IngressNginxController для инлетов [HostPort](https://deckhouse.ru/modules/ingress-nginx/v1.75/cr.html#ingressnginxcontroller-v1-spec-hostport-acceptclientipheadersfrom), [HostPortWithProxyProtocol](https://deckhouse.ru/modules/ingress-nginx/v1.75/cr.html#ingressnginxcontroller-v1-spec-hostportwithproxyprotocol-acceptclientipheadersfrom), [LoadBalancer](https://deckhouse.ru/modules/ingress-nginx/v1.75/cr.html#ingressnginxcontroller-v1-spec-loadbalancer-acceptclientipheadersfrom) и [LoadBalancerWithProxyProtocol](https://deckhouse.ru/modules/ingress-nginx/v1.75/cr.html#ingressnginxcontroller-v1-spec-loadbalancerwithproxyprotocol-acceptclientipheadersfrom) добавлен параметр `acceptClientIPHeadersFrom`.
  Параметр определяет список подсетей (CIDR), для которых HTTP-заголовки `X-Forwarded-*` считаются доверенными и не перезаписываются при проксировании.

- В модуле `cni-cilium` расширены возможности наблюдаемости сетевого трафика и событий в кластере:
  - добавлены расширенные метрики для мониторинга взаимодействия между заданными объектами (Pods, Services, Namespaces).
    Метрики настраиваются через параметр [`extendedMetrics`](https://deckhouse.ru/modules/cni-cilium/v1.75/cr.html#hubblemonitoringconfig-v1alpha1-spec-extendedmetrics) ресурса HubbleMonitoringConfig;
  - реализован экспорт flow-логов трафика для анализа сетевых взаимодействий между заданными объектами.
    Экспорт настраивается через параметр [`flowLogs`](https://deckhouse.ru/modules/cni-cilium/v1.75/cr.html#hubblemonitoringconfig-v1alpha1-spec-flowlogs) ресурса HubbleMonitoringConfig.

- Исправлена проблема, при которой после миграции виртуальной машины Deckhouse Virtualization Platform (DVP)
  IP-адрес LoadBalancer мог переставать анонсироваться при использовании MetalLB в режиме BGP.

- Изменения в поддержке версий [IngressNginxController](https://deckhouse.ru/modules/ingress-nginx/v1.75/):
  - прекращена поддержка IngressNginxController v1.9;
  - добавлена поддержка IngressNginxController v1.14.

Полный список изменений, включая перечень обновлённых компонентов,
доступен в [журнале изменений (changelog)](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.75.md) на GitHub.

## Версия 1.74

### Обратите внимание

- Добавлена поддержка Kubernetes 1.34 и прекращена поддержка Kubernetes 1.29.
  В будущих релизах DKP будет прекращена поддержка Kubernetes 1.30.
  Версия Kubernetes, используемая по умолчанию ([параметр `kubernetesVersion`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.74/reference/api/cr.html#clusterconfiguration-kubernetesversion) установлен в `Automatic`), изменена на [1.32](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.74/reference/supported_versions.html#kubernetes).

### Основные изменения

- Добавлена поддержка управления отдельными экспериментальными и предварительными возможностями Kubernetes ([feature gates](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/))
  с помощью [параметра `enabledFeatureGates`](https://deckhouse.ru/modules/control-plane-manager/v1.74/configuration.html#parameters-enabledfeaturegates) модуля `control-plane-manager`.
  Подробнее об управлении этими возможностями в DKP можно почитать в [документации модуля](https://deckhouse.ru/modules/control-plane-manager/v1.74/#feature-gates).

- Параметр `--terminated-pod-gc-threshold` (порог удаления выселенных подов)
  теперь рассчитывается автоматически в зависимости от количества узлов в кластере.
  Это обеспечивает баланс между стабильностью кластера и возможностями для диагностики.
  Пороговые значения приведены в [документации модуля `control-plane-manager`](https://deckhouse.ru/modules/control-plane-manager/v1.74/#динамическое-пороговое-значение-удаления-выселенных-подов).

- Максимальное число подов ([`maxPods`](https://deckhouse.ru/modules/node-manager/v1.74/cr.html#nodegroup-v1-spec-kubelet-maxpods))
  для группы узлов теперь устанавливается автоматически на основе [параметра `podSubnetNodeCIDRPrefix`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.74/reference/api/cr.html#clusterconfiguration-podsubnetnodecidrprefix)
  ресурса ClusterConfiguration (если параметр не задан явно).
  Это позволяет поддерживать стабильную работу кластера.

- В ресурс HuaweiCloudInstanceClass облачного провайдера Huawei Cloud добавлен [параметр `vipAddress`](https://deckhouse.ru/modules/cloud-provider-huaweicloud/v1.74/cr.html#huaweicloudinstanceclass-v1-spec-vipaddress),
  позволяющий задавать виртуальный IP-адрес для всех узлов инстанс-класса.

- В настройках провайдера Huawei Cloud добавлена возможность переопределять главную сеть для основного сетевого интерфейса ([параметр `mainNetwork`](https://deckhouse.ru/modules/cloud-provider-huaweicloud/v1.74/cluster_configuration.html#huaweicloudclusterconfiguration-nodegroups-instanceclass-mainnetwork)) и указывать подсети для дополнительных сетевых интерфейсов ([параметр `additionalNetworks`](https://deckhouse.ru/modules/cloud-provider-huaweicloud/v1.74/cluster_configuration.html#huaweicloudclusterconfiguration-nodegroups-instanceclass-additionalnetworks)) в CloudPermanent- и CloudEphemeral-узлах.

  **Важно.** При обновлении до DKP 1.74 на Huawei Cloud возможно массовое пересоздание CloudEphemeral-узлов. Из-за временной нехватки узлов часть подов может не запуститься.

- Сборка архива с данными для отладки теперь выполняется с помощью [утилиты `d8`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.74/cli/d8/).
  При необходимости собрать данные воспользуйтесь [инструкцией](https://deckhouse.ru/modules/deckhouse/v1.74/faq.html#как-собрать-информацию-для-отладки).

- Теперь при ошибках выполнения хуков сохраняются все собранные ими метрики.
  Это позволяет не терять отладочную информацию и упрощает последующий анализ причин возникновения ошибки.

- Исправлено зависание очереди Deckhouse в гибридных кластерах, возникавшее при неполном наборе параметров в конфигурации модуля.

### Безопасность

- Внедрён контроль целостности модулей на базе файловой системы EROFS
  для обеспечения безопасности и предотвращения несанкционированных изменений.
  Для работы новой системы необходима поддержка dm-verity и EROFS в ядре.

### Сеть

- Добавлена поддержка протокола SCTP (Stream Control Transmission Protocol).
  Теперь в кластере Deckhouse можно запускать приложения, использующие этот тип соединения.

- В [модуль `ingress-nginx`](https://deckhouse.ru/modules/ingress-nginx/) добавлен вспомогательный компонент `geoproxy`.
  Он нацелен на повышение стабильности Ingress-контроллера при работе с GeoIP-базами и предоставляет следующие возможности:
  - экономия лицензий MaxMind (загрузка баз происходит из одной точки, раз в день);
  - постоянное хранение данных (перезагрузка компонентов не приводит к обращениям на серверы MaxMind);
  - возможность указать своё зеркало для загрузки баз.

- Добавлена метрика Prometheus `bpf_progs_complexity_max_verified_insts` для оценки количества инструкций eBPF-подпрограмм,
  загружаемых в ядро системы на узлах кластера.
  Новая метрика позволяет оценить совместимость сетевой подсистемы с имеющимися ядрами.
  Для работы метрики необходимо наличие ядра Linux версии 5.16 и выше.

- Добавлена новая метрика `geoip_version` и дашборды для оценки работоспособности GeoIP в кластере.

- Исправлены проблемы с одновременным обновлением [модуля `node-local-dns`](https://deckhouse.ru/modules/node-local-dns/)
  совместно с сетевой подсистемой.

Полный список изменений, включая перечень обновлённых компонентов,
доступен в [журнале изменений (changelog)](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.74.md) на GitHub.

## Версия 1.73

### Обратите внимание

- Релиз содержит ряд важных изменений, повышающих безопасность. Устранен ряд известных уязвимостей. В частности, устранена уязвимость в модуле `user-authn` (CVE-2025-22868), потенциально позволявшая обходить проверку аутентификации. Рекомендуется запланировать обновление. Подробнее в разделе [Безопасность](#безопасность).

- Модуль `dashboard` будет удален в будущих версиях DKP. Используйте [веб-интерфейс Deckhouse](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.73/user/web/ui.html) (необходим включенный модуль [`console`](https://deckhouse.ru/modules/console/)).

- Модуль `runtime-audit-engine` теперь загружается из  внешнего источника (ModuleSource `deckhouse`).

- В процессе обновления будут перезапущены все компоненты DKP.

### Основные изменения

- Добавлена возможность смены настроек registry (параметр [`mode`](https://deckhouse.ru/modules/deckhouse/v1.73/configuration.html#parameters-registry-mode)) в режиме `Unmanaged` аналогично режиму `Direct`. Также, добавлена настройка режима проверки registry при смене настроек (параметр [`checkMode`](https://deckhouse.ru/modules/deckhouse/v1.73/configuration.html#parameters-registry-direct-checkmode)):
  - `Default` — проверяется наличие всех образов для системных модулей и образы deckhouse-controller.
  - `Relax` — проверяется наличие только образов deckhouse-controller.

- Изменение требований к полю `email` объекта [User](https://deckhouse.ru/modules/user-authn/v1.73/cr.html#user). Адрес `email` должен быть в нижнем регистре. Для уже существующих пользователей сохранена совместимость.

- Обновление Dex до **v2.44.0** теперь позволяет выполнять аутентификацию через подключенные провайдеры, если один из них не работает. Также, аутентификация через провайдера аутентификации теперь может работать через прокси.

- В статусе объекта [User](https://deckhouse.ru/modules/user-authn/v1.73/cr.html#user) теперь выводится информация о причине блокировки пользователя (управляется параметром [`lockout`](https://deckhouse.ru/modules/user-authn/v1.73/configuration.html#parameters-passwordpolicy-lockout)).

- В провайдере для интеграции с Deckhouse Virtualization Platform добавлен параметр [`additionalDisks`](https://deckhouse.ru/modules/cloud-provider-dvp/v1.73/cluster_configuration.html#dvpclusterconfiguration-masternodegroup-instanceclass-additionaldisks) — позволяет создавать и подключать к VM дополнительные диски в NodeGroup (задаются `size`, StorageClass). Упрощает разнесение данных по нескольким дискам.

- В провайдере для интеграции с VMware Cloud Director добавлена поддержка [`additionalMetadata`](https://deckhouse.ru/modules/cloud-provider-vcd/v1.73/cr.html#vcdinstanceclass-v1-spec-additionalmetadata) для объектов (сети, ВМ, диски). Метаданные объединяются с существующими, значения из `additionalMetadata` имеют приоритет.  Изменение `additionalMetadata` приводит к пересозданию групп узлов с типом CloudEphemeral, использующих соответствующий VCDInstanceClass.

- В провайдере для интеграции с VMware vSphere добавлена возможность указания ID политики хранения SPBM (параметр [`storagePolicyID`](https://deckhouse.ru/modules/cloud-provider-vsphere/v1.73/cluster_configuration.html#vsphereclusterconfiguration-storagepolicyid)) и автоматическое создание StorageClass для каждой доступной политики хранения SPBM. Теперь можно явно выбирать политику для master- и worker-узлов, и получать соответствующие классы хранения.

- Добавлены алерты, помогающие запланировать отключение модуля или миграцию:
  - [`ModuleIsDeprecated`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.73//reference/alerts.html#monitoring-deckhouse-moduleisdeprecated) — сообщает о наличии устаревшего модуля, поддержка которого скоро прекратится.
  - [`D8ModuleOutdatedByMajorVersion`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.73//reference/alerts.html#monitoring-deckhouse-d8moduleoutdatedbymajorversion) — сообщает о том, что модуль отстаёт по мажорным версиям.

- Добавлен алерт [`GeoIPDownloadErrorDetected`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.73/reference/alerts.html#ingress-nginx-geoipdownloaderrordetected), сообщающий об ошибках загрузки GeoIP-баз MaxMind.

- Механика [оповещений об обновлениях](https://deckhouse.ru/modules/deckhouse/v1.73/usage.html#оповещение-об-обновлении-deckhouse) изменена — релиз применяется только при успешной доставке уведомления на настроенный вебхук. При ошибке доставки применение обновления приостанавливается до восстановления вебхука.

- Изменена структура внутрикластерной документации. Документация всех модулей (включая подключенные) теперь находится [в разделе «Модули»](https://deckhouse.ru/modules/). Обновлен поиск.

- Для Ingress-контроллера версии 1.10 появилась возможность включать профилировщик (параметр [`nginxProfilingEnabled`](https://deckhouse.ru/modules/ingress-nginx/v1.73/cr.html#ingressnginxcontroller-v1-spec-nginxprofilingenabled)). Включение профилировщика увеличивает потребление ресурсов, но может потребоваться при поиске причин ошибок в работе контроллера.

- При отправке метрик мониторинга SLA по протоколу Prometheus Remote Write теперь можно установить HTTP-заголовки аутентификации (параметр [`headers`](https://deckhouse.ru/modules/upmeter/v1.73/cr.html#upmeterremotewrite-v1-spec-config-headers) ресурса UpmeterRemoteWrite).

- Оптимизирована загрузка модулей ядром DKP, снижено время запуска при большом числе модулей.

- В аудит-логах теперь видно, какой OIDC-провайдер выдал токен аутентификации.

- Обновлена версия Deckhouse CLI (`d8`) до v0.23.0:
  - Добавлена команда [`d8 status`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.73/cli/d8/reference/#d8-status). Быстрый сводный отчёт о состоянии кластера (узлы, релизы, поды Deckhouse, алерты, registry, настройки Deckhouse, CNI, состояние очереди).
  - Добавлена команда [`d8 k debug`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.73/cli/d8/reference/#d8-k-debug). Запускает встроенный в DKP debug-контейнер — образ можно переопределить через `--image`. Упрощает интерактивную отладку подов.
  - Для команды [`d8 system queue list`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.73/cli/d8/reference/#d8-system-queue-list) добавлен режим наблюдения за состоянием очереди (ключ `--watch`).

### Безопасность

- Повышена базовая безопасность контейнеров. У следующих модулей обновлены профили безопасности для использования только необходимых привилегий и прав доступа: `cni-cilium`, `cni-flannel`, `cni-simple-bridge`, `ingress-nginx`, `istio`, `keepalived`, `kube-dns`, `kube-proxy`, `node-local-dns`, `network-gateway`, `network-policy-engine`, `open-vpn`.

- Добавлен флаг [`allowRbacWildcards`](https://deckhouse.ru/modules/admission-policy-engine/v1.73/cr.html#securitypolicy-v1alpha1-spec-policies-allowrbacwildcards) политики безопасности (SecurityPolicy) позволяющий управлять возможностью использования wildcard при описании объектов Role и RoleBinding (по умолчанию `true`). Также, политиками безопасности теперь можно ограничивать интерактивные подключения к подам (`CONNECT` для `pods/exec` и `pods/attach`) в пространствах имён.

- Добавлена возможность управления запретом создания подов с tolerations из указанного списка (параметр [`policies.disallowedTolerations`](https://deckhouse.ru/modules/admission-policy-engine/v1.73/cr.html#operationpolicy-v1alpha1-spec-policies-disallowedtolerations) операционной политики). Это помогает предотвратить попадание пользовательской нагрузки на узлы, отведенные под выделенные задачи.

- Ingress-контроллер обновлен до версии 1.12 — добавлены изменения, повышающие безопасность (distroless-образ, устранение уязвимостей и др.).

- Исправлены известные уязвимости в модулях: `operator-trivy`, `registry`, `user-authn`, `cloud-provider-dvp`, `multitenancy-manager`, `admission-policy-engine`, `ingress-nginx`, `alertmanager`, `metallb`, `istio`, `node-local-dns`, `kube-apiserver`.

### Сеть

- В провайдере для интеграции с VMware Cloud Director добавлена поддержка [LoadBalancer](https://deckhouse.ru/modules/cloud-provider-vcd/v1.73/environment.html#использование-балансировщика-нагрузки) на базе VMware NSX Advanced Load Balancer (ALB/Avi) при использовании `NSX-T`. Для работы нужна активированная функция Load Balancer на Edge Gateway. Если LoadBalancer включен после создания кластера, DKP автоматически учтет изменения в течение часа. Для открытых портов создаются пары `Pool + Virtual Service`; при наличии межсетевого экрана необходимо добавить разрешающие правила для внешнего IP-адреса балансировщика и соответствующих портов.

Полный список изменений, включая перечень обновлённых компонентов,
доступен в [журнале изменений (changelog)](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.73.md) на GitHub.

## Версия 1.72

### Обратите внимание

- В процессе обновления будут перезапущены все компоненты DKP.
- Теперь, чтобы использовать [экспериментальные модули](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.72/architecture/module-development/versioning/#жизненный-цикл-модуля) в кластере, необходимо явно дать разрешение (параметр [allowExperimentalModules](https://deckhouse.ru/modules/deckhouse/configuration.html#parameters-allowexperimentalmodules)). По умолчанию использование экспериментальных модулей отключено. Уже включенные перед обновлением DKP экспериментальные модули отключены не будут. В случае ручного отключения включенного при обновлении DKP экспериментального модуля, для возможности его повторного включения необходимо дать разрешение на использование экспериментальных модулей.
- Если на узлах кластера присутствуют WireGuard-интерфейсы, необходимо обновить ядро Linux до версии не ниже 6.8.

### Основные изменения

- Добавлен новый [модуль registry](https://deckhouse.ru/modules/registry/) и возможность менять параметры работы с container registry без перезапуска всех компонентов DKP. Теперь доступно [два режима работы](https://deckhouse.ru/modules/deckhouse/configuration.html#parameters-registry-mode) с container registry в DKP: `Unmanaged` (способ, доступный в предыдущих версиях) и `Direct` (новый способ). В режиме `Direct` DKP создает в кластере виртуальный адрес container registry, через который осуществляется работа всех компонентов DKP. Изменение адреса container registry (например, переключение на другой container registry или изменение редакции DKP в кластере) в этом случае не приводит к безусловному перезапуску всех компонентов DKP.

- Добавлена возможность использования рекурсивных DNS-серверов (управляется секцией параметров [recursiveSettings](https://deckhouse.ru/modules/cert-manager/configuration.html#parameters-recursivesettings) модуля `cert-manager`), которые применяются для проверки существования записи в DNS перед запуском процесса подтверждения владения доменом методом ACME DNS-01. Это актуально, если один и тот же домен используется как публично, так и внутри кластера, а также, если для домена существуют выделенные авторитетные DNS-серверы.

- Введено разделение модулей на критические и функциональные с помощью флага `critical` в `module.yaml`. Критические модули запускаются первыми, а функциональные — после завершения bootstrap-процесса, при этом их задачи выполняются параллельно и не блокируют очередь в случае сбоя. Это ускоряет установку кластера и повышает отказоустойчивость при запуске модулей.

- Добавлена возможность включения логирования всех DNS-запросов (параметр [enableLogs](https://deckhouse.ru/modules/node-local-dns/configuration.html#parameters-enablelogs) модуля `node-local-dns`).

- В модуле `cloud-provider-vcd` добавлена новая [схема размещения WithNAT](https://deckhouse.ru/modules/cloud-provider-vcd/layouts.html#withnat) для развертывания кластера. Она автоматически настраивает NAT и, при необходимости, правила брандмауэра для доступа к узлам через узел-бастион, а также поддерживает работу как с `NSX-T`, так и с `NSX-V`. Это позволяет развёртывать кластер в VMware Cloud Director без предварительной настройки окружения (в отличие от схемы размещения `Standard`).

### Безопасность

- К [событиям аудита](https://deckhouse.ru/modules/control-plane-manager/#аудит) (Kubernetes audit log) добавлены поля `user-authn.deckhouse.io/name` и `user-authn.deckhouse.io/preferred_username` для вывода информации о claim пользователя от OIDC-провайдера. Это улучшает контроль и отладку процессов аутентификации.

- Версии Kubernetes 1.30–1.33 обновлены до последних патч-версий.

- Для провайдера AWS добавлена возможность отключить создание групп безопасности по умолчанию (параметр [disableDefaultSecurityGroup](https://deckhouse.ru/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-disabledefaultsecuritygroup)). В этом случае группы безопасности необходимо создать вручную и указать явно в AWSClusterConfiguration, AWSInstanceClass и NodeGroup. Новая возможность расширяет контроль над настройками безопасности.

- Реализована поддержка политики паролей для локальных пользователей (управляется в секции параметров [passwordPolicy](https://deckhouse.ru/modules/user-authn/configuration.html#parameters-passwordpolicy)). Теперь можно настраивать минимальный уровень сложности пароля, срок его действия и обязательную ротацию, предотвращать повторное использование старых паролей, а также блокировать аккаунт после заданного числа неудачных попыток входа. Эти изменения позволяют администраторам кластера централизованно применять требования к паролям и повышают уровень безопасности.

Полный список изменений, включая перечень обновлённых компонентов,
доступен в [журнале изменений (changelog)](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.72.md) на GitHub.

## Версия 1.71

### Обратите внимание

- Prometheus заменён на Deckhouse Prom++. Если вы хотите продолжать использовать Prometheus, **до обновления платформы** явно выключите модуль `prompp` командой `d8 system module disable prompp`.

- Добавлена поддержка Kubernetes 1.33 и прекращена поддержка Kubernetes 1.28. В будущих релизах DKP поддержка Kubernetes 1.29 будет прекращена. Версия Kubernetes используемая по умолчанию (параметр [`kubernetesVersion`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/installing/configuration.html#clusterconfiguration-kubernetesversion) установлен в `Automatic`) изменена на [1.31](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/supported_versions.html#kubernetes).

- Обновление кластера до версии Kubernetes 1.31 требует последовательного обновления всех узлов с остановкой нагрузки (drain узла). Управлять настройками применения обновлений узла, требующих остановки нагрузки, можно с помощью секции параметров [`disruptions`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions).

- Вместо встроенных модулей `snapshot-controller` и `static-routing-manager` автоматически будут использоваться модули `snapshot-controller` и `static-routing-manager`, загружаемые из внешнего источника (ModuleSource `deckhouse`).

- Новая версия Cilium требует, чтобы ядро Linux на узлах было версии 5.8 или новее. Если на каком-либо из узлов кластера установлено ядро версии ниже 5.8, обновление Deckhouse Kubernetes Platform будет заблокировано. При обновлении поды `cilium` будут перезапущены .

- Все компоненты DKP будут перезапущены в процессе обновления.

### Основные изменения

- Добавлена возможность включения обязательного использования двухфакторной аутентификации для статических пользователей. Управляется секцией параметров [`staticUsers2FA`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/user-authn/configuration.html#parameters-staticusers2fa) модуля `user-authn`.

- Добавлена поддержка GPU на узлах. Доступно управление тремя режимами разделения ресурсов GPU: `Exclusive` (без разделения), `TimeSlicing` (разделение по времени), `MIG` (разделение одного GPU на несколько экземпляров). Для управления режимом разделения ресурсов GPU используется секция параметров [spec.gpu](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#nodegroup-v1-spec-gpu) в NodeGroup. Использование GPU на узле возможно после установки NVIDIA Container Toolkit и драйвера GPU.

- При включении модуля (`d8 system module enable`) или при редактировании ресурса ModuleConfig, теперь выводится предупреждение, если для модуля найдено несколько источников модуля. В этом случае требуется явно указать источник модуля в параметре [`source`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/cr.html#moduleconfig-v1alpha1-spec-source) конфигурации модуля.

- Улучшена обработка ошибок конфигурации модулей. Теперь ошибки при работе модуля не блокируют работу DKP, а отображаются в статусах объектов Module и ModuleRelease.

- Улучшена поддержка виртуализации:
  - Добавлен [провайдер интеграции с платформой виртуализации](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/cloud-provider-dvp/) (Deckhouse Virtualization Platform). Новый провайдер позволяет разворачивать кластеры DKP поверх DVP.
  - В модуле `cni-cilium` добавлена поддержка вложенной виртуализации на узлах.

- В модуле `node-manager` добавлены новые возможности для повышения надёжности и управляемости узлов:
  - Добавлена возможность запретить перезапуск узла, если на нем все еще расположены критичные поды (отмеченные лейблом `pod.deckhouse.io/inhibit-node-shutdown`). Это может быть необходимо для обеспечения корректной работы со stateful-нагрузками (например, выполняющих долгую миграцию данных).
  - Добавлена новая версия API `v1alpha2` ресурса [SSHCredential](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#sshcredentials), в которой параметр [`sudoPasswordEncoded`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#sshcredentials) позволяет задавать пароль `sudo` в формате Base64.
  - Параметр [`capiEmergencyBrake`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/node-manager/configuration.html#parameters-capiemergencybrake) позволяет отключить CAPI в экстренной ситуации, предотвращая потенциально разрушительные изменения. Поведение аналогично существующей настройке [`mcmEmergencyBrake`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/node-manager/configuration.html#parameters-mcmemergencybrake).

- Добавлена проверка подключения к хранилищу образов контейнеров DKP перед установкой.

- Улучшен механизм ротации файлов при использовании кратковременного хранилища логов (модуль `loki`). Добавлен алерт [`LokiInsufficientDiskForRetention`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/alerts.html#loki-lokiinsufficientdiskforretention), предупреждающий о нехватке размера хранилища.

- В документацию добавлена [справка](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/deckhouse-cli/reference/) по командам и параметрам Deckhouse CLI (утилита `d8`).

- При использовании кодирования CEF при сборе логов из [Apache Kafka](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-kafka-encoding-cef) или [из сокета](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-socket-encoding-cef), появилась возможность настраивать служебные поля формата, такие как Device Product, Device Vendor и Device ID.

- Поле [`passwordHash`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#nodeuser-v1-spec-passwordhash) в ресурсе NodeUser больше не является обязательным. Это позволяет создавать пользователей без пароля, например, в кластерах с внешними системами аутентификации (например, PAM, LDAP).

- Добавлена поддержка CRI Containerd версии 2, использующей CgroupsV2. В новой версии применяется другой формат конфигурации, а также реализован механизм миграции между Containerd V1 и V2. Изменить тип используемого на узлах CRI можно с помощью параметра [`cri.type`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#nodegroup-v1-spec-cri-type), а управлять настройками - с помощью параметра [`cri.containerdV2`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#nodegroup-v1-spec-cri-containerdv2).

### Безопасность

- Возможность [проверки подписей контейнерных образов](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/admission-policy-engine/cr.html#securitypolicy-v1alpha1-spec-policies-verifyimagesignatures) стала доступна в редакции DKP SE+. Теперь эта возможность доступна в DKP SE+, EE и CSE.

- Модули `log-shipper`, `deckhouse-controller` и `Istio` (версия 1.21) переведены на distroless-сборку. Это повышает безопасность и обеспечивает более прозрачный и контролируемый процесс сборки.

- Добавлены правила аудита для регистрации взаимодействия с containerd. Отслеживается доступ к сокету `/run/containerd/containerd.sock`, изменение директорий `/etc/containerd`, `/var/lib/containerd` и файла `/opt/deckhouse/bin/containerd`.

- Закрыты известные уязвимости в модулях `loki`, `extended-monitoring`, `operator-prometheus`, `prometheus`, `prometheus-metrics-adapter`, `user-authn`, `cloud-provider-zvirt`, `cloud-provider-dynamix`.

### Сеть

- Добавлена поддержка Istio версии 1.25.2. Для этой версии используется оператор Sail вместо Istio Operator, поддержка которого прекращена. Также добавлена поддержка Kiali версии 2.7, без поддержки Ambient Mesh. Версия Istio 1.19 считается устаревшей.

- Добавлена возможность шифрования трафика между узлами и подами с использованием протокола WireGuard (параметр [`encryption.mode`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/cni-cilium/configuration.html#parameters-encryption-mode)).

- Исправлена логика определения готовности сервиса (ресурс [ServiceWithHealthcheck](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/service-with-healthchecks/cr.html#servicewithhealthchecks)). Ранее поды без IP-адреса (например, находящихся в состоянии `Pending`) могли ошибочно попадать в список балансировки.

- Добавлена поддержка алгоритма балансировки нагрузки least-conn для сервисов. Алгоритм least-conn направляет трафик на бэкенд сервиса с наименьшим числом активных подключений, что повышает производительность приложений с большим количеством соединений (например, WebSocket). Чтобы управлять алгоритмом балансировки, необходимо включить параметр [`extraLoadBalancerAlgorithmsEnabled`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/modules/cni-cilium/configuration.html#parameters-extraloadbalanceralgorithmsenabled) в настройках модуля `cni-cilium`, и использовать на уровне сервиса аннотацию `service.cilium.io/lb-algorithm`, выбрав поддерживаемый алгоритм (random, maglev или least-conn).

- В Cilium 1.17 исправлена ошибка в `cilium-operator`, из-за которой IP-адреса могли не переиспользоваться после удаления `CiliumEndpoint`. Это происходило из-за некорректной очистки фильтра приоритетов, что могло привести к исчерпанию IP-пула в больших кластерах.

- Детализирован [список портов, используемых при сетевом взаимодействии](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.71/network_security_setup.html):
  - Добавлено и обновлено:
    - `4287/UDP` — порт WireGuard для шифрования трафика в CNI Cilium.
    - `4295-4297/UDP` — порт используется модулем `cni-cilium` для VXLAN-инкапсуляции трафика между подами при множественной вложенной виртуализации — когда DKP с включенным модулем `virtualization` развернут внутри виртуальных машин, также созданных в DKP с включенным модулем `virtualization`.
    - `4298/UDP` — порт используется модулем `cni-cilium` для VXLAN-инкапсуляции трафика между подами, если кластер был развернут на DKP, начиная с версии **1.71** (для кластеров, развернутых на DKP до версии **1.71**, см. примечание для портов `4299/UDP`, `8469/UDP` и `8472/UDP`).
    - `4299/UDP` — порт **для кластеров, развернутых на DKP версий 1.64–1.70.** Используется модулем `cni-cilium` для VXLAN-инкапсуляции трафика между подами. Обновление DKP до более новых версий не изменит занимаемый порт, если не включается модуль `virtualization`. **Обратите внимание,** что в таких кластерах включение модуля `virtualization` на DKP до версии 1.70 меняет порт на `4298/UDP`.
    - `8469/UDP` — порт **для кластеров, развернутых на DKP версии 1.63 и ниже с модулем `virtualization`, включенным до DKP версии 1.63.** Используется модулем `cni-cilium` для VXLAN-инкапсуляции трафика между подами. Обновление DKP до более новых версий не изменит занимаемый порт.
    - `8472/UDP` — порт **для кластеров, развернутых на DKP версии 1.63 и ниже.** Используется модулем `cni-cilium` для VXLAN-инкапсуляции трафика между подами. Обновление DKP до более новых версий не изменит занимаемый порт, если не включается модуль `virtualization`. **Обратите внимание,** что в таких кластерах включение модуля `virtualization` на DKP до версии 1.70 меняет порт:
      - включение модуля `virtualization` на DKP версии 1.63 и ниже изменит его на `8469/UDP` и не изменит при последующих обновлениях DKP.
      - включение модуля `virtualization` на DKP, начиная с версии 1.64, изменит его на `4298/UDP` и не изменит при последующих обновлениях DKP.
  - Удалено:
    - `49152`, `49153/TCP` — порты использовались для live-миграции виртуальных машин (модуль `virtualization`). Теперь миграция работает через сеть подов.  

Полный список изменений, включая перечень обновлённых компонентов,
доступен в [журнале изменений (changelog)](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.71.md) на GitHub.

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

- В провайдерах для `Yandex Cloud`, `Zvirt` и `Basis Dynamix` вместо Terraform теперь используется `OpenTofu`. Это позволит приносить изменения в провайдер, например, чтобы устранять известные уязвимости (CVE).

- Исправлены CVE-уязвимости в модулях: `chrony`, `descheduler`, `dhctl`, `node-manager`, `registry-packages-proxy`, `falco`, `cni-cilium`, `vertical-pod-autoscaler`.

Полный список изменений, включая перечень обновлённых компонентов,
доступен в [журнале изменений (changelog)](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.70.md) на GitHub.

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

- Закрыты известные уязвимости в модулях `ingress-nginx`, `istio`, `prometheus` и `local-path-provisioner`.

Полный список изменений, включая перечень обновлённых компонентов,
доступен в [журнале изменений (changelog)](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.69.md) на GitHub.

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

- У аутентификатора (ресурс DexAuthenticator) появился [параметр `highAvailability`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.68/modules/user-authn/cr.html#dexauthenticator-v1-spec-highavailability),
  который управляет режимом высокой доступности.
  В режиме высокой доступности запускается несколько реплик аутентификатора.
  Ранее режим высокой доступности всех аутентификаторов определялся [настройками глобального параметра](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.68/deckhouse-configure-global.html#parameters-highavailability)
  или настройками модуля `user-authn`.
  Все аутентификаторы, развёрнутые самим DKP, теперь наследуют режим высокой доступности соответствующего модуля.

- Лейблы узлов теперь можно добавлять, удалять и изменять,
  используя файлы, хранящиеся на узле в директории `/var/lib/node_labels` и её поддиректориях.
  Полный набор применённых лейблов хранится в аннотации `node.deckhouse.io/last-applied-local-labels`.

- Добавлена поддержка [облачного провайдера Huawei Cloud](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.68/modules/cloud-provider-huaweicloud/).

- Новый параметр [`keepDeletedFilesOpenedFor`](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.68/modules/log-shipper/cr.html#clusterloggingconfig-v1alpha1-spec-kubernetespods-keepdeletedfilesopenedfor) в модуле `log-shipper` позволяет настроить период,
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
  Параметр имеет влияние только для следующих облаков: AWS, Azure, GCP, Yandex Cloud, OpenStack, Basis Dynamix.

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

Полный список изменений, включая перечень обновлённых компонентов,
доступен в [журнале изменений (changelog)](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.68.md) на GitHub.
