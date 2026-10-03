{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Подготовьте окружение {{ page.platform_name[page.lang] }}, чтобы Deckhouse Platform мог управлять ресурсами платформы виртуализации. Полная инструкция приведена [на странице подготовки окружения](/modules/cloud-provider-zvirt/environment.html) модуля `cloud-provider-zvirt`.

Для работы Deckhouse Platform требуется zVirt версии 4.0–4.4.

Выполните предварительные настройки в zVirt:

1. [Подготовьте образ операционной системы](/modules/cloud-provider-zvirt/environment.html#подготовка-образа-операционной-системы).
1. [Подготовьте шаблон виртуальной машины](/modules/cloud-provider-zvirt/environment.html#подготовка-шаблона-виртуальной-машины).
