---
title: "Глобальные настройки"
permalink: ru/deckhouse-configure-global.html
description: "Описание глобальных настроек Deckhouse Kubernetes Platform"
lang: ru
---

Глобальные настройки Deckhouse хранятся в ресурсе `ModuleConfig/global` (см. [конфигурация Deckhouse](./#конфигурация-deckhouse)).

{% alert %}
В параметре [publicDomainTemplate](#parameters-modules-publicdomaintemplate) указывается шаблон DNS-имен, с учётом которого некоторые модули Deckhouse создают Ingress-ресурсы.

Если у вас нет возможности заводить wildcard-записи DNS, для тестирования можно воспользоваться сервисом [sslip.io](https://sslip.io) или его аналогами.

Домен, указанный в шаблоне, не должен совпадать с доменом, заданным в параметре [clusterDomain](installing/configuration.html#clusterconfiguration-clusterdomain), а также с доменом внутренней сервисной зоны сети.  
Например, если `clusterDomain` установлен в `cluster.local`, а внутренняя зона — `ru-central1.internal`, то publicDomainTemplate не может быть ни `%s.cluster.local`, ни `%s.ru-central1.internal`.
{% endalert %}

Пример ресурса `ModuleConfig/global`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 2
  settings: # <-- Параметры модуля из раздела "Параметры" ниже.
    defaultClusterStorageClass: 'default-fast'
    modules:
      publicDomainTemplate: '%s.kube.company.my'
      resourcesRequests:
        controlPlane:
          cpu: 1000m
          memory: 500M
      placement:
        customTolerationKeys:
        - dedicated.example.com
      storageClass: 'default-fast'
```

## Параметры

{{ site.data.schemas.global.config-values | format_module_configuration: "global" }}
