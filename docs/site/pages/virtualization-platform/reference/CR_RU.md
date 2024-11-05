---
title: "Кастомные ресурсы"
permalink: ru/virtualization-platform/reference/cr.html
lang: ru
---

{%- for module in site.data.schemas.virtualization-platform.crds %}
  {%- assign moduleCRDs = module[1]["crds"]  %}
  {%- for crd in moduleCRDs %}
<div markdown="0">
    {{ crd[1] | format_crd: module[0] }}
</div>
  {%- endfor %}

{%- endfor %}
