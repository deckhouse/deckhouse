---
title: Storage and load balancing in Amazon Web Services
permalink: en/admin/integrations/public/amazon/load-balancing-and-storage.html
---

## Storage (CSI and StorageClass)

Deckhouse Platform integrates with AWS storage via CSI.
This allows the cluster to automatically provision and attach disks to its nodes.

StorageClasses are created automatically for the following disk types:

- `gp3`
- `gp2`
- `sc1`
- `st1`

Disk types `io1` and `io2` are also supported with IOPS and throughput settings in ModuleConfig.

To exclude unnecessary StorageClasses from the cluster, specify filters in [`settings.storageClass.exclude`](/modules/cloud-provider-aws/configuration.html#parameters-storageclass-exclude):

```yaml
settings:
  storageClass:
    exclude:
    - sc.*
    - st1
```

You can also define and configure custom StorageClasses explicitly, including parameters such as `iops`, `throughput`, and `type`:

```yaml
settings:
  storageClass:
    provision:
    - name: fast-io
      type: io2
      iopsPerGB: "50"
```

### Expanding volume size

To resize a volume (for example, when running low on disk space), follow these steps:

1. Modify the `spec.resources.requests.storage` parameter in the corresponding PersistentVolumeClaim object.
   The operation is performed automatically and usually takes less than a minute.
   You can monitor progress with the following command:

   ```shell
   d8 k describe pvc <claim-name>
   ```

1. After resizing, wait at least 6 hours and ensure the volume status is `in-use` or `available`.
   Only then you can safely perform additional size changes.
   See [AWS documentation](https://docs.aws.amazon.com/ebs/latest/userguide/modify-volume-requirements.html) for details.

## Load balancing

Deckhouse Platform supports LoadBalancer Services via AWS Load Balancer Controller.

To control how AWS resources are created, use annotations on the Service object:

```yaml
metadata:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
    service.beta.kubernetes.io/aws-load-balancer-subnets: "subnet-12345,subnet-67890"
```

Where:

- `service.beta.kubernetes.io/aws-load-balancer-type`: If set to none, only a target group is created (no LoadBalancer).
- `service.beta.kubernetes.io/aws-load-balancer-backend-protocol`: Only applicable when `aws-load-balancer-type` is `none`.
  Specifies the protocol for communication with the target group.
  Supported values:
  - `tcp` (default)
  - `tls`
  - `http`
  - `https`

{% alert level="info" %}
When you change this annotation, `cloud-controller-manager` will attempt to recreate the target group.
If it’s already associated with an NLB or ALB, the deletion will fail, and the controller will enter a retry loop.
To avoid this, manually detach the load balancer from the group.

If Ingress nodes are not available in all zones, explicitly specify subnets using `aws-load-balancer-subnets`.
{% endalert %}

### Configuring LoadBalancer when Ingress nodes are not in all zones

If your AWS cluster does not have Ingress nodes in every zone,
you must explicitly list the subnets to be used by the Load Balancer via the `service.beta.kubernetes.io/aws-load-balancer-subnets` annotation:

```yaml
metadata:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-subnets: "subnet-foo,subnet-bar"
```

This is especially important when manually configuring [Ingress controllers](../../../../glossary.html#ingress-controller) or using non-standard node [layout](../../../../glossary.html#layout).

To retrieve the current list of subnets used in your Deckhouse Platform setup, run the following:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse \
  -- deckhouse-controller module values cloud-provider-aws -o json | \
  jq -r '.cloudProviderAws.internal.zoneToSubnetIdMap'
```
