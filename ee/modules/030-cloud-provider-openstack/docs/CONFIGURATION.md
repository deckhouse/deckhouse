---
title: "Cloud provider — OpenStack: configuration"
---

The module is automatically enabled for all cloud clusters deployed in OpenStack.

You can configure the number and parameters of ordering machines in the cloud via the [`NodeGroup`](../../modules/040-node-manager/cr.html#nodegroup) custom resource of the node-manager module. Also, in this custom resource, you can specify the instance class's name for the above group of nodes (the `cloudInstances.ClassReference` NodeGroup parameter). In the case of the OpenStack-based cloud provider, the instance class is the [`OpenStackInstanceClass`](cr.html#openstackinstanceclass) custom resource that stores specific parameters of the machines.

## Parameters

The module settings are set automatically based on the placement strategy chosen. In most cases, you do not have to configure the module manually.

If you need to configure a module because, say, you have a bare metal cluster and you need to enable additional instances from vSphere, then refer to the [How to configure a Hybrid cluster in vSphere](faq.html#how-do-i-create-a-hybrid-cluster) section.

> **Note (!)** that if the parameters provided below are changed (i.e., the parameters specified in the deckhouse ConfigMap), the **existing Machines are NOT redeployed** (new machines will be created with the updated parameters). Redeployment is only performed when `NodeGroup` and `OpenStackInstanceClass` parameters are changed. You can learn more in the [node-manager](../../modules/040-node-manager/faq.html#how-do-i-redeploy-ephemeral-machines-in-the-cloud-with-a-new-configuration). module's documentation.
To authenticate using the `user-authn` module, you need to create a new `Generic` application in the project's Crowd.

* `connection` — this section contains parameters required to connect to the cloud provider's API;
  * `authURL` — an OpenStack Identity API URL.
  * `caCert` — specify the CA x509 certificate used for signing if the OpenStack API has a self-signed certificate;
    * Format — a string; The certificate must have a PEM format.
    * An optional parameter;
  * `domainName` — the domain name;
  * `tenantName` — the project name;
    * Cannot be used together with `tenantID`;
  * `tenantID` — the project id;
    * Cannot be used together with `tenantName`;
  * `username` — the name of the user that has full project privileges;
  * `password` — the user's password;
  * `region` — the OpenStack region where the cluster will be deployed;
* `internalNetworkNames` — additional networks that are connected to the VM. cloud-controller-manager uses them to insert InternalIPs into `.status.addresses` in the Node API object.
  * Format — an array of strings.

  Example:
  ```yaml
  internalNetworkNames:
  - KUBE-3
  - devops-internal
  ```

* `externalNetworkNames` — additional networks that are connected to the VM. cloud-controller-manager uses them to insert ExternalIPs into `.status.addresses` in the Node API object;
  * Format — an array of strings.

  Example:
  ```yaml
  externalNetworkNames:
  - KUBE-3
  - devops-internal
  ```

* `additionalExternalNetworkNames` — specifies additional networks that can be connected to the VM. `cloud-controller-manager` uses them to insert `ExternalIP` to `.status.addresses` field in the Node API object;
  * Format — an array of strings;

  Example:
  ```yaml
  cloudProviderOpenstack: |
    additionalExternalNetworkNames:
    - some-bgp-network
  ```

* `podNetworkMode` — sets the traffic mode for the network that the pods use to communicate with each other (usually, it is an internal network; however, there can be exceptions).
  * Possible values:
    * `DirectRouting` — means that there is a direct routing between the nodes;
    * `DirectRoutingWithPortSecurityEnabled` - direct routing is enabled between the nodes, but only if  the range of addresses of the internal network is explicitly allowed in OpenStack for Ports;
      * **Caution!** Make sure that the `username` can edit AllowedAddressPairs on Ports connected to the `internalNetworkName` network. Generally, an OpenStack user doesn't have such a privilege if the network has the `shared` flag set;
    * `VXLAN` — direct routing between the nodes isn't available; VXLAN should be used;
  * An optional parameter; By default, it is set to `DirectRoutingWithPortSecurityEnabled`;
* `instances` — instance parameters that are used when creating virtual machines:
  * `sshKeyPairName` — the name of the OpenStack `keypair` resource; it is used for provisioning instances;
    * A mandatory parameter;
    * Format — a string;
  * `securityGroups` — a list of securityGroups to assign to the provisioned instances. Defines firewall rules for the provisioned instances;
    * An optional parameter;
    * Format — an array of strings;
  * `imageName` — the name of the image;
    * An optional parameter;
    * Format — a string;
  * `mainNetwork` — the path to the network that will serve as the primary network (the default gateway) for connecting to the VM;
    * An optional parameter;
    * Format — a string;
  * `additionalNetworks` — a list of networks to connect to the instance;
    * An optional parameter;
    * Format — an array of strings;
* `loadBalancer` — Load Balancer parameters:
  * `subnetID` — an ID of the Neutron subnet to create the load balancer virtual IP in;
    * Format — a string;
    * An optional parameter;
  * `floatingNetworkID` — an ID of the external network for floating IPs;
    * Format — a string;
    * An optional parameter;
* `zones` — the default list of zones for provisioning instances. Can be redefined for each NodeGroup individually;
  * Format — an array of strings;
* `tags` — a dictionary of tags that will be available on all provisioned instances;
  * An optional parameter;
  * Format — key-value pairs;

If you have instances in the cluster that use External Networks (other than those set out in the placement strategy), you must pass them via the `additionalExternalNetworkNames` parameter.

## Storage

The module automatically creates StorageClasses that are available in OpenStack. Also, it can filter out the unnecessary StorageClasses (you can do this via the `exclude` parameter).

* `exclude` — a list of StorageClass names (or regex expressions for names) to exclude from the creation in the cluster;
  * Format — an array of strings;
  * An optional parameter;
* `default` — the name of StorageClass that will be used in the cluster by default;
  * Format — a string;
  * An optional parameter;
  * If the parameter is omitted, the default StorageClass is either: 
    * an arbitrary StorageClass present in the cluster that has the default annotation;
    * the first StorageClass created by the module (in accordance with the order in OpenStack).
* `topologyEnabled` - this feature enables driver to consider the topology constraints while creating the volume. It is used only during volume provisioning, existing PersistentVolumes are not affected;
  * **Attention!** If it is set to `false` all-new PersistentVolumes are provisioned without topology constraints.
  * OpenStackClusterConfiguration has parameter [bindVolumesToZone](https://deckhouse.io/en/documentation/v1/modules/030-cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration), that configures volume bindings to availability zone when you use dhctl.
  * Format — bool. An optional parameter;
  * Set to `true` by default.

```yaml
cloudProviderOpenstack: |
  storageClass:
    exclude:
    - .*-hdd
    - iscsi-fast
    default: ceph-ssd
```
