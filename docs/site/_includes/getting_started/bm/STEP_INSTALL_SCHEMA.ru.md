{% assign installType = site.data.getting_started.dkp_data.installTypes[page.platform_code] %}
{% if installType and installType.description and installType.description[page.lang] %}
  {{ installType.description[page.lang] }}
{% endif %}

{% include getting_started/global/partials/STEP_INSTALL_SCHEMA_RU.liquid presentation="/presentations/getting_started_bm_ru.pdf" %}

<!-- Source: https://docs.google.com/presentation/d/12Ep9k0jb1niU1NSviYRYm2-UUZFLtLBPjf-HO0NIn_k/ -->
