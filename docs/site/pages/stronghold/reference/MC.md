---
title: "Modules settings"
permalink: en/stronghold/reference/mc.html
documentation_state: prod
---

{% if site.data.schemas.stronghold.openapi.size > 0 %}
  {%- assign modulesData = site.data.schemas.stronghold.openapi | sort  %}
  
  {%- for module in modulesData %}
    {%- assign moduleConfigs = module[1]  %}
    {%- for moduleConfig in moduleConfigs %}
  <h2>{{ module[0] }}</h2>
  <div markdown="0">
     {{ moduleConfig[1] | format_module_configuration: module[0] }}
  </div>
    {%- endfor %}
  
  {%- endfor %}
{%- endif %}
