Prepare the installation configuration of the **Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}**:
- Generate an SSH key on the installer machine for accessing the cloud-based virtual machines. In Linux and macOS, you can generate a key using the `ssh-keygen` command-line tool. The public key must be included in the configuration file (it will be used for accessing nodes in the cloud).

- Select your layout — the way how resources are located in the cloud *(there are several pre-defined layouts for each provider in Deckhouse Platform)*. For the Openstack example, we will use the **Standard** layout. In this layout, an internal cluster network is created with a gateway to the public network; the nodes do not have public IP addresses. A floating IP is assigned to the master node.

- Define the three primary sections with parameters of the prospective cluster in the `config.yml` file:

  ```yaml
  # general cluster parameters (ClusterConfiguration)
  # version of the Deckhouse API
  apiVersion: deckhouse.io/v1alpha1
  # type of the configuration section
  kind: ClusterConfiguration
  # type of the infrastructure: bare metal (Static) or Cloud (Cloud)
  clusterType: Cloud
  # cloud provider-related settings
  cloud:
    # type of the cloud provider
    provider: OpenStack
    # prefix to differentiate cluster objects (can be used, e.g., in routing)
    prefix: "mailru-demo"
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
  apiVersion: deckhouse.io/v1alpha1
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
  # section containing the parameters of the cloud provider
  # version of the Deckhouse API
  apiVersion: deckhouse.io/v1alpha1
  # type of the configuration section
  kind: OpenStackClusterConfiguration
  # pre-defined layout from Deckhouse
  layout: Standard
  # parameters of the master node group
  masterNodeGroup:
    # parameters of the VM image
    instanceClass:
      # flavor in use
      flavorName: Standard-2-4-50
      # VM image in use
      imageName: ubuntu-18-04-cloud-amd64
      # disk size
      rootDiskSize: 30
    # number of replicas
    # if more than 1 master node exists, control-plane will be automatically deployed on all master nodes
    replicas: 1
    # disk type
    volumeTypeMap:
      DP1: dp1-high-iops
  # cloud access parameters
  provider:
    authURL: https://infra.mail.ru:35357/v3/
    domainName: users
    password: '***'
    region: RegionOne
    tenantID: '***'
    username: somename@somemail.com
  # public SSH key for accessing cloud nodes
  sshPublicKey: ssh-rsa <SSH_PUBLIC_KEY>
  standard:
    # the name assigned to the external subnetwork
    externalNetworkName: ext-net
    # address space for the internal subnetwork
    internalNetworkCIDR: 192.168.198.0/24
    # assigned DNS servers
    internalNetworkDNSServers:
      - 8.8.8.8
      - 8.8.4.4
    # enabling security policies in the cluster's internal network
    internalNetworkSecurity: true
  ```

> To learn more about the Deckhouse Platform release channels, please see the [relevant documentation](/en/documentation/v1/deckhouse-release-channels.html).
