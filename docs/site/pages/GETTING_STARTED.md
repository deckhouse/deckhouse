---
title: "Getting started"
permalink: en/getting_started.html
layout: page-nosidebar
toc: false
---

{::options parse_block_html="false" /}

<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>

<div markdown="1">
Probably, you have already familiarized yourself with the main [Deckhouse Platform features](/en/#features). This getting started guide walks you through the step-by-step process of installing the platform.

The Deckhouse platform runs both on bare metal servers and on the infrastructure of the supported cloud providers. The installation process differs depending on the infrastructure chosen, thus we provide various installation examples below.

## Installation

### Requirements and preparatory steps

The general approach to installing Deckhouse includes the following steps:

-  Run the Docker container on the local machine that will be used to install Deckhouse.
-  Pass a private SSH key and the cluster's config file in the YAML format (e.g., `config.yml`) to this container.
-  Connect the container to the target machine (for bare metal installations) or the cloud via SSH. After that, you may proceed to installing and configuring the Kubernetes cluster.

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
  onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_bm');">
    Bare Metal
  </a>
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure"
    onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_yc');">
    Yandex.Cloud
  </a>
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure"
    onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_aws');">
    Amazon AWS
  </a>
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure"
    onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_gcp');">
    Google Cloud
  </a>
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure"
    onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_azure');">
    Microsoft Azure
  </a>
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure"
    onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_openstack');">
    OpenStack
  </a>
  <a href="javascript:void(0)" class="tabs__btn tabs__btn_infrastructure"
    onclick="openTab(event, 'tabs__btn_infrastructure', 'tabs__content_infrastructure', 'infrastructure_existing');">
    Existing cluster
  </a>
</div>

<div id="infrastructure_bm" class="tabs__content tabs__content_infrastructure active" markdown="1">
{% include getting_started/STEP1_BAREMETAL.md %}

{% include getting_started/STEP2.md mode="baremetal" %}

{% include getting_started/STEP3.md mode="baremetal" %}
</div>

<div id="infrastructure_yc" class="tabs__content tabs__content_infrastructure" markdown="1">
{% include getting_started/STEP1_CLOUD_YANDEX.md %}

{% include getting_started/STEP2.md mode="cloud" %}

{% include getting_started/STEP3.md mode="cloud" %}
</div>

<div id="infrastructure_aws" class="tabs__content tabs__content_infrastructure" markdown="1">
{% include getting_started/STEP1_CLOUD_AWS.md %}

{% include getting_started/STEP2.md mode="cloud" %}

{% include getting_started/STEP3.md mode="cloud" %}
</div>

<div id="infrastructure_gcp" class="tabs__content tabs__content_infrastructure" markdown="1">
{% include getting_started/STEP1_CLOUD_GCP.md %}

{% include getting_started/STEP2.md mode="cloud" %}

{% include getting_started/STEP3.md mode="cloud" %}
</div>

<div id="infrastructure_openstack" class="tabs__content tabs__content_infrastructure" markdown="1">
{% include getting_started/STEP1_CLOUD_OPENSTACK.md %}

{% include getting_started/STEP2.md mode="cloud" provider="openstack" %}

{% include getting_started/STEP3.md mode="cloud" %}
</div>

<div id="infrastructure_azure" class="tabs__content tabs__content_infrastructure" markdown="1">
{% include getting_started/STEP1_CLOUD_AZURE.md %}

{% include getting_started/STEP2.md mode="cloud" provider="azure" %}

{% include getting_started/STEP3.md mode="cloud" %}
</div>

<div id="infrastructure_existing" class="tabs__content tabs__content_infrastructure" markdown="1">
{% include getting_started/STEP1_EXISTING.md %}

{% include getting_started/STEP2.md mode="existing" %}

{% include getting_started/STEP3.md mode="existing" %}
</div>

<div markdown="1">
## Next steps

### Using modules

The Deckhouse module system allows you to add modules to the cluster and delete them on the fly. All you need to do is edit the cluster config — Deckhouse will apply all the necessary changes automatically.

Let's, for example, add the [user-authn](/en/documentation/v1/modules/150-user-authn/) module:

-  Open the Deckhouse configuration:
   ```yaml
kubectl -n d8-system edit cm/deckhouse
```
-  Find the `data` section and add enable the module there:
   ```yaml
data:
  userAuthnEnabled: "true"
```
-  Save the configuration file. At this point, Deckhouse notices the changes and installs the module automatically.

To edit the module settings, repeat step 1 (make changes to the configuration and save them). The changes will be applied automatically.

To disable the module, set the parameter to `false`.

### What can I do next?

Now that everything is up and running as intended, you can refer to [the documentation](/en/documentation/v1/) about the system in general and Deckhouse components in particular.

Please, reach us via our [online community](/en/community/about.html#online-community) if you have any questions.
</div>
