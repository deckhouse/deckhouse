---
title: "Cloud provider — VMware vSphere: provider configuration"
---

## VsphereClusterConfiguration
A particular placement strategy is defined via the `VsphereClusterConfiguration` struct. It has the following fields:
* `layout` — the way resources are located in the cloud;
  * Possible values: `Standard` (the description is provided below);
* `provider` — parameters for connecting to the vCenter;
  * `server` — the host or the IP address of the vCenter server;
  * `username` — the login ID;
  * `password` — the password;
  * `insecure` — can be set to `true` if vCenter has a self-signed certificate.
    * Format — boolean;
    * An optional parameter. It is set to `false` by default;
* `masterNodeGroup` — parameters of the master's NodeGroup;
  * `replicas` — the number of master nodes to create;
  * `zones` — nodes can only be created in these zones;
  * `instanceClass` — partial contents of the fields of the [VsphereInstanceClass](cr.html#vsphereinstanceclass) CR. Mandatory parameters: `numCPUs`, `memory`, `template`, `mainNetwork`, `datastore`.  The parameters in **bold** are unique for  `VsphereClusterConfiguration`. Possible parameters:
    * `numCPUs`
    * `memory`
    * `template`
    * `mainNetwork`
    * `additionalNetworks`
    * `datastore`
    * `rootDiskSize`
    * `resourcePool`
    * `runtimeOptions`
      * `nestedHardwareVirtualization`
      * `cpuShares`
      * `cpuLimit`
      * `cpuReservation`
      * `memoryShares`
      * `memoryLimit`
      * `memoryReservation`
    * **`mainNetworkIPAddresses`** —  a list of static IP addresses (with a CIDR prefix) sequentially allocated to master nodes in the `mainNetwork`;
      * An optional parameter. By default, the DHCP client is enabled;
      * `address` — an IP address with a CIDR prefix;
        * An example: `10.2.2.2/24`;
      * `gateway` — the IP address of the default gateway. It must be located in the subnet specified in the `address` parameter;
        * An example: `10.2.2.254`;
      * `nameservers`
        * `addresses` — a list of DNS servers;
          * An example: `- 8.8.8.8`;
        * `search` — a list of DNS search domains;
          * An example: `- tech.lan`;
* `nodeGroups` — an array of additional NodeGroups for creating static nodes (e.g., for dedicated front nodes or gateways). NodeGroup parameters:
  * `name` — the name of the NodeGroup to use for generating node names;
  * `replicas` — the number of nodes to create;
  * `zones` — nodes can only be created in these zones;
  * `instanceClass` — partial contents of the fields of the [VsphereInstanceClass](cr.html#vsphereinstanceclass) CR. Mandatory parameters: `numCPUs`, `memory`, `template`, `mainNetwork`, `datastore`.  The parameters in **bold** are unique for  `VsphereClusterConfiguration`. Possible parameters:
    * `numCPUs`
    * `memory`
    * `template`
    * `mainNetwork`
    * `additionalNetworks`
    * `datastore`
    * `rootDiskSize`
    * `resourcePool`
    * `runtimeOptions`
      * `nestedHardwareVirtualization`
      * `cpuShares`
      * `cpuLimit`
      * `cpuReservation`
      * `memoryShares`
      * `memoryLimit`
      * `memoryReservation`
    * **`mainNetworkIPAddresses`** — a list of static IP addresses (with a CIDR prefix) sequentially allocated to master nodes in the `mainNetwork`;
      * An optional parameter. By default, the DHCP client is enabled;
      * `address` — an IP address with a CIDR prefix;
        * An example: `10.2.2.2/24`;
      * `gateway` — the IP address of the default gateway. It must be located in the subnet specified in the `address` parameter;
        * An example: `10.2.2.254`;
      * `nameservers`
        * `addresses` — an array of DNS servers;
          * An example: `- 8.8.8.8`;
        * `search` — an array of DNS search domains;
          * An example: `- tech.lan`;
  * `nodeTemplate` — parameters of Node objects in Kubernetes to add after registering the node;
    * `labels` — the same as the `metadata.labels` standard [field](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta);
      * An example:
        ```yaml
        labels:
          environment: production
          app: warp-drive-ai
        ```
    * `annotations` — the same as the `metadata.annotations` standard [field](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta);
      * An example
        ```yaml
        annotations:
          ai.fleet.com/discombobulate: "true"
        ```
    * `taints` — the same as the `.spec.taints` field of the [Node](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#taint-v1-core) object. **Caution!** Only the `effect`, `key`, `values` fields are available;
      * An example

        ```yaml
        taints:
        - effect: NoExecute
          key: ship-class
          value: frigate
        ```
* `internalNetworkCIDR` — subnet for master nodes in the internal network. Addresses are allocated starting with the tenth address. E.g., if you have the `192.168.199.0/24` subnet, addresses will be allocated starting with  `192.168.199.10`. The `internalNetworkCIDR` is used if `additionalNetworks` are defined in `masterInstanceClass`;
* `vmFolderPath` — the path to the VirtualMachine Folder where the cloned VMs will be created;
  * An example: `dev/test`;
* `regionTagCategory`— the name of the tag **category** used to identify the region (vSphere Datacenter);
  * Format — a string;
  * An optional parameter. By default, it is set to `k8s-region`;
* `zoneTagCategory` — the name of the tag **category** used to identify the zone (vSphere Cluster);
  * Format — a string;
  * An optional parameter; By default, it is set to `k8s-zone`;
* `disableTimesync` — disable time synchronization on the vSphere side. **Caution!** Note that this parameter will not disable the NTP daemons in the guest OS, but only disable the time correction on the part of ESXi;
  * Format — boolean;
  * An optional parameter; It is set to `true` by default;
* `region` — is a tag added to the vSphere Datacenter where all actions will occur: provisioning VirtualMachines, storing virtual disks on datastores, connecting to the network.
* `baseResourcePool` — a path (relative to vSphere Cluster) to the existing parent `resourcePool` for all `resourcePool` created in each zone;
* `sshPublicKey` — a public key for accessing nodes;
* `externalNetworkNames` — names of networks (just the name and not the full path) connected to VirtualMachines and used by vsphere-cloud-controller-manager to insert ExternalIP into the `.status.addresses` field in the Node API object.
  * Format — an array of strings. For example:

    ```yaml
    externalNetworkNames:
    - MAIN-1
    - public
    ```

    * An optional parameter;
* `internalNetworkNames` — names of networks (just the name and not the full path) connected to VirtualMachines and used by vsphere-cloud-controller-manager to insert InternalIP into the `.status.addresses` field in the Node API object.
  * Format — an array of strings. For example:

    ```yaml
    internalNetworkNames:
    - KUBE-3
    - devops-internal
    ```

  * An optional parameter;
* `zones` — a limited set of zones in which nodes can be created;
  * A mandatory parameter;
  * Format — an array of strings;
