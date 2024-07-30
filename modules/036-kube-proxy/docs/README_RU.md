---
title: "Модуль kube-proxy"
---

Модуль удаляет весь комплект kube-proxy (`DaemonSet`, `ConfigMap`, `RBAC`) от `kubeadm` и устанавливает свой.

По умолчанию в целях безопасности при использовании сервисов с типом NodePort подключения принимаются только на InternalIP узлов. Для снятия данного ограничения предусмотрена аннотация на узел — `node.deckhouse.io/nodeport-bind-internal-ip: "false"`.

Пример аннотации для NodeGroup:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: myng
spec:
  nodeTemplate:
    annotations:
      node.deckhouse.io/nodeport-bind-internal-ip: "false"
...
```

> **Внимание!** После добавления, удаления или изменения значения аннотации необходимо самостоятельно выполнить рестарт подов kube-proxy.
>
> **Внимание!** Модуль kube-proxy автоматически отключается при включении модуля [cni-cilium](../021-cni-cilium/).
