{% assign revision=include.revision %}

{% if revision == 'ee' %}
{% include getting_started/global/EE_ACCESS.md %}
{% endif %}

The recommended settings for a Deckhouse Platform {% if revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %} installation are generated below:
- `config.yml` — a file with the configuration needed to bootstrap the cluster. Contains the installer parameters, {% if page. platform_type== 'cloud' %}cloud provider related parameters (such as credentials, instance type, etc){% else %} access parameters{% endif %}, and the initial cluster parameters.
- `resources.yml` — description of the resources that must be installed after the installation (nodes description, ingress controller description, etc).

**Please pay attention to**:
- <span class="mustChange">highlighted</span> parameters you *must* define.
- <span class="mightChange">parameters</span> you might want to change.

> The other available cloud provider related options are described in the [documentation](https://deckhouse.io/en/documentation/v1/kubernetes.html).
>
> To learn more about the Deckhouse Platform release channels, please see the [relevant documentation](/en/documentation/v1/deckhouse-release-channels.html).

{% snippetcut name="config.yml" selector="config-yml" %}
{% include_file "_includes/getting_started/{{ page.platform_code }}/partials/config.yml.{{ include.layout }}.{{ revision }}.inc" syntax="yaml" %}
{% endsnippetcut %}

<!-- TODO -->
{% snippetcut name="resources.yml" selector="resources-yml" %}
{% include_file "_includes/getting_started/{{ page.platform_code }}/partials/resources.yml.minimal.inc" syntax="yaml" %}
{% endsnippetcut %}

To install the **Deckhouse Platform**, we will use a prebuilt Docker image. It is necessary to transfer configuration files to the container, as well as ssh-keys for access to the master nodes:

{%- if revision == 'ee' %}
{% snippetcut selector="docker-login" %}
```shell
docker login -u license-token -p <LICENSE_TOKEN> registry.deckhouse.io
docker run -it -v "$PWD/config.yml:/config.yml" -v "$PWD/resources.yml:/resources.yml" -v "$HOME/.ssh/:/tmp/.ssh/" \
{% if page.platform_type == "existing" %} -v "$PWD/kubeconfig:/kubeconfig" \
{% endif %}{% if page.platform_type == "cloud" %} -v "$PWD/dhctl-tmp:/tmp/dhctl" {% endif %} registry.deckhouse.io/deckhouse/ee/install:alpha bash
```
{% endsnippetcut %}
{% else %}
{% snippetcut %}
```shell
docker run -it -v "$PWD/config.yml:/config.yml" -v "$PWD/resources.yml:/resources.yml" -v "$HOME/.ssh/:/tmp/.ssh/" \
{% if page.platform_type == "existing" %} -v "$PWD/kubeconfig:/kubeconfig" \
{% endif %}{% if page.platform_type == "cloud" %} -v "$PWD/dhctl-tmp:/tmp/dhctl" {% endif %} registry.deckhouse.io/deckhouse/ce/install:stable bash
```
{% endsnippetcut %}
{% endif %}

{%- if page.platform_type == "existing" %}
Notes:
- Kubeconfig with access to Kubernetes API must be used in kubeconfig mount.
{% endif %}

Now, to initiate the process of installation, you need to execute inside the container:

{% snippetcut %}
```shell
{%- if page.platform_type == "existing" %}
dhctl bootstrap-phase install-deckhouse \
  --kubeconfig=/kubeconfig \
  --config=/config.yml \
  --resources=/resources.yml
{%- elsif page.platform_type == "baremetal" %}
dhctl bootstrap \
  --ssh-user=<username> \
  --ssh-host=<master_ip> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml \
  --resources=/resources.yml
{%- elsif page.platform_type == "cloud" %}
dhctl bootstrap \
  --ssh-user=<username> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml \
  --resources=/resources.yml
{%- endif %}
```
{% endsnippetcut %}

{%- if page.platform_type == "baremetal" or page.platform_type == "cloud" %}
{%- if page.platform_type == "baremetal" %}
`username` variable here refers to the user that generated the SSH key.
{%- else %}
`username` variable here refers to
{%- if page.platform_code == "openstack" %} the default user for the relevant VM image.
{%- elsif page.platform_code == "azure" %} `azureuser` (for the images suggested in this documentation).
{%- elsif page.platform_code == "gcp" %} `user` (for the images suggested in this documentation).
{%- else %} `ubuntu` (for the images suggested in this documentation).
{%- endif %}
{%- endif %}

Notes:
<ul>
{%- if page.platform_type == "cloud" %}
<li>
<div markdown="1">
The `-v "$PWD/dhctl-tmp:/tmp/dhctl"` parameter enables saving the state of the Terraform installer to a temporary directory on the startup host. It allows the installation to continue correctly in case of a failure of the installer's container.
</div>
</li>
{%- endif %}
<li><p>If any problems {% if page.platform_type="cloud" %}on the cloud provider side {% endif %}occur, you can stop the process of installation using the following command (the configuration file should be the same you’ve used to initiate the installation):</p>
<div markdown="0">
{% snippetcut %}
```shell
dhctl bootstrap-phase abort \
  --ssh-user=<username> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml
```
{% endsnippetcut %}
</div></li>
{% endif %}
</ul>

After the installation is complete, you will be returned to the command line.

Almost everything is ready for a fully-fledged Deckhouse Platform to work!

In order to use any Deckhouse Platform module, you need to add nodes to the cluster.
