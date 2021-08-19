---
title: "Модуль ingress-nginx: настройки"
---

Модуль по умолчанию **включен** в кластерах начиная с версии 1.14. Для выключения добавьте в CM `d8-system/deckhouse`:
```yaml
ingressNginxEnabled: "false"
```

## Параметры

<!-- SCHEMA -->

Конфигурация Ingress-контроллеров выполняется с помощью Custom Resource [IngressNginxController](cr.html#ingressnginxcontroller).
