# [<en>] Section for bootstrapping the Deckhouse cluster (InitConfiguration).
# [<en>] Version of the Deckhouse API.
# [<ru>] Секция первичной инициализации кластера Deckhouse (InitConfiguration).
# [<ru>] Используемая версия API Deckhouse.
apiVersion: deckhouse.io/v1
# [<en>] Type of the configuration section.
# [<ru>] Тип секции конфигурации.
kind: InitConfiguration
# [<en>] Deckhouse parameters.
# [<ru>] Секция с параметрами Deckhouse.
deckhouse:
  # [<en>] Address of the Docker registry where the Deckhouse images are located.
  # [<ru>] Адрес Docker registry с образами Deckhouse.
  imagesRepo: registry.deckhouse.io/deckhouse/ee
  # [<en>] A special string with your token to access Docker registry (generated automatically for your license token).
  # [<ru>] Строка с ключом для доступа к Docker registry (сгенерировано автоматически для вашего токена доступа).
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
  # [<en>] The release channel in use.
  # [<ru>] Используемый канал обновлений.
  releaseChannel: Stable
  # [<en>] The Minimal bundle is used when installing Deckhouse in an existing cluster.
  # [<ru>] При установке Deckhouse в существующий кластер используется вариант поставки — Minimal.
  bundle: Minimal
  configOverrides:
    global:
      modules:
        # [<en>] Template for Ingress resources of Deckhouse modules.
        # [<en>] E.g., Grafana for %s.example.com will be available as 'grafana.example.com'.
        # [<en>] The nip.io service is used as as a working example.
        # [<ru>] Шаблон для создания Ingress-ресурсов модулей Deckhouse.
        # [<ru>] Например, Grafana для %s.example.com будет доступна на домене grafana.example.com.
        # [<ru>] В качестве рабочего примера используется сервис nip.io.
        publicDomainTemplate: "%s-127-0-0-1.nip.io"
        https:
          mode: Disabled
    operatorPrometheusCrdEnabled: true
    operatorPrometheusEnabled: true
    prometheus:
      longtermRetentionDays: 0
    prometheusCrdEnabled: true
    prometheusEnabled: true
    monitoringKubernetesControlPlaneEnabled: true
    ingressNginxEnabled: true
