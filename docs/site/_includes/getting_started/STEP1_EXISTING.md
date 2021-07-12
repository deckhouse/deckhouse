- Set up SSH access between the machine used for the installation and the existing cluster master node.
- Create configuration file (`config.yml`) for Deckhouse:

{% offtopic title="config.yml for CE" %}
```yaml
# section for bootstrapping the Deckhouse cluster (InitConfiguration)
# version of the Deckhouse API
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: InitConfiguration
# Deckhouse parameters
deckhouse:
  # address of the registry where the installer image is located; in this case, the default value for Deckhouse CE is set
  imagesRepo: registry.deckhouse.io/deckhouse/ce
  # a special string with parameters to access Docker registry
  registryDockerCfg: "eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5pbyI6IHt9fX0="
  # the release channel in use
  releaseChannel: Beta
  configOverrides:
    deckhouse:
      bundle: Minimal
    global:
      # the cluster name (it is used, e.g., in Prometheus alerts' labels)
      clusterName: main
      # the cluster's project name (it is used for the same purpose as the cluster name)
      project: someproject
      modules:
        # template that will be used for system apps domains within the cluster
        # e.g., Grafana for %s.somedomain.com will be available as grafana.somedomain.com
        publicDomainTemplate: "%s.somedomain.com"
```
{% endofftopic %}
{% offtopic title="config.yml for EE" %}
```yaml
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
    deckhouse:
      bundle: Minimal
    global:
      # the cluster name (it is used, e.g., in Prometheus alerts' labels)
      clusterName: main
      # the cluster's project name (it is used for the same purpose as the cluster name)
      project: someproject
      modules:
        # template that will be used for system apps domains within the cluster
        # e.g., Grafana for %s.somedomain.com will be available as grafana.somedomain.com
        publicDomainTemplate: "%s.somedomain.com"
```
{% endofftopic %}
