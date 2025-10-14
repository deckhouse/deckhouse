---
title: "Глобальные настройки"
permalink: ru/reference/api/global.html
description: "Описание глобальных настроек Deckhouse Platform Certified Security Edition"
module-kebab-name: global
lang: ru
search: global settings, global configuration, platform settings, default settings, global parameters, глобальные настройки, глобальная конфигурация, настройки платформы, настройки по умолчанию, глобальные параметры
---

Глобальные настройки Deckhouse Platform Certified Security Edition позволяют вам настраивать параметры, которые используются по умолчанию всеми модулями и компонентами. Некоторые модули могут переопределять часть этих параметров (это можно узнать в разделе настройки соответствующего модуля в документации модуля).

Глобальные настройки Deckhouse хранятся в ModuleConfig `global`.

{% alert %}
В параметре [publicDomainTemplate](#parameters-modules-publicdomaintemplate) указывается шаблон DNS-имен, с учётом которого некоторые модули Deckhouse создают Ingress-ресурсы. Если параметр не указан, Ingress-ресурсы создаваться не будут.

Если у вас нет возможности заводить wildcard-записи DNS, для тестирования можно воспользоваться сервисом [sslip.io](https://sslip.io) или его аналогами.

Домен, указанный в шаблоне, не может совпадать или быть поддоменом домена, заданного в параметре [`clusterDomain`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain). Мы не рекомендуем менять значение `clusterDomain` без особой необходимости.

Для корректной работы шаблона необходимо предварительно настроить службы DNS как в сетях, где будут располагаться узлы кластера, так и в сетях, из которых к служебным веб-интерфейсам платформы будут обращаться клиенты.

В случае, если шаблон совпадает с доменом сети узлов, используйте только А записи для назначения служебным веб-интерфейсам платформы адресов Frontend узлов. Например, для узлов заведена зона `company.my`, а шаблон имеет вид `%s.company.my`.
{% endalert %}

<div>
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

{{ site.data.schemas.modules.global.config-values | format_module_configuration: "global" }}
