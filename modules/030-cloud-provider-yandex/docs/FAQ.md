---
title: "Cloud provider — Yandex Cloud: FAQ"
---

## How do I set up the INTERNAL LoadBalancer?

Attach one of the following annotations to the service:

1.
   ```yaml
   yandex.cpi.flant.com/listener-subnet-id: SubnetID
   ```
   
   The annotation links the LoadBalancer with the appropriate Subnet.

2. 
   ```yaml
   yandex.cloud/load-balancer-type: Internal
   ```

   LoadBalancer will listen to the first available subnet.

## How to reserve a public IP address?

This on is used in `externalIPAddresses` and `natInstanceExternalAddress`. It also can be used for a bastion host.

```shell
$ yc vpc address create --external-ipv4 zone=ru-central1-a
id: e9b4cfmmnc1mhgij75n7
folder_id: b1gog0h9k05lhqe5d88l
created_at: "2020-09-01T09:29:33Z"
external_ipv4_address:
  address: 178.154.226.159
  zone_id: ru-central1-a
  requirements: {}
reserved: true
```

## dhcpOptions-related problems and ways to address them

Using DNS servers that differ from those provided by Yandex Cloud in the DHCP settings is a temporary solution. It will be abandoned after Yandex Cloud will introduce the Managed DNS service. To get around the restrictions described below, we recommend using `stubZones` from the [`kube-dns`](../042-kube-dns/) module.

### Editing parameters

Pay attention to the following nuances:

1. When changing parameters, you need to invoke `netplan apply` or a similar command that forces the update of the DHCP lease.
2. You will need to restart all hostNetwork Pods (especially `kube-dns`) for the new `resolv.conf` settings to take effect.

### Aspects of the use

If the dhcpOptions parameter is set, all DNS are routed to the DNS servers specified. These DNS servers **must** serve DNS requests to the Internet and (if needed) resolve intranet resources.

**Do not use** this option if the recursive DNSs specified cannot resolve the same list of zones that the recursive DNSs in the Yandex Cloud subnet can resolve.

## How to set a custom StorageClass as default?

Do the following to set a custom StorageClass as default:

1. Add `storageclass.kubernetes.io/is-default-class='true'` annotation to the StorageClass:

   ```shell
   kubectl annotate sc $STORAGECLASS storageclass.kubernetes.io/is-default-class='true'
   ```

2. Specify the StorageClass name in the [storageClass.default](configuration.html#parameters-storageclass-default) parameter in the `cloud-provider-yandex` module settings. Note that after doing so, the `storageclass.kubernetes.io/is-default-class='true'` annotation will be removed from the StorageClass that was previously listed in the module settings as the default one.

   ```shell
   kubectl edit mc cloud-provider-yandex
   ```

## Adding CloudStatic nodes to a cluster

For VMs that you want to add to the cluster as nodes, add the `node-network-cidr` key to the metadata (Edit VM -> Metadata) with a value equal to the cluster's `nodeNetworkCIDR`.

You can find out the `nodeNetworkCIDR` of the cluster using the command below:

```shell
kubectl -n kube-system get secret d8-provider-cluster-configuration -o json | jq --raw-output '.data."cloud-provider-cluster-configuration.yaml"' | base64 -d | grep '^nodeNetworkCIDR'
```

## How do I create a cluster in a new VPC and set up bastion host to access the nodes?

1. Bootstrap the base-infrastructure of the cluster:

   ```shell
   dhctl bootstrap-phase base-infra --config config.yml
   ```

2. Create a bastion host:

   ```shell
   yc compute instance create \
   --name bastion \
   --hostname bastion \
   --create-boot-disk image-family=ubuntu-2204-lts,image-folder-id=standard-images,size=20,type=network-hdd \
   --memory 2 \
   --cores 2 \
   --core-fraction 100 \
   --ssh-key ~/.ssh/id_rsa.pub \
   --zone ru-central1-a \
   --public-address 178.154.226.159
   ```

3. Continue installing the cluster by specifying the bastion host data. Answer `y` to the question about the Terraform cache:

   ```shell
   dhctl bootstrap --ssh-bastion-host=178.154.226.159 --ssh-bastion-user=yc-user \
   --ssh-user=ubuntu --ssh-agent-private-keys=/tmp/.ssh/id_rsa --config=/config.yml --resources=/resources.yml
   ```
