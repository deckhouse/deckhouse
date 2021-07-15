## Installation schema

The general approach to installing Deckhouse Platform {% if page.platform_type == 'baremetal' %}on{% elsif page.platform_type == 'existing' %}into{% else %}in{% endif %} {{ page.platform_name }} includes the following steps:

-  Run the Docker container on the local machine that will be used to install Deckhouse Platform.
-  Pass a private SSH key and the cluster's config file in the YAML format (e.g., `config.yml`) to this container.
-  Connect the container to the target machine (for bare metal installations) or the cloud via SSH. After that, you may proceed to installing and configuring the Kubernetes cluster.
{% if page.platform_type == 'cloud' %}
> The "regular" computing resources of the cloud provider are used if Deckhouse is installed into the public cloud (instead of resources specially adapted to the managed Kubernetes solution of the provider in question).
{%- endif %}
>
> To learn more about the Deckhouse Platform release channels, please see the [relevant documentation](/en/documentation/v1/deckhouse-release-channels.html).

## Requirements/limitations:
-   The Docker runtime must be present on the machine used for the installation.
-   Connection to the Internet; access to the operating system's standard repositories to install additional packages.
-   Deckhouse supports different Kubernetes versions: from 1.16 through 1.21. However, please note, only the following versions are tested for K8s installations “from scratch”: 1.16, 1.19, 1.20, and 1.21. All configurations below will use v1.19 as an example.
-   Recommended minimum hardware configuration of master nodes for a future cluster:
    -   at least 4 CPU cores;
    -   at least 8 GB of RAM;
    -   at least 40 GB of disk space for the cluster and etcd data;
    -   OS: Ubuntu Linux 16.04/18.04/20.04 LTS or CentOS 7.

{% if ee_only != true %}
## Select the Deckhouse Platform revision to continue installation {% if page.platform_type == 'baremetal' %}on{% else %}in{% endif %} {{ page.platform_name }}

[Compare](/en/products/enterprise_edition.html#ce-vs-ee) Enterprise Edition to Community Edition.
{% endif %}
