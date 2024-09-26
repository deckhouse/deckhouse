{% if site.data.getting_started.data.installTypes[page.platform_code].description %}
{{ site.data.getting_started.data.installTypes[page.platform_code].description[page.lang] }}
{% endif %}

{% include getting_started/global/partials/STEP_INSTALL_SCHEMA.liquid presentation="/presentations/getting_started_bm_en.pdf" %}
<!-- Source: https://docs.google.com/presentation/d/1xjZg8-bjEaxO5WQhycL3VSaIw8seffAEd5M2SIQZWwQ/ -->
