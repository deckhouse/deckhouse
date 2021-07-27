{% assign revision=include.revision %}

{% if revision == 'ee' %}
{% include getting_started/global/EE_ACCESS.md %}
{% endif %}

The recommended settings for a Deckhouse Platform {% if revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %} installation are generated below:
- `config.yml` — a file with the configuration needed to bootstrap the cluster. Contains the installer parameters, {% if page. platform_type== 'cloud' %}cloud provider related parameters (such as credentials, instance type etc...){% else %} access parameters{% endif %}, and the initial cluster parameters.
- `resources.yml` — description of the resources that must be installed after the installation (nodes description, ingress controller description, etc).

**Please pay attention to**:
- <span class="mustChange">highlighted</span> parameters you *must* define.
- <span class="mightChange">parameters</span> you might want to change.

> The other available cloud provider related options are described in the [documentation](https://early.deckhouse.io/en/documentation/v1/kubernetes.html).
>
> To learn more about the Deckhouse Platform release channels, please see the [relevant documentation](/en/documentation/v1/deckhouse-release-channels.html).

{% snippetcut name="config.yml" selector="config-yml" %}
{% include_file "_includes/getting_started/{{ page.platform_code }}/partials/config.yml.{{ include.layout }}.{{ revision }}.inc" syntax="yaml" %}
{% endsnippetcut %}
