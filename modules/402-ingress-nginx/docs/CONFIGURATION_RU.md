---
title: "Модуль ingress-nginx: настройки"
---

> Если модуль был выключен и необходимо его включить, обратите внимание на глобальный параметр [publicDomainTemplate](../../deckhouse-configure-global.html#параметры). Если он не был указан необходимо его указать. В противном случае, Ingress-ресурсы для служебных компонентов DKP (dashboard, user-auth, grafana, upmeter и т.д.) не будут созданы.

Конфигурация Ingress-контроллеров выполняется с помощью Custom Resource [IngressNginxController](cr.html#ingressnginxcontroller).

<!-- SCHEMA -->
