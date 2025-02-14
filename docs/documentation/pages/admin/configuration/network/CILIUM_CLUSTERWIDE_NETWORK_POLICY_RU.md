---
title: "Определение сетевых политик на уровне всего кластера (CiliumClusterwideNetworkPolicy)"
permalink: ru/admin/network/cilium-clusterwide-network-policy.html
lang: ru
---

Для определения сетевых политик на уровне всего кластера в Deckhouse Kubernetes Platform можно использовать CiliumClusterwideNetworkPolicies модуля [Cilium](#).

<!-- перенесено с некоторыми изменениями из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/cni-cilium/#%D0%B8%D1%81%D0%BF%D0%BE%D0%BB%D1%8C%D0%B7%D0%BE%D0%B2%D0%B0%D0%BD%D0%B8%D0%B5-ciliumclusterwidenetworkpolicies -->

Для использования CiliumClusterwideNetworkPolicies следует применить:

1. Первичный набор объектов `CiliumClusterwideNetworkPolicy`, установив конфигурационную опцию `policyAuditMode` в `true`. Отсутствие опции может привести к некорректной работе Control plane или потере доступа ко всем узлам кластера по SSH. Опция может быть удалена после применения всех `CniliumClusterwideNetworkPolicy`-объектов и проверки корректности их работы в Hubble UI.
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

В случае, если CiliumClusterwideNetworkPolicies не будут использованы, Control plane может некорректно работать до одной минуты во время перезагрузки `cilium-agent`-подов. Это происходит из-за [сброса Conntrack-таблицы](https://github.com/cilium/cilium/issues/19367). Привязка к entity `kube-apiserver` позволяет обойти эту проблему.
