---
title: "Глобальные настройки"
permalink: ru/deckhouse-configure-global.html
lang: ru
---

Глобальные настройки Deckhouse хранятся в ресурсе `ModuleConfig/global` (см. [конфигурация Deckhouse](./#конфигурация-deckhouse)).

{% alert %}
В параметре [publicDomainTemplate](#parameters-modules-publicdomaintemplate) указывается шаблон DNS-имен, с учетом которого некоторые модули Deckhouse создают Ingress-ресурсы.

Если у вас нет возможности заводить wildcard-записи DNS, для тестирования можно воспользоваться сервисом [sslip.io](https://sslip.io) или его аналогами.

Домен, используемый в шаблоне, не должен совпадать с доменом, указанным в параметре [clusterDomain](installing/configuration.html#clusterconfiguration-clusterdomain). Например, если `clusterDomain` установлен в `cluster.local` (значение по умолчанию), то `publicDomainTemplate` не может быть `%s.cluster.local`.
{% endalert %}

Пример ресурса `ModuleConfig/global`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings: # <-- Параметры модуля из раздела "Параметры" ниже.
    modules:
      publicDomainTemplate: '%s.kube.company.my'
      resourcesRequests:
        controlPlane:
          cpu: 1000m
          memory: 500M      
      placement:
        customTolerationKeys:
        - dedicated.example.com
    storageClass: sc-fast
```

## Параметры

{{ site.data.schemas.global.config-values | format_module_configuration: "global" }}
