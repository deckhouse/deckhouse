---
title: "Deckhouse Kubernetes Platform web UI"
description: "Deckhouse Kubernetes Platform web interface. Monitoring, parameter configuration, node and module management, security and network configuration."
permalink: en/user/web/ui.html
search: web UI, web interface
---

The Deckhouse Kubernetes Platform (DKP) web UI provides access to cluster management, including monitoring, parameter configuration, node and module management, as well as security and network configuration. Most operations available from the command line through the `d8` utility ([Deckhouse CLI](../../cli/d8/)) or `kubectl` can also be performed through the web UI.

## Accessing the web UI

To open the web UI, do as follows:

1. Enter `console.<CLUSTER_NAME_TEMPLATE>` in the browser address bar, where `<CLUSTER_NAME_TEMPLATE>` is a string corresponding to the cluster DNS name template specified in the global parameter [`modules.publicDomainTemplate`](../../reference/api/global.html#parameters-modules-publicdomaintemplate). The address format may differ depending on the system configuration. Contact your administrator to find out the URL.

1. During the first login, enter the user credentials provided by the security administrator.
   After successful authentication, the main page of the web UI opens. The page contains the following sections:
   - ["Deckhouse"](#deckhouse)
   - ["Node management"](#node-management)
   - ["Multitenancy"](#multitenancy)
   - ["Access"](#access)
   - ["Network"](#network)
   - ["Storage"](#storage)
   - ["Security"](#security)
   - ["Monitoring"](#monitoring)
   - ["Logging"](#logging)

## Deckhouse

### "Overview" subsection

The "Overview" subsection contains key information about the Deckhouse Kubernetes Platform (DKP) cluster and its components.

!["Overview" subsection interface](../../images/console/console_main.png)

Main elements of the web UI:

- **"Deckhouse" and "Kubernetes" panels**: Display the current Kubernetes version and general platform information.
- **"Tools" panel**: Contains buttons for quick access to the following sections:
  - "Kubernetes dashboard"
  - "Documentation"
  - "Grafana" (metrics monitoring)
  - "Prometheus" (metrics collection and storage)
  - "Status page"
  - "Component availability"
  - "Kubeconfig generator"
- **"Warnings" panel**: Displays errors, pending updates, and active alerts.
- **"Node groups with issues" panel**: Analyzes node health and displays problematic groups.
- **"Subsystem statuses" panel**: Displays the state of cluster services.
- **Resource monitoring**: Includes charts and indicators showing load changes.
- **Sidebar menu**: Navigation through the main sections (the visibility of sections depends on the user access level).
- **User menu (bottom left)**:
  - User information
  - Settings (changing system parameters and viewing the web UI version)
  - YAML editor for configuration management

### "Updates" subsection

The "Updates" subsection contains information about DKP releases.

!["Updates" subsection interface](../../images/console/releases.png)

### "Modules" subsection

The "Modules" subsection lists enabled and disabled modules. A filter is available to search for a required module.

!["Modules" subsection interface](../../images/console/modules.png)

### "Global settings" subsection

This subsection provides access to critical DKP cluster settings:

- **"Global cluster settings"**: Configuration of the DNS name template and the list of tolerations.
- **"Global module settings"**: Configuration of the high availability mode, and fields for specifying IngressClass and StorageClass.
- **"HTTPS operating mode"**: Configuration of security certificates.
- **"Kubernetes control plane component resources"**: Configuration of the number of allocated CPU cores and memory size.

{% alert level="warning" %}
These settings affect cluster stability, security, and fault tolerance, so modify them with caution.
{% endalert %}

!["Global settings" subsection interface](../../images/console/global_settings.png)

### Kubernetes

The tabs in the "Kubernetes" subsection contain basic information about the Kubernetes cluster.

!["Configuration" tab of the "Kubernetes" subsection](../../images/console/system-management-deckhouse-cluster-configuration.png)

!["Cloud" tab of the "Kubernetes" subsection](../../images/console/system-management-deckhouse-cluster-configuration-cloud.png)

## Node management

### "Node groups" subsection

The settings in this subsection let you manage Kubernetes node groups, monitor their state, and add new groups.

The node group creation form contains the following fields:

- "Name"
- "Node count"
- "Static instance selector" (cannot be changed after creation)
- "Label expressions"

The lower part of the form contains additional settings, including node update parameters, the node template, system parameters, and Chaos Monkey parameters, which can also be expanded for detailed configuration.

!["Node groups" subsection interface](../../images/console/system-management-nodes-node-groups-ftname-worker.png)

![Group status information in the "Node groups" subsection](../../images/console/system-management-nodes-node-groups-worker.png)

![Editing node template parameters in the "Node groups" subsection](../../images/console/system-management-nodes-node-groups-worker-3.png)

![Editing node system parameters in the "Node groups" subsection](../../images/console/system-management-nodes-node-groups-worker-2.png)

The node group card displays:

- Node type and Kubernetes version
- Total number of nodes, their readiness and update status
- Load metrics (CPU, memory, disk)
- Taints and labels

![Group card in the "Node groups" subsection](../../images/console/master_card.png)

The "Create" button in the upper-right corner opens the form for creating a new node group.

![New group creation button in the "Node groups" subsection](../../images/console/system-management-nodes-node-groups-ftname-worker-2.png)

The form lets you specify the required parameters for the new node group.

![Editing general parameters](../../images/console/system-management-nodes-node-groups-new-type-cloudephemeral-3.png)

![Editing autoscaling parameters](../../images/console/system-management-nodes-node-groups-new-type-cloudephemeral-4.png)

![Editing node update parameters](../../images/console/system-management-nodes-node-groups-new-type-cloudephemeral-5.png)

![Editing node template parameters](../../images/console/system-management-nodes-node-groups-new-type-cloudephemeral-6.png)

![Editing node system parameters](../../images/console/system-management-nodes-node-groups-new-type-cloudephemeral-8.png)

### "Instance classes" subsection

This subsection is intended for managing instance classes used in the cluster.
The class list can be sorted.

!["Instance classes" subsection interface](../../images/console/system-management-dvpinstanceclasses-deckhouse-io.png)

![Information about the selected instance class](../../images/console/system-management-dvpinstanceclasses-deckhouse-io-worker.png)

The "Create resource" form lets you specify parameters for a new instance configuration for the cluster.

![Instance class creation form](../../images/console/add_class.png)

### "Nodes" subsection

This subsection provides summary information about all DKP cluster nodes with filtering and sorting capabilities.

The node card displays:

- Current node state
- Group
- Date and time
- Availability zone
- Internal and external IP addresses
- Container runtime in use
- Kernel version
- Kubelet version
- Operating system

The subsection also contains charts showing CPU, memory, disk, and network traffic usage, allowing you to monitor node performance. To manage node availability in the cluster, the "Cordon" and "Cordon+Drain" buttons are available.

!["Nodes" subsection interface](../../images/console/system-management-nodes-nodes.png)

### "Static instances" subsection

This subsection is intended for managing static instances in the cluster and includes two tabs:

- "Instances": For working with static instances.
- "SSH access": For configuring SSH access.

The interface allows you to quickly find and manage static instances.

The "Add instance" button on the "Instances" tab opens the form for adding a new instance to the cluster. The form requires the following information:

- Instance name
- Instance address
- SSH access method from the drop-down list

Additionally, you can specify labels by defining keys and values for further identification and management of the instance.

![New instance creation form](../../images/console/new_machine.png)

The "Add SSH access" button on the "SSH access" tab opens the form for configuring SSH connections to nodes. The form requires the following information:

- SSH access name
- Username
- Private SSH key
- (Optional) `sudo` password for executing privileged commands

Additional fields are available for changing the SSH port and adding extra SSH arguments.

![SSH access creation form](../../images/console/ssh_access.png)

## Multitenancy

### "Projects" subsection

This subsection is intended for creating projects based on prepared templates that define the set of resources to be created and their parameters.

During project creation, the following operations are performed:

- Parameter validation against the OpenAPI schema.
- Template rendering through Helm.
- Deployment of all described resources in an automatically created namespace.

Standard Kubernetes mechanisms are used for access control, resource limits, and network isolation. This allows you to manage security and workload inside the namespace.

!["Projects" subsection interface](../../images/console/projects.png)

The "Create project" button opens the form for adding a new project based on the selected template. In the form, specify the project name and optionally add labels and annotations.

In the central part of the form, select the project template based on which the required resources will be created.
If necessary, leave a comment.

The lower part of the form contains fields for entering template parameters and displaying its structure.

![New project creation form](../../images/console/system-management-multitenancy-projects-new-spec-2.png)

### "Project templates" subsection

This subsection is intended for creating project templates.

Project templates include basic usage scenarios by default and demonstrate DKP capabilities. To add a new template, use the "Create" button in the upper-right corner.

!["Project templates" subsection interface](../../images/console/system-management-projecttemplates-deckhouse-io.png)

The "New project template" form lets you specify the template name and add labels and annotations for identification.

The form contains two tabs:

- "OpenAPI schema": For describing the value specification in JSON format.
- "Project resource template": For defining Helm-compatible resources and managing the project environment.

!["OpenAPI schema" tab](../../images/console/system-management-multitenancy-project-templates-default-spec.png)

!["Project resource template" tab](../../images/console/system-management-multitenancy-project-templates-default-spec-template.png)

A new template can be created based on an existing one.

![Creating a new template based on an existing template](../../images/console/system-management-multitenancy-project-templates-new-spec-copyfrom-default.png)

### "User namespaces" subsection

This subsection is intended for managing user project namespaces.

The "Overview" section provides information about the general namespace state.

!["Overview" section of the "User namespaces" subsection](../../images/console/projects-user-namespaces-commander-project-1.png)

Here you can manage Deployment parameters and other cluster entities.

![Configuring a Deployment in a user namespace](../../images/console/projects-user-namespaces-commander-project-1-apps-deployments-nginx-spec-placement-affinity.png)

You can also configure resource limits and quotas.

![Configuring limits in a user namespace](../../images/console/projects-user-namespaces-commander-project-1-limitranges-core-new-2.png)

![Configuring quotas in a user namespace](../../images/console/projects-user-namespaces-commander-project-1-resourcequotas-core-new.png)

## Access

### "User operations" subsection

This subsection provides an interface for user management.

!["User operations" subsection interface](../../images/console/operation-with-users-1.png)

If necessary, you can block a user or reset a password.

![User blocking form](../../images/console/operation-with-users-2.png)

![User password reset form](../../images/console/operation-with-users-6.png)

## Network

### "Ingress controllers" subsection

This subsection displays information about the current Ingress controllers responsible for traffic routing inside the DKP cluster.
If necessary, you can sort the list of controllers and add new ones.

The "Nginx Ingress controllers" card displays its main parameters:

- Inbound connection type (LoadBalancer)
- IP address and Ingress class (`nginx`)
- Load balancer access level
- Node selector that determines on which nodes the controller runs

The lower part of the card contains monitoring charts for CPU usage, memory usage, network traffic, and requests per second (RPS), allowing you to monitor controller performance. The "Create from" option is available for cloning the configuration, and the "Delete" option is available for removing the Ingress controller.

The "Add" button in the upper-right corner allows you to select the type of a new inbound connection (inlet) for the Ingress controller.
Available options:

- HostPort
- HostPort with Proxy Protocol
- HostPort with a failover controller
- LoadBalancer
- LoadBalancer with Proxy Protocol

!["Nginx Ingress controllers" card](../../images/console/ingress.png)

## Storage

### "Storage classes" subsection

This subsection is intended for managing storage classes.

!["Storage classes" subsection interface](../../images/console/system-management-storage-storage-classes.png)

### "Persistent volume" subsection

This subsection is intended for managing persistent volumes.

!["Persistent volume" subsection interface](../../images/console/system-management-persistentvolumes-core.png)

### "Local volumes" subsection

This subsection is intended for managing local volumes. It contains four sections for managing pools, volume groups, logical volumes, and block devices.

!["Pools" section interface](../../images/console/system-management-storage-lvm-replicatedstoragepools.png)

!["Volume groups" section interface](../../images/console/system-management-storage-lvm-volume-groups.png)

!["Logical volumes" section interface](../../images/console/system-management-storage-lvm-logical-volumes.png)

!["Block devices" section interface](../../images/console/system-management-blockdevices-storage-deckhouse-io.png)

## Security

### "CVE scanner" subsection

This subsection is intended for scanning container images for vulnerabilities (CVEs) in the DKP cluster.

The subsection contains the "Vulnerability reports" and "Scanned namespaces" tabs:

- "Vulnerability reports" tab displays the results of the latest scans. It provides information about the scanned object, including its name, namespace, type, and resource name, as well as the container and the image in use. If no vulnerabilities are found, a green indicator is displayed. The "Rescan" button allows you to perform a new scan.
- "Scanned namespaces" tab is intended for managing namespaces that are subject to scanning. The interface supports sorting by name and scan parameters. You can optionally hide system namespaces. The required namespaces can be selected for scanning, and scans can be started using the "Rescan" button. Reports for each object are also available for viewing.

!["CVE scanner" subsection interface](../../images/console/scaner_cve.png)

## Monitoring

### "Overview" subsection

This subsection displays the state of Prometheus instances, load metrics, and a section with settings.

The "Overview" subsection contains the "Status" and "Configuration" tabs intended for monitoring and configuring Prometheus instances.

The "Status" tab displays a list of running pods with their names, assigned nodes, statuses, IP addresses, ages, and CPU and memory usage. Each pod includes components such as `init-config-reloader`, [`prometheus`](/modules/prometheus/), `config-reloader`, and `kube-rbac-proxy`, which ensure its operation. The button on the right allows you to delete the selected pod.

!["Status" tab](../../images/console/monitoring_review.png)

The "Configuration" tab contains expandable sections for configuring various aspects of Prometheus operation, including real-time and retrospective metrics, authentication and Grafana integration, as well as resource management. This interface is intended for monitoring metric states in real time and for flexible integration with other services.

!["Configuration" tab](../../images/console/monitoring_review1.png)

### "Metric processing" subsection

This subsection allows you to create and manage metric processing rules. When adding a new rule, specify its name and processing group. This makes it possible to organize and modify incoming metrics before they are forwarded further.

![New metric processing rule creation form](../../images/console/metrics_processing.png)

### "Metric delivery" subsection

This subsection is intended for configuring data export to a local or external Prometheus server. When adding a new resource, specify the URLs for metric delivery, and configure TLS parameters, authentication, and optional metric preprocessing before delivery.

![Metric delivery configuration form](../../images/console/sending_metrics.png)

### "Grafana data sources" subsection

In this subsection, you can configure Grafana integration with various data sources used in dashboards and visualize data.

To create a new data source, specify its name, type, URL, access parameters, and authentication settings.

![New Grafana data source creation form](../../images/console/add_grafana.png)

### "Grafana dashboards" subsection

This subsection is intended for managing dashboards used for metric visualization.

The main interface displays a list of available dashboards.
You can sort by creation time and filter by name or folder.

The dashboard row displays its name, associated folder, and the "Create from" option for creating a new dashboard based on it, as well as the "Delete" option.

To add a new dashboard:

1. Click the "Add" button.
1. Specify the new dashboard name and the folder in which it will be stored (if the folder does not exist, it will be created automatically).
1. Enter a JSON manifest containing the dashboard configuration description. It is important that the manifest does not contain a local ID (except for the UID), otherwise the dashboard may not be displayed correctly in Grafana.

!["Grafana dashboards" subsection interface](../../images/console/grafana_dashboard.png)

### "Active alerts" subsection

This subsection displays a list of current monitoring system alerts.
Alerts can be sorted by name and status.

The alert row displays the alert name, severity level, creation time, and last update time. Related components and modules are also displayed to help identify the source of the issue.

To get detailed information about an alert, click "Read description". At the bottom, an explanation of the alert trigger reason is displayed.

!["Active alerts" subsection interface](../../images/console/active_alerts.png)

A list of all monitoring system alerts available in DKP is provided on a [separate documentation page](../../reference/alerts.html).

## Logging

This section is intended for configuring log sources and managing log delivery.

### "Log delivery" subsection

This subsection is intended for managing logging and configuring log delivery to various storage systems.

To configure log delivery:

1. Click the "Add" button.
1. Select one of the target storage systems:
   - Loki
   - ElasticSearch
   - Logstash
   - Vector
   - Kafka
   - Splunk
1. Specify the storage system name and connection address.
1. If necessary, specify additional settings:
   - TLS parameters for a secure connection.
   - Authentication ("Basic" or "Bearer token").
   - Additional labels for filtering and organizing logs.
   - Buffer parameters that define how logs are stored before delivery (on disk or in memory).
   - Delivery limits that allow you to configure the log record delivery rate.
   - Exclusions that allow you to filter specific logs.

![Log delivery configuration form](../../images/console/logs-send.png)

### "Log collection" subsection

This subsection is intended for configuring log sources that can then be delivered to target storage systems. You can sort and filter existing log collection rules and add new sources.

To configure collection sources:

1. Click the "Add" button.
1. Select one of the sources:
   - File (collecting logs from the file system)
   - KubernetesPods (collecting logs from Kubernetes pods)
1. Specify the name and configure the parameters:
   - "Storage": Specify where the collected logs will be delivered.
   - "File filters": Specify the paths to log files that should or should not be read. Wildcards are supported.
   - "Line delimiter" (optional): Specify the character separating records in the file;
   - "Log filtering" (optional): Add label filtering rules and filters to keep only the required records.

![Log collection source configuration form](../../images/console/log_rule.png)

## Viewing API information

To view API information in Swagger format, click the question mark button in the lower-left corner of the interface in the drop-down user menu.

![Button for viewing API information](../../images/console/releases.png)

![Window with general API information](../../images/console/swagger-1.png)

![Viewing detailed request information](../../images/console/swagger-3.png)
