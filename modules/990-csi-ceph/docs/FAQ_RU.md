---
title: "Модуль csi-ceph: FAQ"
---

## Как получить список томов RBD, разделенный по узлам?

```shell
kubectl -n d8-csi-ceph get po -l app=csi-node-rbd -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName --no-headers \
  | awk '{print "echo "$2"; kubectl -n d8-csi-ceph exec  "$1" -c node -- rbd showmapped"}' | bash
```

## Какие версии Ceph кластеров поддерживаются

Официально сейчас поддерживаются версии >= 16.2.0. Из нашей практики текущая версия способна работать с кластерами версий >=14.2.0, но мы рекомендуем обновить версию Ceph.

## Какие режимы работы томов поддерживаются

RBD поддерживает только ReadWriteOnce (RWO, доступ к тому в рамках одной ноды). CephFS поддерживает как ReadWriteOnce, так и ReadWriteMany (RWX, одновременный доступ к тому с нескольких нод)
