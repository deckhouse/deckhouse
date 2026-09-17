---
title: "Cloud provider — DVP: настройки"
force_searchable: true
---

Модуль автоматически включается для всех облачных кластеров, развернутых в DVP.

Учётные данные для доступа к API родительского кластера в настройках модуля не хранятся. Их платформа читает из секрета `d8-credentials`, формат которого описан в разделе [«Секрет с учётными данными»](environment.html#секрет-с-учётными-данными).

{% include module-alerts.liquid %}

{% include module-enable.liquid %}

{% include module-configure.liquid %}

{% include module-requirements.liquid %}

{% include module-conversion.liquid %}

{% include module-settings.liquid %}
