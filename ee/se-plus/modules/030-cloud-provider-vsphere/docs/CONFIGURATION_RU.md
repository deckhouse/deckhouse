---
title: "Cloud provider — VMware vSphere: настройки"
force_searchable: true
---

Модуль автоматически включается для всех облачных кластеров, развёрнутых в vSphere.

{% include module-alerts.liquid %}

{% include module-enable.liquid %}

{% include module-configure.liquid %}

{% include module-requirements.liquid %}

{% include module-conversion.liquid %}

Источник настроек зависит от того, где размещён control plane кластера.
Если control plane работает на виртуальных машинах или bare metal, модуль использует собственные настройки, приведённые ниже.
Если control plane размещён в облаке, модуль использует ресурс [VsphereClusterConfiguration](cluster_configuration.html#vsphereclusterconfiguration).

Количество узлов и параметры их заказа задаются в ресурсе [NodeGroup](/modules/node-manager/cr.html#nodegroup) модуля `node-manager`.
Там же в параметре `cloudInstances.classReference` указывается инстанс-класс группы узлов.
Инстанс-классом для vSphere служит кастомный ресурс [VsphereInstanceClass](cr.html#vsphereinstanceclass), который описывает параметры самих виртуальных машин.

Требования к окружению, подключение к vCenter, сети, входящий трафик и хранилище описаны в разделе [«Инфраструктура»](environment.html#инфраструктура).
Как добавлять и удалять узлы кластера, описано в документации модуля [`node-manager`](/modules/node-manager/), пример группы узлов для vSphere приведён в разделе [«Создание группы узлов»](examples.html#создание-группы-узлов).

{% include module-settings.liquid %}
