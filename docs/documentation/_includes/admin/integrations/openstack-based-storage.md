## Storage

### Cinder CSI resilience during re-authentication in OpenStack

The Cinder CSI driver supports OpenStack re-authentication with service catalog refresh. This improves the reliability of volume operations in `cinder-csi-plugin` pods that have been running for a long time without being restarted.

When a token expires or a `401 Unauthorized` response is received, the driver re-authenticates with OpenStack and updates the data used to access OpenStack API services. As a result, PersistentVolume operations, including volume attachment and detachment, continue without requiring CSI driver pods to be restarted.

This behavior is especially important for clusters where `cinder-csi-plugin` pods run for a long time without being restarted, while the OpenStack service catalog may change between the initial authentication and subsequent API requests.

### Configuring online volume resize

When resizing a disk via the OpenStack API, Cinder does not pass updated size information to Nova. As a result, the disk size inside the guest OS remains unchanged.

To fix this, you must configure access to the Nova API in the `cinder.conf` file. For example:

{% raw %}

```ini
[nova]
interface = admin
insecure = {{ keystone_service_internaluri_insecure | bool }}
auth_type = {{ cinder_keystone_auth_plugin }}
auth_url = {{ keystone_service_internaluri }}/v3
password = {{ nova_service_password }}
project_domain_id = default
project_name = service
region_name = {{ nova_service_region }}
user_domain_id = default
username = {{ nova_service_user_name }}
```

{% endraw %}

For more details, see the [OpenStack-Ansible bug report](https://bugs.launchpad.net/openstack-ansible/+bug/1902914).

### Using rootDiskSize

If the instance class of the group sets the [`rootDiskSize`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) parameter, the root disk is created in Cinder. In clouds where volume types are bound to availability zones, set the volume type for each zone in the [`volumeTypeMap`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-volumetypemap) parameter of the OpenStackInstanceClass resource. Otherwise, provisioning a node in a zone where the default volume type is unavailable fails with the `HTTP 400 Availability zone '…' is invalid` error.

#### Disks in {{ site.data.admin.cloud-types.types[page.cloud_type].name }}

A node's disk can be either local or network-attached. In {{ site.data.admin.cloud-types.types[page.cloud_type].name }} terminology:

- A local disk is called an ephemeral disk.
- A network-attached disk is called a persistent disk (Cinder storage).

The ephemeral disk is deleted along with the VM, while the persistent disk remains in the cloud even after the VM is deleted.

- For master nodes, network-attached disks are preferred so the nodes could migrate between hypervisors.
- For ephemeral nodes, local disks are preferred for cost efficiency.
  However, not all cloud providers support local disks. If not supported, you have to use network-attached disks for ephemeral nodes.

| Local disk (ephemeral)    | Network-attached disk (persistent)                    |
| ----------------------------- | -------------------------------------------- |
| Deleted with the VM         | Remains in cloud and can be reused |
| Less expensive                       | More expensive                                       |
| Suitable for ephemeral nodes  | Suitable for master nodes                    |

#### rootDiskSize parameter

The OpenStackInstanceClass resource includes a [`rootDiskSize`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) parameter. In {{ site.data.admin.cloud-types.types[page.cloud_type].name }}, flavors may also define a disk size.

The resulting disk type depends on the combination of the following parameters:

|                              | flavor disk size = 0                 | flavor disk size > 0                              |
| ---------------------------- | ------------------------------------ | ------------------------------------------------- |
| **`rootDiskSize` not set** | ❗Disk size must be specified. VM creation will fail without it. | Local disk with size from the flavor.               |
| **`rootDiskSize` set**    | Network-attached disk with size that equals `rootDiskSize`. | ❗ Both network-attached (`rootDiskSize`) and local (from flavor). Avoid this setup, as the cloud provider may charge for both. |

{% if page.cloud_type != 'selectel' %}

> In Selectel Cloud, when creating nodes with CloudEphemeral type outside zone A,
> you must pre-create a flavor with the target disk size.
> In this case, you do not need to set the [`rootDiskSize`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) parameter.

{% endif %}

**Recommendations for master nodes and bastion with network-attached disks**

- Use a flavor with zero disk size.
- Set [`rootDiskSize`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) in the OpenStackInstanceClass resource.
- Ensure the disk type is correct:
  - If specified in the OS image, it will be used.
  - If not specified, the type will be taken from [`volumeTypeMap`](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-masternodegroup-volumetypemap).

**Recommendations for ephemeral nodes with local disks**

- Use a flavor with a defined disk size.
- Do not use the [`rootDiskSize`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) parameter in OpenStackInstanceClass.
- Ensure the disk type is correct:
  - If specified in the OS image, it will be used.
  - If not specified, the cloud provider's default disk type will be used.

#### Checking the disk size in a flavor

Run the following command:

```shell
openstack flavor show m1.medium-50g -c disk
```

Example output:

```console
+-------+-------+
| Field | Value |
+-------+-------+
| disk  | 50    |
+-------+-------+
```

### Overriding the cloud provider's default disk type

If your cloud provider offers multiple volume types, you can set the default disk type for an image by specifying the disk type name in the image metadata. All VMs created from that image will use the specified network disk type.

You can also create a new {{ site.data.admin.cloud-types.types[page.cloud_type].name }} image and upload it.

Example:

```shell
openstack volume type list
openstack image set ubuntu-18-04-cloud-amd64 --property cinder_img_volume_type=VOLUME_NAME
```

{% if page.cloud_type != 'vk-private' and page.cloud_type != 'vk' %}

### Offline disk resizing

If the following error occurs during disk resizing, take these steps:

- Reduce the number of StatefulSet replicas to 0.
- Wait for the disk size to update.
- Restore the original number of replicas.

```text
Warning  VolumeResizeFailed     5s (x11 over 41s)  external-resizer cinder.csi.openstack.org                                   
resize volume "pvc-555555-ab66-4f8d-947c-296520bae4c1" by resizer "cinder.csi.openstack.org" failed: 
rpc error: code = Internal desc = Could not resize volume "bb5a275b-3f30-4916-9480-9efe4b6dfba5" to size 2: 
Expected HTTP response code [202] when accessing 
[POST https://public.infra.myfavourite-cloud-provider.ru:8776/v3/555555555555/volumes/bb5a275b-3f30-4916-9480-9efe4b6dfba5/action], but got 406 instead
{"computeFault": {"message": "Version 3.42 is not supported by the API. Minimum is 3.0 and maximum is 3.27.", "code": 406}}
```

{% endif %}

## LoadBalancer configuration

{% alert level="warning" %}
To correctly determine the client IP address, use a LoadBalancer with Proxy Protocol support.
{% endalert %}

It is recommended to limit the list of nodes added to the load balancer pool using the [`loadbalancer.openstack.org/node-selector`](https://github.com/kubernetes/cloud-provider-openstack/blob/master/docs/openstack-cloud-controller-manager/using-openstack-cloud-controller-manager.md#load-balancer) annotation.

Without a `node-selector` restriction, cloud-controller-manager may use all suitable cluster nodes as load balancer targets. As a result, adding or removing nodes that are not related to the workload served by the load balancer may trigger an update of the load balancer pool membership. In large or frequently changing clusters, such updates may occur regularly and, in some configurations, may cause brief disruptions to existing connections.

The `annotations` field of the corresponding inlet configuration in the [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) resource supports the following annotations:

- `loadbalancer.openstack.org/node-selector`: Selects the nodes that will be used as LoadBalancer targets.
- `loadbalancer.openstack.deckhouse.io/load-balancer-id`: Instructs OpenStack CCM to use a pre-created Octavia load balancer.
- `loadbalancer.openstack.deckhouse.io/load-balancer-address`: Instructs OpenStack CCM to associate a pre-allocated floating IP with the load balancer it creates.

Deckhouse Platform automatically adds the specified annotations to the generated Service object of type LoadBalancer.

When using the `loadbalancer.openstack.deckhouse.io/load-balancer-id` annotation, the load balancer must meet the following requirements:

- reside in the cluster subnet
- be in the `ACTIVE` state

If `loadbalancer.openstack.deckhouse.io/load-balancer-id` is used to reference a pre-created load balancer with a custom name, associate the floating IP with its VIP port before creating the cluster. In this case, do not specify the `loadbalancer.openstack.deckhouse.io/load-balancer-address` annotation.

When using only `loadbalancer.openstack.deckhouse.io/load-balancer-address`, the floating IP must meet the following requirements:

- not be associated with any port
- reside in the floating network configured for OpenStack CCM

If the specified floating IP is unavailable, OpenStack CCM will not be able to assign an external IP address to the Service object.

Do not add the `loadbalancer.openstack.deckhouse.io/load-balancer-id` or `loadbalancer.openstack.deckhouse.io/load-balancer-address` annotations to application Ingress resources. Specify them only in the IngressNginxController configuration. Deckhouse Platform will add them to the generated Service object, which is processed by `openstack-cloud-controller-manager`.

### IngressNginxController with a pre-created load balancer

In the example below:

- Ingress controller pods are scheduled on frontend nodes.
- `loadbalancer.openstack.org/node-selector` limits the load balancer target pool to those frontend nodes.
- `loadbalancer.openstack.deckhouse.io/load-balancer-id` references a pre-created Octavia load balancer whose VIP port already has a floating IP associated with it.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancerWithProxyProtocol
  loadBalancerWithProxyProtocol:
    annotations:
      loadbalancer.openstack.deckhouse.io/load-balancer-id: "df7c6f73-8c68-4a11-a3e2-6268a655ce9b"
      loadbalancer.openstack.org/node-selector: "node-role.deckhouse.io/frontend="
      loadbalancer.openstack.org/proxy-protocol: "true"
      loadbalancer.openstack.org/timeout-member-connect: "2000"
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    operator: Equal
    value: frontend
```

### IngressNginxController with a pre-allocated floating IP

In the example below, OpenStack CCM creates a load balancer and associates the specified unassigned floating IP with it:

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancerWithProxyProtocol
  loadBalancerWithProxyProtocol:
    annotations:
      loadbalancer.openstack.deckhouse.io/load-balancer-address: "203.0.113.10"
      loadbalancer.openstack.org/node-selector: "node-role.deckhouse.io/frontend="
      loadbalancer.openstack.org/proxy-protocol: "true"
      loadbalancer.openstack.org/timeout-member-connect: "2000"
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    operator: Equal
    value: frontend
```
