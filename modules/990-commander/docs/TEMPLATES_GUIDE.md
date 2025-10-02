---
title: "Templating clusters"
---

{% raw %}
To manually install the Deckhouse Kubernetes Platform, the `dhctl` utility is used. It takes three
sets of data as input:

1. SSH connection configuration for the machine that will become the first master node. The
   configuration is used as dhctl command-line keys (it can be specified in YAML format). We will
   refer to this as `SSHConfig`.
2. Infrastructure and basic cluster configuration in the form of a YAML file, referred to as
   `config.yaml`.
3. An optional set of resources to be created in the final installation step, referred to as
   `resources.yaml`.

The file names and their contents are provided for illustrative purposes and compatibility with
older versions of dhctl. Modern versions of dhctl do not restrict the user by the number or names of
configuration files for installation. What logical parts are contained in this data?

1. `SSHConfig`
   1. Username, password, and key for connecting to an existing machine or one that will be created
      during cluster creation (the ability to set a password for sudo is in development).
   2. IP address of the machine if the cluster is being deployed on pre-created machines ("static"
      cluster type)
   3. Other parameters can be found in the dhctl command help; additional details are not
      significant for this documentation.
2. `InitConfiguration` — container registry access configuration (dockerconfig)
3. `ClusterConfiguration` — Kubernetes configuration: version, pod subnets, services, etc.
   1. Infrastructure placement parameters
      1. `<Provider>ClusterConfiguration` — cluster placement parameters in the cloud or
         virtualization API if Deckhouse Kubernetes Platform is not installed on static resources;
      2. or `StaticClusterConfiguration` for a static cluster
4. `resources.yaml` — arbitrary configuration of Deckhouse Kubernetes Platform in the form of
   Kubernetes manifests

All this data in various combinations is used for creating, modifying, and deleting clusters. This
same data is used in Deckhouse Commander, with dhctl used as an internal component.

### Deckhouse Commander

Deckhouse Commander uses the same set of data for configuring clusters as dhctl. However, Deckhouse
Commander adds the ability to automatically synchronize the entire desired configuration with the
cluster.

| Configuration Type        | Configuration Part                             | Installation | Modification | Deletion |
| ------------------------- | ---------------------------------------------- | ------------ | ------------ | -------- |
| Access                    | SSH connection to the master node              | ✓            |              | ✓        |
| Access                    | Container registry<br> `InitConfiguration`     | ✓            |              |          |
| Infrastructure/Access     | Placement<br> `<Provider>ClusterConfiguration` | ✓            | ✓            | ✓        |
| Infrastructure/Kubernetes | Kubernetes<br> `ClusterConfiguration`          | ✓            | ✓            |          |
| Kubernetes                | Platform operating configuration               | ✓            | ✓            |          |

* Creating and deleting a cluster requires SSH access to the master node, while changes do not
  require it.
* Access to the container registry is configured only during installation (in the future, it will be
possible to manage registry parameter changes)
* The basic infrastructure configuration is managed via cloud API access, while the operational
  configuration is used with the commander-agent (hereafter referred to as "agent") — an auxiliary
  module that is included in the cluster during its creation or attachment to Deckhouse Commander.

Deckhouse Commander is the source of truth for configuration. Deckhouse Commander ensures that the
cluster configuration matches the specified one. If Deckhouse Commander detects a discrepancy, it
attempts to bring the cluster to the specified configuration. Hereafter, we'll refer to this process
as "synchronization."

* Infrastructure synchronization is carried out by the cluster-manager component, which uses dhctl.
* Operational configuration synchronization is carried out by the agent component.

Deckhouse Commander divides the configuration of the Deckhouse Kubernetes Platform based on the
principle of traceability. Users can decide which part of the configuration to synchronize and which
to set once during cluster creation. This is how the configuration looks from the perspective of
Deckhouse Commander:

| Configuration Type        | Configuration Part                             | Synchronizing Component |
| ------------------------- | ---------------------------------------------- | ----------------------- |
| Access                    | SSH connection to the master node              | —                       |
| Access                    | Container registry<br> `InitConfiguration`     | —                       |
| Infrastructure/Access     | Placement<br> `<Provider>ClusterConfiguration` | cluster-manager         |
| Infrastructure/Kubernetes | Kubernetes<br> `ClusterConfiguration`          | cluster-manager         |
| Kubernetes                | Platform operational configuration             | commander-agent         |

### Templates

#### The Idea

Deckhouse Commander is designed to manage typical clusters. Since all types of configuration in Deckhouse Commander are
represented in YAML format, clustering templatization is a markup of the required YAML configuration
with parameters and a description of the input parameters scheme of the template. To templatize
YAML, the go template syntax and the sprig function set are used. A custom syntax for fields is used
to describe the scheme of input parameters.

| Type of Deckhouse Commander config | Type          | Purpose                                                                                        |
| --------------------------------- | ------------- | ---------------------------------------------------------------------------------------------- |
| Input parameters                  | Scheme        | Scheme of input parameters of the template                                                     |
| Kubernetes                        | YAML Template | Kubernetes configuration <br> `ClusterConfiguration`                                             |
| Deployment                        | YAML Template | Deployment configuration <br> `<Provider>ClusterConfiguration` or `StaticClusterConfiguration`    |
| SSH parameters                    | YAML Template | SSH connection to the master node                                                              |
| Resources                         | YAML Template | Cluster resources, including any `ModuleConfig`                                                 |
| Primary resources                 | YAML Template | Cluster resources, including any `ModuleConfig`                                                 |
| Installation                      | YAML Template | Installation configuration <br> `InitConfiguration`                                              |

The cluster configuration is created by substituting the input parameters into the configuration
templates. The input parameters are validated by the scheme specified for them.

#### Template versions

An important feature of a template is evolution. It is not enough to create a cluster fleet based on
templates. Templates are improved and updated to meet the new software versions and new requirements
for cluster operation. An updated template allows not only creating new clusters that meet modern
requirements, but also updating existing clusters.

To evolve templates in Deckhouse Commander, a versioning mechanism is provided. When a template receives
updates, a new version is created for it. The version can be accompanied by a comment. Based on the
template version, you can create a cluster and test its performance. If the template version is
unsuitable for use, it can be marked as unavailable for use. Then, cluster administrators will not
be able to switch the cluster to the template version.

In Deckhouse Commander, each cluster is tied to a specific template version. However, technically, the cluster
can be transferred to any other template and any available template version, subject to an invalid
configuration that Deckhouse Commander will not allow to save. When the cluster is transferred to a new
version or template, it is necessary to update the input parameters so that the updated
configuration is created for the cluster. Deckhouse Commander will detect that the target configuration does
not match the last applied configuration and create a task to synchronize the cluster.

#### Complexity of the template

Creating and testing a template is an engineering task, while creating clusters based on a template
does not require a deep dive into technical details in general.

The input parameters of a template are presented to the user in the form of an online form, where
the user enters or selects the parameters necessary to create a cluster. The entire set of input
parameters is defined by the author of the template: which parameters are available, which are
mandatory, in what order they are filled in, what test they are accompanied by, and how they are
formatted for ease of perception by the end user.

Only the author of the template determines how easy or difficult it will be for the end user to use
the template, and what decisions the user needs to make in order to successfully create a cluster.
The more complex the template, the more complex the templating code and the more complex the form of
the template parameters. Deckhouse Commander users themselves determine the ratio of complexity of the
template and the number of templates for different scenarios. Deckhouse Commander is a flexible enough tool.
With it, you can create both one template for all occasions and many templates for each individual
use case.

#### Creating a template

You can add a template to Deckhouse Commander in two ways: by importing an existing one (for example, one
created earlier in another installation of Deckhouse Commander) or by creating a new one from scratch.
Ultimately, the templated configuration must comply with the features of dhctl and Deckhouse
Kubernetes Platform of the version that will be installed using the template.

Where to find documentation for configuration types

* Input Parameters
  * [Parameter Schema](#cluster-and-inventory-record-parameter-scheme)
* Placement
  * [Static Resources](https://deckhouse.io/documentation/v1/installing/configuration.html#staticclusterconfiguration)
  * [Yandex Cloud](https://deckhouse.io/documentation/v1/modules/030-cloud-provider-yandex/cluster_configuration.html)
  * [OpenStack](https://deckhouse.io/documentation/v1/modules/030-cloud-provider-openstack/cluster_configuration.html)
  * [VMware vSphere](https://deckhouse.io/documentation/v1/modules/030-cloud-provider-vsphere/cluster_configuration.html)
  * [VMware Cloud Director](https://deckhouse.io/documentation/v1/modules/030-cloud-provider-vcd/cluster_configuration.html)
  * [Amazon Web Services](https://deckhouse.io/documentation/v1/modules/030-cloud-provider-aws/cluster_configuration.html)
  * [Google Cloud Platform](https://deckhouse.io/documentation/v1/modules/030-cloud-provider-gcp/cluster_configuration.html)
  * [Microsoft Azure](https://deckhouse.io/documentation/v1/modules/030-cloud-provider-azure/cluster_configuration.html)
  * [zVirt](https://deckhouse.io/documentation/v1/modules/cloud-provider-zvirt/cluster_configuration.html)
  * [Bazis.DynamiX](https://deckhouse.io/documentation/v1/modules/cloud-provider-dynamix/cluster_configuration.html)
* Kubernetes
  * [ClusterConfiguration](https://deckhouse.io/documentation/v1/installing/configuration.html#clusterconfiguration)
* Access to Container Registry
  * [InitConfiguration](https://deckhouse.io/documentation/v1/installing/configuration.html#initconfiguration)
* SSH Parameters
  * See examples below

#### Special variables

There are several special variables in the cluster templates.

| Variable         | Purpose                                                                                                                                    |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| dc_sshPublicKey  | The public part of the SSH key. A pair of SSH keys is created for each cluster.<br> Can be used for cloud-init of cloud clusters.          |
| dc_sshPrivateKey | The private part of the SSH key. A pair of SSH keys is created for each cluster.<br> Can be used to access master nodes of cloud clusters. |
| dc_clusterUUID   | UUID of the current cluster. Generated for each cluster.<br>Can be used to tag metrics and logs of the cluster.                            |
| dc_domain        | The domain on which Deckhouse Commander is hosted. Common for the entire application. <br> Example: `commander.example.com`                |
| dc_caBundle      | The Deckhouse Commander certificate, encoded in base64.<br> Used by the agent to verify connections to the Deckhouse Commander server.      |

#### Required manifests

At the moment, Deckhouse Commander does not create invisible configuration, so the author of the template
needs to take into account several manifests in the template to get a full experience using Deckhouse Commander.
In the future, Deckhouse Commander will be improved to reduce the impact of technical features on
the experience of working with it.

##### SSH parameters for a cloud cluster

For a cloud cluster, you can use the private key created by Deckhouse Commander if you do not provide a
predefined key in the OS image. Also, in the virtual machine images, a user will be created under
which Deckhouse Commander will connect to the created machine to start it up as a master node.

```yaml
apiVersion: dhctl.deckhouse.io/v1
kind: SSHConfig
# The name of the user for SSH is defined in the "Hosting" section of the OS image
sshUser: ubuntu
sshPort: 22
# The private key that will be used to connect to VMs via SSH
sshAgentPrivateKeys:
- key: |
    {{- .dc_sshPrivateKey | nindent 4 }}
```

##### SSH and resources for static cluster

Since the machines are created in advance and SSH server, user and key are configured on them, these
data must be provided in the input parameters of the cluster. Unlike the cloud configuration above,
we use not an embedded parameter but one explicitly passed by the user. Some data can always be set
within the template if their parameterization is not considered appropriate.

Pay attention to the `SSHHost` manifests. They declare IP addresses to which Deckhouse Commander has access.
In this example, it is assumed that the input parameter `.masterHosts` is a list of IP addresses
based on which the configuration will contain SSH hosts. Since these are masters, they should be
specified in the quantity of 1 or 3.

```yaml
apiVersion: dhctl.deckhouse.io/v1
kind: SSHConfig
# username and port for SSH configured on the machines
sshUser: {{ .sshUser }}
sshPort: {{ .sshPort }}
# private key used on machines is passed as an input parameter to the cluster
sshAgentPrivateKeys:
- key: |
    {{- .sshPrivateKey | nindent 4 }}

{{- range $masterHost := .masterHosts }}
---
apiVersion: dhctl.deckhouse.io/v1
kind: SSHHost
host: {{ $masterHost.ext_ip }}
{{- end }}
```

Deckhouse Commander will connect to only one SSH host in the list provided, trying hosts in order until a
successful connection is made. The first connected host will become the master node of the cluster.
Once Deckhouse is installed on the first master node, it will be able to add the remaining two
master nodes to the cluster itself if they are declared in the template. To do this, you need to
tell Deckhouse that the machines exist, how to access them, and that they need to be added to the
cluster. To do this, create a **StaticInstance** for the two masters, define **SSHCredentials** for
them, and explicitly write the group of **master** nodes with the parameter
`spec.staticInstances.count=2`. This will ensure that not only are the two static master nodes known
to Deckhouse, but they are also claimed as master nodes. It is advisable to define this part of the
template in the **Resources**. Below is the code of the template for this task:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: SSHCredentials
metadata:
  name: commander-ssh-credentials
  labels:
    heritage: deckhouse-commander
spec:
  sshPort: {{ .sshPort }}
  user: {{ .sshUser }}
  privateSSHKey: {{ .sshPrivateKey | b64enc }}

{{- if gt (len .masterHosts) 1 }}
{{-   range $masterInstance := slice .masterHosts 1 }}
---
apiVersion: deckhouse.io/v1alpha1
kind: StaticInstance
metadata:
  labels:
    type: master
    heritage: deckhouse-commander
  name: {{ $masterInstance.hostname | quote }}
spec:
  address: {{ $masterInstance.ip | quote }}
  credentialsRef:
    apiVersion: deckhouse.io/v1alpha1
    kind: SSHCredentials
    name: commander-ssh-credentials
{{-   end }}
{{- end }}

{{- if gt (len .masterHosts) 1 }}
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
  labels:
    heritage: deckhouse-commander
spec:
  disruptions:
    approvalMode: Manual
  nodeTemplate:
    labels:
      node-role.kubernetes.io/control-plane: ""
      node-role.kubernetes.io/master: ""
    taints:
    - effect: NoSchedule
      key: node-role.kubernetes.io/master
    - effect: NoSchedule
      key: node-role.kubernetes.io/control-plane
  nodeType: Static
  staticInstances:
    count: 2
    labelSelector:
      matchLabels:
        type: master
{{- end }}
```

##### Resources: deckhouse-commander-agent module

Deckhouse Commander synchronizes resources using the deckhouse-commander-agent module. This module is
installed on the target cluster. The commander-agent application requests the current list of
resources for the cluster and updates them in the cluster where it is running. To configure the
agent correctly, you need to create a manifest in the resources that includes the module.

Please pay attention to `commanderUrl`. You will have to specify the scheme of this address: HTTP or
HTTPS.

## Cluster and Inventory Record parameter scheme

There are two forms based on input parameter scheme:

* cluster parameter form, the input scheme is specified in a cluster template;
* record form, the input scheme is specified in its catalog.

The scheme defines an object. There is always an option to edit it in the visual form editor.

### Common fields

#### Text input

A string with default value

```yaml
- key: something
  type: string
  title: Something
  default: No input
```

Optional parameter

```yaml
- key: something
  type: string
  title: Something
  optional: true
```

The parameter is filled once during the creation of the record or cluster, and then it cannot be edited (`immutable`).

```yaml
- key: something
  type: string
  title: Something
  immutable: true
```

The password and its description (see `format` and `description`)

```yaml
- key: password
  type: string
  format: password
  minLength: 8
  span: 2
  title: Password
  description: |
    Create a good password.

    The password must contain such and such elements and be updated every N days.
```

#### Number input

One can specify the maximum value and mark the field as optional

```yaml
- key: ordinarySize
  type: number
  optional: true
  max: 13
```

One can set a minimum and make the input field span over 4 columns (it is maximum width).

```yaml
- key: eliteSize
  type: number
  description: In cm
  min: 18
  span: 4
```

One can set maximum and minimum values.

```yaml
- key: elephantSize
  type: number
  description: In meters
  min: 0.7
  max: 2.5
  span: 1
```

#### Predefined values

##### Simple values

Selection of predefined values. The user sees in the interface the value that they select.

```yaml
- key: kubeVersion
  type: string
  title: Kubernetes Version
  enum:
    - Automatic
    - "1.25"
    - "1.26"
    - "1.27"
```

##### Complex values

Selection of pre-defined object values. In the interface, the user sees a value from `text`, while
technically `value` is chosen for the template. Note that in `value` only object (key-value) type
values are available. This value is not described by a schema, the structure of the object is
arbitrary.

```yaml
- key: kubeVersion
  title: Kubernetes Version
  select:
    - text: Default
      value:
        version: Automatic
        isSupported: true
    - text: 1.25 (EOL in March)
      value:
        version: 1.25.8
        isSupported: true
    - text: 1.26
      value:
        version: 1.26.4
        isSupported: true
    - text: 1.27
      value:
        version: 1.27.3
        isSupported: true
    - text: 1.28 (experimental support)
      value:
        version: 1.28.0
        isSupported: false
```

### Fields supported in cluster template

#### Single record from a catalog

Catalogs have an unchangeable technical name (slug). It is specified in the `catalog` property:

```yaml
- key: slot
  catalog: yandex-cloud-slot
  title: Cloud slot for the cluster
  immutable: true
  description: >
    Select a letter. It will determine the domain, prefix in the cloud, and IP address.
    This slot is unique for all clusters regardless of the template.

    Let's say you chose 'N'. The domain template will be '%s.X.kube.example.com'.
    We recommend naming the cluster 'dev-X'.

    The login and password are always 'admin@example.com'.
```

#### Multiple records from a catalog

Multiple choice is provided by a pair of properties: `minItems` and `maxItems`. Any field can be made a list of data if both of these fields are specified.

```yaml
- key: slot
  catalog: virtual-machines
  title: Worker nodes
  description: Any count of worker nodes
  minItems: 0
  maxItems: 10000
```

#### Auto-selection

Sometimes it is not important to the user which record is selected from the catalog.
Therefore, auto-selection makes a substitution of a unused record automatically. The automatically
selected record can be replaced manually with another one.

```yaml
- key: publicAddressesForFrontendNodes
  title: Public addresses
  catalog: public-ip-addresses
  minItems: 3
  maxItems: 3
  autoselect: true
```

### Parts

#### Separators

##### `header`

* Type: _**string**_

The only type of a divider is a header. It supports only text. It has no other properties.

```yaml
- header: Access to container images
```

#### Properties of input fields

##### `key`

* Type: _**string**_
* Required

It is necessary to identify the value of an input field in a template. Therefore, there must always be a key field property — this field name will be used in the template during configuration rendering.

In the schema:

```yaml
- key: podSubnet
  title: Pod subnet
  type: string
```

In a template:

```yaml
podSubnet: {{.podSubnet | quote }}
```

##### `type`

* Type: _**string**_
* Required
* Supported values:
  * `string`
  * `number`
  * `boolean`

The value has a predefined type: string, number, or boolean.

##### `title`

* Type: _**string**_
* Required

The field has a title that conveys meaning. It is one line of text. It is displayed in the parameter form and in the audit.

##### `description`

* Type: _**string**_

A field may have a comment that reveals the meaning, explains boundary conditions, recording format
or exceptions. There may be several lines of text.

##### `default`

* Type: _depends on `type`_

The Default value is filled in if the field is marked optional. Also, this value is shown to the
user, for example, in the form of a placeholder. The value type of this property must match the
`type`.

##### `format`

* Type: _**string**_
* Supported values:
  * `password`
  * `date-time`
  * `url`
  * `email`
  * `uuid`
  * `cuid`
  * `cuid2`
  * `ulid`
  * `emoji`

String parameters can have a format that determines the specifics of their display and validation.

##### `span`

* Type: _**number**_
* Supported values: `1`, `2`, `3`, `4`
* Default: `1`

This is a decorative property that specifies how much width to occupy on the screen in fractions:
from 1 to 4. Input fields fill the form one line at a time horizontally, like text. In this case,
the width of a “line” in the form is 4 elements.

##### `optional`

* Type: _**boolean**_
* Default: `false`

This flag indicates that the field is optional. An empty value will be ignored and the property will
not be passed to the template.

##### `immutable`

* Type: _**boolean**_
* Default: `false`

This flag indicates that the field is filled only once when it appears in the input parameters. The
field becomes unavailable for editing if it has already been filled. This means that when you update
a cluster to a new template with an immutable field, you can fill it in.

Immunity depends on the life cycle of the parameter in the form, not the cluster.

##### `enum`

* Type: _**array**_

Lists the possible values that the field accepts. The field is represented by a select regardless of
the value type selected in `type`.

##### `selector`

* Type: _**array**_

This is a more complex version of `enum`. It provides a string representation of an object (`text`)
and an arbitrarily complex value in `value` that will be selected for the template. The text is
provided for humans, and the values are for the template.

Example:

```yaml
- key: kubeVersion
  title: Kubernetes Version
  select:
    - text: Default
      value:
        version: Automatic
        isSupported: true
    - text: 1.25 (EOL in March)
      value:
        version: 1.25.8
        isSupported: true
    - text: 1.26
      value:
        version: 1.26.4
        isSupported: true
    - text: 1.27
      value:
        version: 1.27.3
        isSupported: true
    - text: 1.28 (experimental support)
      value:
        version: 1.28.0
        isSupported: false
```

##### `catalog`

* Type: _**string**_

Selecting one value from the catalog. Write the _slug_ of the catalog in the
value. The `type` field does not need to be specified, because in fact it is `object`, the schema of
which is described in the specified catalog. To select several values (and get a list of records
at the entrance to the template), use `minItems` and `maxItems`.

Example:

```yaml
- key: workerMachine
  title: Virtual machine
  catalog: virtual-machines

- key: workerMachines
  title: Virtual machines
  catalog: virtual-machines
  minItems: 1
  maxItems: 10
```

##### `maxLength` (for strings)

* Type: _**number**_

For `type: string`, this field adds validation on the string length.

##### `minItems`, `maxItems` (for records selection)

* Type: _**number**_

Validation of the number of items selected from the catalog. This pair of fields is
optional, but it is prohibited to use them separately: if used, then both at once.

##### `autoselect` (for records selection)

* Type: _**boolean**_
* Default: `false`

Sometimes, it is not so important for the user which specific record is chosen. In this case, the form chooses an available records for the user. However, the user always has the option to change them.

Example:

```yaml
- key: publicAddressesForFrontendNodes
  title: Public addresses
  catalog: public-ip-addresses
  minItems: 3
  maxItems: 3
  autoselect: true
```

##### `identifier`

* Type: _**boolean**_
* Default: _depends on values_

A record is a flat object. A record has a compact one-line representation made up of the field
values of the record (without keys). The record values are separated by commas in the order
specified by the schema. This compact representation can be seen both in the list of records
inside the directory and in the selection of a record in a cluster view (dropdown lists). To
select a limited set of fields for a compact display of a record, use the `identifier` property.

For example, consider a record and its possible schema options.

```yaml
## Record
login: John
password: E3xE#%DH@hW
age: 42
```

<table>
<tr>
<th>   </th>
<th> Show all fields </th>
<th> Hide a field </th>
<th> Choose shown explicitly </th>
</tr>
<tr>
<td style="vertical-align: top">
Scheme
</td>
<td style="vertical-align: top">

```yaml
- key: login
  type: string
  title: Username
  unique: true
  pattern: ^[a-z0-9.-]+$

- key: password
  type: string
  title: Password
  format: password

- key: age
  type: number
  title: Age
```

</td>
<td style="vertical-align: top">

```diff
 - key: login
   type: string
   title: Username
   unique: true
   pattern: ^[a-z0-9.-]+$

 - key: password
   type: string
   title: Password
   format: password
+  identifier: false

 - key: age
   type: number
   title: Age
```

</td>
<td style="vertical-align: top">

```diff
 - key: login
   type: string
   title: Username
   unique: true
   pattern: ^[a-z0-9.-]+$
+  identifier: true

 - key: password
   type: string
   title: Password
   format: password

 - key: age
   type: number
   title: Age
+  identifier: true
```

</td>
</tr>

<tr>
<td style="vertical-align: top"> Representation </td>
<td style="vertical-align: top">

`John, E3xE#%DH@hW, 42`

</td>
<td style="vertical-align: top">

`John, 42`

</td>
<td style="vertical-align: top">

`John, 42`

</td>
</tr>

<tr>
<td style="vertical-align: top"> Behavior </td>
<td style="vertical-align: top">

`identifier=true` is the default for all fields

</td>
<td style="vertical-align: top">

`identifier=true` is the default for all fields, `identifier=false` applies individually

</td>
<td style="vertical-align: top">

`identifier=false` is the default for all fields if `identifier=true` is explicitly set somewhere

</td>
</tr>
</table>

##### `unique`

* Type: _**boolean**_
* Default: `false`
* This flag marks record fields that must be unique within a catalog. It is prohibited to create,
  restore from archive, or import non-unique data.

This flag also marks cluster fields that must be unique among all clusters. Saving a cluster with
non-unique data will result in a validation error and prevent creation and editing of the cluster.

The selection from a catalog for clusters is unique to the extent that records are unique. It is
not necessary to mark a selection from a catalog with this flag.

```yaml
- key: subnetCIDR
  type: string
  title: Subnet CIDR
  unique: true
```
{% endraw %}
