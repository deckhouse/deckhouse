---
title: "The virtualization module: configuration examples"
---

## Getting a list of available images

Deckhouse comes with several base images that you can use to create virtual machines. To get a list of them, run:

```shell
kubectl get cvmi
```

Example of the output:

```console
NAME           AGE
alpine-3.16    30d
centos-7       30d
centos-8       30d
debian-9       30d
debian-10      30d
fedora-36      30d
rocky-9        30d
ubuntu-16.04   30d
ubuntu-18.04   30d
ubuntu-20.04   30d
ubuntu-22.04   30d
```

## Creating a virtual machine

The minimal resource for creating a virtual machine looks like this:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachine
metadata:
  name: vm100
  namespace: default
spec:
  running: true
  resources:
    memory: 512M
    cpu: "1"
  userName: ubuntu
  sshPublicKey: "ssh-rsa asdasdkflkasddf..."
  bootDisk:
    source:
      kind: ClusterVirtualMachineImage
      name: ubuntu-22.04
    size: 10Gi
    storageClassName: linstor-thindata-r2
    autoDelete: true
```

In the [bootDisk](cr.html#virtualmachine-v1alpha1-spec-bootdisk) parameter, you can also specify the name of an existing virtual machine disk. In this case, it will be connected to it directly without performing a clone operation.
This parameter also defines the name of the disk to be created. If it is not specified, the default template is `<vm_name>-boot`.

An example:

```yaml
bootDisk:
  name: "myos"
  size: 10Gi
  autoDelete: false
```

The [autoDelete](cr.html#virtualmachine-v1alpha1-spec-bootdisk-autodelete) parameter allows you to specify whether the disk should be deleted after deleting the virtual machine.

## Working with IP addresses

Each virtual machine is assigned a separate IP address, which it uses throughout its life.  
For this, the IPAM (IP Address Management) mechanism is used, which represents two resources: [VirtualMachineIPAddressClaim](cr.html#virtualmachineipaddressclaim) and [VirtualMachineIPAddressLease](cr.html#virtualmachineipaddresslease).

While `VirtualMachineIPAddressLease` is a clustered resource and reflects the fact that the address for the virtual machine has been issued. The `VirtualMachineIPAddressClaim` is a user resource and is used to request such an address. By creating a `VirtualMachineIPAddressClaim` you can request the desired IP address for the virtual machine.

An example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineIPAddressClaim
metadata:
  name: vm100
  namespace: default
spec:
  address: 10.10.10.10
  static: true
```

The desired IP address should be in the range of networks defined in the [vmCIDRs](configuration.html#parameters-vmcidrs) parameter of the module configuration and should not be occupied by any other virtual machine.

If the `VirtualMachineIPAddressClaim` was not created by the user beforehand, then it will be created automatically with the virtual machine creation. In this case, the next free IP address in the `vmCIDRs` range will be assigned.  

When deleting the virtual machine, the `VirtualMachineIPAddressClaim` associated with it will also be deleted. To prevent this from happening, you need to mark such an IP address as static. To do this, you need to edit the generated `VirtualMachineIPAddressClaim` and set the `static: true` field in it. After deleting the VM, the static IP address remains reserved in the namespace.

Use the following command to get the list of all issued IP addresses:

```shell
kubectl get vmip
```

Example of the output:

```console
NAME    ADDRESS       STATIC   STATUS   VM      AGE
vm1     10.10.10.0    false    Bound    vm1     9d
vm100   10.10.10.10   true     Bound    vm100   172m
```

Just delete the `VirtualMachineIPAddressClaim` resource to free up the IP address:

```shell
kubectl delete vmip vm100
```

`VirtualMachineIPAddressClaim` is named as the virtual machine by default, but it is possible to pass any other arbitrary name by using the [ipAddressClaimName](cr.html#virtualmachine-v1alpha1-spec-ipaddressclaimname) parameter.

## Creating disks for storing persistent data

Additional disks should be created manually:

En example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachineDisk
metadata:
  name: mydata
spec:
  storageClassName: linstor-data
  size: 10Gi
```

It is possible to create a disk from an existing image, just specify the corresponding [ClusterVirtualMachineImage](cr.html#clustervirtualmachineimage) in the [source](cr.html#virtualmachinedisk-v1alpha1-spec-source) parameter.

An example:

```yaml
source:
  kind: ClusterVirtualMachineImage
  name: centos-7
```

Attaching additional disks is done using the [diskAttachments](cr.html#virtualmachine-v1alpha1-spec-diskattachments) parameter.

An example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachine
metadata:
  name: vm100
  namespace: default
spec:
  running: true
  resources:
    memory: 512M
    cpu: "1"
  userName: admin
  sshPublicKey: "ssh-rsa asdasdkflkasddf..."
  bootDisk:
    source:
      kind: ClusterVirtualMachineImage
      name: ubuntu-22.04
    size: 10Gi
    storageClassName: linstor-fast
    autoDelete: true
  diskAttachments:
  - name: mydata
    bus: virtio
```

## Using cloud-init

Optionally, you can pass the cloud-init configuration in the [cloud-init](cr.html#virtualmachine-v1alpha1-spec-cloudinit) parameter.

An example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VirtualMachine
metadata:
  name: vm1
  namespace: default
spec:
  running: true
  resources:
    memory: 512M
    cpu: "1"
  userName: admin
  sshPublicKey: "ssh-rsa asdasdkflkasddf..."
  bootDisk:
    source:
      kind: ClusterVirtualMachineImage
      name: ubuntu-22.04
    size: 10Gi
  cloudInit:
    userData: |-
      password: hackme
      chpasswd: { expire: False }
```

The cloud-init configuration can also be saved in Secret and passed to the virtual machine like this:

```yaml
cloudInit:
  secretRef:
    name: my-vmi-secret
```
