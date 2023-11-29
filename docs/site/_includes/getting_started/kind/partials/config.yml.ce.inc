# [<en>] Section for bootstrapping the Deckhouse cluster.
# [<en>] https://deckhouse.io/documentation/v1/installing/configuration.html#initconfiguration
# [<ru>] Секция первичной инициализации кластера Deckhouse.
# [<ru>] https://deckhouse.ru/documentation/v1/installing/configuration.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
  configOverrides:
    operatorPrometheusCrdEnabled: true
    operatorPrometheusEnabled: true
    prometheusCrdEnabled: true
    monitoringDeckhouseEnabled: true
---
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
    releaseChannel: Stable
    logLevel: Info
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings: # <-- Параметры модуля из раздела "Параметры" ниже.
    modules:
      # [<en>] Template for Ingress resources of Deckhouse modules.
      # [<en>] E.g., Grafana for %s.example.com will be available as 'grafana.example.com'.
      # [<en>] The sslip.io service is used as as a working example.
      # [<ru>] Шаблон для создания Ingress-ресурсов модулей Deckhouse.
      # [<ru>] Например, Grafana для %s.example.com будет доступна на домене grafana.example.com.
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
  version: 1
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-kubernetes
spec:
  version: 1
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-kubernetes-control-plane
spec:
  enabled: true
