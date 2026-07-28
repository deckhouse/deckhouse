---
title: "Cloud provider — VMware Cloud Director: настройки"
force_searchable: true
---

Модуль автоматически включается для всех облачных кластеров, развернутых в VMware Cloud Director.

## Список необходимых сервисов VMware Cloud Director

Для работы Deckhouse Kubernetes Platform должен быть доступен следующий сервис VMware Cloud Director:

| Сервис                    | Версия API    |
| :------------------------ | :--------:    |
| VMware Cloud Director API | `37.2` и выше |

{% alert level="info" %}
Для версий VMware Cloud Director API ниже `37.2` используется режим совместимости с устаревшими версиями API.
{% endalert %}

{% include module-alerts.liquid %}

{% include module-enable.liquid %}

{% include module-configure.liquid %}

{% include module-requirements.liquid %}

{% include module-conversion.liquid %}

{% include module-settings.liquid %}
