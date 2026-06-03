---
title: "Host firewall на узлах"
permalink: ru/admin/configuration/network/policy/host_firewall.html
description: |
  Настройка host firewall в Deckhouse Kubernetes Platform на основе CiliumClusterwideNetworkPolicy с nodeSelector. Безопасный порядок включения, обязательные правила, защита control plane.
lang: ru
---

Host firewall — это режим работы Cilium, в котором сетевые политики применяются не к подам, а к самим узлам кластера. В DKP host firewall настраивается ресурсами [`CiliumClusterwideNetworkPolicy`](cilium_networkpolicy.html) (CCNP) с полем `nodeSelector`. Доступен только в кластерах с модулем [`cni-cilium`](/modules/cni-cilium/).

{% alert level="danger" %}
Ошибка в host-политиках может привести к потере SSH-доступа к узлам, нарушению работы control plane и недоступности kubelet или etcd. Включайте host firewall только через режим [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) и после проверки вердиктов в Hubble.
{% endalert %}

## Чем host firewall отличается от обычных политик

CCNP с `nodeSelector` применяется к специальному endpoint-у узла с лейблом `reserved:host` и фильтрует трафик, входящий на узел и исходящий из него (включая трафик подов в режиме `hostNetwork`). Политики для подов с `endpointSelector` на host-endpoint не действуют — это разные сущности.

Host-политики не заменяют сетевую защиту инфраструктуры (физический файрвол, security groups облачного провайдера) — они работают как дополнительный уровень фильтрации внутри кластера.

## Безопасный порядок включения

Внедрение host firewall выполняется по шагам с использованием режима аудита:

1. Включите [`policyAuditMode: true`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) в настройках модуля `cni-cilium`. В этом режиме политики не блокируют трафик, а только логируют вердикты.
2. Примените набор host-политик. Минимум — политика для control plane (см. ниже) и политики для worker-узлов с разрешёнными SSH и сервисными портами.
3. Проверьте вердикты в Hubble UI и через `hubble observe --type policy-verdict`. Все ожидаемые соединения должны иметь `verdict=ALLOWED`; всё, что попадает в `verdict=AUDITED`, после отключения режима аудита будет заблокировано.
4. Доработайте политики, пока в логе не останется неожиданных `AUDITED`-записей. Особое внимание уделите трафику kubelet, etcd, kube-apiserver, ingress-контроллеров, мониторинга и DNS.
5. Отключите режим аудита (`policyAuditMode: false`).

При обнаружении проблемы после отключения режима аудита быстрее всего восстановить связность через временное возвращение `policyAuditMode: true` или удаление CCNP. Способы экстренного восстановления описаны в [Emergency Recovery документации Cilium](https://docs.cilium.io/en/v1.17/security/host-firewall/#emergency-recovery).

## Обязательные разрешения

В набор host-политик обязательно включают следующее:

- доступ от kube-apiserver к webhook-эндпоинтам и kubelet (порты 10250, 10255 и порты webhook'ов компонентов);
- доступ между узлами по портам etcd (2379, 2380) — только между control plane-узлами;
- доступ от worker-узлов к API-серверу;
- BGP и сервисные порты, если используется MetalLB или сторонний балансировщик;
- порты модулей платформы из диапазона 4200–4299 (см. [список сетевого взаимодействия компонентов платформы](../../../../reference/network_interaction.html));
- SSH с доверенных адресов администрирования;
- ICMP echo (опционально, для диагностики);
- DNS-egress на kube-dns или внешний резолвер;
- доступ от мониторинга к node-exporter и cilium-agent.

При определении разрешений используйте сущности (`entities`):

- `host` — собственный узел;
- `remote-node` — другие узлы кластера;
- `kube-apiserver` — API-сервер Kubernetes;
- `cluster` — все поды и узлы;
- `world` — внешний мир (используйте вместе с `toCIDR`/`fromCIDR` для уточнения).

## Пример: разрешение трафика к API-серверу для control plane

CCNP, привязывающая control plane-узлы к сущности `kube-apiserver`. Без неё во время перезапуска подов `cilium-agent` возможно кратковременное нарушение работы control plane из-за [сброса Conntrack-таблицы Cilium](https://github.com/cilium/cilium/issues/19367):

```yaml
apiVersion: cilium.io/v2
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: allow-control-plane-connectivity
spec:
  nodeSelector:
    matchLabels:
      node-role.kubernetes.io/control-plane: ""
  ingress:
    - fromEntities:
        - kube-apiserver
```

## Пример: SSH с доверенных адресов администрирования

Разрешает входящий SSH (TCP/22) только из указанных подсетей на все узлы:

```yaml
apiVersion: cilium.io/v2
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: allow-ssh-admin
spec:
  nodeSelector: {}
  ingress:
    - fromCIDR:
        - 192.0.2.0/24
        - 198.51.100.10/32
      toPorts:
        - ports:
            - port: "22"
              protocol: TCP
```

Подставьте подсети, из которых разрешена административная работа. Не оставляйте `fromEntities: [world]` без сужающего CIDR — это эквивалентно открытому SSH.

## Пример: разрешения для worker-узлов

Разрешает worker-узлам обмен трафиком внутри кластера, обращения к API-серверу и DNS:

```yaml
apiVersion: cilium.io/v2
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: allow-worker-baseline
spec:
  nodeSelector:
    matchExpressions:
      - key: node-role.kubernetes.io/control-plane
        operator: DoesNotExist
  ingress:
    - fromEntities:
        - cluster
        - remote-node
  egress:
    - toEntities:
        - kube-apiserver
        - cluster
        - remote-node
    - toEndpoints:
        - matchLabels:
            io.kubernetes.pod.namespace: kube-system
            k8s-app: kube-dns
      toPorts:
        - ports:
            - port: "53"
              protocol: UDP
            - port: "53"
              protocol: TCP
```

Этот пример — стартовая точка. Дополните политику разрешениями для мониторинга, ingress-контроллеров, балансировщиков и SSH согласно вашей конфигурации.

## См. также

- [Host Firewall — документация Cilium](https://docs.cilium.io/en/v1.17/security/host-firewall/)
- [CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy](cilium_networkpolicy.html)
- [Список сетевого взаимодействия компонентов платформы](../../../../reference/network_interaction.html)
- [Диагностика и наблюдаемость политик](troubleshooting.html)
