---
title: "Кастомные ресурсы"
permalink: ru/virtualization-platform/reference/cr.html
lang: ru
---

{%- assign CRDs = site.data.schemas.virtualization-platform.crds | sort  %}

  {%- for crd in CRDs %}
<div markdown="0">
    {{ crd[1] | format_crd: "" }}
</div>
  {%- endfor %}

