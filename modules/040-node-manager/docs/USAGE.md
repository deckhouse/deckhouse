---
title: "Managing nodes: usage"
---

## An example of the `NodeGroup` configuration

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    zones:
    - eu-west-1a
    - eu-west-1b
    minPerZone: 1
    maxPerZone: 2
    classReference:
      kind: AWSInstanceClass
      name: test
  nodeTemplate:
    labels:
      tier: test
```
## An example of the `NodeUser` configuration

```yaml
apiVersion: deckhouse.io/v1
kind: NodeUser
metadata:
  name: testuser
spec:
  uid: 1001
  sshPublicKey: "ssh-rsa xxx"
  passwordHash: $2a$10$GAwx2h0D1jiEeBt.JECSEenGvShJbu.mOSQ/jaRk1ly9c31TcHjim
  isSudoer: true
```
