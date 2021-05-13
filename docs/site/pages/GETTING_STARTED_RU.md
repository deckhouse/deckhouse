---
title: "Быстрый старт"
permalink: ru/getting_started.html
layout: page-nosidebar
lang: ru
toc: false
---

{::options parse_block_html="false" /}

<div markdown="1">
Probably, you have already familiarized yourself with the main [Deckhouse features](/en/features.html).

This getting started guide walks you through the step-by-step process of installing the Community Edition of Deckhouse. *(See the [products](/en/products.html) section for more info on licensing options and the differences between the CE and EE versions).*

The Deckhouse platform runs both on bare metal servers and on the infrastructure of the supported cloud providers. However, the installation process differs depending on the infrastructure chosen. That is why we provide various installation examples below.

## Installation

### Requirements and preparatory steps

Deckhouse is a [modular system](/en/documentation/v1/). The candi (Cluster and Infrastructure) subsystem is responsible for installing Deckhouse (you can find the detailed documentation [here](/en/documentation/v1/features/candi.html#)).

The general approach to installing Deckhouse includes the following steps:

1.  Run the Docker container on the local machine that will be used to install Deckhouse.
2.  Pass a public SSH key and the cluster's config file in the YAML format (e.g., `config.yml`) to this container.
3.  Connect the container to the target machine (for bare metal installations) or the cloud via SSH. After that, you may proceed to installing and configuring the Kubernetes cluster.

***Note** that the "regular" computing resources of the cloud provider are used if Deckhouse is installed into the public cloud (instead of resources specially adapted to the managed Kubernetes solution of the provider in question).*

Requirements/limitations:

-   The Docker runtime must be present on the machine used for the installation.
-   Although Deckhouse supports different Kubernetes versions (1.16+), only the latest ones have been tested for installing from scratch. Currently, version 1.19 is considered the latest; all subsequent installation examples are based on it.
-   Minimum hardware recommendations for a future cluster:
    -   at least 4 CPU cores;
    -   at least 8 GB of RAM;
    -   at least 40 GB of disk space for the cluster and etcd data;
-   OS: Ubuntu Linux 16.04/18.04/20.04 LTS or CentOS 7;
-   connection to the Internet; access to the operating system's standard repositories to install additional packages.


## Step 1. Configuration

Select the infrastructure type to install Deckhouse in:
</div>

<div class="tabs">
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure active"
  onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infratructure_bm')">
    Bare Metal
  </a>
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure"
    onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infratructure_yc')">
    Yandex.Cloud
  </a>
</div>

<div id="infratructure_bm" class="tabs__content tabs__content_infrastructure active">
<ul>
<li>
Set up SSH access between the machine used for the installation and the cluster's prospective master node.
</li>
<li>
Create the cluster configuration file (<code>config.yml</code>) and insert the following three sections into it:

{% offtopic title="config.yml" %}
```yaml
------------------------------------------------------------------
# general cluster parameters (ClusterConfiguration)
------------------------------------------------------------------
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: ClusterConfiguration
# type of the infrastructure: bare-metal (Static) or Cloud (Cloud)
clusterType: Static
# address space of the cluster's pods
podSubnetCIDR: 10.111.0.0/16
# address space of the cluster's services
serviceSubnetCIDR: 10.222.0.0/16
# Kubernetes version to install
kubernetesVersion: "1.19"
# cluster domain (used for local routing)
clusterDomain: "cluster.local"

------------------------------------------------------------------
# section for bootstrapping the Deckhouse cluster (InitConfiguration)
------------------------------------------------------------------
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: InitConfiguration
# Deckhouse configuration
deckhouse:
  # address of the registry where the installer image is located; in this case, the default value for the official Deckhouse CE build is set
  # for more information, see the description of the next step
  imagesRepo: registry.flant.com/sys/antiopa
  # the update channel used
  releaseChannel: EarlyAccess
  configOverrides:
    global:
      # the cluster name (it is used, e.g., in Prometheus alerts' labels)
      clusterName: main
      # the project name (it is used for the same purpose as the cluster name)
      project: someproject
      modules:
        # public domain name (clusterName is substituted for %s)
        # used when accessing the cluster from outside
        publicDomainTemplate: "%s.somedomain.com"

------------------------------------------------------------------
# section with the parameters of the bare metal cluster (StaticClusterConfiguration)
------------------------------------------------------------------
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: StaticClusterConfiguration
# address space for the cluster's internal network
internalNetworkCIDRs:
- 10.0.4.0/24
```
{% endofftopic %}
</li>
</ul>
</div>

<div id="infratructure_yc" class="tabs__content tabs__content_infrastructure">
  <div markdown="1">
  You have to create a service account with the editor role with the cloud provider so that Deckhouse can manage cloud resources. The detailed instructions for creating a service account with Yandex.Cloud are available in the provider's [documentation](https://cloud.yandex.com/en/docs/resource-manager/operations/cloud/set-access-bindings). Below, we will provide a brief overview of the necessary actions:
  </div>
  <ol>
    <li>
      Create a user named <code>candi</code>. The command response will contain its parameters: 
<div markdown="1">
```yaml
yc iam service-account create --name candi
id: <userId>
folder_id: <folderId>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: candi
```
</div>
    </li>
    <li>
      Assign the editor role to the newly created user:
<div markdown="1">
```yaml
yc resource-manager folder add-access-binding <cloudname> --role editor --subject serviceAccount:<userId>
```
</div>
    </li>
    <li>
      Create a JSON file containing the parameters for user authorization in the cloud (These parameters will be used to log in to the cloud):
<div markdown="1">
```yaml
yc iam key create --service-account-name candi --output candi-sa-key.json
```
</div>
      <ul>
        <li>
          Generate an SSH key on the local machine for accessing the cloud-based virtual machines. In Linux and macOS, you can generate a key using the <code>ssh-keygen</code> command-line tool. The public key must be included in the configuration file (it will be used for accessing nodes in the cloud).
        </li>
        <li>
          Select the layout – the way how objects are located in the cloud. We will use the <strong>WithoutNAT</strong> layout for the Yandex.Cloud example. In this layout, NAT (of any kind) is not used, and each node is assigned a public IP. Other available layouts are described in the <a href="/en/documentation/v1/features/candi.html">Cloud providers</a> section of the candi subsystem documentation.
        </li>
        <li>
          Define the three primary sections with parameters of the prospective cluster in the <code>config.yml</code> file:
{% offtopic title="config.yml" %}
```yaml
-----------------------------------------------------------------
# general cluster parameters (ClusterConfiguration)
------------------------------------------------------------------
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: ClusterConfiguration
# type of the infrastructure: bare-metal (Static) or Cloud (Cloud)
clusterType: Cloud
# cloud provider-related settings
cloud:
  # type of the cloud provider
  provider: Yandex
  # prefix to differentiate cluster objects (can be used, e.g., in routing)
  prefix: "yandex-demo"
# address space of the cluster's pods
podSubnetCIDR: 10.111.0.0/16
# address space of the cluster's services
serviceSubnetCIDR: 10.222.0.0/16
# Kubernetes version to install
kubernetesVersion: "1.19"
# cluster domain (used for local routing)
clusterDomain: "cluster.local"

-----------------------------------------------------------------
# section for bootstrapping the Deckhouse cluster (InitConfiguration)
-----------------------------------------------------------------
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: InitConfiguration
# Deckhouse parameters
deckhouse:
# address of the registry where the installer image is located; in this case, the default value for the official Deckhouse CE build is set
# for more information, see the description of the next step
  imagesRepo: registry.flant.com/sys/antiopa
# the update channel used
releaseChannel: EarlyAccess
configOverrides:
  global:
      # the cluster name (it is used, e.g., in Prometheus alerts' labels)
    clusterName: somecluster
      # the cluster's project name (it is used for the same purpose as the cluster name)
    project: someproject
    modules:
      # public domain name (clusterName is substituted for %s)
      # used when accessing the cluster from outside
      publicDomainTemplate: "%s.somedomain.com"

-----------------------------------------------------------------
# section containing the parameters of the cloud provider (YandexClusterConfiguration)
-----------------------------------------------------------------
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: YandexClusterConfiguration
# public SSH key for accessing cloud nodes
sshPublicKey: ssh-rsa
<ssh public key>
# layout — the way resources are located in the cloud
layout: WithoutNAT
# address space of the cluster's nodes
nodeNetworkCIDR: 10.100.0.0/21
# parameters of the master node group
masterNodeGroup:
# number of replicas
replicas: 1
# the amount of CPU, RAM, HDD resources, the VM image, and the policy of assigning external IP addresses
instanceClass:
  cores: 4
  memory: 8192
  imageID: fd8vqk0bcfhn31stn2ts
  diskSizeGB: 40
  externalIPAddresses:
  - Auto
# Yandex.Cloud's cloud and folder IDs
provider:
  cloudID: ***
  folderID: ***
  # parameters of the cloud provider service account that can create and manage virtual machines
  # the same as the contents of the candi-sa-key.json file generated earlier
  serviceAccountJSON: |
    {
      "id": "***",
      "service_account_id": "***",
      "created_at": "2020-08-17T08:56:17Z",
      "key_algorithm": "RSA_2048",
      "public_key": ***,
      "private_key": ***
    }
```
{% endofftopic %}
        </li>
      </ul>
    </li>
  </ol>
  <div markdown="1">
Notes:
-   The complete list of supported cloud providers and their specific settings is available in the [Cloud providers](/en/documentation/v1/features/candi.html) section of the candi subsystem documentation.
-   [Click here](/releases.html) to learn more about the Deckhouse release channels.
  </div>
</div>

<div markdown="1">

## Step 2. Installation

To proceed with the installation, you will need a Docker image of the Deckhouse installer. We will use the ready-made official image.

The command below pulls the Docker image of the Deckhouse installer and passes the public SSH key/config file to it (we have created them on the previous step). Note that the command below uses the default paths to files.

```yaml
docker run -v ./config.yml -v ./ssh/<public_ssh_keyname>.pub registry.deckhouse.io/installer:v1.0.0
```

After the installation is complete, you will be returned to the command line. Congratulations: your cluster is ready! Now you can manage modules, deploy applications, etc.

## Step 3. Checking the status

You can verify the status of the Kubernetes cluster right after (or even during) the Deckhouse installation. By default, the `.kube/config` file used to communicate with Kubernetes is generated on the cluster's host. Thus, you can connect to the host via SSH and use regular k8s tools (such as `kubectl`) to interact with Kubernetes.

For example, you can use the following classic command to view the cluster status:

```yaml
kubectl -n d8-system get pods
```

In the command's output, all `d8-system***` pods must be listed as Ready. Such status indicates that modules are installed successfully, and the cluster is ready for use.

For more convenient control over the cluster, a [module](/en/documentation/v1/modules/500-dashboard/) with the official Kubernetes dashboard is provided. It gets enabled by default after installation is complete and is available at `https://<publicDomainTemplate>` (the *User* access level is required). (The user-authz module documentation provides a detailed overview of access levels.)

Logs are stored in JSON format, so you might want to use the `jq` utility to browse them:

```yaml
kubectl -n namespace system logs podname -f --tail=10 | jq -rc .msg
```
Note that there is also a [marm](/en/documentation/v1/features/marm.html) subsystem for full-fledged and detailed monitoring of the cluster.

## Next steps

### Using modules

The Deckhouse module system allows you to add modules to the cluster and delete them on the fly. All you need to do is edit the cluster config --- Deckhouse will apply all the necessary changes automatically.

Let's, for example, add the [extended-monitoring](/en/documentation/v1/modules/340-extended-monitoring/) module of the marm subsystem:

1.  Open the Deckhouse configuration:
    ```yaml
    kubectl -n d8-system edit cm/deckhouse
    ```
2.  Find the data section and add the parameter extended-monitoringEnabled to it:
    ```yaml
    data:
      global:
        extended-monitoringEnabled: "True"
    ```
3.  Save the configuration file. At this point, Deckhouse notices the changes and installs the module automatically.

To edit the module settings, repeat step 1 (make changes to the configuration and save them). The changes will be applied automatically.

To disable the module, set the parameter to `False`.

### What do I do next?

Now that everything is up and running as intended, you can refer to [detailed information](/en/documentation/v1/) about the system in general and Deckhouse components in particular.
Please, reach us via our [online community](/en/community.html) if you have any questions.

</div>