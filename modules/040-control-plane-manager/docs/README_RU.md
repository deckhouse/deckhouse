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
- **Настройка kubeconfig**. Обеспечивает всегда актуальную конфигурацию для работы kubectl. Генерирует, продлевает, обновляет kubeconfig с правами cluster-admin и создает symlink пользователю root, чтобы kubeconfig использовался по умолчанию.
- **Расширение работы планировщика**, за счет подключения внешних плагинов через вебхуки. Управляется ресурсом [KubeSchedulerWebhookConfiguration](cr.html#kubeschedulerwebhookconfiguration). Позволяет использовать более сложную логику при решении задач планирования нагрузки в кластере. Например:
  - размещение подов приложений организации хранилища данных ближе к самим данным,
  - приоритизация узлов в зависимости от их состояния (сетевой нагрузки, состояния подсистемы хранения и т. д.),
  - разделение узлов на зоны, и т. п.

## Управление сертификатами

Управляет SSL-сертификатами компонентов `control-plane`:

- Серверными сертификатами для `kube-apiserver` и `etcd`. Они хранятся в Secret'е `d8-pki` пространства имен `kube-system`:
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
Эта функция применяется только в средах, где параметр `terminated-pod-gc-threshold` можно настраивать. В управляемых Kubernetes-кластерах, таких как EKS, GKE, AKS, это значение контролируется провайдером.
{% endalert %}

## Управление версиями

Обновление **patch-версии** компонентов control plane (то есть в рамках минорной версии, например с `1.30.13` на `1.30.14`) происходит автоматически вместе с обновлением версии Deckhouse. Управлять обновлением patch-версий нельзя.

Обновлением **минорной-версии** компонентов control plane (например, с `1.30.*` на `1.31.*`) можно управлять с помощью параметра [kubernetesVersion](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion), в котором можно выбрать автоматический режим обновления (значение `Automatic`) или указать желаемую минорную версию control plane. Версию control plane, которая используется по умолчанию (при `kubernetesVersion: Automatic`), а также список поддерживаемых версий Kubernetes можно найти в [документации](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html).

Обновление control plane выполняется безопасно и для single-master-, и для multi-master-кластеров. Во время обновления может быть кратковременная недоступность API-сервера. На работу приложений в кластере обновление не влияет и может выполняться без выделения окна для регламентных работ.

Если указанная для обновления версия (с параметром [kubernetesVersion](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion)) не соответствует текущей версии control plane в кластере, запускается умная стратегия изменения версий компонентов:

- Общие замечания:
  - Обновление в разных NodeGroup выполняется параллельно. Внутри каждой NogeGroup узлы обновляются последовательно, по одному.
- При upgrade:
  - Обновление происходит **последовательными этапами**, по одной минорной версии: 1.30 -> 1.31, 1.31 -> 1.32, 1.32 -> 1.33.
  - На каждом этапе сначала обновляется версия control plane, затем происходит обновление kubelet на узлах кластера.  
- При downgrade:
  - Успешное понижение версии гарантируется только на одну версию вниз от максимальной минорной версии control plane, когда-либо использовавшейся в кластере.
  - Сначала на узлах кластера выполняется понижение версии kubelet, после чего производится понижение версии компонентов control plane.

## Аудит

Если требуется журналировать операции с API или отдебажить неожиданное поведение, для этого в Kubernetes предусмотрен [Auditing](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/). Его можно настроить путем создания правил [Audit Policy](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy), а результатом работы аудита будет лог-файл `/var/log/kube-audit/audit.log` со всеми интересующими операциями.

В установках Deckhouse по умолчанию созданы базовые политики, которые отвечают за логирование событий, которые:

- связаны с операциями создания, удаления и изменения ресурсов;
- совершаются от имен сервисных аккаунтов из системных Namespace `kube-system`, `d8-*`;
- совершаются с ресурсами в системных пространствах имен `kube-system`, `d8-*`.

Для выключения базовых политик установите флаг [basicAuditPolicyEnabled](configuration.html#parameters-apiserver-basicauditpolicyenabled) в `false`.

При настройке OIDC-аутентификации в аудит-логах дополнительно включается информация о пользователе в поле `user.extra`:
- `user-authn.deckhouse.io/name` — отображаемое имя пользователя
- `user-authn.deckhouse.io/preferred_username` — предпочитаемое имя пользователя
- `user-authn.deckhouse.io/dex-provider` — идентификатор провайдера Dex (требует scope `federated:id`)

Настройка политик аудита подробнее рассмотрена в [одноименной секции FAQ](faq.html#как-настроить-дополнительные-политики-аудита).

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
  version: 2
  enabled: true
  settings:
    enabledFeatureGates:
      - ComponentFlagz
      - ComponentStatusz
```

Если feature gate не поддерживается или имеет статус `deprecated`, в системе мониторинга будет сгенерирован алерт [D8ProblematicFeatureGateInUse](/products/kubernetes-platform/documentation/v1/reference/alerts.html#control-plane-manager-d8problematicfeaturegateinuse), информирующий о том, что feature gate не будет применен.

{% alert level="warning" %}
Обновление версии Kubernetes (управляется параметром [kubernetesVersion](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion)) не произойдёт, если в списке включенных feature gates, заданных для новой версии Kubernetes, есть feature gates в статусе `deprecated`.
{% endalert %}

Описание feature gates доступно в [документации Kubernetes](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/){:target="_blank"}.

{% include feature_gates.liquid %}
