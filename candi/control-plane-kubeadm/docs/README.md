---
title: Control Plane Kubeadm
---

Go-шаблоны для подготовки конфигурации kubeadm. 

* `config.yaml.tpl` - основная конфигурация
* `kustomize/` -  патчи, применяемые к компонентам control-plane при подготовке кластера Kubernetes

Используются при создании первого узла в кластере и в модуле [control-plane-manager]({{ site.baseurl }}/modules/040-control-plane-manager/).

### Как скомпилировать control-plane-kubeadm?

Скомпилировать control-plane-kubeadm можно воспользовавшись утилитой deckhouse-candi.
```bash
deckhouse-candi render kubeadm-config --config=/config.yaml
```

Пример `config.yaml`:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: KubeadmTemplateData
clusterConfiguration:
  cloud:
    prefix: pivot
    provider: OpenStack
  clusterDomain: cluster.local
  clusterType: Cloud
  kubernetesVersion: "1.16"
  podSubnetCIDR: 10.111.0.0/16
  podSubnetNodeCIDRPrefix: "24"
  serviceSubnetCIDR: 10.222.0.0/16
nodeIP: 192.168.199.161
extraArgs: {}
```
