---
title: "Cloud provider — GCP: FAQ"
---

## How do I create a cluster?

1. Set up a cloud environment.
2. Enable the module or pass the `--extra-config-map-data base64_encoding_of_custom_config` flag with the [module parameters](configuration.html) to the `install.sh` script.
3. Create one or more [GCPInstanceClass](cr.html#gcpinstanceclass) custom resources.
4. Create one or more [NodeGroup](../../modules/node-manager/cr.html#nodegroup) custom resources for managing the number and the process of provisioning machines in the cloud.

## Adding CloudStatic nodes to a cluster

For the VMs, you want to add to a cluster as nodes, add a `Network Tag` similar to the cluster prefix.

You can find out `prefix` using the command:

```shell
d8 k -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' \
  | base64 -d | grep prefix
```
