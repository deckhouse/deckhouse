---
title: Управление режимом HA
permalink: ru/admin/high-reliability-and-availability/enable.html
description: Управление режимом HA
lang: ru
---

{% alert level="info" %}
Обратите внимание, что если в кластере **более одного master-узла**, режим HA **включается автоматически**. Это правило верно как при развёртывании кластера сразу с тремя master-узлами, так и при увеличении количества master-узлов с одного до трёх.
{% endalert %}

Включить режим HA глобально для DKP можно одним из следующих способов:

-  Установите в `ModuleConfig/global` параметр `settings.highAvailability` в значение `true`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: global
   spec:
     version: 2
     settings: 
       highAvailability: true
   ...
   ```
   
   Чтобы убедиться, что режим включился, можно, например, проверить количество подов `deckhouse` в пространстве имён `d8-system`. Для этого выполните команду:
   
   ```shell
   sudo -i d8 k -n d8-system get po | grep deckhouse
   ```
   
   Количество подов `deckhouse` должно быть больше одного:
   
   ```text
   deckhouse-57695f4d68-8rk6l                           2/2     Running   0             3m49s
   deckhouse-5764gfud68-76dsb                           2/2     Running   0             3m49s
   deckhouse-fgrhy4536s-fhu6s                           2/2     Running   0             3m49s
   ```

- Если в кластере включен модуль [`console`](/products/kubernetes-platform/modules/console/stable/), откройте веб-интерфейс Deckhouse, перейдите в раздел «Deckhouse» — «Глобальные настройки» — «Глобальные настройки модулей» и установите переключатель «Режим отказоустойчивости» в положение «Да».
   

## Включение режима HA для отдельных компонентов

Некоторые модули DKP могут иметь собственные настройки режима HA. Чтобы включить режим высокой надежности в конкретном модуле, установите параметр `settings.highAvailability` в его настройках. При этом работа режима HA в отдельных модулях не зависит от состояния глобального режима HA.

Перечень модулей, для которых доступно управление режимом HA:

* `deckhouse`;
* `openvpn`;
* `istio`;
* `dashboard`;
* `multitenancy-manager`;
* `user-authn`;
* `ingress-nginx`;
* `prometheus-monitoring`;
* `monitoring-kubernetes`;
* `snapshot-controller`.

Например, чтобы вручную включить режим HA для модуля `deckhouse`, добавьте в его конфигурацию параметр `settings.highAvailability`:

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

Чтобы убедиться, что режим включился, проверьте количество подов выбранного модуля. Например, для проверки работы режима в модуле `deckhouse`, проверьте количество подов в пространстве имён `d8-system`, выполнив следующую команду:

```shell
sudo -i d8 k -n d8-system get po | grep deckhouse
```

Количество подов `deckhouse` должно быть больше одного:

```text
deckhouse-57695f4d68-8rk6l                           2/2     Running   0             3m49s
deckhouse-5764gfud68-76dsb                           2/2     Running   0             3m49s
deckhouse-fgrhy4536s-fhu6s                           2/2     Running   0             3m49s
```
