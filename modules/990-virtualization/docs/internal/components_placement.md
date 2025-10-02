## Placement strategies

| Component                 | nodeSelector strategy | tolerations strategy | Specifics                                  |
|---------------------------|-----------------------|----------------------|--------------------------------------------|
| virt-api                  | master                | any-node             | -> ApiService, webhooks                    |
| virt-operator             | master                | any-node             | -> system/system                           |
| cdi-apiserver             | master                | any-node             | ApiService, webhooks                       |
| virtualization-controller | master                | any-node             | validationwebhooks                         |
| virtualization-api        | master                | any-node             | ApiService                                 |
| virt-controller           | system                | system               |                                            |
| cdi-deployment            | system                | system               | strategy set with infra settings in config |
| cdi-operator              | system                | system               |                                            |
| dvcr                      | system                | system               |                                            |
| virt-handler              |                       | any-node             |                                            |
| vm-route-forge            |                       | any-node             | (should be equal to virt-handler)          |


**master + any-node** - Schedule to control-plane nodes.

**system + system** - Schedule to first matching node: NodeGroup/virtualization, NodeGroup/system and then control-plane.

**any-node** - Schedule to any node, including control-plane.

TODO d8 helm templates adds HA specifics, research and add them here.

TODO remove NodeGroup/virtualization when removed from helm templates.

TODO add more explanation on why these strategies was choosen
