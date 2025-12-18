---
title: "Модуль ingress-nginx: настройки"
---

{% alert level="info" %}
Если модуль был выключен и необходимо его включить, обратите внимание на глобальный параметр [publicDomainTemplate](/products/kubernetes-platform/documentation/v1/reference/api/global.html#параметры). Укажите его, если он не указан, иначе Ingress-ресурсы для служебных компонентов DKP (dashboard, user-auth, grafana, upmeter  и т. п.) не будут созданы.
{% endalert %}

Конфигурация Ingress-контроллеров выполняется с помощью Custom Resource [IngressNginxController](cr.html#ingressnginxcontroller).

<!-- SCHEMA -->
