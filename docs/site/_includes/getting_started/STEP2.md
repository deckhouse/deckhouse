## Step 2. Installation

To proceed with the installation, you will need a Docker image of the Deckhouse installer. We will use the ready-made official image. The instructions on how you can build your own image from the sources, will be available in the [project's repository](https://github.com/deckhouse/deckhouse).

The commands below pull the Docker image of the Deckhouse installer and pass the public SSH key/config file to it (we have created them on the previous step). Note that it uses the default paths to files. The interactive terminal of this image's system will be launched then.

-  For CE installations:

   ```shell
docker run -it -v "$PWD/config.yml:/config.yml" -v "$HOME/.ssh/:/tmp/.ssh/"
{%- if include.mode == "existing" %} -v "$PWD/kubeconfig:/kubeconfig" {% endif %}
{%- if include.mode == "cloud" %} -v "$PWD/dhctl-tmp:/tmp" {% endif %} registry.deckhouse.io/deckhouse/ce/install:beta bash
```

-  For EE installations:

   ```shell
docker login -u license-token -p <LICENSE_TOKEN> registry.deckhouse.io
docker run -it -v "$PWD/config.yml:/config.yml" -v "$HOME/.ssh/:/tmp/.ssh/"
{%- if include.mode == "existing" %} -v "$PWD/kubeconfig:/kubeconfig" {% endif %}
{%- if include.mode == "cloud" %} -v "$PWD/dhctl-tmp:/tmp" {% endif %} registry.deckhouse.io/deckhouse/ee/install:beta bash
```

{%- if include.mode == "existing" %}
Notes:
- Kubeconfig with access to Kubernetes API must be used in kubeconfig mount.
{% endif %}

Now, to initiate the process of installation, you need to execute:

```shell
{%- if include.mode == "existing" %}
dhctl bootstrap-phase install-deckhouse \
  --kubeconfig=/kubeconfig \
  --config=/config.yml
{%- elsif include.mode == "baremetal" %}
dhctl bootstrap \
  --ssh-user=<username> \
  --ssh-host=<master_ip> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml
{%- elsif include.mode == "cloud" %}
dhctl bootstrap \
  --ssh-user=<username> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml
{% endif %}
```

{%- if include.mode == "baremetal" or include.mode == "cloud" %}
{%- if include.mode == "baremetal" %}
`username` variable here refers to the user that generated the SSH key.
{%- else %}
`username` variable here refers to
{%- if include.provider == "openstack" %} the default user for the relevant VM image.
{%- elsif include.provider == "azure" %} `azureuser` (for the images suggested in this documentation).
{%- else %} `ubuntu` (for the images suggested in this documentation).
{%- endif %}
{%- endif %}

Notes:
{%- if include.mode == "cloud" %}
- The `-v "$PWD/dhctl-tmp:/tmp"` parameter enables saving the state of the Terraform installer to a temporary directory on the startup host. It allows the installation to continue correctly in case of a failure of the installer's container.
{%- endif %}
- If any problems {% if include.mode="cloud" %}on the cloud provider side {% endif %}occur, you can stop the process of installation using the following command (the configuration file should be the same you’ve used to initiate the installation):

  ```shell
dhctl bootstrap-phase abort --config=/config.yml
```
{% endif %}

After the installation is complete, you will be returned to the command line. Congratulations: your cluster is ready! Now you can manage modules, deploy applications, etc.
