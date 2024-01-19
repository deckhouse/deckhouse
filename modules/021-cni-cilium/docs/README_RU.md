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
   * Для работы модуля `cni-cilium` совместно с модулем [istio](../110-istio/), [openvpn](../500-openvpn/) или [node-local-dns]({% if site.d8Revision == 'CE' %}{{ site.urls.ru}}/documentation/v1/modules/{% else %}..{% endif %}/350-node-local-dns/) необходимо ядро Linux версии >= `5.7`.
4. Проблемы совместимости с ОС:
   * Ubuntu:
     * не работоспособно на 18.04
     * для работы на 20.04 необходима установка ядра HWE
   * Astra Linux:
     * не работоспособно на издании "Смоленск"
   * CentOS:
     * 7 (необходимо новое ядро из [репозитория](http://elrepo.org))
     * 8 (необходимо новое ядро из [репозитория](http://elrepo.org))

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
