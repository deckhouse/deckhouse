apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    bundle: Minimal
    # [<ru>] Канал обновлений Deckhouse. Канал Early Access достаточно стабилен, его можно использовать в продуктивных окружениях.
    # [<ru>] Если планируется использовать несколько кластеров, то рекомендуется установить на них разные каналы обновлений.
    # [<ru>] Подробнее: https://deckhouse.ru/documentation/v1/deckhouse-release-channels.html
    # [<en>] Deckhouse release channel. The Early Access channel is stable enough to be used in productive environments.
    # [<en>] If you plan to use several clusters, it is recommended to use different release channels on them.
    # [<en>] More info: https://deckhouse.io/documentation/v1/deckhouse-release-channels.html
    releaseChannel: EarlyAccess
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
      # [<en>] Template that will be used for system apps domains within the cluster.
      # [<en>] E.g., Grafana for %s.example.com will be available as 'grafana.example.com'.
      # [<en>] You can change it to your own or follow the steps in the guide and change it after installation.
      # [<ru>] Шаблон, который будет использоваться для составления адресов системных приложений в кластере.
      # [<ru>] Например, Grafana для %s.example.com будет доступна на домене 'grafana.example.com'.
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
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cert-manager
spec:
  version: 1
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: documentation
spec:
  version: 1
  enabled: true
