---
title: "Сетевые политики"
permalink: ru/admin/configuration/network/policy/configuration.html
description: |
  Обзор реализаций сетевых политик в Deckhouse Kubernetes Platform: NetworkPolicy, CiliumNetworkPolicy, CiliumClusterwideNetworkPolicy, host firewall.
lang: ru
search: network policy, network policies, NetworkPolicy, CiliumNetworkPolicy, CiliumClusterwideNetworkPolicy, host firewall, сетевые политики, сетевая безопасность
---

Сетевые политики ограничивают сетевое взаимодействие подов друг с другом, с внешними системами и узлами кластера. В Deckhouse Kubernetes Platform (DKP) реализация сетевых политик зависит от выбранного CNI.

## Реализация сетевых политик в DKP

Доступные форматы политик и движок их обработки определяются включённым модулем CNI:

- В кластерах с модулем [`cni-cilium`](/modules/cni-cilium/) реализация встроена в Cilium и поддерживает три формата политик:
  - стандартный [`NetworkPolicy`](https://kubernetes.io/docs/concepts/services-networking/network-policies/) уровней L3 и L4;
  - [`CiliumNetworkPolicy`](https://docs.cilium.io/en/v1.17/network/kubernetes/policy/#ciliumnetworkpolicy) — namespaced-ресурс с правилами L3–L7;
  - [`CiliumClusterwideNetworkPolicy`](https://docs.cilium.io/en/v1.17/network/kubernetes/policy/#ciliumclusterwidenetworkpolicy) — cluster-scoped-ресурс, дополнительно поддерживает `nodeSelector` для защиты узлов (host firewall).
- В кластерах с модулем `cni-flannel` или другим CNI без поддержки политик обработку обеспечивает модуль [`network-policy-engine`](/modules/network-policy-engine/) на базе [kube-router](https://github.com/cloudnativelabs/kube-router). Поддерживается только стандартный `NetworkPolicy` уровней L3 и L4. Политики транслируются в правила `iptables` и `ipset` на каждом узле.

{% alert level="warning" %}
Не включайте модули `cni-cilium` и `network-policy-engine` одновременно: в Cilium уже есть собственная реализация сетевых политик.
{% endalert %}

## Что доступно в зависимости от движка

При выборе формата политики учитывайте возможности движка:

- стандартный `NetworkPolicy` (L3/L4, namespaced) — поддерживается обоими движками;
- `CiliumNetworkPolicy` (L3–L7, FQDN, deny-правила, namespaced) — только при включённом `cni-cilium`;
- `CiliumClusterwideNetworkPolicy` (L3–L7, FQDN, deny-правила, cluster-scoped) — только при включённом `cni-cilium`;
- host firewall на узлах через `CiliumClusterwideNetworkPolicy` с `nodeSelector` — только при включённом `cni-cilium`;
- режим аудита политик ([`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode)) — только при включённом `cni-cilium`.

## Требования к сетевой инфраструктуре

Если на уровне инфраструктуры есть требования по ограничению сетевого взаимодействия между серверами, при настройке кластера выполните следующие условия:

- Включите режим туннелирования трафика подов: [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode) для CNI Cilium, [`podNetworkMode`](/modules/cni-flannel/configuration.html#parameters-podnetworkmode) для CNI Flannel. Дополнительно разрешите взаимодействие между узлами по VXLAN-порту из [списка сетевого взаимодействия компонентов платформы](../../../../reference/network_interaction.html).
- Разрешите передачу трафика между подсетями подов ([`podSubnetCIDR`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-podsubnetcidr)), инкапсулированного в VXLAN, если в сети выполняется инспектирование трафика.
- Разрешите взаимодействие с внешними системами, с которыми интегрируется кластер (LDAP, SMTP, внешние API).
- Разрешите локальное сетевое взаимодействие в рамках каждого узла.
- Разрешите взаимодействие между узлами по портам из [списка сетевого взаимодействия компонентов платформы](../../../../reference/network_interaction.html). Большинство портов входит в диапазон 4200–4299; новым компонентам платформы порты выделяются из этого диапазона при наличии возможности.

## Разделы

- [Стандартный NetworkPolicy Kubernetes](kubernetes_networkpolicy.html) — модель изоляции, селекторы, default-политики, ограничения API.
- [CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy](cilium_networkpolicy.html) — расширения Cilium, entities, правила L7, FQDN, режим аудита.
- [Host firewall на узлах](host_firewall.html) — защита самих узлов с помощью `CiliumClusterwideNetworkPolicy` и `nodeSelector`.
- [Типовые примеры политик](examples.html) — рецепты для частых задач.
- [Диагностика и наблюдаемость политик](troubleshooting.html) — как проверить применение политики и расследовать проблемы.

## Дополнительная документация

- [Network Policies — документация Kubernetes](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
- [Network Policy — документация Cilium](https://docs.cilium.io/en/v1.17/network/kubernetes/policy/)
- [Overview of Network Policy — документация Cilium](https://docs.cilium.io/en/v1.17/security/policy/)
- [Host Firewall — документация Cilium](https://docs.cilium.io/en/v1.17/security/host-firewall/)
