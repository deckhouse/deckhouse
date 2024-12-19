---
title: "Module embedded-registry: cloud architecture"
description: ""
---

## Bootstrap

For the initial master node during the bootstrap phase, the parameter [SystemRegistryConfig.IsEnable](dhctl/pkg/config/system_registry.go#L28) is passed in the `terraform apply` command. This parameter determines whether a disk for the embedded registry will be created.

During the execution of bashible, [disk mounting](candi/bashible/common-steps/node-group/005_integrate_system_registry_data_device.sh.tpl) is performed.

After the cluster is started, a [secret is applied](http://dhctl/pkg/operations/converge/infra/hook/controlplane/hook_for_update_pipeline.go#L225), which contains information on the disks created on the master nodes. This secret is populated with data if a disk was created or removed for the current and other master nodes.

When a second master node is added, data from the secret is used ([disk mounting step in bashible](candi/bashible/common-steps/node-group/005_integrate_system_registry_data_device.sh.tpl)).

## Converge
