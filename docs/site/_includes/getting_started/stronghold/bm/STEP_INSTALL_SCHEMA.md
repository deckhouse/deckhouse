{% assign installType = site.data.getting_started.dkp_data.installTypes[page.platform_code] %}
{% if installType and installType.description and installType.description[page.lang] %}
  {{ installType.description[page.lang] }}
{% endif %}

{% include getting_started/global/partials/STEP_INSTALL_SCHEMA.liquid presentation="/presentations/getting_started_bm_en.pdf" %}

<!-- Source: https://docs.google.com/presentation/d/1xjZg8-bjEaxO5WQhycL3VSaIw8seffAEd5M2SIQZWwQ/ -->
