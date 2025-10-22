---
title: Storage and load balancing
permalink: en/admin/integrations/public/amazon/storage.html
---

This section covers the following additional aspects of integrating Deckhouse Kubernetes Platform (DKP) with AWS:

- Connecting cloud disks using CSI.
- Automatic StorageClass creation.
- Using LoadBalancer.
- Accessing via bastion host.
- Connecting manually created CloudStatic nodes.

## Storage (CSI and StorageClass)

DKP integrates with AWS storage via CSI.
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
   kubectl describe pvc <claim-name>
   ```

1. After resizing, wait at least 6 hours and ensure the volume status is `in-use` or `available`.
   Only then you can safely perform additional size changes.
   See [AWS documentation](https://docs.aws.amazon.com/ebs/latest/userguide/modify-volume-requirements.html) for details.

## Load balancing

DKP supports LoadBalancer Services via AWS Load Balancer Controller.

To control how AWS resources are created, use annotations on the Service object:

```yaml
metadata:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
    service.beta.kubernetes.io/aws-load-balancer-subnets: "subnet-12345,subnet-67890"
```

Where:

- `service.beta.kubernetes.io/aws-load-balancer-type`: If set to none, only a Target Group is created (no LoadBalancer).
- `service.beta.kubernetes.io/aws-load-balancer-backend-protocol`: Only applicable when `aws-load-balancer-type` is `none`.
  Specifies the protocol for communication with the Target Group.
  Supported values:
  - `tcp` (default)
  - `tls`
  - `http`
  - `https`

{% alert level="info" %}
When you change this annotation, `cloud-controller-manager` will attempt to recreate the Target Group.
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

This is especially important when manually configuring Ingress controllers or using non-standard node layout.

To retrieve the current list of subnets used in your DKP setup, run the following:

```shell
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse \
  -- deckhouse-controller module values cloud-provider-aws -o json | \
  jq -r '.cloudProviderAws.internal.zoneToSubnetIdMap'
```

## Connecting CloudStatic nodes

To connect manually created EC2 instances to a DKP cluster, follow these steps:

1. Attach the IAM role `<prefix>-node`.
1. Attach the security group `<prefix>-node`.
1. Add the following tags:

   ```yaml
   "kubernetes.io/cluster/<cluster_uuid>" = "shared"
   "kubernetes.io/cluster/<prefix>" = "shared"
   ```

To get the `cluster_uuid`, use the following command:

```shell
kubectl -n kube-system get cm d8-cluster-uuid -o json | jq -r '.data."cluster-uuid"'
```

To get the `prefix`, use the following command:

```shell
kubectl -n kube-system get secret d8-cluster-configuration -o json | \
  jq -r '.data."cluster-configuration.yaml"' | base64 -d | grep prefix
```
