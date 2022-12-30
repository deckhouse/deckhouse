# [<en>] Section for bootstrapping the Deckhouse cluster.
# [<en>] https://deckhouse.io/documentation/v1/installing/configuration.html#initconfiguration
# [<ru>] Секция первичной инициализации кластера Deckhouse.
# [<ru>] https://deckhouse.ru/documentation/v1/installing/configuration.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/ee
  # [<en>] A special string with your token to access Docker registry (generated automatically for your license token).
  # [<ru>] Строка с ключом для доступа к Docker registry (сгенерировано автоматически для вашего токена доступа).
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
  releaseChannel: Stable
  # [<en>] The Minimal bundle is used when installing Deckhouse in an existing cluster.
  # [<ru>] При установке Deckhouse в существующий кластер используется вариант поставки — Minimal.
  bundle: Minimal
  configOverrides:
    global:
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
    operatorPrometheusCrdEnabled: true
    operatorPrometheusEnabled: true
    prometheusCrdEnabled: true
    prometheusEnabled: true
    monitoringKubernetesEnabled: true
    monitoringDeckhouseEnabled: true
    monitoringKubernetesControlPlaneEnabled: true
    ingressNginxEnabled: true
    prometheus:
      longtermRetentionDays: 0
