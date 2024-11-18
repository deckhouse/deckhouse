---
title: "Custom Resources"
permalink: en/virtualization-platform/reference/cr.html
---

{%- assign CRDs = site.data.schemas.virtualization-platform.crds | sort  %}

  {%- for crd in CRDs %}
<div markdown="0">
    {{ crd[1] | format_crd: "" }}
</div>
  {%- endfor %}

