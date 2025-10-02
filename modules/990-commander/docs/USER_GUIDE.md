---
title: "User Guide"
---

## Address

If the [public domain
template](https://deckhouse.io/documentation/v1/deckhouse-configure-global.html#parameters-modules-publicdomaintemplate)
in the `%s.example.com` cluster, the web application can be accessed at
`https://commander.example.com`.

## Workspaces

Working with Deckhouse Commander entities is carried out within workspaces. Workspaces can be
created and given a new name. In the future, access to workspaces will be possible to control in
detail.

Users manage clusters, cluster templates and inventory within a workspace. Also, an API access token
is issued within the workspace. The visibility of objects for such a token will be limited only to
what is located in the workspace.

## Clusters

We recommend installing Deckhouse Commander in the control cluster. This cluster should serve the
purpose of centralized management and information collection from the entire application
infrastructure, including application clusters. Clusters managed by Deckhouse Commander are called
application clusters. Deckhouse Commander is the source of truth for cluster configuration. We will
look at how this works in practice.

### Cluster configuration

The cluster configuration consists of several sections:

1. **Input parameters** are formed based on the *input parameter schema* of the cluster template.
2. **Infrastructure section**:
   * Kubernetes — `ClusterConfiguration`.
   * Placement — `<Provider>ClusterConfiguration` or `StaticClusterConfiguration`.
   * SSH settings — connection to the cluster master nodes.
   * Container registry — `InitConfiguration`, contains dockerconfig.
3. **Kubernetes section** — an arbitrary number of tabs with Kubernetes manifests.

#### Cluster parameters (input parameters)

This is a user configuration template for the template user. See [Input parameters](usage.html#схема-параметров-кластера-и-ресурса).

#### Infrastructure section

##### Kubernetes

Settings for the Kubernetes version, pod and service subnets. See
[ClusterConfiguration](https://deckhouse.io/documentation/v1/installing/configuration.html#clusterconfiguration).

##### Placement

Features of cluster placement in the infrastructure. Here, for a static cluster, the configuration may remain empty.

* [Static resources](https://deckhouse.io/documentation/v1/installing/configuration.html#staticclusterconfiguration)

For cloud clusters, specifics of access to the cloud API, a set of nodes that will be created automatically and monitored (including master nodes), availability zone settings, etc., are specified.

* [OpenStack](https://deckhouse.io/documentation/v1/modules/030-cloud-provider-openstack/cluster_configuration.html)
* [VMware Cloud Director](https://deckhouse.io/documentation/v1/modules/cloud-provider-vcd/cluster_configuration.html)
* [VMware vSphere](https://deckhouse.io/documentation/v1/modules/030-cloud-provider-vsphere/cluster_configuration.html)
* [Yandex Cloud](https://deckhouse.io/documentation/v1/modules/030-cloud-provider-yandex/cluster_configuration.html)
* [zVirt](https://deckhouse.io/documentation/v1/modules/cloud-provider-zvirt/cluster_configuration.html)
* [Basis.DynamiX](https://deckhouse.io/documentation/v1/modules/cloud-provider-dynamix/cluster_configuration.html)
* [Amazon Web Services](https://deckhouse.io/documentation/v1/modules/030-cloud-provider-aws/cluster_configuration.html)
* [Google Cloud Platform](https://deckhouse.io/documentation/v1/modules/030-cloud-provider-gcp/cluster_configuration.html)
* [Microsoft Azure](https://deckhouse.io/documentation/v1/modules/030-cloud-provider-azure/cluster_configuration.html)

### SSH parameters

```yaml
apiVersion: dhctl.deckhouse.io/v1
kind: SSHConfig

sshBastionHost: 10.1.2.3              # Jump host parameters, if any.
sshBastionPort: 2233
sshBastionUser: debian

sshUser: ubuntu
sshPort: 22
sshAgentPrivateKeys:                  # List of private keys, at least one key is required.
  - key: |                            #
      -----BEGIN RSA PRIVATE KEY-----
      .............................
      -----END RSA PRIVATE KEY-----
    passphrase: qwerty123             # Key password, if the key is protected by it.

sshExtraArgs: -vvv                    # Additional SSH parameters.

---

apiVersion: dhctl.deckhouse.io/v1     # Description of hosts for connection.
kind: SSHHost                         # Usually there are 1 or 3 hosts assigned
host: 172.16.0.1                      # for the role of master nodes of static clusters.
---
apiVersion: dhctl.deckhouse.io/v1
kind: SSHHost
host: 172.16.0.2
---
apiVersion: dhctl.deckhouse.io/v1
kind: SSHHost
host: 172.16.0.3
```

### Container registry

Here (see
[InitConfiguration](https://deckhouse.io/documentation/v1/installing/configuration.html#initconfiguration)).
This manifest contains the container registry where the Deckhouse Kubernetes Platform images will be
taken from.

#### Kubernetes section

Arbitrary manifests of Kubernetes and Deckhouse resources. Depending on the management mode, they
will either be enforced in the cluster or created once and then ignored.

Many tabs with manifests can be created, each of which can set the management mode. This allows you
to flexibly manage the mandatory and recommended configuration of the cluster.

### Cluster status

#### Infrastructure

Managing clusters boils down to three types of operations: creating, deleting, and changing a
cluster. At any given time, a cluster in Deckhouse Commander has one of the infrastructure statuses:

* **New** — the cluster configuration has been created in Deckhouse Commander, but the cluster
  itself is still awaiting creation.
* **Configuration error** — the cluster configuration was created in Deckhouse Commander with
  errors, so the cluster will not be created.
* **Creating** — Deckhouse Commander is deploying the cluster.
* **Ready** — the cluster has been created, and the infrastructure state matches the configuration
  specified in Deckhouse Commander.
* **Changing** — Deckhouse Commander is bringing the cluster state to the specified configuration.
* **Change error**, **creation error**, **deletion error** — internal or external errors that
  occurred during cluster management.
* **Archived** — the cluster is no longer monitored by Deckhouse Commander; it was previously
  deleted or left unmanaged by Deckhouse Commander.

Deckhouse Commander performs operations asynchronously using jobs that perform operations on the
cluster. Jobs, and consequently operations, can be cluster installation, removal, modification, or
verification of its configuration against the actual state. Operations are shown inside the cluster
on the «cloud» tab (including for static clusters). A log of execution is available for each job.
The result of the job determines the infrastructure status of the cluster.

Infrastructure operations are performed by the *cluster manager* component. With a given
*verification interval*, the cluster manager verifies that the target and actual infrastructure
configurations match. If they do not, it brings the infrastructure to the target state. If the
cluster has manual processing mode enabled, the user must manually confirm the changes being made.
In automatic cluster processing mode, confirmation is not requested.

The speed at which the cluster manager takes jobs is determined by the number of clusters and the
number of cluster manager replicas. If the total number of jobs greatly exceeds the number of
cluster manager replicas, then operations on clusters will be delayed.

#### Kubernetes

In addition to the infrastructure status, the cluster also has a Kubernetes configuration status. It
indicates whether the cluster matches the configuration of the Kubernetes manifests. Resource
manifests (hereinafter referred to as simply «resources») are part of the cluster configuration.

The Kubernetes configuration status can have three values:

* **Configured** — full compliance.
* **Not configured** — discrepancy between the configuration and the state of the cluster.
* **No data** — data on the configuration status is outdated.

The component responsible for ensuring that the cluster matches the specified resource configuration
is installed inside the application cluster — the *Deckhouse Commander agent* or commander-agent
(hereinafter referred to as simply the «agent»). The agent always tries to bring the cluster
configuration to the specified one.

The agent connects to the Deckhouse Commander API and downloads resource manifests, then applies
them. If a resource created by the agent is deleted in the application cluster, the agent will
recreate the resource within a minute. If a resource is deleted from the cluster configuration, the
agent will delete the resource in the application cluster. If the agent cannot apply a resource for
some reason, the Kubernetes status in Deckhouse Commander will be «not configured».

Each group of Kubernetes resources can have an independent configuration control mode. The
«Kubernetes» configuration does not explicitly participate in the reconciliation operation; the
agent does this in a separate hidden cycle. The result of the agent's work is presented in a
separate «Kubernetes» table on the cluster page.

In addition to synchronizing resource configurations in Kubernetes, the agent reports telemetry data
to Deckhouse Commander:

* The current version of Deckhouse Kubernetes Platform.
* Availability of an update to a new version of Deckhouse Kubernetes Platform.
* Deckhouse Kubernetes Platform update channel.
* Kubernetes version.
* Availability of system components.
* Warnings requiring user attention (alerts, manual confirmation of node reboot, etc.).
* Key cluster metrics: total CPU, memory, disk storage, and total number of nodes.

### Creation

Clusters are created based on cluster templates. To create a cluster, the user selects a template,
fills in the template input parameters (they are provided by the template), and then clicks the
«install» button. This gives the cluster a configuration, and the cluster is assigned to the
template, to a specific version of the template. The template version or the template itself can be
changed.

As the input parameters are filled in, the cluster configuration is rendered in YAML format. If
errors are found in the configuration, the Deckhouse Commander interface will display them. If a new
cluster with errors is saved, its installation will not start until the errors are corrected. In
other words, the cluster will have the «Configuration error» status, and the installation job will
not be created until the configuration is changed to a correct one. Errors in the cluster
configuration can be caused by either template code or incorrectly filled in input parameters.

When the configuration becomes valid, a job to install the cluster is created, after which the
cluster manager creates the cluster. If the cluster is created on pre-existing machines, Deckhouse
Commander configures the Deckhouse Kubernetes Platform components on them, and then creates the
specified Kubernetes resources. If the cluster uses the API of a cloud platform or virtualization
platform, then before the steps mentioned above, Deckhouse Commander creates the infrastructure. The
exact set of cloud resources depends on the cloud provider.

After the cluster is successfully installed, Deckhouse Commander will periodically reconcile its
configuration. If the infrastructure configuration diverges from the one declared in Deckhouse Commander,
it will create a job to change the infrastructure to bring it to the
declared state. Configuration divergence can occur on either the infrastructure side or the
Deckhouse Commander side. In the first case, this means a change in the cloud API, for example, if
something was manually changed in the cloud resource configuration. In the second case, this means a
change in the cluster configuration, which we will discuss in the next section.

### Updating

Changing the cluster configuration means that a new configuration has been saved to the cluster,
different from the previous one. This can be the result of changing the input parameters of the
current cluster template. It can also be the result of switching the cluster to a new version of the
template or even to a different template.

When the cluster configuration changes, Deckhouse Commander creates a task to change the cluster
infrastructure. The agent brings the Kubernetes configuration to the desired state in parallel with
the infrastructure change.

Changes to the cluster configuration can lead to destructive changes in the infrastructure. For
example, this can include changes to virtual machines that require their deletion or re-creation.
Another example is a change in the composition of cloud availability zones. When Deckhouse Commander
detects destructive changes, it does not enforce these changes until the user confirms them.

### Deletion

Deleting clusters in Deckhouse Commander can be achieved in two ways. Both methods are available in
the cluster on an equal basis.

The first method is to clear the infrastructure of the cluster. In this case, Deckhouse Commander
creates a deletion task. Static resources are cleared of Deckhouse Kubernetes Platform components,
and cloud resources are deleted (for example, virtual machines). After deletion, the cluster
configuration does not disappear; the cluster goes into the archive. Its configuration can be
returned to if necessary, and this cluster will no longer be listed among active clusters. This
distinguishes the archived cluster from the active one.

The second way to delete a cluster is manual deletion. Deckhouse Commander will move the cluster to
the archive but will not clean up the infrastructure. This method can be useful if, for some reason,
Deckhouse Commander cannot handle the correct deletion of the cluster using the first method. In
this case, the cluster will have the «deletion error» status. The user will need to manually clean
up the resources occupied by Deckhouse Kubernetes Platform and move the cluster to the archive
manually.

### Cluster migration between workspaces

Clusters are created within a workspace. However, it is possible to transfer the created cluster
from one workspace to another. During the transfer, the cluster will be detached from its template,
and the template will remain in the original workspace. The inventory used in the cluster will be
transferred or copied to the new cluster workspace, depending on the mode of using records:
exclusively used records will be transferred, and non-exclusively used ones will be copied. At the
same time, missing directories with the correct identifier will be created in the new workspace.

### Attachment

Deckhouse Commander supports attachment of existing **DKP clusters**. Unlike the cluster creation process, the attachment procedure requires an already existing cluster.

To join an existing DKP cluster, click the **Attach** button on the cluster list page, then specify the cluster name and **SSH Parameters** for connecting to the master node.
Example of basic configuration:

```yaml
apiVersion: dhctl.deckhouse.io/v1
host: MASTER_NODE_ADDRESS
kind: SSHHost
---
apiVersion: dhctl.deckhouse.io/v1
kind: SSHConfig
sshAgentPrivateKeys:
  - key: "PRIVATE_KEY"
sshPort: 22
sshUser: ubuntu
```

{{<alert level="info">}}
The full list of parameters can be found in the SSHConfig section of the dhctl specification.
{{</alert>}}

Once all the necessary parameters have been specified:

1. Click the **Connect** button.
1. Deckhouse Commander will verify the configuration's correctness:
   * If the configuration is correct, it will attempt to connect to the cluster.
1. Upon successful connection:
   * The cluster will be saved with the status **Ready to Join**.
   * It will become available in the general list of clusters.
1. Navigate to the cluster's page.
1. Click the **Join** button.
1. Wait for the joining procedure to complete.
1. Upon successful completion:
   * The cluster will change to the **Ready** status.

### Detachment

Detachment of a cluster is necessary to remove it from the control of **Deckhouse Commander**. As a result, the cluster's configuration and Kubernetes resources will no longer be synchronized, and the cluster will become autonomous. Subsequently, such a cluster can be reattached, but information about the template version from which it was deployed will be **lost**.

To detach a cluster, go to the target cluster's page, select the **Detach** option from the operation dropdown menu (3 vertical dots), and confirm your choice. After the detachment procedure is completed, the cluster will be moved to the **Archive**.

## Templates

Deckhouse Commander was created to manage typical clusters. Since all sections of the cluster
configuration are presented in YAML format, clustering templating involves marking the required YAML
configuration with parameters and describing the schema of these parameters. For YAML templating,
the go template syntax and a set of sprig functions are used. To describe the schema of input
parameters, a proprietary syntax similar to OpenAPI3 is used, but simpler.

The cluster configuration is created by substituting input parameters into section templates. The
input parameters are validated by the schema specified for them. The input parameter schema in the
Deckhouse Commander web application can be specified either using text configuration or using a
visual form builder. Read about input parameters in the section on working with templates.

Templates have versions. When a template is updated, a new version of the template is created. The
previous version of the template remains available so that it can be used in clusters. However, the
template author can mark template versions as unavailable for use.

Each cluster in Deckhouse Commander has a configuration that was obtained from a template (unless an
existing cluster was imported). The cluster also «remembers» which template and which version of it
was used to configure it. Thanks to this binding, a set of input parameters for the cluster is
displayed in the cluster in the form of a web form from a given version of a given template.

When a cluster is switched to a new template or a new version of a template, the set of input
parameters may change. This may include mandatory parameters that were not initially filled in and
do not have default values. Then, when switching from one template to another (or from one version
of a template to another version of the same template), it may be necessary to change or supplement
the input parameters so that the new configuration is created correctly.

Inside the template interface, there is a list of clusters whose configuration is based on this
template at the moment. From this interface, you can translate many clusters to a new (or old)
version of the template in a few clicks. This operation will fail if the cluster configuration
contains errors. This can happen, among other things, because mandatory input parameters that are
not provided in the current version of the template are missing, but are present in the new one.

Creating and maintaining a template can be a painstaking engineering task that requires testing the
installation and updating of clusters. Template versions can accumulate during this work. To make it
easier to navigate versions, Deckhouse Commander provides the ability to leave a comment for the
template version. It is also possible to make template versions unavailable in the cluster
interface. This can be useful to protect users from заведомо non-working template versions.

## Inventory, Catalogs

### Catalogs and records

In some cases, in clusters, it is necessary to use the same data repeatedly. For example, for many
clusters, you can provide the option to choose a release channel for updating the Deckhouse
Kubernetes Platform or the address of the container registry from which images will be fetched.

To avoid having to fix such data in templates, use Inventory. Inventory is a collection of catalogs
with data. Each catalog defines a data schema, after which the catalog is populated with records.
The records are validated against the specified data schema.

When creating a catalog, you can choose how to use the records:

1. A record in the catalog can be used simultaneously in several clusters.
2. A record in the catalog can only be used in one cluster; deleting or detaching a cluster frees up
   the record for use in other clusters.

The first option is suitable for reusable configuration. The second option is for using pre-prepared
infrastructure. This can include dedicated subnets, pre-created load balancers, virtual machines,
domain names, IP addresses, and so on. It is convenient to prepare such data in advance and track
whether they are being used and, if so, in which clusters.

During catalog creation, the user specifies the name of the catalog, the schema, and the identifier.
The identifier cannot be changed, while the catalog name can be changed at any time. The data schema
can only be changed if there are no records in the catalog that are used in any cluster.

The data schema for the catalog is defined by the same syntax and visual constructor as the input
parameters for the cluster template. An example of a catalog schema:

```yaml
- key: hostname
  type: string
  title: Hostname
  unique: true
  pattern: ^[a-z0-9.-]+$
  identifier: true

- key: ip
  type: string
  title: IP Address
  format: ipv4
  unique: true
  identifier: true
```

### How to use a catalog in a cluster

In the cluster template, you need to indicate that the field is a selection from the catalog, for
this, use its identifier. Example of a parameter:

```yaml
- key: workerMachines     # parameter name in the template
  title: Workers
  catalog: worker-nodes   # the catalog identifier
  minItems: 1
  maxItems: 10
```

Even though specific catalog is defined in the template input parameters, when in cluster, the
catalog might be switched to any other catalog accessible in the workspace.

### Importing data for catalogs

Catalogs can be imported via API or through the interface by uploading a JSON
file. If the identifier of an existing catalog is specified in this file, then
records during import will be added to it regardless of compliance with the data
schema. The data schema will not be overwritten if the catalog already exists. An
example of a catalog file with records that can be imported:

```json
{
  "name": "Worker nodes",
  "slug": "worker-nodes",
  "params": [
    {
      "key": "hostname",
      "type": "string",
      "title": "Hostname",
      "unique": true,
      "pattern": "^[a-z0-9.-]+$",
      "identifier": true
    },
    {
      "key": "ip",
      "type": "string",
      "title": "IP address",
      "format": "ipv4",
      "unique": true,
      "identifier": true
    }
  ],
  "resources": [
    { "values": { "ip": "10.128.0.39", "hostname": "worker-1" } },
    { "values": { "ip": "10.128.0.47", "hostname": "worker-2" } },
    { "values": { "ip": "10.128.0.24", "hostname": "worker-3" } },
    { "values": { "ip": "10.128.0.17", "hostname": "worker-4" } },
    { "values": { "ip": "10.128.0.55", "hostname": "worker-5" } },
    { "values": { "ip": "10.128.0.49", "hostname": "worker-6" } }
  ]
}
```

## Cluster migration between workspaces

Clusters are created within a workspace. However, it is possible to transfer the created cluster
from one workspace to another. During the transfer, the cluster will be detached from its template,
and the template will remain in the original workspace. The inventory used in the cluster will be
transferred or copied to the new cluster workspace, depending on the mode of using records:
exclusively used records will be transferred, and non-exclusively used ones will be copied. At the
same time, missing directories with the correct identifier will be created in the new workspace.

## Integration API and Tokens

The Deckhouse Commander API provides a limited set of actions:

1. Create, change, and delete clusters
2. Create, change and delete resources in catalogs
3. Read templates
4. Read resource catalogs

To access the API in Deckhouse Commander, you can issue a token. The token can have either rights to all
possible operations in the API or only read rights.

Details of the API implementation are described in the [Integration API](./integration_api.html) section.

## Audit

For all entities, Deckhouse Commander keeps a history of changes. Clusters, templates, resources, catalogs,
API access tokens - for all of them, a history of actions and changes is recorded, which can be used
to track who, when and what actions were performed in Deckhouse Commander.

Currently, this functionality only relates to work related to the Deckhouse Commander API. In the future, an
audit log from application clusters will be available in Deckhouse Commander.
