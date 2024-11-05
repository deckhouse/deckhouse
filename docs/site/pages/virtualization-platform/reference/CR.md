---
title: "Custom Resources"
permalink: en/virtualization-platform/reference/cr.html
---

{%- for module in site.data.schemas.virtualization-platform.crds %}
  {%- assign moduleCRDs = module[1]["crds"]  %}
  {%- for crd in moduleCRDs %}
<div markdown="0">
    {{ crd[1] | format_crd: module[0] }}
</div>
  {%- endfor %}

{%- endfor %}
