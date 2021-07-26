To proceed with the Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}** {% if page.platform_type == 'baremetal' %}on{% else %}in{% endif %} {{ page.platform_name[page.lang] }} installation, you will need a Docker image of the Deckhouse installer. We will use the ready-made official image. The instructions on how you can build your own image from the sources, will be available in the [project's repository](https://github.com/deckhouse/deckhouse).

The commands below pull the Docker image of the Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %} installer and pass the public SSH key/config file to it (we have created them on the previous step). Note that it uses the default paths to files. The interactive terminal of this image's system will be launched then.

{%- if page.revision == 'ee' %}
   ```shell
docker login -u license-token -p <LICENSE_TOKEN> registry.deckhouse.io
docker run -it -v "$PWD/config.yml:/config.yml" -v "$HOME/.ssh/:/tmp/.ssh/"
{%- if page.platform_type == "existing" %} -v "$PWD/kubeconfig:/kubeconfig" {% endif %}
{%- if page.platform_type == "cloud" %} -v "$PWD/dhctl-tmp:/tmp" {% endif %} registry.deckhouse.io/deckhouse/ee/install:alpha bash
```
{% else %}
   ```shell
docker run -it -v "$PWD/config.yml:/config.yml" -v "$HOME/.ssh/:/tmp/.ssh/"
{%- if page.platform_type == "existing" %} -v "$PWD/kubeconfig:/kubeconfig" {% endif %}
{%- if page.platform_type == "cloud" %} -v "$PWD/dhctl-tmp:/tmp" {% endif %} registry.deckhouse.io/deckhouse/ce/install:alpha bash
```
{% endif %}

{%- if page.platform_type == "existing" %}
Notes:
- Kubeconfig with access to Kubernetes API must be used in kubeconfig mount.
{% endif %}

Now, to initiate the process of installation, you need to execute:

```shell
{%- if page.platform_type == "existing" %}
dhctl bootstrap-phase install-deckhouse \
  --kubeconfig=/kubeconfig \
  --config=/config.yml
{%- elsif page.platform_type == "baremetal" %}
dhctl bootstrap \
  --ssh-user=<username> \
  --ssh-host=<master_ip> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml
{%- elsif page.platform_type == "cloud" %}
dhctl bootstrap \
  --ssh-user=<username> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml
{%- endif %}
```

{%- if page.platform_type == "baremetal" or page.platform_type == "cloud" %}
{%- if page.platform_type == "baremetal" %}
`username` variable here refers to the user that generated the SSH key.
{%- else %}
`username` variable here refers to
{%- if page.platform_code == "openstack" %} the default user for the relevant VM image.
{%- elsif page.platform_code == "azure" %} `azureuser` (for the images suggested in this documentation).
{%- else %} `ubuntu` (for the images suggested in this documentation).
{%- endif %}
{%- endif %}

Notes:
{%- if page.platform_type == "cloud" %}
- The `-v "$PWD/dhctl-tmp:/tmp"` parameter enables saving the state of the Terraform installer to a temporary directory on the startup host. It allows the installation to continue correctly in case of a failure of the installer's container.
{%- endif %}
- If any problems {% if page.platform_type="cloud" %}on the cloud provider side {% endif %}occur, you can stop the process of installation using the following command (the configuration file should be the same you’ve used to initiate the installation):

  ```shell
dhctl bootstrap-phase abort --config=/config.yml
```
{% endif %}

After the installation is complete, you will be returned to the command line.

Almost everything is ready for a high-grade Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %} work!

In order to use any Deckhouse Platform module, you need to add nodes to the cluster.
