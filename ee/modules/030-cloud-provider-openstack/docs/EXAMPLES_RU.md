---
title: "Cloud provider — OpenStack"
---

Ниже представлен пример конфигурации cloud-провайдера OpenStack.

## Пример

Пример представляет собой конфигурацию модуля с именем `cloud-provider-openstack`, которая используется с OpenStack. Конфигурация модуля содержит настройки подключения, имена сетей, настройки безопасности и теги, которые могут использоваться для управления и мониторинга экземпляров, работающих на OpenStack.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-openstack
spec:
  version: 1
  enabled: true
  settings:
    connection:
      authURL: https://test.tests.com:5000/v3/
      domainName: default
      tenantName: default
      username: jamie
      password: nein
      region: HetznerFinland
    externalNetworkNames:
    - public
    internalNetworkNames:
    - kube
    instances:
      sshKeyPairName: my-ssh-keypair
      securityGroups:
      - default
      - allow-ssh-and-icmp
    zones:
    - zone-a
    - zone-b
    tags:
      project: cms
      owner: default
```
