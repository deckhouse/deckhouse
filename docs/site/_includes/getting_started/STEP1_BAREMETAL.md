- Set up SSH access between the machine used for the installation and the cluster's prospective master node.
- Create the cluster configuration file (`config.yml`) and insert the following three sections into it:

{% offtopic title="config.yml for CE" %}
```yaml
# general cluster parameters (ClusterConfiguration)
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
---
# section for bootstrapping the Deckhouse cluster (InitConfiguration)
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: InitConfiguration
# Deckhouse configuration
deckhouse:
  # address of the registry where the installer image is located; in this case, the default value for Deckhouse CE is set
  imagesRepo: registry.deckhouse.io/deckhouse/ce
  # a special string with parameters to access Docker registry
  registryDockerCfg: eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5pbyI6IHt9fX0=
  # the release channel in use
  releaseChannel: Beta
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
    cniFlannelEnabled: true
    cniFlannel:
      podNetworkMode: vxlan
    prometheusMadisonIntegrationEnabled: false
    nginxIngressEnabled: false
---
# section with the parameters of the bare metal cluster (StaticClusterConfiguration)
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: StaticClusterConfiguration
# address space for the cluster's internal network
internalNetworkCIDRs:
- 10.0.4.0/24
```
{% endofftopic %}
{% offtopic title="config.yml for EE" %}
```yaml
# general cluster parameters (ClusterConfiguration)
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
---
# section for bootstrapping the Deckhouse cluster (InitConfiguration)
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: InitConfiguration
# Deckhouse configuration
deckhouse:
  # address of the registry where the installer image is located; in this case, the default value for Deckhouse EE is set
  imagesRepo: registry.deckhouse.io/deckhouse/ee
  # a special string with your token to access Docker registry (generated automatically for your demo token)
  registryDockerCfg: <YOUR_ACCESS_STRING_IS_HERE>
  # the release channel in use
  releaseChannel: Beta
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
    cniFlannelEnabled: true
    cniFlannel:
      podNetworkMode: vxlan
    prometheusMadisonIntegrationEnabled: false
    nginxIngressEnabled: false
---
# section with the parameters of the bare metal cluster (StaticClusterConfiguration)
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: StaticClusterConfiguration
# address space for the cluster's internal network
internalNetworkCIDRs:
- 10.0.4.0/24
```
{% endofftopic %}
