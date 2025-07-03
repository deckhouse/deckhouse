---
title: "Глобальные настройки"
permalink: ru/deckhouse-configure-global.html
description: "Описание глобальных настроек Deckhouse Kubernetes Platform"
module-kebab-name: "global"
lang: ru
---

Глобальные настройки Deckhouse хранятся в ресурсе `ModuleConfig/global` (см. [конфигурация Deckhouse](./#конфигурация-deckhouse)).

{% alert %}
В параметре [publicDomainTemplate](#parameters-modules-publicdomaintemplate) указывается шаблон DNS-имен, с учётом которого некоторые модули Deckhouse создают Ingress-ресурсы. Если параметр не указан, Ingress-ресурсы создаваться не будут.

Если у вас нет возможности заводить wildcard-записи DNS, для тестирования можно воспользоваться сервисом [sslip.io](https://sslip.io) или его аналогами.

Домен, указанный в шаблоне, не может совпадать или быть поддоменом домена, заданного в параметре [`clusterDomain`](./installing/configuration.html#clusterconfiguration-clusterdomain). Мы не рекомендуем менять значение `clusterDomain` без особой необходимости.

Для корректной работы шаблона необходимо предварительно настроить службы DNS как в сетях, где будут располагаться узлы кластера, так и в сетях, из которых к служебным веб-интерфейсам платформы будут обращаться клиенты.

В случае, если шаблон совпадает с доменом сети узлов, используйте только А записи для назначения служебным веб-интерфейсам платформы адресов Frontend узлов. Например, для узлов заведена зона `company.my`, а шаблон имеет вид `%s.company.my`.
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

{% include module-conversion.liquid %}

## Параметры

{{ site.data.schemas.global.config-values | format_module_configuration: "global" }}
