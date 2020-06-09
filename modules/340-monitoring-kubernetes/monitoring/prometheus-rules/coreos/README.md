---
---

Правила CoreOS для Prometheus в Kubernetes
==========================================

Эти правила аккуратно вытащены из [helm-чартов](https://github.com/coreos/prometheus-operator/tree/v0.17.0/helm) лежащих вместе с prometheus-opertor (на данный момент из тега `v0.17.0`).

Список файлов с указанием источника:
* [general.yml](https://github.com/coreos/prometheus-operator/blob/v0.17.0/helm/d8-system/templates/general.rules.yaml)
* [kubernetes.yml](https://github.com/coreos/prometheus-operator/blob/v0.17.0/helm/exporter-kubernetes/templates/kubernetes.rules.yaml)
* [kube-controller-manager.yml](https://github.com/coreos/prometheus-operator/blob/v0.17.0/helm/exporter-kube-controller-manager/templates/kube-controller-manager.rules.yaml)
* [kube-etcd3.yml](https://github.com/coreos/prometheus-operator/blob/v0.17.0/helm/exporter-kube-etcd/templates/etcd3.rules.yaml)
* [kubelet.yml](https://github.com/coreos/prometheus-operator/blob/v0.17.0/helm/exporter-kubelets/templates/kubelet.rules.yaml)
* [prometheus.yml](https://github.com/coreos/prometheus-operator/blob/v0.17.0/helm/prometheus/templates/prometheus.rules.yaml)
* [kube-scheduler.yml](https://github.com/coreos/prometheus-operator/blob/v0.17.0/helm/exporter-kube-scheduler/templates/kube-scheduler.rules.yaml)
* [node.yml](https://github.com/coreos/prometheus-operator/blob/v0.17.0/helm/exporter-node/templates/node.rules.yaml)

При обновлении нужно (о ужас) для каждого файла отдельно:
* Посмотреть в репе prometheus-operator, что изменилось (с тега v0.17.0 до того тега, до которого синхронизируемся).
* Посмотреть у нас что изменилось (с комита, в котором написано `upstream:v0.17.0`).
* Руками внести изменения.
