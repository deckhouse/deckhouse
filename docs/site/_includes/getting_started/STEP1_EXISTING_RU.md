-   Организуйте SSH-доступ между машиной, с которой будет производиться установка, и существующим master-узлом кластера.
-   Определите параметры для Deckhouse, создав новый конфигурационный файл `config.yml`:

{% offtopic title="config.yml для CE" %}
```yaml
# секция первичной инициализации Deckhouse (InitConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: InitConfiguration
# конфигурация Deckhouse
deckhouse:
  # адрес реестра с образом инсталлятора; указано значение по умолчанию для CE-сборки Deckhouse
  imagesRepo: registry.deckhouse.io/deckhouse/ce
  # строка с параметрами подключения к Docker registry
  registryDockerCfg: eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5pbyI6IHt9fX0=
  # используемый канал обновлений
  releaseChannel: Beta
  configOverrides:
    deckhouse:
      bundle: Minimal
    global:
      # имя кластера; используется, например, в лейблах алертов Prometheus
      clusterName: main
      # имя проекта; используется для тех же целей
      project: someproject
      modules:
        # шаблон, который будет использоваться для составления адресов системных приложений в кластере
        # например, Grafana для %s.somedomain.com будет доступна на домене grafana.somedomain.com
        publicDomainTemplate: "%s.somedomain.com"
```
{% endofftopic %}
{% offtopic title="config.yml для EE" %}
```yaml
# секция первичной инициализации Deckhouse (InitConfiguration)
# используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# тип секции конфигурации
kind: InitConfiguration
# конфигурация Deckhouse
deckhouse:
  # адрес реестра с образом инсталлятора; указано значение по умолчанию для EE-сборки Deckhouse
  imagesRepo: registry.deckhouse.io/deckhouse/ee
  # строка с ключом для доступа к Docker registry (сгенерировано автоматически для вашего демонстрационного токена)
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
  # используемый канал обновлений
  releaseChannel: Beta
  configOverrides:
    deckhouse:
      bundle: Minimal
    global:
      # имя кластера; используется, например, в лейблах алертов Prometheus
      clusterName: main
      # имя проекта; используется для тех же целей
      project: someproject
      modules:
        # шаблон, который будет использоваться для составления адресов системных приложений в кластере
        # например, Grafana для %s.somedomain.com будет доступна на домене grafana.somedomain.com
        publicDomainTemplate: "%s.somedomain.com"
```
{% endofftopic %}
