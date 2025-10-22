---
title: "Web interface of the platform"
permalink: en/user/web/ui.html
search: web UI, web interface
---

The [console](/modules/console/) module's web UI provides access to cluster management, including monitoring, parameter configuration, node and pod management, and security and network configuration. Most operations available on the command line via the d8 (Deckhouse CLI) or kubectl utilities are also performed through the web interface.

## Accessing the console web UI

1. To open the web UI, enter `console.<CLUSTER_NAME_TEMPLATE>` in your browser's address bar,
   where `<CLUSTER_NAME_TEMPLATE>` is the DNS name template of the cluster
   defined in the global `modules.publicDomainTemplate` parameter.
   The exact URL format may vary depending on your system configuration.
   Check with your administrator for details.

1. On your first login, enter the user credentials provided by your security administrator.
   After a successful authentication, the main [console](/modules/console/) web UI page will open.
   It includes the following sections:

   - [Deckhouse](#deckhouse)
   - [Node management](#node-management)
   - [Multitenancy](#multitenancy)
   - [Network](#network)
   - [Security](#security)
   - [Monitoring](#monitoring)
   - [Logging](#logging)

## Deckhouse

### “Overview” subsection

The "Overview" subsection provides key information about the cluster and its components.

![Web console](../../images/console/console_main.png)

**Main interface elements:**

- **Deckhouse and Kubernetes panels**: Show the current Kubernetes version and key platform details.
- **“Tools” panel**: Contains quick access buttons to:
  - Kubernetes dashboard.
  - Documentation.
  - Grafana (metrics monitoring).
  - Prometheus (metrics collection and storage).
  - Status page.
  - Component availability information.
  - Kubeconfig generator.
- **“Alerts” panel**: Displays errors, pending updates, and active alerts.
- **“Node groups with issues” panel**: Analyzes node health and highlights problematic groups.
- **“Subsystem statuses” panel**: Shows the status of cluster services.
- **Resource monitoring**: Includes graphs and indicators for tracking resource load.
- **Side menu**: Navigation between main sections (visible options may vary depending on user access level).
- **User menu (bottom left)** contains:
  - User information.
  - Settings (including system parameters and console module version displayed).
  - YAML editor to manage configuration.

### Updates

The "Updates" subsection provides information about available releases.

### Modules

The "Modules" subsection lists both enabled and disabled modules.
A filter is available to quickly find the required module.

![Modules](../../images/console/modules.png)

### Global settings

Provides access to the following critical cluster parameters:

- **Global cluster-wide settings** (DNS names, tolerations)
- **Global module settings** (HA mode, Ingress class, StorageClass)
- **HTTPS settings** (certificates)
- **Control plane resources** (CPU, memory)

These settings affect the cluster's stability, security, and fault tolerance.
Be cautious when making changes.

![Global settings](../../images/console/global_settings.png)

## Node management

### “Node groups” subsection

This subsection lets you manage Kubernetes node groups, view their status, and add new groups.
The node creation form includes:

- Name
- Number of nodes
- Static machine selector (immutable after creation)
- Label expressions

The lower part of the form provides advanced settings, including update policies, node template, system parameters,
and optional Chaos Monkey settings for advanced configuration.

![Node groups](../../images/console/node_group.png)

A node group card displays:

- Node type and Kubernetes version
- Total count, readiness, update status
- Load monitoring (CPU, memory, disk)
- Taints and labels

Taints and labels are also shown separately, as they help control Pod placement and node behavior.

![A node group card](../../images/console/master_card.png)

### “Machine classes” subsection

This subsection lets you manage the configurations of machines used in the cluster, with sorting options for the list.

![Machine classes](../../images/console/mashine_class.png)

The “Add machine class” menu lets you define parameters for a new machine configuration.
The configuration section includes the class name, while the resource block allows you to set:
- number of virtual CPUs,
- CPU architecture,
- memory size,
- baseline performance,
- number of GPUs,
- image ID.

You can also enable support for preemptible VMs, configure disk size and type, select the primary subnet and network type,
and assign a public IP if needed.

Additional subnets and labels can also be added for more flexible infrastructure setup.

![Add machine class](../../images/console/add_class.png)

### “All group nodes” subsection

This subsection provides a summary view of all cluster nodes, with filtering and sorting by various characteristics.
Each node card displays:
- current state,
- node group,
- date and time,
- availability zone,
- internal and external IP addresses,
- container runtime (CRI),
- kernel version,
- kubelet version,
- operating system.

Graphs for CPU, memory, disk, and network usage are also available to help assess node performance.
The **Cordon** and **Cordon+Drain** buttons let you manage node availability in the cluster.

![All group nodes](../../images/console/groups_nodes.png)

### “Static machines” subsection

This subsection lets you manage static nodes in the cluster via two tabs:
- "Machines" for working with static nodes,
- "SSH Access" for managing authorization.

This interface enables fast discovery and control of static machines in your infrastructure.

The **Add machine** button in the "Machines" tab is used to add a new machine to the cluster.
Required fields include the machine name, address, and SSH access method (selectable from a dropdown list).
You can also assign labels using key-value pairs for identification and management.

![Static machines](../../images/console/new_mashine.png)

The **Add SSH access** button in the "SSH access" tab is used to configure SSH access to nodes.
You will need to provide an access name, username, and private SSH key.
Optionally, you can enter a `sudo` password to run privileged commands.
Fields are also available for customizing the SSH port and adding extra SSH arguments.

![SSH access](../../images/console/ssh_access.png)

## Multitenancy

### “Project templates” subsection

This subsection is used to create project templates.
Default templates include common usage scenarios and serve as examples of functionality.
To add a new template, click **Create project template**.

![Templates](../../images/console/examples.png)

The "New project template" form lets you specify the project name, as well as add labels and annotations for identification.
It has two tabs:
- "OpenAPI schema" for describing value specifications in JSON format,
- "Project resource template" for defining Helm-compatible resources to manage the project's environment.

![New project template](../../images/console/examples_new.png)

### “Projects” subsection

This subsection is used to create a new project based on a preconfigured template
that defines the resources and their parameters.
During creation, the system validates the input values against the OpenAPI schema, renders the template via Helm,
and deploys all described resources into a newly created namespace.
A project uses native Kubernetes mechanisms for access control, resource quotas, and network isolation,
allowing secure and manageable environments in namespaces.

![Projects](../../images/console/projects.png)

To create a project, click **Create project**.

The "New project" form allows the user to create a project based on a preconfigured template.
Set the project name, along with optional labels and annotations.

The middle section lets you select the project template and leave a comment.

The lower section contains input fields for required parameters and a preview of the template structure.

![Create project](../../images/console/projects_new.png)

## Network

### “Ingress controllers” subsection

This subsection displays information about current Ingress controllers used to route traffic within the cluster.

The list can be sorted, and new controllers can be added.
The card for the "nginx" controller includes main parameters:
- inlet type (for example, LoadBalancer),
- IP address,
- Ingress class (for example, nginx) access level to the load balancer,
- node selector indicating which nodes run the controller.

The lower part includes monitoring graphs for CPU, memory, network traffic, and requests per second (RPS),
allowing you to assess the controller's performance.

Available options include "Create based on" (to clone a configuration) and "Delete" (to remove an Ingress controller).

Click **Add** to choose a new inlet type for the controller. Available options:
- HostPort,
- HostPort with Proxy Protocol,
- HostPort with a backup controller,
- LoadBalancer,
- LoadBalancer with Proxy Protocol.

![Ingress](../../images/console/ingress.png)

## Security

### “CVE scanner” subsection

This subsection is used to scan container images in the cluster for vulnerabilities (CVEs).

The "Overview" section includes two tabs:

- "Vulnerability reports": Shows the results of recent scans.
  Each entry includes object name, namespace, resource type and name, container, and used image.
  If no vulnerabilities are found, a green indicator appears.
  You can also manually trigger a re-scan by clicking **Rescan**.
- "Scannable namespaces": Allows managing namespaces included in scanning.
  Supports sorting by name and scan settings.
  System namespaces can be optionally hidden.
  You can select namespaces to scan, trigger scans with the **Rescan** button, and view reports per each object.

![CVE scanner](../../images/console/scaner_cve.png)

## Monitoring

### Overview

The UI displays Prometheus instances, load status, and configuration options.

The "Overview" subsection includes two tabs: Status and Configuration, which are intended to monitor and configure Prometheus instances.

The "Status" tab lists active Pods with name, node, status, IP address, age, CPU and memory usage.
You can see components of each Pod (such as `init-config-reloader`, [prometheus](/modules/prometheus/), `config-reloader`, and `kube-rbac-proxy`).
Pods can be deleted from here.

![Overview](../../images/console/monitoring_review.png)

The "Configuration" tab features expandable sections to configure Prometheus operation settings,
such as operational and historical metrics, authentication, Grafana integration, and resource management.
This interface supports real-time metric tracking and fine-tuning integration with other services.

![Overview](../../images/console/monitoring_review1.png)

### Metrics processing

This section lets you create and manage rules for processing metrics.
When adding a rule, specify a name and processing group to organize and modify incoming metrics before they are forwarded.

![Metrics processing](../../images/console/metrics_processing.png)

### Metrics export

This section lets you configure data export to local or external Prometheus servers.
When creating a resource, specify target URL, TLS settings, authentication options, and optional preprocessing rules for metrics.

![Metrics export](../../images/console/sending_metrics.png)

### Grafana data sources

This interface lets you integrate Grafana with data sources used in dashboards for data visualization.

To add a new data source, specify its name, type, URL, access parameters, and authentication settings.

![Grafana data sources](../../images/console/add_grafana.png)

### Grafana dashboards

This subsection manages dashboards used for metrics visualization.

The main interface displays a list of available dashboards, with the following sorting options:

- By creation time,
- Filtering by name or folder.

Each dashboard includes a name, folder, and options to create a new instance or delete it.

To add a new dashboard:

1. Click **Add**.
1. Specify its name and folder (if the folder doesn’t exist, it will be created automatically).
1. Enter the JSON manifest that describes the dashboard configuration.
   It's important that the manifest doesn't contain any local ID fields, except for UID, to ensure proper visualization in Grafana.

![Grafana dashboards](../../images/console/grafana_dashboard.png)

### Active alerts

This subsection shows a list of current alerts from the monitoring system.
Available filters:

- By name
- By status
- By alert title

Each alert includes its name, severity level, creation and update timestamps,
and associated components or modules to help identify the source of the issue.

To view alert details, click **Read description**.
A brief explanation of the alert's trigger appears at the bottom of the UI.

![Active alerts](../../images/console/active_alerts.png)

## Logging

The "Logging" section is for configuring log sources and managing log exports.

### Log exports

This interface is used to configure log exporting to a log storage.

To set up log forwarding:

1. Click **Add**.
1. Choose one of the target storages:
   - Loki
   - Elasticsearch
   - Logstash
   - Vector
   - Kafka
   - Splunk
1. Set the destination name and connection address.
1. Optionally configure:
   - TLS settings for secure connection.
   - Authentication (Basic or Bearer token).
   - Additional labels for organizing logs.
   - Buffering options that define how logs are stored before forwarding (disk or memory).
   - Forwarding rate limits.
   - Log exclusions (to filter out specific logs).

![Log exports](../../images/console/logs-send.png)

### Log collection

This subsection lets you configure log sources to be forwarded to storage destinations.
You can sort, filter, and add new collection rules.

To configure log sources, follow these steps:

1. Click **Add**.
1. Choose a source type:
   - File (logs from the filesystem).
   - KubernetesPods (logs from Kubernetes Pods).
1. Specify:
   - Storage destination.
   - File filters (paths to include or exclude, wildcards supported).
   - (Optional) Record delimiter character.
   - (Optional) Label and filter rules to retain only specific logs.

![Log collection](../../images/console/log_rule.png)
