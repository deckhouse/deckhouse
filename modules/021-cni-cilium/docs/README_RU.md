---
title: "Модуль cni-cilium"
description: Модуль cni-cilium Deckhouse обеспечивает работу сети в кластере Kubernetes с помощью Cilium.
---

Модуль `cni-cilium` обеспечивает работу сети в кластере. Основан на проекте [Cilium](https://cilium.io/).

## Ограничения

1. Сервисы с типом `NodePort` и `LoadBalancer` несовместимы с hostNetwork-эндпойнтами в LB-режиме `DSR`. Переключитесь на режим `SNAT`, если это требуется.
2. `HostPort` поды связываются только [с одним IP-адресом](https://github.com/deckhouse/deckhouse/issues/3035). Если в ОС есть несколько интерфейсов/IP, Cilium выберет один, предпочитая «серые» «белым».
3. Требования к ядру:
   * ядро Linux версии не ниже `5.7` для работы модуля `cni-cilium` и его совместной работы с модулями [istio](../110-istio/), [openvpn](../500-openvpn/), [node-local-dns]({% if site.d8Revision == 'CE' %}{{ site.urls.ru}}/products/kubernetes-platform/documentation/v1/modules/{% else %}..{% endif %}/350-node-local-dns/).
4. Совместимость с ОС:
   * Ubuntu:
     * несовместим с версией 18.04;
     * для работы с версией 20.04 необходима установка ядра HWE.
   * Astra Linux:
     * несовместим с изданием «Смоленск».
   * CentOS:
     * для версий 7 и 8 необходимо новое ядро из [репозитория](http://elrepo.org).

## Обработка внешнего трафика в разных режимах работы `bpfLB` (замена kube-proxy от Cilium)

В Kubernetes обычно используются схемы, где трафик приходит на балансировщик, который распределяет его между многими серверами. Через балансировщик проходят и входящий, и исходящий трафики. Таким образом, общая пропускная способность ограничена ресурсами и шириной канала балансировщика. Для оптимизации трафика и разгрузки балансировщика и был придуман механизм `DSR`, в котором входящие пакеты проходят через балансировщик, а исходящие идут напрямую с терминирующих серверов. Так как обычно ответы имеют много больший размер чем запросы, то такой подход позволяет значительно увеличить общую пропускную способность схемы.

В модуле возможен [выбор режима работы](../configuration.html#parameters-bpflbmode), влияющий на поведение `Service` с типом `NodePort` и `LoadBalancer`:

* `SNAT` (Source Network Address Translation) — один из подвидов NAT, при котором для каждого исходящего пакета происходит трансляция IP-адреса источника в IP-адрес шлюза из целевой подсети, а входящие пакеты, проходящие через шлюз, транслируются обратно на основе таблицы трансляций. В этом режиме `bpfLB` полностью повторяет логику работы `kube-proxy`:
  * если в `Service` указан `externalTrafficPolicy: Local`, то трафик будет передаваться и балансироваться только в те целевые поды, которые запущены на том же узле, на который этот трафик пришел. Если целевой под не запущен на этом узле, то трафик будет отброшен.
  * если в `Service` указан `externalTrafficPolicy: Cluster`, то трафик будет передаваться и балансироваться во все целевые поды в кластере. При этом, если целевые поды находятся на других узлах, то при передаче трафика на них будет произведен SNAT (IP-адрес источника будет заменен на InternalIP узла).

   ![Схема потоков данных SNAT](../../images/021-cni-cilium/snat.png)

* `DSR` - (Direct Server Return) — метод, при котором весь входящий трафик проходит через балансировщик нагрузки, а весь исходящий трафик обходит его. Такой метод используется вместо `SNAT`. Часто ответы имеют много больший размер чем запросы и `DSR` позволяет значительно увеличить общую пропускную способность схемы:
  * если в `Service` указан `externalTrafficPolicy: Local`, то поведение абсолютно аналогично `kube-proxy` и `bpfLB` в режиме `SNAT`.
  * если в `Service` указан `externalTrafficPolicy: Cluster`, то трафик так же будет передаваться и балансироваться во все целевые поды в кластере.  
  При этом важно учитывать следующие особенности:
    * если целевые поды находятся на других узлах, то при передаче на них входящего трафика будет сохранен IP-адрес источника;
    * исходящий трафик пойдет прямо с узла, на котором был запущен целевой под;
    * IP-адрес источника будет заменен на внешний IP-адрес ноды, на которую изначально пришел входящий запрос.

   ![Схема потоков данных DSR](../../images/021-cni-cilium/dsr.png)

{% alert level="warning" %}
В случае использования режима `DSR` и `Service` с `externalTrafficPolicy: Cluster` требуются дополнительные настройки сетевого окружения.
Сетевое оборудование должно быть готово к ассиметричному прохождению трафика: отключены или настроены соответствующим образом средства фильтрации IP адресов на входе в сеть (`uRPF`, `sourceGuard` и т.п.).
{% endalert %}

* `Hybrid` — в данном режиме TCP-трафик обрабатывается в режиме `DSR`, а UDP — в режиме `SNAT`.

## Использование CiliumClusterwideNetworkPolicies

Для использования CiliumClusterwideNetworkPolicies следует применить:

1. Первичный набор объектов `CiliumClusterwideNetworkPolicy`, поставив конфигурационную опцию `policyAuditMode` в `true`. Отсутствие опции может привести к некорректной работе Control plane или потере доступа ко всем узлам кластера по SSH. Опция может быть удалена после применения всех `CniliumClusterwideNetworkPolicy`-объектов и проверки корректности их работы в Hubble UI.
2. Правило политики сетевой безопасности:

   ```yaml
   apiVersion: "cilium.io/v2"
   kind: CiliumClusterwideNetworkPolicy
   metadata:
     name: "allow-control-plane-connectivity"
   spec:
     ingress:
     - fromEntities:
       - kube-apiserver
     nodeSelector:
       matchLabels:
         node-role.kubernetes.io/control-plane: ""
   ```

В случае, если CiliumClusterwideNetworkPolicies не будут использованы, Control plane может некорректно работать до одной минуты во время перезагрузки `cilium-agent`-подов. Это происходит из-за [сброса Conntrack-таблицы](https://github.com/cilium/cilium/issues/19367). Привязка к entity `kube-apiserver` позволяет обойти баг.

## Смена режима работы Cilium

При смене режима работы Cilium (параметр [tunnelMode](configuration.html#parameters-tunnelmode)) c `Disabled` на `VXLAN` или обратно, необходимо перезагрузить все узлы, иначе возможны проблемы с доступностью подов.

## Выключение модуля kube-proxy

Cilium полностью заменяет собой функционал модуля `kube-proxy`, поэтому `kube-proxy` автоматически отключается при включении модуля `cni-cilium`.

## Использование Egress Gateway

{% alert level="warning" %} Функция доступна только в Enterprise Edition {% endalert %}

### Базовый режим

Используются предварительно настроенные IP-адреса на egress-узлах.

<div data-presentation="../../presentations/021-cni-cilium/egressgateway_base_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/12l4w9ZS3Hpax1B7eOptm2dQX55VVAFzRTtyihw4Ie0c/ --->

### Режим с Virtual IP

Реализована возможность динамически назначать дополнительные IP-адреса узлам.

<div data-presentation="../../presentations/021-cni-cilium/egressgateway_virtualip_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1tmhbydjpCwhNVist9RT6jzO1CMpc-G1I7rczmdLzV8E/ --->
