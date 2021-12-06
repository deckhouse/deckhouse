---
title: "Модуль ingress-nginx: настройки"
---

Модуль по умолчанию **включен** в кластерах начиная с версии 1.14. Для выключения добавьте в CM `d8-system/deckhouse`:
```yaml
ingressNginxEnabled: "false"
```
> Если модуль был выключен и вы его включаете, то обратите внимание на глобальный параметр [publicDomainTemplate](../../deckhouse-configure-global.html#параметры). Укажите его, если он не указан, иначе Ingress-ресурсы для служебных компонент Deckhouse (dashboard, user-auth, grafana, upmeter  и т.п) создаваться не будут.

## Параметры

<!-- SCHEMA -->

Конфигурация Ingress-контроллеров выполняется с помощью Custom Resource [IngressNginxController](cr.html#ingressnginxcontroller).
