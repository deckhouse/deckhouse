---
title: Bashible
permalink: ru/architecture/cluster-and-infrastructure/node-management/bashible.html
lang: ru
search: архитектура bashible, bashible-api-server
description: Архитектура bashible в Deckhouse Kubernetes Platform — выполнение bash-скриптов для настройки узлов, работа bashible-api-server.
---

## Bashible-скрипты и служба bashible

Функции управления узлами реализуются с помощью специально подготовленных bash-скриптов, называемых **bashible**. Так же называется служба, которая работает на узлах кластера и используется для запуска данных скриптов. Набор скриптов называется бандлом (bundle).

Используются 4 бандла:

* [скрипты установки bashible](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/bootstrap);
* [скрипты бутстрапа первого узла](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/common-steps/cluster-bootstrap);
* скрипты настройки узла для определенного облачного провайдера (например, [AWS](https://github.com/deckhouse/deckhouse/tree/main/modules/030-cloud-provider-aws/candi/bashible));
* [основные (common) скрипты](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/common-steps/all).

Скрипты представляют собой *gotemplate*-шаблоны, что позволяет гибко настраивать узел в зависимости от группы. Сами скрипты должны быть написаны так, чтобы они могли корректно выполняться при повторном запуске в случае ошибки и при повторном прогоне. Отдельный скрипт называется степом (шагом).

Основные этапы настройки узла:

* Настройка NodeUser для обеспечения доступа к узлу.
* Установка CA-сертификатов.
* Создание и добавление в `PATH` каталога `/opt/deckhouse/bin`, в котором хранятся бинарные файлы.
* Скачивание необходимых пакетов из `registrypackages`.
* Установка и настройка CRI containerd.
* Скачивание и настройка **kubernetes-api-proxy**. Компонент отвечает за доступ к API Kubernetes, представляет собой NGINX с набором upstream-серверов к master-узлам. Это обеспечивает HA-доступ к API на случай, если один master-узел недоступен, а также балансировку нагрузки к API.
* Установка, настройка и запуск [kubelet](../../kubernetes-and-scheduling/kubelet.html).
* Запуск службы bashible, которая выполняет `bashible.sh` каждую минуту.
* Перезагрузка узла при необходимости.

## Bashible-api-server

Учитывая большое количество модификаций bashible-скриптов для разных поддерживаемых ОС, хранить все варианты в базе etcd невозможно из-за ограничения на размер ключа, а также избыточной нагрузки на etcd. По этой причине был разработан компонент **bashible-api-server**, который генерирует bashible-скрипты из шаблонов, хранящихся в кастомных ресурсах.

Bashible-api-server представляет собой [Kubernetes Extension API Server](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-extension-api-server/), который развертывается на master-узлах.

При обращении к kube-apiserver за ресурсами, содержащими бандлы bashible, kube-apiserver перенаправляет запрос в bashible-api-server и возвращает сформированный результат. Взаимодействия bashible и bashible-api-server показаны на схемах архитектуры модуля `node-manager` (например, на [схеме для CloudEphemeral-узлов](cloud-ephemeral-nodes.html)).

Bashible-api-server возвращает следующие ресурсы:

* **bootstrap-скрипт** второй фазы, который загружается из первой фазы;
* **bashibles** — скрипт `bashible.sh`;
* **nodegroupbundles** — в нем рендерится бандл, включающий набор скриптов для бутстрапа и настройки узла.

Все эти ресурсы можно получить как через API, так и с помощью команды `kubectl`, указав имя группы узлов:

* `kubectl get bootstrap.bashible.deckhouse.io master -o yaml`;
* `kubectl get bashibles.bashible.deckhouse.io master -o yaml`;
* `kubectl get nodegroupbundles.bashible.deckhouse.io master -o yaml`.

Также bashible-api-server вычисляет контрольную сумму всех скриптов группы узлов. Это необходимо для реализации механизма обновления и корректного обновления статуса группы. Контрольные суммы записываются в секрет `d8-cloud-instance-manager/configuration-checksums`. Изменение контрольной суммы инициирует перезапуск bashible-скриптов на узлах при изменении конфигурации. Кроме того, контрольная сумма службы bashible сбрасывается каждые 4 часа для принудительного перезапуска bashible.
