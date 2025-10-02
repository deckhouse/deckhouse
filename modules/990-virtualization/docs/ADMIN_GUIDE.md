---
title: "Admin guide"
weight: 40
---

## Introduction

This guide is intended for administrators of Deckhouse Virtualization Platform (DVP) and describes how to create and modify cluster resources.

The administrator also has rights to manage project resources, which are described in the [User guide](./user_guide.html).

## Module parameters

The configuration of the `virtualization` module is specified via the ModuleConfig resource in YAML format. The following is an example of a basic configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  enabled: true
  version: 1
  settings:
    dvcr:
      storage:
        persistentVolumeClaim:
          size: 50G
          storageClassName: sds-replicated-thin-r1
        type: PersistentVolumeClaim
    virtualMachineCIDRs:
      - 10.66.10.0/24
```

How to configure the `virtualization` module in the web interface:

- Go to the "System" tab, then to the `Deckhouse` -> "Modules" section.
- Select the `virtualization` module from the list.
- In the pop-up window, select the "Configuration" tab.
- To display the settings, click the "Advanced settings" switch.
- Configure the settings. The names of the fields on the form correspond to the names of the parameters in YAML.
- To apply the settings, click the "Save" button.

### Parameter description

**Enable the module**

The module state is controlled through the `.spec.enabled` field. Specify:

- `true`: To enable the module.
- `false`: To disable the module.

**Configuration version**

The `.spec.version` parameter defines the version of the configuration schema. The parameter structure may change between versions. The current values are given in the settings section.

**Deckhouse Virtualization Container Registry (DVCR)**

The `.spec.settings.dvcr.storage` block configures a persistent volume for storing images:

- `.spec.settings.dvcr.storage.persistentVolumeClaim.size`: Volume size (for example, `50G`). To expand the storage, increase the value of the parameter.
- `.spec.settings.dvcr.storage.persistentVolumeClaim.storageClassName`: StorageClass name (for example, `sds-replicated-thin-r1`).

{% alert level="warning" %}
The storage serving this storage class (`.spec.settings.dvcr.storage.persistentVolumeClaim.storageClassName`) must be accessible on the nodes where DVCR is running (system nodes, or worker nodes if there are no system nodes).
{% endalert %}

**Network settings**

The `.spec.settings.virtualMachineCIDRs` block specifies subnets in CIDR format (for example, `10.66.10.0/24`). IP addresses for virtual machines are allocated from these ranges automatically or on request.

Example:

```yaml
spec:
  settings:
    virtualMachineCIDRs:
      - 10.66.10.0/24
      - 10.66.20.0/24
      - 10.77.20.0/16
```

The first and the last subnet address are reserved and not available for use.

{% alert level="warning" %}
The subnets in the `.spec.settings.virtualMachineCIDRs` block must not overlap with cluster node subnets, services subnet, or pods subnet (`podCIDR`).

It is forbidden to delete subnets if addresses from them have already been issued to virtual machines.
{% endalert %}

**Storage class settings for images**

The storage class settings for images are defined in the `.spec.settings.virtualImages` parameter of the module settings.

Example:

```yaml
spec:
  ...
  settings:
    virtualImages:
      allowedStorageClassNames:
      - sc-1
      - sc-2
      defaultStorageClassName: sc-1
```

Where:

- `allowedStorageClassNames` (optional): A list of the allowed StorageClasses for creating a VirtualImage that can be explicitly specified in the resource specification.
- `defaultStorageClassName` (optional): The StorageClass used by default when creating a VirtualImage if the `.spec.persistentVolumeClaim.storageClassName` parameter is not set.

**Storage class settings for disks**

The storage class settings for disks are defined in the `.spec.settings.virtualDisks` parameter of the module settings.

Example:

```yaml
spec:
  ...
  settings:
    virtualDisks:
      allowedStorageClassNames:
      - sc-1
      - sc-2
      defaultStorageClassName: sc-1
```

Where:

- `allowedStorageClassNames` (optional): A list of the allowed StorageClass for creating a VirtualDisk that can be explicitly specified in the resource specification.
- `defaultStorageClassName` (optional): The StorageClass used by default when creating a VirtualDisk if the `.spec.persistentVolumeClaim.storageClassName` parameter is not specified.

**Security Event Audit**

{% alert level="warning" %}
Not available in CE edition.
{% endalert %}

{% alert level="warning" %}
To set up auditing, the following modules must be enabled:

- `log-shipper`,
- `runtime-audit-engine`.
{% endalert %}

To enable security event auditing, set the module’s `.spec.settings.audit.enabled` parameter to` true`:

```yaml
spec:
  enabled: true
  settings:
    audit:
      enabled: true
```

{% alert level="info" %}
For a complete list of configuration options, see [Configuration](./configuration.html).
{% endalert %}

## Images

The ClusterVirtualImage resource is used to load virtual machine images into the intra-cluster storage. After that it can be used to create virtual machine disks. It is available in all cluster namespaces and projects.

The image creation process includes the following steps:

1. The user creates a ClusterVirtualImage resource.
1. Once created, the image is automatically uploaded from the source specified in the specification to the storage (DVCR).
1. Once the upload is complete, the resource becomes available for disk creation.

There are different types of images:

- **ISO image**: An installation image used for the initial installation of an operating system (OS). Such images are released by OS vendors and are used for installation on physical and virtual servers.
- **Preinstalled disk image**: contains an already installed and configured operating system ready for use after the virtual machine is created. You can obtain pre-configured images from the distribution developers' resources or create them manually.

Examples of resources for obtaining virtual machine images:

| Distribution                                                                      | Default user.             |
| --------------------------------------------------------------------------------- | ------------------------- |
| [AlmaLinux](https://almalinux.org/get-almalinux/#Cloud_Images)                    | `almalinux`               |
| [AlpineLinux](https://alpinelinux.org/cloud/)                                     | `alpine`                  |
| [CentOS](https://cloud.centos.org/centos/)                                        | `cloud-user`              |
| [Debian](https://cdimage.debian.org/images/cloud/)                                | `debian`                  |
| [Rocky](https://rockylinux.org/download/)                                         | `rocky`                   |
| [Ubuntu](https://cloud-images.ubuntu.com/)                                        | `ubuntu`                  |

The following preinstalled image formats are supported:

- `qcow2`
- `raw`
- `vmdk`
- `vdi`

Image files can also be compressed with one of the following compression algorithms: `gz`, `xz`.

Once a resource is created, the image type and size are automatically determined, and this information is reflected in the resource status.

Images can be downloaded from various sources, such as HTTP servers where image files are located or container registries. It is also possible to download images directly from the command line using the `curl` utility.

Images can be created from other images and virtual machine disks.

For a full description of the ClusterVirtualImage resource configuration parameters, refer to [Custom Resources](cr.html#clustervirtualimage).

### Creating an image from an HTTP server

In this example, let's create a cluster image.

1. To create a ClusterVirtualImage resource, run the following command:

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: ubuntu-22-04
   spec:
     # Source for creating an image.
     dataSource:
       type: HTTP
       http:
         url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
   EOF
   ```

1. To verify that the ClusterVirtualImage has been created, run the following command:

   ```bash
   d8 k get clustervirtualimage ubuntu-22-04

   # A short version of the command.
   d8 k get cvi ubuntu-22-04
   ```

   In the output, you should see information about the resource:

   ```console
   NAME           PHASE   CDROM   PROGRESS   AGE
   ubuntu-22-04   Ready   false   100%       23h
   ```

Once created, the ClusterVirtualImage resource can be in one of the following states (phases):

- `Pending`: Waiting for all dependent resources required for image creation to be ready.
- `WaitForUserUpload`: Waiting for the user to upload the image (this phase is present only for `type=Upload`).
- `Provisioning`: The image is being created.
- `Ready`: The image has been created and is ready for use.
- `Failed`: An error occurred when creating the image.
- `Terminating`: The image is being deleted. It may "get stuck" in this state if it is still connected to the virtual machine.

As long as the image has not entered the `Ready` phase, the contents of the `.spec` block can be changed. If you change it, the disk creation process will start again. Once it is in the `Ready` phase, the `.spec` block contents **cannot be changed**.

Diagnosing problems with a resource is done by analyzing the information in the `.status.conditions` block.

You can trace the image creation process by adding the `-w` key to the command used for verification of the created resource:

```bash
d8 k get cvi ubuntu-22-04 -w
```

Example output:

```console
NAME           PHASE          CDROM   PROGRESS   AGE
ubuntu-22-04   Provisioning   false              4s
ubuntu-22-04   Provisioning   false   0.0%       4s
ubuntu-22-04   Provisioning   false   28.2%      6s
ubuntu-22-04   Provisioning   false   66.5%      8s
ubuntu-22-04   Provisioning   false   100.0%     10s
ubuntu-22-04   Provisioning   false   100.0%     16s
ubuntu-22-04   Ready          false   100%       18s
```

You can get additional information about the downloaded image from the description of the ClusterVirtualImage resource.
To check on the description, run the following command:

```bash
d8 k describe cvi ubuntu-22-04
```

How to create an image from an HTTP server in the web interface:

- Go to the "System" tab, then to the "Virtualization" -> "Cluster Images" section.
- Click "Create Image", then select "Download Data via Link (HTTP)" from the drop-down menu.
- Enter the image name in the "Image Name" field.
- Specify the link to the image in the "URL" field.
- Click "Create".
- Wait until the image status changes to `Ready`.

### Creating an image from a container registry

An image stored in a container registry has a certain format. Let's look at an example:

1. First, download the image locally:

   ```bash
   curl -L https://cloud-images.ubuntu.com/minimal/releases/jammy/release/ubuntu-22.04-minimal-cloudimg-amd64.img -o ubuntu2204.img
   ```

1. Next, create a `Dockerfile` with the following contents:

   ```Dockerfile
   FROM scratch
   COPY ubuntu2204.img /disk/ubuntu2204.img
   ```

1. Build the image and upload it to the container registry. The example below uses `docker.io` as the container registry. You would need to have a service account and a configured environment to run it.

   ```bash
   docker build -t docker.io/<username>/ubuntu2204:latest
   ```

   Where `username` is the username specified when registering at `docker.io`.

1. Upload the created image to the container registry:

   ```bash
   docker push docker.io/<username>/ubuntu2204:latest
   ```

1. To use this image, create a resource as an example:

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: ubuntu-2204
   spec:
     dataSource:
       type: ContainerImage
       containerImage:
         image: docker.io/<username>/ubuntu2204:latest
   EOF
   ```

How to create an image from the container registry in the web interface:

- Go to the "System" tab, then to the "Virtualization" -> "Cluster Images" section.
- Click "Create Image", then select "Load Data from Container Image" from the drop-down list.
- Enter the image name in the "Image Name" field.
- Specify the link to the image in the "Image in Container Registry" field.
- Click "Create".
- Wait until the image changes to the `Ready` status.

### Uploading an image via CLI

1. To upload an image using CLI, first create the following resource as shown below with the ClusterVirtualImage example:

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: some-image
   spec:
     dataSource:
       type: Upload
   EOF
   ```

   Once created, the resource will enter the `WaitForUserUpload` phase, which means it is ready for uploading the image.

1. There are two options available for uploading: from a cluster node and from an arbitrary node outside the cluster:

   ```bash
   d8 k get cvi some-image -o jsonpath="{.status.imageUploadURLs}"  | jq
   ```

   Example output:

   ```console
   {
     "external":"https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm",
     "inCluster":"http://10.222.165.239/upload"
   }
   ```

   Where:

   - `inCluster`: A URL used to download the image from one of the cluster nodes.
   - `external`: A URL used in all other cases.

1. As an example, download the Cirros image:

   ```bash
   curl -L http://download.cirros-cloud.net/0.5.1/cirros-0.5.1-x86_64-disk.img -o cirros.img
   ```

1. Upload the image using the following command:

   ```bash
   curl https://virtualization.example.com/upload/g2OuLgRhdAWqlJsCMyNvcdt4o5ERIwmm --progress-bar -T cirros.img | cat
   ```

1. After the upload is complete, the image should have been created and entered the `Ready` phase:
   To verify this, run the following command:

   ```bash
   d8 k get cvi some-image
   ```

   Example output:

   ```console
   NAME         PHASE   CDROM   PROGRESS   AGE
   some-image   Ready   false   100%       1m
   ```

How to perform the operation in the web interface:

- Go to the "System" tab, then to the "Virtualization" -> "Cluster Images" section.
- Click "Create Image", then select "Upload from Computer" from the drop-down menu.
- Enter the image name in the "Image Name" field.
- In the "Upload File" field, click the "Select File on Your Computer" link.
- Select the file in the file manager that opens.
- Click the "Create" button.
- Wait until the image changes to `Ready` status.

## Virtual machine classes

The VirtualMachineClass resource is designed for centralized configuration of preferred virtual machine settings. It allows you to define CPU instructions, configuration policies for CPU and memory resources for virtual machines, as well as define ratios of these resources. In addition, VirtualMachineClass provides management of virtual machine placement across platform nodes. This allows administrators to effectively manage virtualization platform resources and optimally place virtual machines on platform nodes.

By default, a single VirtualMachineClass `generic` resource is automatically created, which represents a universal CPU model that uses the rather old but supported by most modern processors Nehalem model. This allows you to run VMs on any nodes in the cluster with the possibility of live migration.

{% alert level="info" %}
It is recommended that you create at least one VirtualMachineClass resource in the cluster with the `Discovery` type immediately after all nodes are configured and added to the cluster. This allows virtual machines to utilize a generic CPU with the highest possible CPU performance considering the CPUs on the cluster nodes. This allows the virtual machines to utilize the maximum CPU capabilities and migrate seamlessly between cluster nodes if necessary.

For a configuration example, see [vCPU Discovery configuration example](#vcpu-discovery-configuration-example)
{% endalert %}

To list all VirtualMachineClass resources, run the following command:

```bash
d8 k get virtualmachineclass
```

Example output:

```console
NAME               PHASE   AGE
generic            Ready   6d1h
```

Make sure to specify the VirtualMachineClass resource in the virtual machine configuration.
The following is an example of specifying a class in the VM specification:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: linux-vm
spec:
  virtualMachineClassName: generic # VirtualMachineClass resource name.
  ...
```

### VirtualMachineClass settings

The VirtualMachineClass resource structure is as follows:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: <vmclass-name>
spec:
  # The section describes virtual processor parameters for virtual machines.
  # This block cannot be changed after the resource has been created.
  cpu: ...

  # (optional) Describes the rules for allocating virtual machines between nodes.
  # When changed, it is automatically applied to all virtual machines using this VirtualMachineClass.
  nodeSelector: ...

  # (optional) Describes the sizing policy for configuring virtual machine resources.
  # When changed, it is automatically applied to all virtual machines using this VirtualMachineClass.
  sizingPolicies: ...
```

How to configure VirtualMachineClass in the web interface:

- Go to the "System" tab, then to the "Virtualization" -> "VM Classes" section.
- Click the "Create" button.
- In the window that opens, enter a name for the VM class in the "Name" field.

Next, let's take a closer look at the setting blocks.

#### vCPU settings

The `.spec.cpu` block allows you to specify or configure the vCPU for the VM.

{% alert level="warning" %}
Settings in the `.spec.cpu` block cannot be changed after the VirtualMachineClass resource is created.
{% endalert %}

Examples of the `.spec.cpu` block settings:

- A class with a vCPU with the required set of processor instructions. In this case, use `type: Features` to specify the required set of supported instructions for the processor:

  ```yaml
  spec:
    cpu:
      features:
        - vmx
      type: Features
  ```

  How to configure vCPU in the web interface in the [VM class creation form](#virtualmachineclass-settings):

  - In the "CPU Settings" block, select `Features` in the "Type" field.
  - In the "Required set of supported instructions" field, select the instructions you need for the processor.
  - To create a VM class, click the "Create" button.

- A class with a universal vCPU for a given set of nodes. In this case, use `type: Discovery`:

  ```yaml
  spec:
    cpu:
      discovery:
        nodeSelector:
          matchExpressions:
            - key: node-role.kubernetes.io/control-plane
              operator: DoesNotExist
      type: Discovery
  ```

  How to perform the operation in the web interface in the [VM class creation form](#virtualmachineclass-settings):

  - In the "CPU Settings" block, select `Discovery` in the "Type" field.
  - Click "Add" in the "Conditions for creating a universal processor" -> "Labels and expressions" block.
  - In the pop-up window, you can set the "Key", "Operator" and "Value" of the key that corresponds to the `spec.cpu.discovery.nodeSelector` settings.
  - To confirm the key parameters, click the "Enter" button.
  - To create a VM class, click the "Create" button.

- The vmclass with `type: Host` uses a virtual vCPU that matches the platform node's vCPU instruction set as closely as possible, ensuring high performance and functionality. It also guarantees compatibility with live migration for nodes with similar vCPU types. For example, it is not possible to migrate a virtual machine between nodes with Intel and AMD processors. This also applies to processors of different generations, as their instruction sets may differ.

  ```yaml
  spec:
    cpu:
      type: Host
  ```

  How to perform the operation in the web interface in the [VM class creation form](#virtualmachineclass-settings):

  - In the "CPU Settings" block, select `Host` in the "Type" field.
  - To create a VM class, click the "Create" button.

- A vmclass with `type: HostPassthrough` uses a physical CPU of the platform node without modification. A virtual machine using this class can only be migrated to a node that has a CPU that exactly matches the CPU of the source node.

  ```yaml
  spec:
    cpu:
      type: HostPassthrough
  ```

  How to perform the operation in the web interface in the [VM class creation form](#virtualmachineclass-settings):

  - In the "CPU Settings" block, select `HostPassthrough` in the "Type" field.
  - To create a VM class, click the "Create" button.

- To create a vCPU of a specific CPU with a predefined instruction set, use `type: Model`. To get a list of supported CPU names for the cluster node, run the command in advance:

  ```bash
  d8 k get nodes <node-name> -o json | jq '.metadata.labels | to_entries[] | select(.key | test("cpu-model.node.virtualization.deckhouse.io")) | .key | split("/")[1]' -r
  ```

  Example output:

  ```console
  Broadwell-noTSX
  Broadwell-noTSX-IBRS
  Haswell-noTSX
  Haswell-noTSX-IBRS
  IvyBridge
  IvyBridge-IBRS
  Nehalem
  Nehalem-IBRS
  Penryn
  SandyBridge
  SandyBridge-IBRS
  Skylake-Client-noTSX-IBRS
  Westmere
  Westmere-IBRS
  ```

  After that specify the following in the VirtualMachineClass resource specification:

  ```yaml
  spec:
    cpu:
      model: IvyBridge
      type: Model
  ```

  How to perform the operation in the web interface in the [VM class creation form](#virtualmachineclass-settings):

  - In the "CPU Settings" block, select `Model` in the "Type" field.
  - Select the required processor model in the "Model" field.
  - To create a VM class, click the "Create" button.

#### Placement settings

The `.spec.nodeSelector` block is optional. It allows you to specify the nodes that will host VMs using this vmclass:

```yaml
  spec:
    nodeSelector:
      matchExpressions:
        - key: node.deckhouse.io/group
          operator: In
          values:
          - green
```

{% alert level="warning" %}
Since changing the `.spec.nodeSelector` parameter affects all virtual machines using this `VirtualMachineClass`, consider the following:

- For the Enterprise edition, this may cause virtual machines to be migrated to new destination nodes if the current nodes do not meet placement requirements.
- For the Community edition, this may cause virtual machines to restart according to the automatic change application policy set in the `.spec.disruptions.restartApprovalMode` parameter.
{% endalert %}

How to perform the operation in the web interface in the [VM class creation form](#virtualmachineclass-settings):

- Click "Add" in the "VM scheduling conditions on nodes" -> "Labels and expressions" block.
- In the pop-up window, you can set the "Key", "Operator" and "Value" of the key that corresponds to the `spec.nodeSelector` settings.
- To confirm the key parameters, click the "Enter" button.
- To create a VM class, click the "Create" button.

#### Sizing policy settings

The `.spec.sizingPolicy` block allows you to set sizing policies for virtual machine resources that use vmclass.

{% alert level="warning" %}
Changes in the `.spec.sizingPolicy` block can also affect virtual machines. For virtual machines whose sizing policy will not meet the new policy requirements, the `SizingPolicyMatched` condition in the `.status.conditions` block will be false (`status: False`).

When configuring `sizingPolicy`, be sure to consider the [CPU topology](./user_guide.html#automatic-cpu-topology-configuration) for virtual machines.
{% endalert %}

The `cores` block is mandatory and specifies the range of cores to which the rule described in the same block applies.

The ranges [min; max] for the cores parameter must be strictly sequential and non-overlapping.

Correct structure (the ranges follow one another without intersections):

```yaml
- cores:
    min: 1
    max: 4...

- cores:
    min: 5   # Start of next range = (previous max + 1)
    max: 8
```

Invalid option (overlapping values):

```yaml
- cores:
    min: 1
    max: 4...

- cores:
    min: 4   # Error: Value 4 is already included in the previous range
    max: 8
```

{% alert level="warning" %}
Rule: Each new range must start with a value that immediately follows the max of the previous range.
{% endalert %}

Additional requirements can be specified for each range of cores:

1. Memory — specify:

    - Either minimum and maximum memory for all cores in the range,
    - Either the minimum and maximum memory per core (`memoryPerCore`).

2. Allowed fractions of cores (`coreFractions`) — a list of allowed values (for example, [25, 50, 100] for 25%, 50%, or 100% core usage).

{% alert level="warning" %}
Important: For each range of cores, be sure to specify:

- Either memory (or `memoryPerCore`),
- Either coreFractions,
- Or both parameters at the same time.
{% endalert %}

Here is an example of a policy with similar settings:

```yaml
spec:
  sizingPolicies:
    # For a range of 1–4 cores, it is possible to use 1–8 GB of RAM in 512Mi increments,
    # i.e., 1 GB, 1.5 GB, 2 GB, 2.5 GB, etc.
    # No dedicated cores are allowed.
    # All `corefraction` options are available.
    - cores:
        min: 1
        max: 4
      memory:
        min: 1Gi
        max: 8Gi
        step: 512Mi
      coreFractions: [5, 10, 20, 50, 100]
    # For a range of 5–8 cores, it is possible to use 5–16 GB of RAM in 1 GB increments,
    # i.e., 5 GB, 6 GB, 7 GB, etc.
    # No dedicated cores are allowed.
    # Some `corefraction` options are available.
    - cores:
        min: 5
        max: 8
      memory:
        min: 5Gi
        max: 16Gi
        step: 1Gi
      coreFractions: [20, 50, 100]
    # For a range of 9–16 cores, it is possible to use 9–32 GB of RAM in 1 GB increments.
    # You can use dedicated cores if needed.
    # Some `corefraction` options are available.
    - cores:
        min: 9
        max: 16
      memory:
        min: 9Gi
        max: 32Gi
        step: 1Gi
      coreFractions: [50, 100]
    # For the range of 17–248 cores, it is possible to use 1–2 GB of RAM per core.
    # Only the dedicated cores are available for use.
    # The only available `corefraction` parameter is 100%.
    - cores:
        min: 17
        max: 248
      memory:
        perCore:
          min: 1Gi
          max: 2Gi
      coreFractions: [100]
```

How to configure sizing policies in the web interface in the [VM class creation form](#virtualmachineclass-settings):

- Click "Add" in the "Resource allocation rules for virtual machines" block.
- In the "PU" block, enter `1` in the "Min" field.
- In the "CPU" block, enter `4` in the "Max" field.
- In the "CPU" block, select the values `5%`, `10%`, `20%`, `50%`, `100%` in order in the "Allow core shares" field.
- In the "Memory" block, set the switch to "Volume per core".
- In the "Memory" block, enter `1` in the "Min" field.
- In the "Memory" block, enter `8` in the "Max" field.
- In the "Memory" block, enter `1` in the "Sampling step" field.
- You can add more ranges using the "Add" button.
- To create a VM class, click the "Create" button.

### vCPU Discovery configuration example

![VirtualMachineClass configuration example](./images/vmclass-examples.png)

Let's imagine that we have a cluster of four nodes. Two of these nodes labeled `group=blue` have a "CPU X" processor with three instruction sets, and the other two nodes labeled `group=green` have a newer "CPU Y" processor with four instruction sets.

To optimally utilize the resources of this cluster, it is recommended that you create three additional virtual machine classes (VirtualMachineClass):

- `universal`: This class will allow virtual machines to run on all nodes in the platform and migrate between them. It will use the instruction set for the lowest CPU model to ensure the greatest compatibility.
- `cpuX`: This class will be for virtual machines that should only run on nodes with a "CPU X" processor. VMs will be able to migrate between these nodes using the available "CPU X" instruction sets.
- `cpuY`: This class is for VMs that should only run on nodes with a "CPU Y" processor. VMs will be able to migrate between these nodes using the available "CPU Y" instruction sets.

{% alert level="info" %}
A CPU instruction set is a list of all the instructions that a processor can execute, such as addition, subtraction, or memory operations. They determine what operations are possible, affect program compatibility and performance, and can vary from one generation of processors to the next.
{% endalert %}

Resource configuration examples for a given cluster:

```yaml
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: universal
spec:
  cpu:
    discovery: {}
    type: Discovery
  sizingPolicies: { ... }
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: cpuX
spec:
  cpu:
    discovery:
      nodeSelector:
        matchExpressions:
          - key: group
            operator: In
            values: ["blue"]
    type: Discovery
  sizingPolicies: { ... }
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: cpuY
spec:
  cpu:
    discovery:
      nodeSelector:
        matchExpressions:
          - key: group
            operator: In
            values: ["green"]
    type: Discovery
  sizingPolicies: { ... }
```

## Reliability mechanisms

### VM Rebalancing

The platform provides the ability to automate the management of already running virtual machines in the cluster. To activate this feature, you need to enable the `descheduler` module.

When you enable the module, it automatically monitors the optimal operation of virtual machines in the cluster. The main features it provides are:

- Load balancing: The system analyses CPU reservation on cluster nodes. When CPU reservations exceed 80% on a node, the system automatically transfers part of the VMs to less loaded nodes. This prevents overload and ensures stable VM operation.
- Appropriate placement: The system checks whether the current node meets the requirements of each VM and whether the placement rules are followed in relation to the node or other VMs in the cluster. For example, if a VM should not be on the same node as another VM, the module transfers it to a more suitable node.

### Migration and maintenance mode

Virtual machine migration is an important feature in virtualized infrastructure management. It allows you to move running virtual machines from one physical node to another without shutting them down. Virtual machine migration is required for a number of tasks and scenarios:

- Load balancing: Moving virtual machines between nodes allows you to evenly distribute the load on servers, ensuring that resources are utilized in the best possible way.
- Node maintenance: Virtual machines can be moved from nodes that need to be taken out of service to perform routine maintenance or software upgrade.
- Upgrading a virtual machine firmware: The migration allows you to upgrade the firmware of virtual machines without interrupting their operation.

#### Start migration of an arbitrary machine

The following is an example of migrating a selected virtual machine.

1. Before starting the migration, check the current status of the virtual machine:

   ```bash
   d8 k get vm
   ```

   Example output:

   ```console
   NAME                                   PHASE     NODE           IPADDRESS     AGE
   linux-vm                              Running   virtlab-pt-1   10.66.10.14   79m
   ```

   We can see that it is currently running on the `virtlab-pt-1` node.

1. To migrate a virtual machine from one node to another taking into account the virtual machine placement requirements, the VirtualMachineOperation (`vmop`) resource with the `Evict` type is used. Create this resource following the example:

   ```yaml
   d8 k create -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachineOperation
   metadata:
     generateName: evict-linux-vm-
   spec:
     # Virtual machine name.
     virtualMachineName: linux-vm
     # An operation for the migration.
     type: Evict
   EOF
   ```

1. Immediately after creating the `vmop` resource, run the following command:

   ```bash
   d8 k get vm -w
   ```

   Example output:

   ```console
   NAME                                   PHASE       NODE           IPADDRESS     AGE
   linux-vm                              Running     virtlab-pt-1   10.66.10.14   79m
   linux-vm                              Migrating   virtlab-pt-1   10.66.10.14   79m
   linux-vm                              Migrating   virtlab-pt-1   10.66.10.14   79m
   linux-vm                              Running     virtlab-pt-2   10.66.10.14   79m
   ```

1. If you need to abort the migration, delete the corresponding `vmop` resource while it is in the `Pending` or `InProgress` phase.

How to start VM migration in the web interface:

- Go to the "Projects" tab and select the desired project.
- Go to the "Virtualization" -> "Virtual Machines" section.
- Select the desired virtual machine from the list and click the ellipsis button.
- Select `Migrate` from the pop-up menu.
- Confirm or cancel the migration in the pop-up window.

#### Maintenance mode

When working on nodes with virtual machines running, there is a risk of disrupting their performance. To avoid this, you can put a node into the maintenance mode and migrate the virtual machines to other free nodes.

To do this, run the following command:

```bash
d8 k drain <nodename> --ignore-daemonsets --delete-emptydir-data
```

Where `<nodename>` is a node scheduled for maintenance, which needs to be freed from all resources (including system resources).

If you need to evict only virtual machines off the node, run the following command:

```bash
d8 k drain <nodename> --pod-selector vm.kubevirt.internal.virtualization.deckhouse.io/name --delete-emptydir-data
```

After running the `d8 k drain` command, the node will enter maintenance mode and no virtual machines will be able to start on it.

To take it out of maintenance mode, stop the `drain` command (Ctrl+C), then execute:

```bash
d8 k uncordon <nodename>
```

![A diagram showing the migration of virtual machines from one node to another](./images/drain.png)

How to perform the operation in the web interface:

- Go to the "System”"tab, then to the "Nodes" section -> "Nodes of all groups".
- Select the desired node from the list and click the "Cordon + Drain" button.
- To remove it from maintenance mode, click the "Uncordon" button.

### ColdStandby

ColdStandby provides a mechanism to recover a virtual machine from a failure on a node it was running on.

The following requirements must be met for this mechanism to work:

- The virtual machine startup policy (`.spec.runPolicy`) must be set to one of the following values: `AlwaysOnUnlessStoppedManually`, `AlwaysOn`.
- The [Fencing mechanism](https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/040-node-manager/cr.html#nodegroup-v1-spec-fencing-mode) must be enabled on nodes running the virtual machines.

Let's see how it works on the example:

1. A cluster consists of three nodes: `master`, `workerA`, and `workerB`. The worker nodes have the Fencing mechanism enabled. The `linux-vm` virtual machine is running on the `workerA` node.
1. A problem occurs on the `workerA` node (power outage, no network connection, etc.).
1. The controller checks the node availability and finds that `workerA` is unavailable.
1. The controller removes the `workerA` node from the cluster.
1. The `linux-vm` virtual machine is started on another suitable node (`workerB`).

![ColdStandBy mechanism diagram](./images/coldstandby.png)
