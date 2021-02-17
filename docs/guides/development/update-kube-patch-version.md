# Обновление патч версий компонентов kubernetes

> По мере выхода новых патч-версий требуется их обновление в коде, пока что это выполняется вручную в нескольких файлах и не вынесено в глобальные переменные.

Текущие патч-версии прописаны в [глобальном VersionMap Candi](../../../candi/version_map.yml).

На момент последней правки версии были следующие:

| major.minor | current | latest |
| --- | --- | --- |
| 1.14 | 1.14.10 | `final` |
| 1.15 | 1.15.12 | `final` |
| 1.16 | 1.16.15 | `final` |
| 1.17 | 1.17.17 |         |
| 1.18 | 1.18.15 |         |
| 1.19 | 1.19.7  |         |

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
