---
title: "Переключение CNI с Flannel на Cilium"
permalink: ru/admin/configuration/network/internal/flannel-to-cilium.html
lang: ru
---

## Процедура переключения CNI с Flannel на Cilium

1. Выключите [модуль `kube-proxy`](/modules/kube-proxy/):

   ```shell
   d8 k apply -f - << EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: kube-proxy
   spec:
     enabled: false
   EOF
   ```

1. Включите [модуль `cni-cilium`](/modules/cni-cilium/):

   ```shell
   d8 k create -f - << EOF
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

1. Убедитесь, что все агенты Cilium перешли в статусе `Running`:

   ```shell
   d8 k get po -n d8-cni-cilium
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

1. Перезагрузите master-узлы.

1. Перезагрузите остальные узлы кластера.

   > Если агенты Cilium не переходят в статус `Running`, перезагрузите проблемные узлы.

1. Выключите [модуль `cni-flannel`](/modules/cni-flannel/):

   ```shell
   d8 k apply -f - << EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cni-flannel
   spec:
     enabled: false
   EOF
   ```

1. Включите [модуль `node-local-dns`](/modules/node-local-dns/):

   ```shell
   d8 k apply -f - << EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: node-local-dns
   spec:
     enabled: true
   EOF
   ```

   После включения модуля дождитесь перехода всех агентов Cilium в состояние `Running`.

1. Убедитесь, что переключение CNI с Flannel на Cilium прошло успешно.

## Проверка успешности переключения CNI с Flannel на Cilium

Чтобы убедиться в том, что переключение CNI с Flannel на Cilium прошло успешно:

1. Проверьте очередь Deckhouse.

   - В случае одного master-узла:

     ```shell
     d8 platform queue list
     ```

   - В случае мультимастерной инсталляции:

     ```shell
     d8 platform queue list
     ```

2. Проверьте агенты Cilium. Они должны быть в статусе `Running`:

   ```shell
   d8 k get po -n d8-cni-cilium
   ```

   Пример вывода:

   ```console
   NAME        READY STATUS  RESTARTS    AGE
   agent-5zzfv 2/2   Running 5 (23m ago) 26m
   agent-gqb2b 2/2   Running 5 (23m ago) 26m
   agent-wtv4p 2/2   Running 5 (23m ago) 26m
   ```

3. Проверьте, что модуль `cni-flannel` выключен:

   ```shell
   d8 k get modules | grep flannel
   ```

   Пример вывода:

   ```console
   cni-flannel                         35     Disabled    Embedded
   ```

4. Проверьте, что модуль `node-local-dns` включен:

   ```shell
   d8 k get modules | grep node-local-dns
   ```

   Пример вывода:

   ```console
   node-local-dns                      350    Enabled     Embedded     Ready
   ```
