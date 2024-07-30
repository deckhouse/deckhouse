# [<en>] Settings for the bootstrapping the Deckhouse cluster
# [<en>] https://deckhouse.io/documentation/v1/installing/configuration.html#initconfiguration
# [<ru>] Настройки первичной инициализации кластера Deckhouse.
# [<ru>] https://deckhouse.ru/documentation/v1/installing/configuration.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  # [<en>] Address of the Docker registry where the Deckhouse images are located.
  # [<ru>] Адрес Docker registry с образами Deckhouse.
  imagesRepo: registry.deckhouse.ru/deckhouse/ee
  # [<en>] A special string with your token to access Docker registry (generated automatically for your license token).
  # [<ru>] Строка с ключом для доступа к Docker registry (сгенерировано автоматически для вашего токена доступа).
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
---
# [<en>] Deckhouse module settings.
# [<en>] https://deckhouse.io/documentation/v1/modules/002-deckhouse/configuration.html
# [<ru>] Настройки модуля deckhouse.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/002-deckhouse/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    bundle: Minimal
    releaseChannel: Stable
    logLevel: Info
---
# [<en>] Global Deckhouse settings.
# [<en>] https://deckhouse.ru/documentation/v1/deckhouse-configure-global.html#parameters
# [<ru>] Глобальные настройки Deckhouse.
# [<ru>] https://deckhouse.ru/documentation/v1/deckhouse-configure-global.html#%D0%BF%D0%B0%D1%80%D0%B0%D0%BC%D0%B5%D1%82%D1%80%D1%8B
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    modules:
      # [<en>] Template that will be used for system apps domains within the cluster.
      # [<en>] E.g., Grafana for %s.example.com will be available as 'grafana.example.com'.
      # [<en>] The domain MUST NOT match the one specified in the clusterDomain parameter of the ClusterConfiguration resource.
      # [<en>] You can change it to your own or follow the steps in the guide and change it after installation.
      # [<ru>] Шаблон, который будет использоваться для составления адресов системных приложений в кластере.
      # [<ru>] Например, Grafana для %s.example.com будет доступна на домене 'grafana.example.com'.
      # [<ru>] Домен НЕ ДОЛЖЕН совпадать с указанным в параметре clusterDomain ресурса ClusterConfiguration.
      # [<ru>] Можете изменить на свой сразу, либо следовать шагам руководства и сменить его после установки.
      publicDomainTemplate: "%s.example.com"
      # [<ru>] If necessary, specify in the customTolerationKeys array
      # [<ru>] all the tains to which Deckhouse should have toleration.
      # [<ru>] The following is an example for the case if you need Deckhouse and its components to be able
      # [<ru>] to run on nodes that have taint SystemLoad.
      # [<ru>] При необходимости, укажите в массиве customTolerationKeys все taint'ы
      # [<ru>] к которым Deckhouse должен иметь toleration.
      # [<ru>] Далее приведен пример для случая, если нужно чтобы Deckhouse и его компоненты смогли запускаться
      # [<ru>] на узлах, имеющих taint SystemLoad
      # [<en>] You might consider changing this.
      # [<ru>] Возможно, захотите изменить.
      placement:
        customTolerationKeys:
        - SystemLoad
---
# [<en>] cert-manager module settings.
# [<en>] https://deckhouse.io/documentation/v1/modules/101-cert-manager/configuration.html
# [<ru>] Настройки модуля user-authn.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/101-cert-manager/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cert-manager
spec:
  version: 1
  enabled: true
---
# [<en>] documentation module settings.
# [<en>] https://deckhouse.io/documentation/v1/modules/810-documentation/configuration.html
# [<ru>] Настройки модуля documentation.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/810-documentation/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: documentation
spec:
  version: 1
  enabled: true
