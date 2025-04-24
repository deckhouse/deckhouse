---
title: Конфигурация и схема размещения
permalink: ru/admin/integrations/public/yandex/yandex-layout.html
lang: ru
---

Для интеграции Deckhouse с Yandex Cloud необходимо описать облачную инфраструктуру в ресурсе YandexClusterConfiguration. Этот ресурс используется модулем `cloud-provider-yandex` и задаёт все параметры размещения кластера: от сетевой схемы и конфигурации узлов до назначения подсетей и зон.

Deckhouse использует этот ресурс при размещении управляющих и рабочих узлов в Yandex Cloud. Ниже приведены обязательные параметры, примеры конфигураций и возможные схемы сетевого размещения.

## Структура YandexClusterConfiguration

