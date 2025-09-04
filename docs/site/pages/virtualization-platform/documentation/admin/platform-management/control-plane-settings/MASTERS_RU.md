---
title: "Master-узлы"
permalink: ru/virtualization-platform/documentation/admin/platform-management/control-plane-settings/masters.html
lang: "ru"
---

## Добавление master-узла

> Важно иметь нечетное количество master-узлов для обеспечения кворума.

Добавление master-узла в кластер ничем не отличается от добавления обычного узла. Проверьте существование NodeGroup c ролью control-plane (обычно это NodeGroup с именем master) и воспользуйтесь инструкцией по [добавлению узла](../node-management/adding-node.html#добавление-узла-в-кластер). Все необходимые действия по настройке компонентов control plane кластера на новом узле будут выполнены автоматически.

Перед добавлением следующего узла дождитесь статуса `Ready` для всех master-узлов:

```shell
d8 k get no -l node-role.kubernetes.io/control-plane=
NAME       STATUS   ROLES                  AGE    VERSION
master-0   Ready    control-plane,master   276d   v1.28.15
master-1   Ready    control-plane,master   247d   v1.28.15
master-2   Ready    control-plane,master   247d   v1.28.15
```

## Удаление роли master-узла с сохранением узла в кластере

1. Сделайте [резервную копию etcd](/products/virtualization-platform/documentation/admin/platform-management/control-plane-settings/etcd.html#%D1%80%D0%B5%D0%B7%D0%B5%D1%80%D0%B2%D0%BD%D0%BE%D0%B5-%D0%BA%D0%BE%D0%BF%D0%B8%D1%80%D0%BE%D0%B2%D0%B0%D0%BD%D0%B8%D0%B5-etcd) и директории `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет алертов, которые могут помешать обновлению master-узлов.
   Список всех алертов можно посмотреть с помощью команды:

   ```shell
   d8 k get clusteralerts
   ```

1. Убедитесь, что очередь Deckhouse пуста.
   Для просмотра состояния всех очередей заданий Deckhouse, выполните следующую команду:

   ```shell
   d8 p queue list
   ```

1. Снимите с узла метки `node.deckhouse.io/group: master` и `node-role.kubernetes.io/control-plane: ""`.
1. Убедитесь, что узел пропал из списка узлов кластера etcd:

   ```bash
   d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

1. Удалите настройки компонентов управляющего слоя на узле:

   ```shell
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   rm -rf /var/lib/etcd/member/
   ```

1. Убедитесь, что число узлов в NodeGroup `master` уменьшилось

   Если было 3 узла, то должно стать 2:

   ```shell
   d8 k get ng master
   NAME     TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE    SYNCED
   master   Static   2       2       2                                                               280d   True
   ```
