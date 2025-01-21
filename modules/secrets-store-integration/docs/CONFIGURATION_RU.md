---
title: "Модуль secrets-store-integration: настройки"
---

> Если модуль был выключен и вы его включаете, обратите внимание на глобальный параметр [publicDomainTemplate](../../deckhouse-configure-global.html#параметры). Укажите его, если он не указан, иначе Ingress-ресурсы для служебных компонентов Deckhouse (dashboard, user-auth, grafana, upmeter  и т. п.) создаваться не будут.

Конфигурация Ingress-контроллеров выполняется с помощью Custom Resource [IngressNginxController](cr.html#ingressnginxcontroller).

<!-- SCHEMA -->
