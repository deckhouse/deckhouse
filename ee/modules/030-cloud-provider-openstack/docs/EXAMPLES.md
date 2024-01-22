---
title: "Cloud provider — OpenStack"
---

Below a simple exampl of OpenStack cloud provider configuration.

## Example

The example is a module configuration named `cloud-provider-openstack`, which is used with OpenStack. The module configuration contains connection settings, network names, security settings, and tags that can be used to manage and monitor instances running on OpenStack.

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
