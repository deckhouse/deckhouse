Prepare the installation configuration of the **Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}**:
- Generate an SSH key on the local machine for accessing the cloud-based virtual machines. In Linux and macOS, you can generate a key using the `ssh-keygen` command-line tool. The public key must be included in the configuration file (it will be used for accessing nodes in the cloud).
- Select the layout – the way how resources are located in the cloud *(there are several pre-defined layouts for each provider in Deckhouse)*. We will use the **WithoutNAT** layout for the Yandex.Cloud example. In this layout, NAT (of any kind) is not used, and each node is assigned a public IP.
- Define the three primary sections with parameters of the prospective cluster in the `config.yml` file:
{% if page.revision == 'ee' %}
  ```yaml
  # general cluster parameters (ClusterConfiguration)
  # version of the Deckhouse API
  apiVersion: deckhouse.io/v1
  # type of the configuration section
  kind: ClusterConfiguration
  # type of the infrastructure: bare metal (Static) or Cloud (Cloud)
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
  ---
  # section for bootstrapping the Deckhouse cluster (InitConfiguration)
  # version of the Deckhouse API
  apiVersion: deckhouse.io/v1
  # type of the configuration section
  kind: InitConfiguration
  # Deckhouse parameters
  deckhouse:
    # address of the registry where the installer image is located; in this case, the default value for Deckhouse EE is set
    imagesRepo: registry.deckhouse.io/deckhouse/ee
    # a special string with your token to access Docker registry (generated automatically for your license token)
    registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
    # the release channel in use
    releaseChannel: Beta
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
  ---
  # section containing the parameters of the cloud provider (YandexClusterConfiguration)
  # version of the Deckhouse API
  apiVersion: deckhouse.io/v1
  # type of the configuration section
  kind: YandexClusterConfiguration
  # public SSH key for accessing cloud nodes
  sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
  # pre-defined layout from Deckhouse
  layout: WithoutNAT
  # address space of the cluster's nodes
  nodeNetworkCIDR: 10.100.0.0/21
  # parameters of the master node group
  masterNodeGroup:
    # number of replicas
    # if more than 1 master node exists, control-plane will be automatically deployed on all master nodes
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
    cloudID: "***"
    folderID: "***"
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
{%- else %}
  ```yaml
  # general cluster parameters (ClusterConfiguration)
  # version of the Deckhouse API
  apiVersion: deckhouse.io/v1
  # type of the configuration section
  kind: ClusterConfiguration
  # type of the infrastructure: bare metal (Static) or Cloud (Cloud)
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
  ---
  # section for bootstrapping the Deckhouse cluster (InitConfiguration)
  # version of the Deckhouse API
  apiVersion: deckhouse.io/v1
  # type of the configuration section
  kind: InitConfiguration
  # Deckhouse parameters
  deckhouse:
    # the release channel in use
    releaseChannel: Beta
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
  ---
  # section containing the parameters of the cloud provider (YandexClusterConfiguration)
  # version of the Deckhouse API
  apiVersion: deckhouse.io/v1
  # type of the configuration section
  kind: YandexClusterConfiguration
  # public SSH key for accessing cloud nodes
  sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
  # pre-defined layout from Deckhouse
  layout: WithoutNAT
  # address space of the cluster's nodes
  nodeNetworkCIDR: 10.100.0.0/21
  # parameters of the master node group
  masterNodeGroup:
    # number of replicas
    # if more than 1 master node exists, control-plane will be automatically deployed on all master nodes
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
    cloudID: "***"
    folderID: "***"
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
{%- endif %}

> To learn more about the Deckhouse Platform release channels, please see the [relevant documentation](/en/documentation/v1/deckhouse-release-channels.html).
