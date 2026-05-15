---
title: "Переключение CNI с Flannel или Simple bridge на Cilium"
permalink: ru/admin/configuration/network/internal/flannel-simple-to-cilium.html
lang: ru
---

{% alert level="info" %}
Инструкция актуальна для Deckhouse Kubernetes Platform версии 1.75 и ниже.

Для DKP версии 1.76 и выше используйте руководство [«Переключение CNI в кластере»](/products/kubernetes-platform/guides/cni-migration.html).
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

   Пример вывода:

   ```console
   NAME                      READY STATUS  RESTARTS    AGE
   agent-5zzfv               2/2   Running 5 (23m ago) 26m
   agent-gqb2b               2/2   Running 5 (23m ago) 26m
   agent-wtv4p               2/2   Running 5 (23m ago) 26m
   operator-856d69fd49-mlglv 2/2   Running 0           26m
   safe-agent-updater-26qpk  3/3   Running 0           26m
   safe-agent-updater-qlbrh  3/3   Running 0           26m
   safe-agent-updater-wjjr5  3/3   Running 0           26m
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

1. Сохраните манифест DaemonSet:

   ```shell
   d8 k -n d8-cni-simple-bridge get ds simple-bridge -o yaml > simple-bridge.yaml
   # или
   d8 k -n d8-cni-flannel get ds flannel -o yaml > flannel.yaml
   ```

1. Отключите модуль `cni-simple-bridge` или `cni-flannel`. Выполняйте шаг после того, как поды Cilium перешли в состояние `Ready`:

   ```shell
   d8 k -n d8-system --as system:sudouser exec -it svc/deckhouse-leader -- deckhouse-controller module disable cni-simple-bridge
   # или
   d8 k -n d8-system --as system:sudouser exec -it svc/deckhouse-leader -- deckhouse-controller module disable cni-flannel
   ```

1. Удалите `validating webhook` (если присутствует):

   ```shell
   d8 k delete validatingwebhookconfigurations.admissionregistration.k8s.io d8-deckhouse-validating-webhook-handler-hooks
   ```

1. Удалите неймспейс старого CNI (если не удалился автоматически):

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

1. Настройте внутренние балансировщики LoadBalancer (при использовании). Для всех TargetGroup, входящих во внутренний балансировщик, установите параметр:

   ```text
   Preserve client IP addresses = Off
   ```

1. Перезагрузите узлы кластера по одному, включая master-узлы.
