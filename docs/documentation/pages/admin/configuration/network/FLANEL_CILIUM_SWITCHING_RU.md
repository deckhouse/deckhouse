---
title: "Переключение с CNI flannel на cilium"
permalink: ru/admin/configuration/network/flannel-cilium-switching.html
lang: ru
---

## Действия для переключения с CNI flannel на cilium

Для переключения с CNI flannel на cilium выполните следующие действия:

1. Выключите модуль `kube-proxy`:

   ```shell
   $ kubectl apply -f - << EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
    name: kube-proxy
   spec:
    enabled: false
   EOF
   ```

1. Включите модуль `cni-cilium`:

   ```shell
   $ kubectl create -f - << EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
    name: cni-cilium
   spec:
    version: 1
    enabled: true
    settings:
    tunnelMode: VXLAN
   EOF
   ```

1. Убедитесь, что все агенты cilium в статусе Running:

   ```shell
   $ kubectl get po -n d8-cni-cilium
   NAME                      READY STATUS  RESTARTS    AGE
   agent-5zzfv               2/2   Running 5 (23m ago) 26m
   agent-gqb2b               2/2   Running 5 (23m ago) 26m
   agent-wtv4p               2/2   Running 5 (23m ago) 26m
   operator-856d69fd49-mlglv 2/2   Running 0           26m
   safe-agent-updater-26qpk  3/3   Running 0           26m
   safe-agent-updater-qlbrh  3/3   Running 0           26m
   safe-agent-updater-wjjr5  3/3   Running 0           26m
   ```

1. Выполните reboot мастер-узлов.

1. Выполните reboot остальных узлов кластера.

   > Если агенты cilium не переходят в статус Running, выполнить reboot проблемных узлов.

1. Выключите модуль `cni-flannel`:

   ```shell
   $ kubectl apply -f - << EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
    name: cni-flannel
   spec:
    enabled: false
   EOF
   ```

1. Включите модуль `node-local-dns`:

   ```shell
   $ kubectl apply -f - << EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
    name: node-local-dns
   spec:
    enabled: true
   EOF
   ```

   После включения модуля дождитесь перехода всех агентов cilium в состояние `Running`.

1. Убедитесь, что переключение с CNI flannel на cilium прошло успешно.

### Проверка успешности переключения с CNI flannel на cilium

Чтобы убедиться в том, что переключение с CNI flannel на cilium прошло успешно:

1. Проверьте очередь Deckhouse:

   В случае одного master-узла:

   ```shell
   kubectl -n d8-system exec -it deploy/deckhouse -c deckhouse -- deckhouse-controller queue list
   ```

   В случае мультимастерной инсталляции:

   ```shell
   kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

2. Проверьте агенты cilium. Они должны быть в статусе Running:

   ```shell
   $ kubectl get po -n d8-cni-cilium
   NAME        READY STATUS  RESTARTS    AGE
   agent-5zzfv 2/2   Running 5 (23m ago) 26m
   agent-gqb2b 2/2   Running 5 (23m ago) 26m
   agent-wtv4p 2/2   Running 5 (23m ago) 26m
   ```

3. Проверьте, что модуль `cni-flannel` выключен:

   ```shell
   $ kubectl get modules | grep flannel
   cni-flannel                         35     Disabled    Embedded
   ```

4. Проверьте, что модуль `node-local-dns` включен:

   ```shell
   $ kubectl get modules | grep node-local-dns
   node-local-dns                      350    Enabled     Embedded     Ready
   ```
