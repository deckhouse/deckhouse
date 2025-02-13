---
title: "Cloud provider - VMware vSphere: схемы размещения"
description: "Описание схем размещения и взаимодействия ресурсов в VMware vSphere при работе облачного провайдера Deckhouse."
---

## Standard

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQolOJQw4clYDug78Mr7rvX7wYPsb2uVhab5cDZrzBKq76Ox6dZhgoBXuq-ta8DRC2grjNUcfEq_AR8/pub?w=667&h=516)
<!--- Исходник: https://docs.google.com/drawings/d/1QOgPkq_xfBWMMI3SEU4Q9lyZM5mIWWbF_MwVsd06diE/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
vmFolderPath: dev
regionTagCategory: k8s-region
zoneTagCategory: k8s-zone
region: X1
internalNetworkCIDR: 192.168.199.0/24
masterNodeGroup:
  replicas: 1
  zones:
  - ru-central1-a
  - ru-central1-b
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: dev/lun_1
    mainNetwork: net3-k8s
nodeGroups:
- name: khm
  replicas: 1
  zones:
  - ru-central1-a
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: dev/lun_1
    mainNetwork: net3-k8s
sshPublicKey: "<SSH_PUBLIC_KEY>"
zones:
- ru-central1-a
- ru-central1-b
```
