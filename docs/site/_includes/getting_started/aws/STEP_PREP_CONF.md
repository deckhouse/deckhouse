Prepare the installation configuration of the **Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}**:
- Select your layout — the way how resources are located in the cloud *(there are several pre-defined layouts for each provider in Deckhouse Platform)*.

  For the AWS example, we will use the **WithoutNAT** layout. In this layout, the virtual machines will access the Internet through a NAT Gateway with a shared and single source IP. All nodes created with Deckhouse Platform can optionally get a public IP (ElasticIP).

  The other available options are described in the [Cloud providers](https://early.deckhouse.io/en/documentation/v1/kubernetes.html) section of the documentation.

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
    provider: AWS
    # prefix to differentiate cluster objects (can be used, e.g., in routing)
    prefix: "aws-demo"
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
        modules:
          # template that will be used for system apps domains within the cluster
          # e.g., Grafana for %s.somedomain.com will be available as grafana.somedomain.com
          publicDomainTemplate: "%s.somedomain.com"
  ---
  # section containing the parameters of the cloud provider
  # version of the Deckhouse API
  apiVersion: deckhouse.io/v1
  # type of the configuration section
  kind: AWSClusterConfiguration
  # pre-defined layout from Deckhouse
  layout: WithoutNAT
  # AWS access parameters
  provider:
    providerAccessKeyId: MYACCESSKEY
    providerSecretAccessKey: mYsEcReTkEy
    # cluster region
    region: eu-central-1
  # parameters of the master node group
  masterNodeGroup:
    # number of replicas
    # if more than 1 master node exists, control-plane will be automatically deployed on all master nodes
    replicas: 1
    # parameters of the VM image
    instanceClass:
      # type of the instance
      instanceType: c5.large
      # Amazon Machine Image id
      ami: ami-0fee04b212b7499e2
  # address space of the AWS cloud
  vpcNetworkCIDR: "10.241.0.0/16"
  # address space of the cluster's nodes
  nodeNetworkCIDR: "10.241.32.0/20"
  # public SSH key for accessing cloud nodes
  sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
  ```
{% else %}
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
    provider: AWS
    # prefix to differentiate cluster objects (can be used, e.g., in routing)
    prefix: "aws-demo"
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
        modules:
          # template that will be used for system apps domains within the cluster
          # e.g., Grafana for %s.somedomain.com will be available as grafana.somedomain.com
          publicDomainTemplate: "%s.somedomain.com"
  ---
  # section containing the parameters of the cloud provider
  # version of the Deckhouse API
  apiVersion: deckhouse.io/v1
  # type of the configuration section
  kind: AWSClusterConfiguration
  # pre-defined layout from Deckhouse
  layout: WithoutNAT
  # AWS access parameters
  provider:
    providerAccessKeyId: MYACCESSKEY
    providerSecretAccessKey: mYsEcReTkEy
    # cluster region
    region: eu-central-1
  # parameters of the master node group
  masterNodeGroup:
    # number of replicas
    # if more than 1 master node exists, control-plane will be automatically deployed on all master nodes
    replicas: 1
    # Parameters of the VM image
    instanceClass:
      # Type of the instance
      instanceType: c5.large
      # Amazon Machine Image id
      ami: ami-0fee04b212b7499e2
  # address space of the AWS cloud
  vpcNetworkCIDR: "10.241.0.0/16"
  # address space of the cluster's nodes
  nodeNetworkCIDR: "10.241.32.0/20"
  # public SSH key for accessing cloud nodes
  sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
  ```
{%- endif %}

> To learn more about the Deckhouse Platform release channels, please see the [relevant documentation](/en/documentation/v1/deckhouse-release-channels.html).
