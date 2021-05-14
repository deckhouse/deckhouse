---
title: "Сloud provider — GCP: FAQ"
---

## How do I create a cluster?

1. Set up a cloud environment.
2. Enable the module or pass the `--extra-config-map-data base64_encoding_of_custom_config` flag with the [module parameters](configuration.html) to the `install.sh` script.
3. Create one or more [GCPInstanceClass](cr.html#gcpinstanceclass) custom resources.
4. Create one or more [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup) custom resources for managing the number and the process of provisioning machines in the cloud.
