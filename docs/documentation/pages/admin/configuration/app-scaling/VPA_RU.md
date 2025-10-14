---
title: "Вертикальное масштабирование"
permalink: ru/admin/configuration/app-scaling/vpa.html
description: "Настройка Vertical Pod Autoscaler (VPA) в Deckhouse Kubernetes Platform. Автоматическое управление ресурсами контейнеров и оптимизация на основе метрик использования для повышения эффективности."
lang: ru
---

## Как работает вертикальное масштабирование (VPA)

Vertical Pod Autoscaler (VPA) позволяет автоматизировать управление ресурсами контейнеров и значительно повысить эффективность работы приложений. VPA полезен в ситуациях, когда заранее неизвестно, сколько ресурсов потребуется приложению. При использовании VPA и включении соответствующего режима работы, запрашиваемые ресурсы устанавливаются на основе фактического потребления ресурсов, полученных [от системы мониторинга](../monitoring/). Также возможно настроить систему таким образом, чтобы она только рекомендовала ресурсы, не внося изменений автоматически.

Если нагрузка на приложение меняется в зависимости от времени суток, запросов пользователей или других факторов, VPA автоматически подстраивает выделенные ресурсы. Это позволяет избежать падений из-за нехватки ресурсов или чрезмерного расходования CPU и памяти.

## Режимы работы VPA

VPA может работать в двух режимах:

- Автоматическое изменение запросов ресурсов:
  - **Auto** (по умолчанию) —  изменяет ресурсы без пересоздания подов. В текущих версиях Kubernetes этот режим ведёт себя так же, как и **Recreate**: при необходимости изменения ресурсов VPA перезапускает под. Однако в будущем, с появлением [in-place обновления ресурсов](https://github.com/kubernetes/design-proposals-archive/blob/main/autoscaling/vertical-pod-autoscaler.md#in-place-updates), режим **Auto** будет использовать его — то есть изменять ресурсы без перезапуска пода.
  - **Recreate** — VPA может изменять ресурсы у работающих подов, перезапуская их. В случае одного пода (replicas: 1) это приведет к недоступности сервиса на время перезапуска. VPA не пересоздает поды, если они были созданы без контроллера.

- Только рекомендации, без изменения ресурсов:
  - **Initial** — ресурсы подов изменяются только при их создании, но не в процессе работы.
  - **Off** — VPA не меняет автоматически ресурсы. Однако, с его помощью можно просматривать рекомендуемые ресурсы с помощью команды `d8 k describe vpa`.

При использовании VPA и включении соответствующего режима, запрашиваемые ресурсы устанавливаются автоматически на основе данных из Prometheus. Также возможно настроить систему таким образом, чтобы она только рекомендовала ресурсы, но не изменяла их.

## Как включить или отключить VPA

Включить VPA можно следующими способами:

1. Через ресурс ModuleConfig (например, ModuleConfig/vertical-pod-autoscaler). Установите параметр `spec.enabled` в значение `true` или `false`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: vertical-pod-autoscaler
   spec:
     enabled: true
   ```

1. Через команду `d8` (в поде `d8-system/deckhouse`):

   ```console
   d8 platform module enable vertical-pod-autoscaler
   ```

1. Через [веб-интерфейс Deckhouse](/modules/console/):

   - Перейдите в раздел «Deckhouse - «Модули»;
   - Найдите модуль `vertical-pod-autoscaler` и нажмите на него;
   - Включите тумблер «Модуль включен».

У модуля [`vertical-pod-autoscaler`](/modules/vertical-pod-autoscaler/) нет обязательных настроек, то есть можно включить его и не настраивать дополнительно. При этом он будет работать со значениями по умолчанию.

После создания ресурса [VerticalPodAutoscaler](/modules/vertical-pod-autoscaler/cr.html#verticalpodautoscaler) посмотреть рекомендации VPA можно с помощью команды:

```console
d8 k describe vpa my-app-vpa
```

В секции `status` будут такие параметры:

- `Target` — рекомендуемое количество ресурсов для пода.
- `Lower Bound`— минимальное рекомендуемое количество ресурсов для приложения.
- `Upper Bound` — максимальное рекомендуемое количество ресурсов для приложения.
- `Uncapped Target`— значение, основанное на крайних метриках без учета истории.

## Настройка VPA

1. Создайте конфигурации модуля VPA.

   Для настройки VPA нужно создать файл конфигурации для модуля. Пример конфигурации:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: vertical-pod-autoscaler
   spec:
     version: 1
     enabled: true
     settings:
       nodeSelector:
         node-role/system: ""
       tolerations:
       - key: dedicated.deckhouse.io
         operator: Equal
         value: system
      ```

1. Примените файл конфигурации для VPA с помощью `d8 k apply -f <имя файла конфигурации>`.

Подробнее про настройку лимитов VPA [см. в разделе Использование](../../../user/configuration/app-scaling/vpa.html#работа-vpa-с-лимитами).

### Пример настройки модуля

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: vertical-pod-autoscaler
spec:
  version: 1
  enabled: true
  settings:
    nodeSelector:
      node-role/system: ""
    tolerations:
    - key: dedicated.deckhouse.io
      operator: Equal
      value: system
```
