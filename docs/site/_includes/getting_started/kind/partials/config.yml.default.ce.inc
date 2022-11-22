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
    # [<en>] Enabling a group of modules required for monitoring.
    # [<ru>] Включение группы модулей, необходимых для работы мониторинга.
    operatorPrometheusCrdEnabled: true
    operatorPrometheusEnabled: true
    prometheusCrdEnabled: true
    prometheusEnabled: true
    # [<en>] Disabling longterm Prometheus in the prometheus module configuration —  it is redundant for Deckhouse testing.
    # [<ru>] В настройках модуля prometheus отключаем запуск longterm Prometheus — для тестирования Deckhouse он избыточен.
    # [<ru>] Подробнее о longterm Prometheus — https://deckhouse.ru/documentation/v1/modules/300-prometheus/
    prometheus:
      longtermRetentionDays: 0
    # [<en>] Enabling a module for cluster control plane monitoring.
    # [<ru>] Включаем модуль мониторинга control plane кластера.
    monitoringKubernetesControlPlaneEnabled: true
    # [<en>] Enabling a module for installing Ingress controller.
    # [<ru>] Включаем модуль установки Ingress-контроллера.
    ingressNginxEnabled: true
