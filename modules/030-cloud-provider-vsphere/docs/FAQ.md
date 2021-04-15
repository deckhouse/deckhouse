---
title: "Сloud provider — VMware vSphere: FAQ"
---

## How do I create a hybrid cluster?

A hybrid cluster combines bare-metal and vSphere nodes. To create such a cluster, you will need an L2 network between all nodes of the cluster.

1. Delete flannel from kube-system:  `kubectl -n kube-system delete ds flannel-ds`;
2. Enable the module and specify the necessary [parameters](#configuration-parameters).

**Caution!** Cloud-controller-manager synchronizes vSphere and Kubernetes states by deleting in Kubernetes nodes that are not in vSphere. In a hybrid cluster, such behavior does not always make sense. That is why cloud-controller-manager automatically skips Kubernetes nodes that do not have the `--cloud-provider=external` parameter set (Deckhouse inserts `static://` to nodes in `.spec.providerID`, and cloud-controller-manager ignores them).

### Configuration parameters

**Note** that if the parameters provided below are changed (i.e., the parameters specified in the deckhouse ConfigMap), the **existing Machines are NOT redeployed** (new machines will be created with the updated parameters). Re-deployment is only performed when `NodeGroup` and `VsphereInstanceClass` are changed. You can learn more in the [node-manager module's documentation](/modules/040-node-manager/faq.html#how-do-i-redeploy-ephemeral-machines-in-the-cloud-with-a-new-configuration).

* `host` — the domain of the vCenter server;
* `username` — the login ID;
* `password` — the password;
* `vmFolderPath` — the path to the VirtualMachine Folder where the cloned VMs will be created;
  * e.g., `dev/test`;
* `insecure` — can be set to `true` if vCenter has a self-signed certificate;
  * Format — bool;
  * An optional parameter; by default `false`;
* `regionTagCategory`— the name of the tag **category** used to identify the region (vSphere Datacenter);
  * Format — string;
  * An optional parameter; by default `k8s-region`;
* `zoneTagCategory` — the name of the tag **category** used to identify the region (vSphere Cluster).
    * Format — string;
    * An optional parameter; by default `k8s-zone`;

* `disableTimesync` — disable time synchronization on the vSphere side. **Note** that this parameter will not disable the NTP daemons in the guest OS, but only disable the time correction on the part of ESXi;
  * Format — bool.
  * An optional parameter; by default `true`;
* `region` — is a tag added to the vSphere Datacenter where all actions will occur: provisioning VirtualMachines, storing virtual disks on datastores, connecting to the network.
* `sshKeys` — a list of public SSH keys in plain-text format;
  * Format — an array of strings;
  * An optional parameter; by default there are no allowed keys for the user;
* `externalNetworkNames` — names of networks (just the name and not the full path) connected to VirtualMachines and used by vsphere-cloud-controller-manager to insert ExternalIP into the `.status.addresses` field in the Node API object.
  * Format — an array of strings; for eaxmple,

        ```yaml
        externalNetworkNames:
        - MAIN-1
        - public
        ```

  * An optional parameter
* `internalNetworkNames` — names of networks (just the name and not the full path) connected to VirtualMachines and used by vsphere-cloud-controller-manager to insert InternalIP into the `.status.addresses` field in the Node API object. 
  * Format — an array of strings; for example,

        ```yaml
        internalNetworkNames:
        - KUBE-3
        - devops-internal
        ```

  * An optional parameter.

#### An example

```yaml
cloudProviderVsphereEnabled: "true"
cloudProviderVsphere: |
  host: vc-3.internal
  username: user
  password: password
  vmFolderPath: dev/test
  insecure: true
  region: moscow-x001
  sshKeys:
  - "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQD5sAcceTHeT6ZnU+PUF1rhkIHG8/B36VWy/j7iwqqimC9CxgFTEi8MPPGNjf+vwZIepJU8cWGB/By1z1wLZW3H0HMRBhv83FhtRzOaXVVHw38ysYdQvYxPC0jrQlcsJmLi7Vm44KwA+LxdFbkj+oa9eT08nQaQD6n3Ll4+/8eipthZCDFmFgcL/IWy6DjumN0r4B+NKHVEdLVJ2uAlTtmiqJwN38OMWVGa4QbvY1qgwcyeCmEzZdNCT6s4NJJpzVsucjJ0ZqbFqC7luv41tNuTS3Moe7d8TwIrHCEU54+W4PIQ5Z4njrOzze9/NlM935IzpHYw+we+YR+Nz6xHJwwj i@my-PC"
  externalNetworkNames:
  - KUBE-3
  - devops-internal
  internalNetworkNames:
  - KUBE-3
  - devops-internal
```
