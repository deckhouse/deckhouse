{% alert level="warning" %}
CloudEphemeral node management is moving from Machine Controller Manager (MCM) to Cluster API (CAPI). Existing [NodeGroups](/modules/node-manager/cr.html#nodegroup) continue to use MCM, while new ones are created with CAPI by default. For migrating existing groups, see [How to migrate node groups to Cluster API (CAPI)](../../../../faq.html#how-to-migrate-node-groups-to-cluster-api-capi).
{% endalert %}

## Configuring node groups

The number of nodes to be provisioned and their parameters are defined in the [NodeGroup](/modules/node-manager/cr.html#nodegroup) custom resource, where you also specify the name of the instance class used for that group (the `cloudInstances.classReference` parameter). For the {{ site.data.admin.cloud-types.types[page.cloud_type].name }} cloud provider, the instance class is a custom resource called [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass), which contains the specific configuration of the VMs.

{% alert level="warning" %}
When the module settings are changed, **existing Machine objects in the cluster are NOT recreated** (new Machine objects will use the updated parameters). The recreation only occurs if you modify the [NodeGroup](/modules/node-manager/cr.html#nodegroup) or the [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) parameters.
{% endalert %}

### Configuration examples

The following are two simple configuration examples for the OpenStack cloud provider.

#### Example 1

The following manifest defines the instance class for the first example.

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  name: test
spec:
  flavorName: m1.large
```

#### Example 2

The following manifest defines the instance class for the second example.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-openstack
spec:
  version: 1
  enabled: true
  settings:
    connection:
      authURL: https://test.tests.com:5000/v3/
      domainName: default
      tenantName: default
      username: jamie
      password: <PASSWORD>
      region: SomeRegion
    externalNetworkNames:
    - public
    internalNetworkNames:
    - kube
    instances:
      sshKeyPairName: my-ssh-keypair
      securityGroups:
      - default
      - allow-ssh-and-icmp
    zones:
    - zone-a
    - zone-b
    tags:
      project: cms
      owner: default
```

Replace `<PASSWORD>` with the OpenStack user password.

{% if page.cloud_type == 'selectel' %}

## Creating preemptible nodes in Selectel

For detailed instructions on creating preemptible nodes in Selectel and the limitations of the [`preemptible`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-preemptible) parameter, see [Creating preemptible nodes in Selectel](/modules/cloud-provider-openstack/faq.html#creating-preemptible-nodes-in-selectel).

{% endif %}
