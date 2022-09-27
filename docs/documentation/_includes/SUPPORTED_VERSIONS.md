{%- assign osVersions = site.data.supported_versions.bashible | sort %}
{%- assign k8sVersions = site.data.supported_versions.k8s  %}
{%- assign langSupportKey = page.lang | append: "_support" %}

## Linux

{{ site.data.i18n.common['os_supported_phrase'][page.lang] }}:

{%- for osItem in osVersions %}
{%- assign osKey = osItem[0] %}
{%- assign osName = site.data.supported_versions.osDistributions[osKey].name | default: osKey  %}
{%- for osData in osItem[1] %}
{%- assign osVersion = osData[0] %}

{%- if osData[1][langSupportKey] and osData[1][langSupportKey] != "true" %}{% continue %}{% endif %}
- {{ osName }} {{ osVersion }}{% if site.data.supported_versions.osDistributions[osKey]['versions'][osVersion] %} ({{ site.data.supported_versions.osDistributions[osKey]['versions'][osVersion]['name'] }}){% endif %}
{%- endfor %}
{%- endfor %}

## Kubernetes

{{ site.data.i18n.common['k8s_supported_phrase'][page.lang] }}::
<table>
<thead>
    <tr>
      <th style="text-align: center">{{ site.data.i18n.common['version'][page.lang] }}</th>
      <th style="text-align: center" colspan="2">{{site.data.i18n.common['status'][page.lang] }}</th>
    </tr>
</thead>
<tbody>
{%- for k8sItem in k8sVersions %}
{%- assign k8sStatus = k8sItem[1].status | default: 'preview' %}
{%- assign iconStatus = k8sStatus| append: '.svg' | prepend: '/images/icons/' %}
<tr>
  <td style="text-align: center; font-weight:bold">{{ k8sItem[0] }}</td>
  <td style="text-align: center">
    <img src="{{ iconStatus }}" alt="" />
  </td>
  <td style="text-align: left">
    <p>{%- if k8sItem[0] == site.data.version_kubernetes.default %}<strong>{{ site.data.i18n.common['default_version'][page.lang] | capitalize }}.</strong> {% endif %}
    {{ site.data.supported_versions.k8s_statuses[k8sStatus][page.lang] }}</p>
  </td>
</tr>
{%- endfor %}
</tbody>
</table>
