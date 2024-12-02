{% assign installType = site.data.getting_started.dvp_data.installTypes[page.platform_code] %}
{% if installType and installType.description and installType.description[page.lang] %}
  {{ installType.description[page.lang] }}
{% endif %}

Coming soon...
