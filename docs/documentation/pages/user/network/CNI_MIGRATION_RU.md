---
title: "Миграция с cni-simple-bridge/cni-flannel на cni-cilium"
description: "Последовательность действий для миграции с cni-simple-bridge/cni-flannel на cni-cilium"
permalink: ru/user/network/cni_migration.html
---

{% alert level="info" %}
Инструкция для версий Deckhouse Kubernetes Platform <= 1.74
{% endalert %}

{% alert level="warning" %}
Работы ведутся с полным простоем кластера, включая в себя перезапуск всех подов и всех нод.
Ожидаемое время выполнения: ~1 час.
{% endalert %}

1) Включаем модуль `cni-cilium`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-cilium
spec:
  version: 1
  settings:
    tunnelMode: VXLAN
  enabled: true
```

2) Если агенты cilium не стартуют, то порядок действия для их перезапуска:

```shell
# Делаем бэкап, в процесе переключения ресурсы пересоздатуться сами
kubectl -n kube-system get ds d8-kube-proxy -oyaml > d8-kube-proxy.yaml

# Исправление crashloop агентов Cilium
kubectl delete validatingadmissionpolicies.admissionregistration.k8s.io label-objects.deckhouse.io
kubectl -n kube-system delete ds d8-kube-proxy
kubectl -n d8-cni-cilium delete po -l app=agent
```

Возможен длительный старт ( больше 5 минут) -  необходим перезапуск прокси.
```shell
kubectl -n kube-system delete po -l k8s-app=kube-proxy
```

3) Выключаем модуль `cni-simple-bridge`/`cni-flannel` (после запуска всех подов `Cilium`). 
На этом этапе pod с webhook-handle могут падать в CrashLoopBackOff.

```shell
# Делаем бэкап, в процесе переключения ресурсы пересоздатуться сами
kubectl -n d8-cni-simple-bridge get ds simple-bridge -oyaml > simple-bridge.yaml
# Или
kubectl -n d8-cni-flannel get ds flannel -oyaml > flannel.yaml
```

```shell
# Выключаем cni-simple-bridge
kubectl delete validatingwebhookconfigurations.admissionregistration.k8s.io d8-deckhouse-validating-webhook-handler-hooks
kubectl exec -it -n d8-system -it svc/deckhouse-leader -- deckhouse-controller module disable cni-simple-bridge
# Или
kubectl exec -it -n d8-system -it svc/deckhouse-leader -- deckhouse-controller module disable cni-flannel
```

```shell
# Должен удалиться сам, если не удалился удаляем сами, может так же зависнуть в Terminating
kubectl delete ns d8-cni-simple-bridge
# Или
kubectl delete ns d8-cni-flannel
```

4) После того как убедимся, что `cni-simple-bridge`/`cni-flannel` не запускается вновь, редактируем iptables:

Шаг опциональный и ожидаеться, что вовремя перезагрузки без удаления интерфейса и правил iptables, но для гарантированного, как пример оставлено удаление.
```shell
ip link delete cni0

iptables -t filter -F
iptables -t filter -X
iptables -t nat -F
iptables -t nat -X
iptables -t mangle -F
iptables -t mangle -X
iptables -t raw -F
iptables -t raw -X
iptables -P INPUT ACCEPT
iptables -P FORWARD ACCEPT
iptables -P OUTPUT ACCEPT
```

5) Перезапускаем все поды:
```shell
kubectl delete pods -A --all --wait=false
```

6) На внутренних loadbalancer поставить галку Preserve client IP addresses в Off для всех TargetGroup входящих в этот балансер

7) По одному перезапускаем все ноды в кластере (включая мастера).
