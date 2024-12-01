---
title: "Modules settings"
permalink: en/virtualization-platform/reference/mc.html
documentation_state: prod
---

{%- assign modulesData = site.data.schemas.virtualization-platform.openapi | sort  %}

{%- for module in modulesData %}
  {%- assign moduleConfigs = module[1]  %}
  {%- for moduleConfig in moduleConfigs %}
<h2>{{ module[0] }}</h2>  
<div markdown="0">
   {{ moduleConfig[1] | format_module_configuration: module[0] }}
</div>
  {%- endfor %}

{%- endfor %}
