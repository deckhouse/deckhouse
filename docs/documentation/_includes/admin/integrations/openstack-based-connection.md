## List of required OpenStack services

The following {{ site.data.admin.cloud-types.types[page.cloud_type].name }} services must be available for Deckhouse Platform to operate:

| Service                      |                         API Version                      |
| :------------------------- | :--------------------------------------------------------: |
| Identity (Keystone)        |    [v3](https://docs.openstack.org/api-ref/identity/v3/)   |
| Compute (Nova)             |     [v2.1](https://docs.openstack.org/api-ref/compute/)    |
| Network (Neutron)          |     [v2.0](https://docs.openstack.org/api-ref/network/)    |
| Block Storage (Cinder)     | [v3](https://docs.openstack.org/api-ref/block-storage/v3/) |
| Load Balancing (Octavia) * |   [v2](https://docs.openstack.org/api-ref/load-balancer/)  |

\* If you need to order a Load Balancer.

{% if page.cloud_type == 'vk-private' or page.cloud_type == 'vk' %}
API endpoints and ports are listed in the [official documentation](https://cloud.vk.com/docs/tools-for-using-services/api/rest-api/endpoints).
{% endif %}

## API access

To manage resources in {{ site.data.admin.cloud-types.types[page.cloud_type].name }} using Deckhouse Platform,
you must connect to the {{ site.data.admin.cloud-types.types[page.cloud_type].name }} API.

The user credentials required to access the {{ site.data.admin.cloud-types.types[page.cloud_type].name }} API
are provided in the OpenStack RC file (openrc file).

Instructions for downloading the openrc file via the standard web interface of {{ site.data.admin.cloud-types.types[page.cloud_type].name }} and using it are available in the {{ site.data.admin.cloud-types.types[page.cloud_type].name }} [documentation](https://docs.openstack.org/ocata/admin-guide/common/cli-set-environment-variables-using-openstack-rc.html#download-and-source-the-openstack-rc-file).

If you’re using a cloud provider’s implementation of the {{ site.data.admin.cloud-types.types[page.cloud_type].name }} API,
the process for obtaining the openrc file may differ.

## Verifying security group support by the cloud provider

Run the `openstack security group list` command.
If you don’t receive any errors, it means that [security groups](https://docs.openstack.org/nova/pike/admin/security-groups.html) are supported.

## Uploading an image to {{ site.data.admin.cloud-types.types[page.cloud_type].name }}

1. Download the latest stable Ubuntu 18.04 image:

   ```shell
   curl -L https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img --output ~/ubuntu-18-04-cloud-amd64
   ```

1. Prepare an openrc file that contains credentials for accessing the {{ site.data.admin.cloud-types.types[page.cloud_type].name }} API.

   > The method of obtaining the openrc file may vary depending on the {{ site.data.admin.cloud-types.types[page.cloud_type].name }} provider.
   > If your provider offers the standard interface for {{ site.data.admin.cloud-types.types[page.cloud_type].name }},
   > follow [this guide](https://docs.openstack.org/ocata/admin-guide/common/cli-set-environment-variables-using-openstack-rc.html#download-and-source-the-openstack-rc-file) to download the file.

1. Choose how to run the OpenStack client:

   - Install the client locally by following the [OpenStack documentation](https://docs.openstack.org/newton/user-guide/common/cli-install-openstack-command-line-clients.html).

   - Run a client container with the openrc file and the downloaded Ubuntu image mounted:

   ```shell
   docker run -ti --rm -v ~/ubuntu-18-04-cloud-amd64:/ubuntu-18-04-cloud-amd64 -v ~/.openrc:/openrc jmcvea/openstack-client
   ```

1. Initialize environment variables from the openrc file:

   ```shell
   source /openrc
   ```

1. List the available volume types:

   ```shell
   openstack volume type list
   ```

   Example output:

   ```console
   +--------------------------------------+---------------+-----------+
   | ID                                   | Name          | Is Public |
   +--------------------------------------+---------------+-----------+
   | 8d39c9db-0293-48c0-8d44-015a2f6788ff | ko1-high-iops | True      |
   | bf800b7c-9ae0-4cda-b9c5-fae283b3e9fd | dp1-high-iops | True      |
   | 74101409-a462-4f03-872a-7de727a178b8 | ko1-ssd       | True      |
   | eadd8860-f5a4-45e1-ae27-8c58094257e0 | dp1-ssd       | True      |
   | 48372c05-c842-4f6e-89ca-09af3868b2c4 | ssd           | True      |
   | a75c3502-4de6-4876-a457-a6c4594c067a | ms1           | True      |
   | ebf5922e-42af-4f97-8f23-716340290de2 | dp1           | True      |
   | a6e853c1-78ad-4c18-93f9-2bba317a1d13 | ceph          | True      |
   +--------------------------------------+---------------+-----------+
   ```

1. Create an image and assign the volume type as a property (this is necessary if {{ site.data.admin.cloud-types.types[page.cloud_type].name }} does not support local disks or they are unsuitable for workloads):

   ```shell
   openstack image create --private --disk-format qcow2 --container-format bare \
     --file /ubuntu-18-04-cloud-amd64 --property cinder_img_volume_type=dp1-high-iops ubuntu-18-04-cloud-amd64
   ```

1. Ensure the image has been created:

   ```shell
   openstack image show ubuntu-18-04-cloud-amd64
   ```

   Example output:

   ```console
   +------------------+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
   | Field            | Value                                                                                                                                                                                                                                                                                     |
   +------------------+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
   | checksum         | 3443a1fd810f4af9593d56e0e144d07d                                                                                                                                                                                                                                                          |
   | container_format | bare                                                                                                                                                                                                                                                                                      |
   | created_at       | 2020-01-10T07:23:48Z                                                                                                                                                                                                                                                                      |
   | disk_format      | qcow2                                                                                                                                                                                                                                                                                     |
   | file             | /v2/images/01998f40-57cc-4ce3-9642-c8654a6d14fc/file                                                                                                                                                                                                                                      |
   | id               | 01998f40-57cc-4ce3-9642-c8654a6d14fc                                                                                                                                                                                                                                                      |
   | min_disk         | 0                                                                                                                                                                                                                                                                                         |
   | min_ram          | 0                                                                                                                                                                                                                                                                                         |
   | name             | ubuntu-18-04-cloud-amd64                                                                                                                                                                                                                                                                  |
   | owner            | bbf506e3ece54e21b2acf1bf9db4f62c                                                                                                                                                                                                                                                          |
   | properties       | cinder_img_volume_type='dp1-high-iops', direct_url='rbd://b0e441fc-c317-4acf-a606-cf74683978d2/images/01998f40-57cc-4ce3-9642-c8654a6d14fc/snap', locations='[{u'url': u'rbd://b0e441fc-c317-4acf-a606-cf74683978d2/images/01998f40-57cc-4ce3-9642-c8654a6d14fc/snap', u'metadata': {}}]' |
   | protected        | False                                                                                                                                                                                                                                                                                     |
   | schema           | /v2/schemas/image                                                                                                                                                                                                                                                                         |
   | size             | 343277568                                                                                                                                                                                                                                                                                 |
   | status           | active                                                                                                                                                                                                                                                                                    |
   | tags             |                                                                                                                                                                                                                                                                                           |
   | updated_at       | 2020-05-01T17:18:34Z                                                                                                                                                                                                                                                                      |
   | virtual_size     | None                                                                                                                                                                                                                                                                                      |
   | visibility       | private                                                                                                                                                                                                                                                                                   |
   +------------------+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
   ```

## Replacing the OS image

The OS image of the nodes is set by the [`imageName`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-imagename) parameter of the OpenStackInstanceClass resource for `CloudEphemeral` nodes and in the `instanceClass` section of the OpenStackClusterConfiguration resource for `CloudPermanent` nodes and master nodes. To move the cluster nodes to a new image, follow the steps from ["How do I replace the OS image in a cloud cluster?"](../../../../faq.html#how-do-i-replace-the-os-image-in-a-cloud-cluster).
