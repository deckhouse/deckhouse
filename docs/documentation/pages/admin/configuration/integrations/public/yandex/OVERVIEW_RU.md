---
title: Введение
permalink: ru/admin/integrations/public/yandex/yandex-overview.html
lang: ru
---

Интеграция Deckhouse Kubernetes Platform с провайдером Yandex Cloud осуществляется с помощью модуля `cloud-provider-yandex`. Этот модуль обеспечивает взаимодействие кластера с облачными ресурсами Yandex Cloud и позволяет использовать инфраструктуру провайдера при заказе и управлении узлами через модуль `node-manager`.

Интеграция доступна во всех редакциях платформы Deckhouse: CE, BE, SE, SE+, EE.

Модуль `cloud-provider-yandex`:

- Управляет ресурсами Yandex Cloud через компонент `cloud-controller-manager`.
- Создаёт сетевые маршруты для сети PodNetwork в облаке.
- Актуализирует метаданные виртуальных машин Yandex Cloud и соответствующих Kubernetes-узлов.
- Удаляет из Kubernetes те узлы, которые были удалены в облаке.
- Заказывает и подключает диски через CSI-обработчик Yandex Cloud.
- Регистрируется в `node-manager`, чтобы использовать YandexInstanceClass в NodeGroup’ах.
- Настраивает CNI (используется simple bridge).
- Поддерживает создание StorageClass’ов под все типы дисков Yandex Cloud.
- Автоматически создает LoadBalancer-ресурсы на базе Yandex Cloud при наличии Kubernetes-сервисов с соответствующим типом.

Полноценная интеграция включает следующие шаги:

1. Подключение и авторизация — создание сервисного аккаунта, назначение IAM-ролей, генерация JSON-ключа и подготовка облачного окружения для работы Deckhouse.

1. Конфигурация и схема размещения — описание параметров YandexClusterConfiguration, выбор схемы сетевого размещения, настройка подсетей и групп безопасности.

1. Интеграция со службами Yandex Cloud — настройка External Secrets Operator и подключение к Yandex Lockbox, а также использование Yandex Managed Service for Prometheus.

1. Хранилище и балансировка нагрузки — конфигурация хранилищ, выбор нужных StorageClass и подключение балансировщиков типа `LoadBalancer` и `INTERNAL`.

1. Особенности эксплуатации — работа с `dhcpOptions`, CloudStatic-узлами, bastion-хостами и рекомендации по корректному применению изменений.

Следующие разделы документации подробно описывают каждый этап.