---
title: "Миграция с cni-simple-bridge/cni-flannel на cni-cilium"
description: "Последовательность действий для миграции с cni-simple-bridge/cni-flannel на cni-cilium"
permalink: ru/user/network/cni_migration.html
---

{% alert level="info" %}
Инструкция актуальна для Deckhouse Kubernetes Platform **<= 1.74**.
{% endalert %}

{% alert level="warning" %}
Миграция выполняется с полным простоем кластера: будут перезапущены все поды и узлы. Оценочное время выполнения — **около 1 часа**.
{% endalert %}

Для миграции выполните следующие шаги:

1. Включите модуль `cni-cilium`:

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

1. Дождитесь запуска компонентов Cilium:

   ```shell
   d8 k -n d8-cni-cilium get pods
   ```

   Если поды агента Cilium не переходят в состояние `Ready`:

   - Сохраните манифест `d8-kube-proxy`:

     ```shell
     d8 k -n kube-system get ds d8-kube-proxy -oyaml > d8-kube-proxy.yaml
     ```

   - Выполните действия для восстановления запуска:

     ```shell
     d8 k delete validatingadmissionpolicies.admissionregistration.k8s.io label-objects.deckhouse.io
     d8 k -n kube-system delete ds d8-kube-proxy
     d8 k -n d8-cni-cilium delete po -l app=agent
     ```

   - Если запуск Cilium продолжается более 5 минут, перезапустите `kube-proxy`:

     ```shell
     d8 k -n kube-system delete pod -l k8s-app=kube-proxy
     ```

1. Отключите `cni-simple-bridge` или `cni-flannel`. Выполняйте шаг после того, как поды Cilium перешли в состояние `Ready`.

1. Сохраните манифест DaemonSet:

   ```shell
   d8 k -n d8-cni-simple-bridge get ds simple-bridge -o yaml > simple-bridge.yaml
   # или
   d8 k -n d8-cni-flannel get ds flannel -o yaml > flannel.yaml
   ```

1. Удалите `validating webhook` (если присутствует):

   ```shell
   d8 k delete validatingwebhookconfigurations.admissionregistration.k8s.io d8-deckhouse-validating-webhook-handler-hooks
   ```

1. Отключите соответствующий модуль:

   ```shell
   d8 k -n d8-system exec -it svc/deckhouse-leader -- deckhouse-controller module disable cni-simple-bridge
   # или
   d8 k -n d8-system exec -it svc/deckhouse-leader -- deckhouse-controller module disable cni-flannel
   ```

1. Удалите пространство имён старого CNI (если не удалился автоматически):

   ```shell
   d8 k delete ns d8-cni-simple-bridge
   # или
   d8 k delete ns d8-cni-flannel
   ```

1. (Опционально) Очистите артефакты старого CNI. При необходимости удалите интерфейс и сбросьте правила iptables:

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

1. Перезапустите все поды:

   ```shell
   d8 k delete pods -A --all --wait=false
   ```

1. Настройте внутренние LoadBalancer (при использовании). Для всех TargetGroup, входящих во внутренний балансировщик, установите параметр:

   ```text
   Preserve client IP addresses = Off
   ```

1. Перезагрузите узлы кластера по одному, включая master-узлы.
