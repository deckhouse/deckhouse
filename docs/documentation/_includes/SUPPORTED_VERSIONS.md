{%- assign osVersions = site.data.version_map.bashible  %}
{%- assign k8sVersions = site.data.version_map.k8s  %}

{%- if page.lang == 'ru' %}
## Linux
В настоящий момент в качестве ОС для узлов поддерживаются следующие дистрибутивы Linux:
{%- else %}
## Linux
The following Linux Distributions are currently supported for nodes:
{%- endif %}

{%- for osItem in osVersions %}
{%- assign osKey = osItem[0] %}
{%- assign osName = site.data.supported_versions.osDistributions[osKey].name | default: osKey  %}
{%- for osData in osItem[1] %}
{%- assign osVersion = osData[0]  %}
  - {{ osName }} {{ osVersion }}{% if site.data.supported_versions.osDistributions[osKey]['versions'][osVersion] %} ({{ site.data.supported_versions.osDistributions[osKey]['versions'][osVersion]['name'] }}){% endif %}{% if page.lang == 'ru' %}.{%- endif %}
{%- endfor %}
{%- endfor %}

{% if page.lang == 'ru' %}
## Kubernetes
В настоящий момент поддерживаются следующие версии Kubernetes:
{%- else %}
## Kubernetes
The following Kubernetes versions are currently supported:
{%- endif %}
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
  <td style="text-align: center">
    <p>{{ site.data.supported_versions.k8s_statuses[k8sStatus][page.lang] }}</p>
  </td>
</tr>
{%- endfor %}
</tbody>
</table>
