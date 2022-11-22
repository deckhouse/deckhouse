---
title: "Модуль ingress-nginx: настройки"
---

{% include module-bundle.liquid %}

> Если модуль был выключен и вы его включаете, то обратите внимание на глобальный параметр [publicDomainTemplate](../../deckhouse-configure-global.html#параметры). Укажите его, если он не указан, иначе Ingress-ресурсы для служебных компонент Deckhouse (dashboard, user-auth, grafana, upmeter  и т.п) создаваться не будут.

## Параметры

<!-- SCHEMA -->

Конфигурация Ingress-контроллеров выполняется с помощью Custom Resource [IngressNginxController](cr.html#ingressnginxcontroller).
