---
title: "Модуль cni-cilium"
---

Обеспечивает работу сети в кластере с помощью модуля [cilium](https://cilium.io/).

## Ограничения

1. Модуль не поддерживает туннелирование.
2. Сервисы с типом `NodePort` и `LoadBalancer` не работают с hostNetwork-эндпоинтами в LB режиме `DSR`. Переключитесь на режим `SNAT`, если это требуется.
3. `HostPort` поды биндятся только [к одному IP](https://github.com/deckhouse/deckhouse/issues/3035). Если в ОС есть несколько интерфейсов/IP, Cilium выберет один из них, предпочитания "серые" IP адреса "белым".
4. Поддержка версий ОС. `cni-cilium` работает только с Linux ядрами >= `5.3`.
   * Ubuntu
     * 18.04
     * 20.04
     * 22.04
   * Debian
     * 11
   * CentOS
     * 7 (необходимо новое ядро из [репозитория](http://elrepo.org))
     * 8 (необходимо новое ядро из [репозитория](http://elrepo.org))

## Заметка о CiliumClusterwideNetworkPolicies

1. Убедитесь, что вы применили первичный набор объектов `CiliumClusterwideNetworkPolicy`, поставив конфигурационную опцию `policyAuditMode` в `true`.
   Отсутствие опции может привести к некорректной работе control plane или потере доступа ко всем узлам кластера по SSH.
   Вы можете удалить опцию после применения всех `CiliumClusterwideNetworkPolicy` объектов и проверке корректности их работы в Hubble UI.
2. Убедитесь, что вы применили следующее правило. В противном случае control plane может некорректно работать до одной минуты во время перезагрузи `cilium-agent` Pod'ов. Это происходит из-за [сброса conntrack таблицы](https://github.com/cilium/cilium/issues/19367). Привязка к entity `kube-apiserver` позволяет "обойти" баг.

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

При смене режима работы Cilium (параметр `tunnelMode`) c `Disabled` на `VXLAN` или обратно, необходимо перезагрузить все узлы, иначе возможны проблемы с доступностью Pod'ов.
