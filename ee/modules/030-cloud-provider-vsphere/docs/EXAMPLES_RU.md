---
title: "Cloud provider — VMware vSphere: Примеры"
---

Ниже представлен пример конфигурации cloud-провайдера VMware vSphere.

## Пример конфигурации

```yaml
cloudProviderVsphereEnabled: "true"
cloudProviderVsphere: |
  host: vc-3.internal
  username: user
  password: password
  vmFolderPath: dev/test
  insecure: true
  region: moscow-x001
  sshKeys:
  - "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQD5sAcceTHeT6ZnU+PUF1rhkIHG8/B36VWy/j7iwqqimC9CxgFTEi8MPPGNjf+vwZIepJU8cWGB/By1z1wLZW3H0HMRBhv83FhtRzOaXVVHw38ysYdQvYxPC0jrQlcsJmLi7Vm44KwA+LxdFbkj+oa9eT08nQaQD6n3Ll4+/8eipthZCDFmFgcL/IWy6DjumN0r4B+NKHVEdLVJ2uAlTtmiqJwN38OMWVGa4QbvY1qgwcyeCmEzZdNCT6s4NJJpzVsucjJ0ZqbFqC7luv41tNuTS3Moe7d8TwIrHCEU54+W4PIQ5Z4njrOzze9/NlM935IzpHYw+we+YR+Nz6xHJwwj i@my-PC"
  externalNetworkNames:
  - KUBE-3
  - devops-internal
  internalNetworkNames:
  - KUBE-3
  - devops-internal
  nsxt:
    defaultIpPoolName: "External IP Pool"
    tier1GatewayPath: flant_tier1
    user: guestuser1
    password: pass
    host: 1.2.3.4
    insecureFlag: true
    size: SMALL
```

## Пример custom resource `VsphereInstanceClass`

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereInstanceClass
metadata:
  name: test
spec:
  numCPUs: 2
  memory: 2048
  rootDiskSize: 20
  template: dev/golden_image
  network: k8s-msk-178
  datastore: lun-1201
```
