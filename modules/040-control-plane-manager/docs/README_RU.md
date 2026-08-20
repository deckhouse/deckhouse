---
title: Управление control plane
description: Deckhouse управляет компонентами control plane Kubernetes — сертификатами, манифестами, версиями. Управляет конфигурацией etcd-кластера и следит за актуальностью конфигурации для kubectl.
---

Управление компонентами control plane кластера осуществляется с помощью модуля `control-plane-manager`, который запускается на всех master-узлах кластера (узлы с лейблом `node-role.kubernetes.io/control-plane: ""`).

Функции управления control plane:

- **Управление сертификатами**, необходимыми для работы control-plane, в том числе продление, выпуск при изменении конфигурации и т. п. Позволяет автоматически поддерживать безопасную конфигурацию control plane и быстро добавлять дополнительные SAN для организации защищенного доступа к API Kubernetes.
- **Настройка компонентов**. Автоматически создает необходимые конфигурации и манифесты компонентов `control-plane`.
- **Upgrade/downgrade компонентов**. Поддерживает в кластере одинаковые версии компонентов.
- **Управление конфигурацией etcd-кластера** и его членов. Масштабирует master-узлы, выполняет миграцию из single-master в multi-master и обратно.
- **Настройка kubeconfig**. Обеспечивает актуальные файлы kubeconfig на узлах control-plane. Генерирует, продлевает и обновляет kubeconfig для компонентов control-plane и admin kubeconfig (`admin.conf`). По умолчанию создаёт символическую ссылку для root-пользователя (`/root/.kube/config` -> `admin.conf`). При включённом модуле [user-authz](/modules/user-authz/) символическую ссылку можно отключить параметром `rootKubeconfigSymlink` в модуле **control-plane-manager** (см. [FAQ](faq.html#модель-административного-доступа-к-кластеру)). Также ужесточает права доступа к файлам `admin.conf` и `super-admin.conf`.
- **Расширение работы планировщика**, за счет подключения внешних плагинов через вебхуки. Управляется ресурсом [KubeSchedulerWebhookConfiguration](cr.html#kubeschedulerwebhookconfiguration). Позволяет использовать более сложную логику при решении задач планирования нагрузки в кластере. Например:
  - размещение подов приложений организации хранилища данных ближе к самим данным,
  - приоритизация узлов в зависимости от их состояния (сетевой нагрузки, состояния подсистемы хранения и т. д.),
  - разделение узлов на зоны, и т. п.
- **Мониторинг control plane**. Модуль организует безопасный сбор метрик и предоставляет базовый набор правил мониторинга компонентов control plane. Подробнее — в подразделе [«Мониторинг control plane»](#мониторинг-control-plane).
- **Периодическая дефрагментация etcd**. В кластерах с тремя и более членами etcd функция включена по умолчанию. Подробнее — в описании параметра [`etcd.defrag`](configuration.html#parameters-etcd-defrag).

## Управление сертификатами

Управляет SSL-сертификатами компонентов `control-plane`:

- Серверными сертификатами для `kube-apiserver` и `etcd`. Они хранятся в Secret'е `d8-pki` неймспейса `kube-system`:
  - корневой CA kubernetes (`ca.crt` и `ca.key`);
  - корневой CA etcd (`etcd/ca.crt` и `etcd/ca.key`);
  - RSA-сертификат и ключ для подписи Service Account'ов (`sa.pub` и `sa.key`);
  - корневой CA для extension API-серверов (`front-proxy-ca.key` и `front-proxy-ca.crt`).
- Клиентскими сертификатами для подключения компонентов `control-plane` друг к другу. Выписывает, продлевает и перевыписывает, если что-то изменилось (например, список SAN). Следующие сертификаты хранятся только на узлах:
  - серверный сертификат API-сервера (`apiserver.crt` и `apiserver.key`);
  - клиентский сертификат для подключения `kube-apiserver` к `kubelet` (`apiserver-kubelet-client.crt` и `apiserver-kubelet-client.key`);
  - клиентский сертификат для подключения `kube-apiserver` к `etcd` (`apiserver-etcd-client.crt` и `apiserver-etcd-client.key`);
  - клиентский сертификат для подключения `kube-apiserver` к extension API-серверам (`front-proxy-client.crt` и `front-proxy-client.key`);
  - серверный сертификат `etcd` (`etcd/server.crt` и `etcd/server.key`);
  - клиентский сертификат для подключения `etcd` к другим членам кластера (`etcd/peer.crt` и `etcd/peer.key`);
  - клиентский сертификат для подключения `kubelet` к `etcd` для helthcheck'ов (`etcd/healthcheck-client.crt` и `etcd/healthcheck-client.key`).

Также позволяет добавить дополнительные SAN в сертификаты, это дает возможность быстро и просто добавлять дополнительные «точки входа» в API Kubernetes.

При изменении сертификатов также автоматически обновляется соответствующая конфигурация kubeconfig.

## Масштабирование

Поддерживается работа `control-plane` в конфигурации как *single-master*, так и *multi-master*.

В конфигурации *single-master*:

- `kube-apiserver` использует только тот экземпляр `etcd`, который размещен с ним на одном узле;
- На узле настраивается прокси-сервер, отвечающий на localhost,`kube-apiserver` отвечает на IP-адрес master-узла.

В конфигурации *multi-master* компоненты `control-plane` автоматически разворачиваются в отказоустойчивом режиме:

- `kube-apiserver` настраивается для работы со всеми экземплярами `etcd`.
- На каждом master-узле настраивается дополнительный прокси-сервер, отвечающий на localhost. Прокси-сервер по умолчанию обращается к локальному экземпляру `kube-apiserver`, но в случае его недоступности последовательно опрашивает остальные экземпляры `kube-apiserver`.

### Масштабирование master-узлов

Масштабирование узлов `control-plane` осуществляется автоматически, с помощью лейбла `node-role.kubernetes.io/control-plane=""`:

- Установка лейбла `node-role.kubernetes.io/control-plane=""` на узле приводит к развертыванию на нем компонентов `control-plane`, подключению нового узла `etcd` в etcd-кластер, а также перегенерации необходимых сертификатов и конфигурационных файлов.
- Удаление лейбла `node-role.kubernetes.io/control-plane=""` с узла приводит к удалению всех компонентов `control-plane`, перегенерации необходимых конфигурационных файлов и сертификатов, а также корректному исключению узла из etcd-кластера.

{% alert level="warning" %}
При масштабировании узлов с 2 до 1 требуются [ручные действия](./faq.html#что-делать-если-кластер-etcd-развалился) с `etcd`. В остальных случаях все необходимые действия происходят автоматически. Обратите внимание, что при масштабировании с любого количества master-узлов до 1 рано или поздно на последнем шаге возникнет ситуация масштабирования узлов с 2 до 1.
{% endalert %}

### Динамическое пороговое значение удаления выселенных подов

Автоматически настраивает оптимальное значение `--terminated-pod-gc-threshold` в зависимости от размера кластера:

- **Малые кластеры** (менее 100 узлов): 1000 завершенных подов.
- **Средние кластеры** (от 100 до 300 узлов): 3000 завершенных подов.  
- **Крупные кластеры** (от 300 узлов): 6000 завершенных подов.

{% alert level="info" %}
Эта функция применяется только в средах, где параметр `--terminated-pod-gc-threshold` можно настраивать. В управляемых Kubernetes-кластерах, таких как EKS, GKE, AKS, это значение контролируется провайдером.
{% endalert %}

## Настройка ресурсных запросов control plane

Модуль позволяет задать суммарные ресурсные запросы CPU и памяти для компонентов control plane на каждом master-узле: `kube-apiserver`, `etcd`, `kube-controller-manager` и `kube-scheduler`.

Для настройки используйте параметр [`resourcesRequests`](configuration.html#parameters-resourcesrequests) в ModuleConfig `control-plane-manager`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 3
  enabled: true
  settings:
    resourcesRequests:
      cpu: 1000m
      memory: 500Mi
```

Указанные значения используются как общий бюджет запросов для компонентов control plane на каждом master-узле. Deckhouse Kubernetes Platform (DKP) распределяет этот бюджет между статическими подами control plane при формировании их манифестов.

{% alert level="info" %}
Эти настройки не применяются, если control plane кластера управляется облачным провайдером, например в GKE, AKS или EKS.
{% endalert %}

## Управление версиями

Обновление **patch-версии** компонентов control plane (то есть в рамках минорной версии, например с `1.31.13` на `1.31.14`) происходит автоматически вместе с обновлением версии DKP. Управлять обновлением patch-версий нельзя.

Обновлением **минорной-версии** компонентов control plane (например, с `1.32.*` на `1.33.*`) можно управлять с помощью параметра [kubernetesVersion](configuration.html#parameters-kubernetesversion) ModuleConfig `control-plane-manager`, в котором можно выбрать режим следования за версией по умолчанию для текущего релиза DKP (значение `Default`) или указать желаемую минорную версию control plane. Версию control plane, которая используется по умолчанию (при `kubernetesVersion: Default`), а также список поддерживаемых версий Kubernetes можно найти в разделе [«Поддерживаемые версии Kubernetes и ОС»](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html).

Версия Kubernetes в кластере определяется в следующем порядке: параметр `kubernetesVersion` в ModuleConfig `control-plane-manager`, затем устаревшее поле [`ClusterConfiguration.kubernetesVersion`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion), затем версия по умолчанию текущего релиза DKP. Значение из ModuleConfig имеет приоритет всегда, когда оно задано, включая `Default`; пока оно не задано, версию определяет устаревшее поле. Алерт `D8ObsoleteKubernetesVersionFieldInClusterConfiguration` появляется в кластере при самом факте присутствия поля — в том числе когда версию уже определяет параметр в ModuleConfig, — и пропадает только после удаления этого поля из `ClusterConfiguration`.

Пример закрепления версии Kubernetes:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 3
  enabled: true
  settings:
    kubernetesVersion: "1.33"
```

Обновление control plane выполняется безопасно и для single-master-, и для multi-master-кластеров. Во время обновления может быть кратковременная недоступность API-сервера. На работу приложений в кластере обновление не влияет и может выполняться без выделения окна для регламентных работ.

Если указанная для обновления версия (с параметром [kubernetesVersion](configuration.html#parameters-kubernetesversion)) не соответствует текущей версии control plane в кластере, запускается умная стратегия изменения версий компонентов:

- Общие замечания:
  - Обновление в разных NodeGroup выполняется параллельно. Внутри каждой NogeGroup узлы обновляются последовательно, по одному.
- При upgrade:
  - Обновление происходит **последовательными этапами**, по одной минорной версии: 1.32 -> 1.33, 1.33 -> 1.34, 1.35 -> 1.36.
  - На каждом этапе сначала обновляется версия control plane, затем происходит обновление kubelet на узлах кластера.  
- При downgrade (не поддерживается для редакций CSE):
  - Успешное понижение версии гарантируется только на одну версию вниз от максимальной минорной версии control plane, когда-либо использовавшейся в кластере.
  - Сначала на узлах кластера выполняется понижение версии kubelet, после чего производится понижение версии компонентов control plane.

## Публикация API kubernetes

Компонент kube-apiserver без дополнительных настроек доступен только во внутренней сети кластера. Этот модуль решает проблему простого и безопасного доступа к API Kubernetes извне кластера.

### Через Ingress

Указанием параметров [`apiserver.publishAPI.ingress`](configuration.html#parameters-apiserver-publishapi-ingress) можно опубликовать API-сервер на специальном домене (подробнее см. [раздел о служебных доменах в документации](/products/kubernetes-platform/documentation/v1/reference/api/global.html)).

При настройке можно указать:

* перечень сетевых адресов и подсетей, с которых разрешено подключение;
* Ingress-контроллер, на котором производится публикация;
* Использовать ли вручную заданный, полученный через cert-manager или автоматический самоподписанный сертификат TLS.

По умолчанию будет сгенерирован специальный сертификат ЦС (CA) и автоматически настроен генератор kubeconfig.

### Через сервис с типом LoadBalancer

Указанием параметров [`apiserver.publishAPI.loadBalancer`](configuration.html#parameters-apiserver-publishapi-loadbalancer) можно создать сервис с типом LoadBalancer `kube-system/d8-control-plane-apiserver`.

При настройке можно указать:

* перечень сетевых адресов и подсетей, с которых разрешено подключение;
* внешний порт сервиса;
* аннотации на сервис для настроек провайдера балансировки.

## Аудит

Если требуется журналировать операции с API или отдебажить неожиданное поведение, для этого в Kubernetes предусмотрен [Auditing](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/). Его можно настроить путем создания правил [Audit Policy](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy), а результатом работы аудита будет лог-файл `/var/log/kube-audit/audit.log` со всеми интересующими операциями.

В установках DKP по умолчанию созданы базовые политики, которые отвечают за логирование событий, которые:

- связаны с операциями создания, удаления и изменения ресурсов;
- совершаются от имен сервисных аккаунтов из системных Namespace `kube-system`, `d8-*`;
- совершаются с ресурсами в системных неймспейсах `kube-system`, `d8-*`.

Для выключения базовых политик установите флаг [basicAuditPolicyEnabled](configuration.html#parameters-apiserver-basicauditpolicyenabled) в `false`.

При настройке OIDC-аутентификации в аудит-логах дополнительно включается информация о пользователе в поле `user.extra`:
- `user-authn.deckhouse.io/name` — отображаемое имя пользователя
- `user-authn.deckhouse.io/preferred_username` — предпочитаемое имя пользователя
- `user-authn.deckhouse.io/dex-provider` — идентификатор провайдера Dex (требует scope `federated:id`)

Настройка политик аудита подробнее рассмотрена в [одноименной секции FAQ](faq.html#как-настроить-дополнительные-политики-аудита).

## Мониторинг control plane

Модуль организует безопасный сбор метрик и предоставляет базовый набор правил мониторинга следующих компонентов кластера:

- `kube-apiserver`;
- `kube-controller-manager`;
- `kube-scheduler`;
- `kube-etcd`.

## Admission-плагины, включаемые по умолчанию

При установке Deckhouse Kubernetes Platform помимо стандартных admission-плагинов, включаемых Kubernetes, модуль включает несколько дополнительных. Подробнее об admission-плагинах — в [документации Kubernetes](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#validatingadmissionwebhook).

### Стандартные admission-плагины, включаемые Kubernetes

Kubernetes включает следующие стандартные admission-плагины:

| Admission‑плагин | Тип | Краткое описание |
| :--- | :--- | :--- |
| **NamespaceLifecycle** | Валидирующий | Не позволяет создавать новые объекты в неймспейсах, которые находятся в процессе удаления (termination), а также в несуществующих неймспейсах. Также он предотвращает удаление системных неймспейсов: `default`, `kube-system` и `kube-public`. |
| **LimitRanger** | Мутирующий и Валидирующий | Наблюдает за входящими запросами и проверяет, не нарушают ли они ограничения, заданные в объекте LimitRange в неймспейсе. Используется для принудительного применения лимитов ресурсов на уровне контейнеров и подов. |
| **ServiceAccount** | Мутирующий и Валидирующий | Автоматизирует работу с ServiceAccount. Если у пода не указан `ServiceAccount`, он автоматически назначает аккаунт `default` из того же неймспейса. Также проверяет, что указанный ServiceAccount существует. |
| **TaintNodesByCondition** | Мутирующий | Стандартный плагин безопасности, который автоматически добавляет taints на вновь созданные узлы в зависимости от их состояния (например, `NotReady` или `Unreachable`). Это позволяет избежать ситуаций, когда поды  будут планироваться на новые узлы до того, как их метки будут обновлены, чтобы точно отражать их заявленное состояние. |
| **PodSecurity** | Валидирующий | Проверяет новые поды перед их запуском и определяет, следует ли их допустить, на основе запрошенного контекста безопасности и ограничений, установленных Pod Security Standards в том неймспейсе, в котором будет находиться данный под. |
| **Priority** | Мутирующий и Валидирующий | Использует поле `priorityClassName` и заполняет целочисленное значение приоритета. Если класс приоритета не найден, под отклоняется. |
| **DefaultTolerationSeconds** | Мутирующий | Устанавливает для подов значения toleration по умолчанию для taints `notready:NoExecute` и `unreachable:NoExecute`, если под не имеет своих tolerations. Значение по умолчанию — **5 минут**. |
| **DefaultStorageClass** | Мутирующий | Наблюдает за созданием объектов PersistentVolumeClaim. Если в запросе не указан конкретный StorageClass, автоматически добавляет StorageClass, помеченный как `default`. Благодаря этому пользователи, не запрашивающие какой-либо StorageClass, получат StorageClass по умолчанию. |
| **StorageObjectInUseProtection** | Мутирующий | Защищает объекты хранения (например, PersistentVolume), которые используются подами, от случайного удаления. Добавляет финализаторы `kubernetes.io/pvc-protection` или `kubernetes.io/pv-protection`, предотвращающие удаление ресурса, пока он используется. |
| **PersistentVolumeClaimResize** | Валидирующий | Выполняет дополнительные проверки для входящих запросов на изменение размера PersistentVolumeClaim. По умолчанию запрещает изменение размера всех заявок (claims), за исключением тех случаев, когда в StorageClass заявки явно разрешено изменение размера путем установки параметра `allowVolumeExpansion` в значение `true`. |
| **RuntimeClass** | Мутирующий и Валидирующий | Учитывает `RuntimeClass` при создании подов. Устанавливает поле `pod.Spec.Overhead` (накладные расходы) в соответствии с выбранным классом выполнения и проверяет корректность запросов. |
| **CertificateApproval** | Валидирующий | Наблюдает за запросами на утверждение (approve) CertificateSigningRequest и выполняет дополнительные проверки авторизации, чтобы убедиться, что у пользователя есть права на утверждение запросов сертификатов. |
| **CertificateSigning** | Валидирующий | Наблюдает за обновлениями поля `status.certificate` в CertificateSigningRequest и проверяет, что у пользователя есть права на подписание (sign) запроса на сертификат, с использованием значения `spec.signerName`, указанного в ресурсе CertificateSigningRequest. |
| **ClusterTrustBundleAttest** | Валидирующий | Анализирует и подтверждает доверие к кластеру Kubernetes. Это может включать проверку сертификатов, настроек безопасности и других параметров, связанных с целостностью кластера. |
| **CertificateSubjectRestriction** | Валидирующий | Наблюдает за созданием CertificateSigningRequest с `spec.signerName` = `kubernetes.io/kube-apiserver-client`. Отклоняет запросы, в которых указана группа `system:masters`. |
| **DefaultIngressClass** | Мутирующий | Наблюдает за созданием объектов Ingress. Если в запросе не указан класс Ingress, автоматически добавляет класс Ingress, помеченный как default. |
| **PodTopologyLabels** | Мутирующий | Добавляет подам, привязанным к узлу, метки топологии (например, зону доступности), соответствующие меткам этого узла. |
| **MutatingAdmissionPolicy** | Валидирующий и Мутирующий | Механизм в рамках системы контроля доступа (admission control), который позволяет модифицировать объекты при их создании или обновлении в процессе приёма запроса. |
| **MutatingAdmissionWebhook** | Мутирующий | Вызывает все мутирующие вебхуки (mutating webhooks), которые соответствуют запросу. Вебхуки могут изменять объект и вызываются последовательно. |
| **ValidatingAdmissionPolicy** | Валидирующий | Позволяет встраивать декларативную валидацию непосредственно в API, без использования внешних HTTP-вызовов. Он проверяет CEL для входящих запросов, соответствующих критериям. Включается, когда одновременно активированы функция `validatingadmissionpolicy` и группа/версия `admissionregistration.k8s.io/v1alpha1`. Если происходит сбой в работе любой из политик `ValidatingAdmissionPolicy`, запрос отклоняется. |
| **ValidatingAdmissionWebhook** | Валидирующий | Вызывает для проверки объекта все валидирующие вебхуки (validating webhooks), которые соответствуют запросу. |
| **ResourceQuota** | Валидирующий | Наблюдает за входящими запросами и проверяет, не нарушают ли они ограничения, заданные в объекте ResourceQuota в неймспейсе. Используется для ограничения суммарного потребления ресурсов (CPU, память, количество объектов) в неймспейсе. |

### Дополнительные admission-плагины, включаемые модулем

В дополнение к [включаемым Kubernetes](#стандартные-admission-плагины-включаемые-kubernetesСтандартные admission-плагины, включаемые Kubernetes) модуль включает следующие admission-плагины (без возможности отключения):

|| Admission‑плагин | Тип | Краткое описание |
| :--- | :--- | :--- |
| **EventRateLimit** | Валидирующий | Позволяет решить проблему перегрузки API-сервера запросами на сохранение новых событий. Позволяет настроить лимиты на уровне неймспейса, пользователя или глобально. |
| **ExtendedResourceToleration** | Мутирующий | Автоматически добавляет toleration к подам, запрашивающим расширенные ресурсы (например, GPU, FPGA). Это позволяет выделить для таких подов специальные узлы, которые заранее помечены taint с именем ресурса, без ручного добавления tolerations на поды. |
| **NodeRestriction** | Валидирующий | Ограничивает набор объектов Node и Pod, которые может изменять kubelet. Повышает безопасность кластера. |
| **PodNodeSelector** | Валидирующий | Определяет и ограничивает, какие селекторы узлов могут использоваться в пределах неймспейса, на основе считывания аннотации неймспейса и глобальной конфигурации. |
| **PodTolerationRestriction** | Мутирующий & Валидирующий | Проверяет toleration пода на конфликты с toleration, заданными на уровне неймспейса. Если конфликтов нет, объединяет toleration пода и неймспейса. Также проверяет под на соответствие «белому списку» tolerations. |

{% alert level="warning" %}

Помимо указанных выше admission-плагинов (включаемых Kubernetes и модулем) можно включить и некоторые дополнительные. Для этого используйте параметр [`apiserver.admissionPlugins`](configuration.html#parameters-apiserver-admissionplugins).

{% endalert %}

## Feature Gates

Управление feature gates осуществляется с помощью параметра [enabledFeatureGates](configuration.html#parameters-enabledFeatureGates) ModuleConfig `control-plane-manager`.

Изменение списка feature gates вызывает перезапуск соответствующего компонента (например, `kube-apiserver`, `kube-scheduler`, `kube-controller-manager`, `kubelet`).

Пример включения feature gates `ComponentFlagz` и `ComponentStatusz`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 3
  enabled: true
  settings:
    enabledFeatureGates:
      - ComponentFlagz
      - ComponentStatusz
```

Если feature gate не поддерживается или имеет статус `deprecated`, в системе мониторинга будет сгенерирован алерт [D8ProblematicFeatureGateInUse](/products/kubernetes-platform/documentation/v1/reference/alerts.html#control-plane-manager-d8problematicfeaturegateinuse), информирующий о том, что feature gate не будет применен.

{% alert level="warning" %}
Обновление версии Kubernetes (управляется параметром [kubernetesVersion](configuration.html#parameters-kubernetesversion)) не произойдёт, если в списке включенных feature gates, заданных для новой версии Kubernetes, есть feature gates в статусе `deprecated`.
{% endalert %}

Описание feature gates доступно в [документации Kubernetes](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/){:target="_blank"}.

{% include feature_gates.liquid %}

## Защита чувствительных полей кастомных ресурсов

Feature gate `CRDSensitiveData` обеспечивает защиту чувствительных данных на уровне полей в ресурсах, помеченных маркером `x-kubernetes-sensitive-data: true`. Функция реализована в виде патча к `kube-apiserver` (`apiextensions-apiserver`)
и поддерживается, начиная с версии Kubernetes 1.31.

Маркер `x-kubernetes-sensitive-data` проверяется `kube-apiserver` при применении ресурса:

- маркер требует, чтобы был включен feature gate `CRDSensitiveData`. Он включается по умолчанию, его не следует указывать вручную;
- маркер не допускается устанавливать на корне схемы (на самом узле `openAPIV3Schema`). Чтобы защитить все поля ресурса, добавьте маркер на свойство `spec` (или на поддерево ниже него), а не на корень схемы — на корне также находятся системные поля (`apiVersion`, `kind`, `metadata`), которые невозможно зашифровать;
- тип поля должен быть одним из типов OpenAPI v3: `string`, `integer`, `number`, `boolean`, `object` или `array`. Маркер на `object` или `array` делает чувствительным всё поддерево;
- поддерживаются поля, объявленные как `x-kubernetes-int-or-string: true`;
- маркер запрещён внутри веток `anyOf`, `oneOf`, `allOf` и `not` (это проверяет валидатор структурной схемы).

Если хотя бы одно поле в схеме ресурса помечено `x-kubernetes-sensitive-data: true`, ко всем кастомным ресурсам этого типа применяются следующие меры защиты:

- **Шифрование в etcd** — весь ресурс шифруется с помощью того же механизма, что и Kubernetes Secrets.
  Требует включения параметра `apiserver.encryptionEnabled`.
- **Фильтрация полей на основе RBAC** — при выполнении запросов `get`, `list`, или `watch` чувствительные поля удаляются из ответа API, если у вызывающей стороны нет прав `get`, `list` или `watch` на субресурс `<resource>/sensitive`.
- **Маскировка в журнале аудита** — значения чувствительных полей всегда заменяются на `"******"` в журнале аудита, независимо от прав RBAC и уровня аудита.

Чтобы добавить к защите чувствительных полей шифрование в etcd, установите [параметр `apiserver.encryptionEnabled`](configuration.html#parameters-apiserver-encryptionenabled) в `true`.
Feature gate `CRDSensitiveData` включается по умолчанию, его не следует указывать вручную:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 3
  enabled: true
  settings:
    apiserver:
      encryptionEnabled: true
```

{% alert level="warning" %}
Включение `encryptionEnabled` необратимо и приводит к перезапуску `kube-apiserver`.
{% endalert %}

За подробностями обратитесь к следующим разделам:

- [«FAQ»](faq.html#как-защитить-чувствительные-поля-кастомных-ресурсов) — инструкция по включению защиты чувствительных полей;
- [«Примеры»](examples.html#защита-ресурсов-с-чувствительными-полями) — примеры конфигурации и результатов.
