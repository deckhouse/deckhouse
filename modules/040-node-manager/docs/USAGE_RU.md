---
title: "Управление узлами: примеры конфигурации"
---

## Пример описания `NodeGroup`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: Cloud
  kubernetesVersion: "1.16"
  cloudInstances:
    zones:
    - eu-west-1a
    - eu-west-1b
    minPerZone: 1
    maxPerZone: 3
    maxUnavailablePerZone: 0
    maxSurgePerZone: 1
    classReference:
      kind: AWSInstanceClass
      name: test
  kubelet:
    maxPods: 150
    rootDir: "/mnt/data/kubelet"
  docker:
    maxConcurrentDownloads: 10
  nodeTemplate:
    labels:
      environment: production
      app: warp-drive-ai
    annotations:
      ai.fleet.com/discombobulate: "true"
    taints:
    - effect: NoExecute
      key: ship-class
      value: frigate
  chaos:
    mode: DrainAndReboot
    period: 24h
  disruptions:
    approvalMode: Manual
```
## Пример описания `NodeUser`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeUser
metadata:
  name: testuser
spec:
  uid: 1001
  sshPublicKey: "ssh-rsa xxx"
  passwordHash: $2a$10$GAwx2h0D1jiEeBt.JECSEenGvShJbu.mOSQ/jaRk1ly9c31TcHjim
  isSudoer: true
  extraGroups:
  - docker
```
