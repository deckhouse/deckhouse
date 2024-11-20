---
title: "Кастомные ресурсы"
permalink: ru/virtualization-platform/reference/cr.html
anchors_disabled: true
lang: ru
---

{%- assign CRDs = site.data.schemas.virtualization-platform.crds | sort  %}

  {%- for crd in CRDs %}
<div markdown="0">
    {{ crd[1] | format_crd: "" }}
</div>
  {%- endfor %}

