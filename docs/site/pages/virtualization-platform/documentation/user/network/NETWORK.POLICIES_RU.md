---
title: "Deckhouse Virtualization Platform"
permalink: ru/virtualization-platform/documentation/user/network/network-policies.html
lang: ru
---
# Network policies

## Основные положения
Для управления входящим и исходящим трафиком виртуальных машин на уровне 3 или 4 модели OSI используются стандартные сетевые политики Kubernetes. Более подробно об этом можно прочитать в официальной документации [Network Policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/).  

Есть два основных типа управления трафиком, входящий и исходящий соответственно:
- `ingress`
- `egress`

Для управления внутрикластерым трафиком рекомендуется использовать `podSelector` и `namespaceSelector`, а для сетевого взаимодействия за пределами кластера - `ipBlock`.

## Примеры использования

Например, есть две виртуальные машины в пространстве имён `netpol-example`. По умолчанию входящий и исходящий трафик неограничены.
```bash
$ kubectl get vm -n netpol-example 
NAME                  PHASE     NODE           IPADDRESS     AGE
netpol-example-vm-a   Running   virtlab-1   10.66.20.68   7m47s
netpol-example-vm-b   Running   virtlab-2   10.66.20.69   7m47s
```

Проверка сетевого доступа от `netpol-example-vm-b` до `netpol-example-vm-a`.
```bash
[21:30:12] cloud@netpol-example-vm-b:~$ ping -c 4 10.66.20.68
PING 10.66.20.68 (10.66.20.68) 56(84) bytes of data.
64 bytes from 10.66.20.68: icmp_seq=1 ttl=63 time=0.581 ms
64 bytes from 10.66.20.68: icmp_seq=2 ttl=63 time=0.621 ms
64 bytes from 10.66.20.68: icmp_seq=3 ttl=63 time=0.715 ms
64 bytes from 10.66.20.68: icmp_seq=4 ttl=63 time=0.847 ms

--- 10.66.20.68 ping statistics ---
4 packets transmitted, 4 received, 0% packet loss, time 3160ms
rtt min/avg/max/mdev = 0.581/0.691/0.847/0.102 ms
```

Виртуальные машины и pod'ы, в которых они запущены, имеют соответствующие метки.
```bash
$ kubectl get vm -n netpol-example -o yaml | less
...
- apiVersion: virtualization.deckhouse.io/v1alpha2
  kind: VirtualMachine
  metadata:
...
    labels:
      vm: a
    name: netpol-example-vm-a
    namespace: netpol-example
...
- apiVersion: virtualization.deckhouse.io/v1alpha2
  kind: VirtualMachine
  metadata:
...
    labels:
      vm: b
    name: netpol-example-vm-b
    namespace: netpol-example
...
```
```bash
$ kubectl get pod -n netpol-example -o yaml | less
...
- apiVersion: v1
  kind: Pod
  metadata:
...
    labels:
...
      vm: a
...
    name: virt-launcher-netpol-example-vm-a-r9h5x
    namespace: netpol-example
...
- apiVersion: v1
  kind: Pod
  metadata:
...
    labels:
...
      vm: b
...
    name: virt-launcher-netpol-example-vm-b-flmxh
    namespace: netpol-example
...
```

Пример сетевой политики, ограничивающей любой входящий трафика для `netpol-example-vm-a`. Для всех виртуальных машин, а значит и для pod'ов с меткой `vm: a` в пространстве имён `netpol-example` будет применена сетевая политика с типом `Ingress`. И так как никаких `ingress` правил в спецификации не указано, то будет ограничен весь входящий трафик.
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: vm-a-deny-ingress
  namespace: netpol-example
spec:
  podSelector:
    matchLabels:
      vm: a
  policyTypes:
    - Ingress
```

После применения политики в namespace, проверяем, что до `netpol-example-vm-a` нет сетевого доступа.
```bash
[21:30:29] cloud@netpol-example-vm-b:~$ ping -c 4 10.66.20.68
PING 10.66.20.68 (10.66.20.68) 56(84) bytes of data.

--- 10.66.20.68 ping statistics ---
4 packets transmitted, 0 received, 100% packet loss, time 3124ms
```

Пример сетевой политики, которая разрешает входящий трафик от `netpol-example-vm-b` до `netpol-example-vm-a`. Здесь снова посредством `podSelector` для всех виртуальных машин и pod'ов с меткой `vm: a` применяется сетевая политика с типом `Ingress`. Но здесь указано `ingress` правило, которое разрешает входящий трафик `from` из виртуальных машин и pod'ов c меткой `vm: b`.
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-ingress-from-vm-b-to-vm-a
  namespace: netpol-example
spec:
  podSelector:
    matchLabels:
      vm: a
  ingress:
    - from:
      - podSelector:
          matchLabels:
            vm: b
  policyTypes:
    - Ingress
```

Проверка входящего трафика от `netpol-example-vm-b` до `netpol-example-vm-a`. При этом видно, что остальной входящий трафик до `netpol-example-vm-a` не разрешён на примере запросов с master узла `virtlab-0`. Но с него же есть доступ до `netpol-example-vm-b`, так как для этой виртуально машины нет ограничений на входящий трафик.
```bash
[22:00:01] cloud@netpol-example-vm-b:~$ ping -c 4 10.66.20.68
PING 10.66.20.68 (10.66.20.68) 56(84) bytes of data.
64 bytes from 10.66.20.68: icmp_seq=1 ttl=63 time=0.982 ms
64 bytes from 10.66.20.68: icmp_seq=2 ttl=63 time=0.380 ms
64 bytes from 10.66.20.68: icmp_seq=3 ttl=63 time=0.698 ms
64 bytes from 10.66.20.68: icmp_seq=4 ttl=63 time=0.927 ms

--- 10.66.20.68 ping statistics ---
4 packets transmitted, 4 received, 0% packet loss, time 3048ms
rtt min/avg/max/mdev = 0.380/0.746/0.982/0.237 ms

root@virtlab-0:~# ping -c 4 10.66.20.69
PING 10.66.20.69 (10.66.20.69) 56(84) bytes of data.
64 bytes from 10.66.20.69: icmp_seq=1 ttl=63 time=0.800 ms
64 bytes from 10.66.20.69: icmp_seq=2 ttl=63 time=0.659 ms
64 bytes from 10.66.20.69: icmp_seq=3 ttl=63 time=0.358 ms
64 bytes from 10.66.20.69: icmp_seq=4 ttl=63 time=0.684 ms

--- 10.66.20.69 ping statistics ---
4 packets transmitted, 4 received, 0% packet loss, time 3053ms
rtt min/avg/max/mdev = 0.358/0.625/0.800/0.163 ms
root@virtlab-0:~# ping -c 4 10.66.20.68
PING 10.66.20.68 (10.66.20.68) 56(84) bytes of data.

--- 10.66.20.68 ping statistics ---
4 packets transmitted, 0 received, 100% packet loss, time 3051ms
```
В данном примере используются две сетевые политики, правила которых действуют одновременно по принципу их складывания для всех ресурсов, которые соответствуют указанным `podSelector`.

## Полезные ссылки
- https://kubernetes.io/docs/concepts/services-networking/network-policies
- https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#networkpolicy-v1-networking-k8s-io
