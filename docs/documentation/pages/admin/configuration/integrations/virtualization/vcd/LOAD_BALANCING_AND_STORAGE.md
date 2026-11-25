---
title: Storage and load balancing in VMware Cloud Director
permalink: en/admin/integrations/virtualization/vcd/load-balancing-and-storage.html
lang: en
---

## Storage usage

* VCD supports CSI. Disks are created as VCD Independent Disks.
* The guest property `disk.EnableUUID` must be enabled for the VM templates in use.
* Deckhouse Platform supports disk resizing starting from version v1.59.1.

You can set the name of StorageClass that will be used in the cluster by default (the [`default`](/modules/cloud-provider-vcd/configuration.html#parameters-storageclass-default) parameter), and filter out the unnecessary StorageClasses (the [`exclude`](/modules/cloud-provider-vcd/configuration.html#parameters-storageclass-exclude) parameter).

## Using the LoadBalancer

- Deckhouse Platform components support `Service` resources of type `LoadBalancer` when deployed on VMware Cloud Director (VCD).
- VMware NSX [Advanced Load Balancer](../../../../glossary.html#advanced-load-balancer) (ALB or Avi Networks) is used as the load balancer.
- Support is available **only** when using the `NSX-T` network virtualization platform.
- The load balancer functionality must be enabled on the Edge Gateway by your VCD provider. You can verify this under Edge Gateway → Load Balancer → General Settings — the `State` parameter must be `Active`.
- If the load balancer was enabled after the Deckhouse Platform cluster was successfully created, the components will automatically pick up the changes within an hour (no additional actions are required).
- For each open port, a Pool + Virtual Service pair is created.
- If a firewall is in place, you must create an allow rule for the load balancer’s external IP address and the corresponding ports.
