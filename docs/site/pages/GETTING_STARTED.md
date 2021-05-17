---
title: "Getting started"
permalink: en/getting_started.html
layout: page-nosidebar
toc: false
---

{::options parse_block_html="false" /}

<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>

<div markdown="1">
Probably, you have already familiarized yourself with the main [Deckhouse features](/en/features.html).

This getting started guide walks you through the step-by-step process of installing the Community Edition of Deckhouse. *(See the [products](/en/products.html#ce-vs-ee) section for more info on licensing options and the differences between the CE and EE versions).*

The Deckhouse platform runs both on bare metal servers and on the infrastructure of the supported cloud providers. However, the installation process differs depending on the infrastructure chosen. That is why we provide various installation examples below.

## Installation

### Requirements and preparatory steps

The general approach to installing Deckhouse includes the following steps:

1.  Run the Docker container on the local machine that will be used to install Deckhouse.
2.  Pass a private SSH key and the cluster's config file in the YAML format (e.g., `config.yml`) to this container.
3.  Connect the container to the target machine (for bare metal installations) or the cloud via SSH. After that, you may proceed to installing and configuring the Kubernetes cluster.

***Note** that the "regular" computing resources of the cloud provider are used if Deckhouse is installed into the public cloud (instead of resources specially adapted to the managed Kubernetes solution of the provider in question).*

Requirements/limitations:

-   The Docker runtime must be present on the machine used for the installation.
-   Deckhouse supports different Kubernetes versions: from 1.16 through 1.21. However, please note, only the following versions are tested for K8s installations “from scratch”: 1.16, 1.19, 1.20, and 1.21. All configurations below will use v1.19 as an example.
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
  imagesRepo: registry.deckhouse.io/deckhouse/fe
  # a special string with your token to access Docker registry
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
  # the release channel used
  releaseChannel: EarlyAccess
  configOverrides:
    global:
      # the cluster name (it is used, e.g., in Prometheus alerts' labels)
      clusterName: main
      # the project name (it is used for the same purpose as the cluster name)
      project: someproject
      modules:
        # template that will be used for system apps domains within the cluster
        # e.g., Grafana for %s.somedomain.com will be available as grafana.somedomain.com
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
      Assign the <code>editor</code> role to the newly created user:
<div markdown="1">
```yaml
yc resource-manager folder add-access-binding <cloudname> --role editor --subject serviceAccount:<userId>
```
</div>
    </li>
    <li>
      Create a JSON file containing the parameters for user authorization in the cloud. These parameters will be used to log in to the cloud:
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
          Select the layout – the way how objects are located in the cloud; each provider has as a few of predefined layouts in Deckhouse. We will use the <strong>WithoutNAT</strong> layout for the Yandex.Cloud example. In this layout, NAT (of any kind) is not used, and each node is assigned a public IP.
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
  imagesRepo: registry.deckhouse.io/deckhouse/fe
  # a special string with your token to access Docker registry
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
  # the release channel used
  releaseChannel: EarlyAccess
  configOverrides:
    global:
      # the cluster name (it is used, e.g., in Prometheus alerts' labels)
      clusterName: somecluster
      # the cluster's project name (it is used for the same purpose as the cluster name)
      project: someproject
      modules:
        # template that will be used for system apps domains within the cluster
        # e.g., Grafana for %s.somedomain.com will be available as grafana.somedomain.com
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
<SSH_PUBLIC_KEY>
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
-   The complete list of supported cloud providers and their specific settings is available in the [Cloud providers](/en/documentation/v1/kubernetes.html) section of the documentation.
-   To learn more about the Deckhouse release channels, please see the [relevant documentation](/en/documentation/v1/deckhouse-release-channels.html) .
  </div>
</div>

<div markdown="1">

## Step 2. Installation

To proceed with the installation, you will need a Docker image of the Deckhouse installer. We will use the ready-made official image. The instructions on how you can build your own image from the sources, will be available in the [project's repository](https://github.com/deckhouse/deckhouse).

The command below pulls the Docker image of the Deckhouse installer and passes the public SSH key/config file to it (we have created them on the previous step). Note that this command uses the default paths to files. The interactive terminal of this image's system will be launched then:

```yaml
docker run -it -v $(pwd)/config.yml:/config.yml -v $HOME/.ssh/:/tmp/.ssh/ registry.deckhouse.io/installer:v1.0.0 bash
```

Now, to initiate the process of installation, you need to execute:

```yaml
dhctl bootstrap \
  --ssh-user=<username> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml
```

`username` variable here refers to:
* the user that generated the SSH key for bare-metal installations;
* the default user for the relevant VM image for cloud deployments (e.g., `ubuntu`, `user`, `azureuser`).

Notes:
-   It’s not recommended to leave the container during the installation process (e.g., if you need to change the configuration). In this case, when you launch the installer for the second time, you’ll have to manually remove resources created in the provider. Instead, you can use an external text editor (e.g., vim) to change your configuration. After you save the file, the modified configuration will become automatically available in the container.
-   If any problems occur, you can stop the process of installation using the following command (the configuration file should be the same you’ve used to initiate the installation):

```yaml
dhctl bootstrap-phase abort --config=config.yml
```

After the installation is complete, you will be returned to the command line. Congratulations: your cluster is ready! Now you can manage modules, deploy applications, etc.

## Step 3. Checking the status

You can verify the status of the Kubernetes cluster right after (or even during) the Deckhouse installation. By default, the `.kube/config` file used to communicate with Kubernetes is generated on the cluster's host. Thus, you can connect to the host via SSH and use regular k8s tools (such as `kubectl`) to interact with Kubernetes.

For example, you can use the following command to view the cluster status:

```yaml
kubectl -n d8-system get deployments/deckhouse
```

In the command's output, the `d8-system***` deployment should be `Ready 1/1`. Such status indicates that modules are installed successfully, and the cluster is ready for use.

For more convenient control over the cluster, a [module](/en/documentation/v1/modules/500-dashboard/) with the official Kubernetes dashboard is provided. It gets enabled by default after installation is complete and is available at `https://dashboard<your-publicDomainTemplate-value>` with the *User* access level. (The [user-authz module](/en/documentation/v1/modules/140-user-authz/) documentation provides a detailed overview of access levels.)

Logs are stored in JSON format, so you might want to use the `jq` utility to browse them:

```yaml
kubectl logs -n d8-system deployments/deckhouse -f --tail=10 | jq -rc .msg
```
Note that there is also a pack of [special modules](/en/documentation/v1/modules/300-prometheus/) to implement full-fledged and detailed monitoring of the cluster.

## Next steps

### Using modules

The Deckhouse module system allows you to add modules to the cluster and delete them on the fly. All you need to do is edit the cluster config — Deckhouse will apply all the necessary changes automatically.

Let's, for example, add the [user-authn](/en/documentation/v1/modules/150-user-authn/) module:

1.  Open the Deckhouse configuration:
    ```yaml
    kubectl -n d8-system edit cm/deckhouse
    ```
2.  Find the `data` section and add enable the module there:
    ```yaml
    data:
      global:
        userAuthnEnabled: "true"
    ```
3.  Save the configuration file. At this point, Deckhouse notices the changes and installs the module automatically.

To edit the module settings, repeat step 1 (make changes to the configuration and save them). The changes will be applied automatically.

To disable the module, set the parameter to `false`.

### What can I do next?

Now that everything is up and running as intended, you can refer to [the documentation](/en/documentation/v1/) about the system in general and Deckhouse components in particular.

Please, reach us via our [online community](/en/community.html#online-community) if you have any questions.

</div>
