---
title: Интеграция с облаком встроенной виртуализации
permalink: ru/admin/integrations/virtualization/dvp/services.html
lang: ru
---

Deckhouse Platform интегрируется с инфраструктурой встроенной виртуализации и использует ресурсы [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) для описания характеристик виртуальных машин, создаваемых в составе кластера.

Основные возможности:

- Управление ресурсами виртуализации через модуль `cloud-controller-manager`;
- Заказ дисков с использованием компонента CSI storage;
- Интеграция с модулем [`node-manager`](/modules/node-manager/) для поддержки DVPInstanceClass при описании [NodeGroup](/modules/node-manager/cr.html#nodegroup).

{% alert level="info" %}
Интеграция с виртуализацией включается автоматически для всех облачных кластеров, развёрнутых во встроенной виртуализации.
Дополнительная настройка не требуется.
{% endalert %}

## Балансировщики нагрузки

Встроенная виртуализация поддерживает объекты Service с типом `LoadBalancer`. Для каждого такого объекта [`cloud-controller-manager`](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-dvp.html#взаимодействия-модуля) создаёт Service с типом `LoadBalancer` в родительском кластере и направляет трафик на узлы дочернего. Объект создаётся в неймспейсе, который задан параметром [`namespace`](/modules/cloud-provider-dvp/configuration.html#parameters-provider-parameters-namespace), рядом с виртуальными машинами кластера.

Внешний адрес назначает родительский кластер, поэтому в нём должен работать контроллер балансировщиков, например модуль [`metallb`](/modules/metallb/). Контроллер обслуживает только те объекты Service, для которых определён класс балансировщика.

Чтобы задать класс по умолчанию, создайте в родительском кластере ресурс [MetalLoadBalancerClass](/modules/metallb/cr.html#metalloadbalancerclass) с параметром [`isDefault`](/modules/metallb/cr.html#metalloadbalancerclass-v1alpha1-spec-isdefault) в значении `true`. Такой класс обслуживает объекты Service, в которых класс не указан, поэтому в дочернем кластере класс можно не указывать.

Чтобы выбрать другой класс, укажите его в параметре [`spec.loadBalancerClass`](https://kubernetes.io/docs/concepts/services-networking/service/#load-balancer-class) объекта Service дочернего кластера. Платформа переносит значение в родительский кластер вместе с параметром [`externalIPs`](https://kubernetes.io/docs/concepts/services-networking/service/#external-ips) и устаревшим параметром `loadBalancerIP`.

{% alert level="warning" %}
Параметр [`spec.loadBalancerClass`](https://kubernetes.io/docs/concepts/services-networking/service/#load-balancer-class) неизменяем. Задайте класс при создании объекта Service, иначе объект придётся пересоздать.
{% endalert %}

Без класса балансировщика объект Service остаётся в состоянии `Pending`, при этом `cloud-controller-manager` не выводит ошибок. Проверьте состояние объекта в родительском кластере:

```shell
d8 k -n <NAMESPACE> get svc
```

Замените `<NAMESPACE>` на неймспейс, заданный параметром [`namespace`](/modules/cloud-provider-dvp/configuration.html#parameters-provider-parameters-namespace).
