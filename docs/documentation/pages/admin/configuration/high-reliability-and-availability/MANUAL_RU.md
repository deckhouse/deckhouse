---
title: Ручное управление режимом HA для определённых модулей
permalink: ru/admin/high-reliability-and-availability/manual.html
description: Ручное управление режимом HA для определённых модулей
lang: ru
---

Некоторые модули, входящие в DKP, могут иметь свои настройки режима HA. Для них можно выставить параметр `settings.highAvailability` в настройках самого модуля вне зависимости от влюченного или выключенного глобального режима HA.

Перечень модулей, для которых доступно управление режимом HA:

* deckhouse;
* openvpn;
* istio;
* dashboard;
* multitenancy-manager;
* user-authn;
* ingress-nginx;
* Prometheus-монитонинг;
* monitoring-kubernetes;
* snapshot-controller.

Например, чтобы вручную включить режим HA для модуля deckhouse добавьте в его конфигурацию параметр `settings.highAvailability`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    highAvailability: true
...
```

Убедиться, что режим включился, можно, посмотрев количество подов выбранного модуля. Например, для `deckhouse` посмотрите в пространстве имён `d8-system` командой:

```text
$ sudo -i d8 k -n d8-system get po | grep deckhouse
```

Количество подов deckhouse должно быть больше одного:

```text
deckhouse-57695f4d68-8rk6l                           2/2     Running   0             3m49s
deckhouse-5764gfud68-76dsb                           2/2     Running   0             3m49s
deckhouse-fgrhy4536s-fhu6s                           2/2     Running   0             3m49s
```
