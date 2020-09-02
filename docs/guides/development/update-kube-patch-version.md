# Обновление патч версий компонентов kubernetes

> По мере выхода новых патч-версий требуется их обновление в коде, пока что это выполняется вручную в нескольких файлах и не вынесено в глобальные переменные.

На момент последней правки версии были следующие:

| major.minor | current | latest |
| --- | --- | --- |
| 1.14 | 1.14.10 | `final` |
| 1.15 | 1.15.12 | `final` |
| 1.16 | <s>1.16.10</s> | **1.16.15** |
| 1.17 | <s>1.17.6</s> | **1.17.11** |
| 1.18 | <s>1.18.3</s> | **1.18.8** |

Патч версии компонентов захардкожены в следующих `.tpl` и `Dockerfile`:

## kubelet
- [candi/bashible/bundles/centos-7/all/034_install_kubelet_and_his_friends.sh.tpl](../../../candi/bashible/bundles/centos-7/all/034_install_kubelet_and_his_friends.sh.tpl)
- [candi/bashible/bundles/ubuntu-lts/all/034_install_kubelet_and_his_friends.sh.tpl](../../../candi/bashible/bundles/ubuntu-lts/all/034_install_kubelet_and_his_friends.sh.tpl)

```gotemplate
{{ if eq .kubernetesVersion "1.14" }}
kubernetes_version="1.14.patch"
{{ else if eq .kubernetesVersion "1.15" }}
kubernetes_version="1.15.patch"
{{ else if eq .kubernetesVersion "1.16" }}
kubernetes_version="1.16.patch"
{{ end }}
```

## kubeadm
- [candi/bashible/bundles/centos-7/cluster-bootstrap/035_install_kubeadm.sh.tpl](../../../candi/bashible/bundles/centos-7/cluster-bootstrap/035_install_kubeadm.sh.tpl)
- [candi/bashible/bundles/ubuntu-lts/cluster-bootstrap/035_install_kubeadm.sh.tpl](../../../candi/bashible/bundles/ubuntu-lts/cluster-bootstrap/035_install_kubeadm.sh.tpl)

```gotemplate
{{ if eq .kubernetesVersion "1.15" }}
  kubernetes_version="1.15.patch"
{{ else if eq .kubernetesVersion "1.16" }}
  kubernetes_version="1.16.patch"
{{ end }}
```

## Конфигурация кластера
[candi/control-plane-kubeadm/config.yaml.tpl](../../../candi/control-plane-kubeadm/config.yaml.tpl)
```gotemplate
apiVersion: kubeadm.k8s.io/v1beta2
kind: ClusterConfiguration
{{- if eq .clusterConfiguration.kubernetesVersion "1.15" }}
kubernetesVersion: 1.15.patch
{{- else if eq .clusterConfiguration.kubernetesVersion "1.16" }}
kubernetesVersion: 1.16.patch
{{- end }}
```
## Образ kubeadm_builder
[modules/040-control-plane-manager/images/control-plane-manager/Dockerfile](../../../modules/040-control-plane-manager/images/control-plane-manager/Dockerfile)

## Компоненты control-plane

### apiserver
- [modules/040-control-plane-manager/images/kube-apiserver-1-15/Dockerfile](../../../modules/040-control-plane-manager/images/kube-apiserver-1-15/Dockerfile)
- [modules/040-control-plane-manager/images/kube-apiserver-1-16/Dockerfile](../../../modules/040-control-plane-manager/images/kube-apiserver-1-16/Dockerfile)
- [modules/040-control-plane-manager/images/kube-apiserver-1-17/Dockerfile](../../../modules/040-control-plane-manager/images/kube-apiserver-1-17/Dockerfile)
- [modules/040-control-plane-manager/images/kube-apiserver-1-18/Dockerfile](../../../modules/040-control-plane-manager/images/kube-apiserver-1-18/Dockerfile)

### controller-manager
- [modules/040-control-plane-manager/images/kube-controller-manager-1-15/Dockerfile](../../../modules/040-control-plane-manager/images/kube-controller-manager-1-15/Dockerfile)
- [modules/040-control-plane-manager/images/kube-controller-manager-1-16/Dockerfile](../../../modules/040-control-plane-manager/images/kube-controller-manager-1-16/Dockerfile)
- [modules/040-control-plane-manager/images/kube-controller-manager-1-17/Dockerfile](../../../modules/040-control-plane-manager/images/kube-controller-manager-1-17/Dockerfile)
- [modules/040-control-plane-manager/images/kube-controller-manager-1-18/Dockerfile](../../../modules/040-control-plane-manager/images/kube-controller-manager-1-18/Dockerfile)

### scheduler
- [modules/040-control-plane-manager/images/kube-scheduler-1-15/Dockerfile](../../../modules/040-control-plane-manager/images/kube-scheduler-1-15/Dockerfile)
- [modules/040-control-plane-manager/images/kube-scheduler-1-16/Dockerfile](../../../modules/040-control-plane-manager/images/kube-scheduler-1-16/Dockerfile)
- [modules/040-control-plane-manager/images/kube-scheduler-1-17/Dockerfile](../../../modules/040-control-plane-manager/images/kube-scheduler-1-17/Dockerfile)
- [modules/040-control-plane-manager/images/kube-scheduler-1-18/Dockerfile](../../../modules/040-control-plane-manager/images/kube-scheduler-1-18/Dockerfile)

### proxy
- [modules/041-kube-proxy/images/kube-proxy-1-14/Dockerfile](../../../modules/041-kube-proxy/images/kube-proxy-1-14/Dockerfile)
- [modules/041-kube-proxy/images/kube-proxy-1-15/Dockerfile](../../../modules/041-kube-proxy/images/kube-proxy-1-15/Dockerfile)
- [modules/041-kube-proxy/images/kube-proxy-1-16/Dockerfile](../../../modules/041-kube-proxy/images/kube-proxy-1-16/Dockerfile)
- [modules/041-kube-proxy/images/kube-proxy-1-17/Dockerfile](../../../modules/041-kube-proxy/images/kube-proxy-1-17/Dockerfile)
- [modules/041-kube-proxy/images/kube-proxy-1-18/Dockerfile](../../../modules/041-kube-proxy/images/kube-proxy-1-18/Dockerfile)

