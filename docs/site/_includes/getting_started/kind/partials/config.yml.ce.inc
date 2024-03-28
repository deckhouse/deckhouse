apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    # [<en>] The Minimal bundle is used when installing Deckhouse in an existing cluster.
    # [<ru>] При установке Deckhouse в существующий кластер используется вариант поставки — Minimal.
    bundle: Minimal
    # [<ru>] Канал обновлений Deckhouse. По умолчанию установлен канал EarlyAccess — рекомендуемый канал обновлений,
    # [<ru>] если вы не уверены в выборе. Подойдет для кластеров, в которых идет активная работа (запускаются,
    # [<ru>] дорабатываются новые приложения и т. п.). Он достаточно стабилен для продакшена, но если планируется
    # [<ru>] использовать несколько кластеров, то рекомендуется установить на них последовательные каналы обновлений,
    # [<ru>] чтобы обновления происходили по очереди. Это позволит минимизировать риски в будущем.
    # [<ru>] Подробнее о каналах обновлений: https://deckhouse.ru/documentation/v1/deckhouse-release-channels.html
    # [<en>] Канал обновлений Deckhouse. По умолчанию установлен канал EarlyAccess — рекомендуемый канал обновлений,
    # [<en>] если вы не уверены в выборе. Подойдет для кластеров, в которых идет активная работа (запускаются,
    # [<en>] дорабатываются новые приложения и т. п.). Он достаточно стабилен для продакшена, но если планируется
    # [<en>] использовать несколько кластеров, то рекомендуется установить на них последовательные каналы обновлений,
    # [<en>] чтобы обновления происходили по очереди. Это позволит минимизировать риски в будущем.
    # [<en>] Подробнее о каналах обновлений: https://deckhouse.io/documentation/v1/deckhouse-release-channels.html
    releaseChannel: EarlyAccess
    logLevel: Info
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    modules:
      # [<en>] Template for Ingress resources of Deckhouse modules.
      # [<en>] E.g., Grafana for %s.example.com will be available as 'grafana.example.com'.
      # [<en>] The domain MUST NOT match the one specified in the clusterDomain parameter of the ClusterConfiguration resource.
      # [<en>] The sslip.io service is used as as a working example.
      # [<ru>] Шаблон для создания Ingress-ресурсов модулей Deckhouse.
      # [<ru>] Например, Grafana для %s.example.com будет доступна на домене grafana.example.com.
      # [<ru>] Домен НЕ ДОЛЖЕН совпадать с указанным в параметре clusterDomain ресурса ClusterConfiguration.
      # [<ru>] В качестве рабочего примера используется сервис sslip.io.
      publicDomainTemplate: "%s.127.0.0.1.sslip.io"
      https:
          mode: Disabled
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus
spec:
  version: 2
  enabled: true
  settings:
    longtermRetentionDays: 0
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: ingress-nginx
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: operator-prometheus-crd
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: operator-prometheus
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus-crd
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-kubernetes
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-deckhouse
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-kubernetes-control-plane
spec:
  enabled: true
