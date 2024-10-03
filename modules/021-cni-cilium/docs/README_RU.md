---
title: "Модуль cni-cilium"
description: Модуль cni-cilium Deckhouse обеспечивает работу сети в кластере Kubernetes с помощью Cilium.
---

Обеспечивает работу сети в кластере с помощью модуля [cilium](https://cilium.io/).

## Ограничения

1. Сервисы с типом `NodePort` и `LoadBalancer` не работают с hostNetwork-эндпоинтами в LB-режиме `DSR`. Переключитесь на режим `SNAT`, если это требуется.
2. `HostPort` поды биндятся только [к одному IP](https://github.com/deckhouse/deckhouse/issues/3035). Если в ОС есть несколько интерфейсов/IP, Cilium выберет один из них, предпочитая «серые» IP-адреса «белым».
3. Требования к ядру:
   * Для работы модуля `cni-cilium` необходимо ядро Linux версии >= `5.7`.
   * Для работы модуля `cni-cilium` совместно с модулем [istio](../110-istio/), [openvpn](../500-openvpn/) или [node-local-dns]({% if site.d8Revision == 'CE' %}{{ site.urls.ru}}/products/kubernetes-platform/documentation/v1/modules/{% else %}..{% endif %}/350-node-local-dns/) необходимо ядро Linux версии >= `5.7`.
4. Проблемы совместимости с ОС:
   * Ubuntu:
     * не работоспособно на 18.04
     * для работы на 20.04 необходима установка ядра HWE
   * Astra Linux:
     * не работоспособно на издании "Смоленск"
   * CentOS:
     * 7 (необходимо новое ядро из [репозитория](http://elrepo.org))
     * 8 (необходимо новое ядро из [репозитория](http://elrepo.org))

## Заметка о `Service` с типом `NodePort` и `LoadBalancer`
В модуле возможен [выбор режима работы](./configuration.html#parameters-bpflbmode), влияющий на поведение `Service` с типом `NodePort` и `LoadBalancer`:

* `DSR` (Direct Server Return) -  трафик от клиента до пода проходит с сохранением адреса отправителя, а обратно - согласно правилам роутинга (минуя балансировщик). Этот режим экономит сетевой трафик, уменьшает задержки, но работает только для TCP трафика.
* `SNAT` (Source Network Address Translation) -  переводит трафик через себя, меняя исходный IP-адрес, чтобы приложения видели трафик, как если бы он пришел напрямую от балансировщика нагрузки соответственно теряется адрес отправителя
* `Hybrid` - TCP трафик обрабатывается в режиме `DSR`, а UDP - в режиме `SNAT`.

При создании `Service` с типом `NodePort` и `LoadBalancer`, также следует учитывать параметр `externalTrafficPolicy`, напрямую связанный с режимом работы Cilium:
* `externalTrafficPolicy: Cluster` (значение по умолчанию)  - весь входящий трафик на `NodePort` или `LoadBalancer` будет приниматься любым узлом в кластере, независимо от того, на каком поде находится целевое приложение. Если целевой под не находится на том же узле, трафик будет перенаправлен на нужный узел. 
Дальнейшее поведение зависит от настроек модуля:  
  * В случае использования модуля в режиме `SNAT`, исходный IP клиента не сохранится, так как будет изменен на IP узла. 
  * В случае использования модуля в режиме `DSR` или `Hybrid`, исходный IP сохраняется, но требуется чтобы на узле, обрабатывающим запрос, был интерфейс, на котором будет доступен ip-адрес отправителя, для формирования ответа(т.е. если трафик приходит с интерфейса с "белым"-IP, то на конечной ноде, обрабатывающей запрос, также должен присутствовать интерфейс с "белым"-IP)
* `externalTrafficPolicy: Local` - входящий трафик будет приниматься только теми узлами, на которых запущен целевой под. Если целевой под не запущен на конкретном узле, весь трафик к этому узлу будет отбрасываться.

## Заметка о CiliumClusterwideNetworkPolicies

1. Убедитесь, что вы применили первичный набор объектов `CiliumClusterwideNetworkPolicy`, поставив конфигурационную опцию `policyAuditMode` в `true`.
   Отсутствие опции может привести к некорректной работе control plane или потере доступа ко всем узлам кластера по SSH.
   Вы можете удалить опцию после применения всех `CiliumClusterwideNetworkPolicy`-объектов и проверки корректности их работы в Hubble UI.
2. Убедитесь, что вы применили следующее правило. В противном случае control plane может некорректно работать до одной минуты во время перезагрузки `cilium-agent`-подов. Это происходит из-за [сброса conntrack таблицы](https://github.com/cilium/cilium/issues/19367). Привязка к entity `kube-apiserver` позволяет обойти баг.

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

## Заметка о смене режима работы Cilium

При смене режима работы Cilium (параметр [tunnelMode](configuration.html#parameters-tunnelmode)) c `Disabled` на `VXLAN` или обратно необходимо перезагрузить все узлы, иначе возможны проблемы с доступностью подов.

## Заметка о выключении модуля kube-proxy

Cilium полностью заменяет собой функционал модуля kube-proxy, поэтому тот автоматически отключается при включении модуля cni-cilium.

## Заметка об отказоустойчивом Egress Gateway

{% alert level="warning" %} Функция доступна только в Enterprise Edition {% endalert %}

### Базовый режим

Используются предварительно настроенные IP-адреса на egress-узлах.

<div data-presentation="../../presentations/021-cni-cilium/egressgateway_base_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/12l4w9ZS3Hpax1B7eOptm2dQX55VVAFzRTtyihw4Ie0c/ --->

### Режим с Virtual IP

Позволяет динамически назначать дополнительные IP-адреса узлам.

<div data-presentation="../../presentations/021-cni-cilium/egressgateway_virtualip_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1tmhbydjpCwhNVist9RT6jzO1CMpc-G1I7rczmdLzV8E/ --->
