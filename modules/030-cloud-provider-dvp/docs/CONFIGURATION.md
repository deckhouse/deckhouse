---
title: "Cloud provider — DVP: configuration"
force_searchable: true
---

The module is automatically enabled for all cloud clusters deployed in DVP.

The credentials for accessing the parent cluster API are not stored in the module settings. The platform reads them from the `d8-credentials` Secret, whose format is covered in the [Credentials Secret](environment.html#credentials-secret) section.

{% include module-alerts.liquid %}

{% include module-enable.liquid %}

{% include module-configure.liquid %}

{% include module-requirements.liquid %}

{% include module-conversion.liquid %}

{% include module-settings.liquid %}
