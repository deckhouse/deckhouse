---
title: Bashible
permalink: ru/architecture/cluster-and-infrastructure/bashible/
lang: ru
search: bashible
---

## Bashible-скрипты/служба bashible

Функции управления узлами реализуются при помощи запуска специальным образом написанных bash-скриптов, называемых **bashible**. Так же называется служба, которая работает на узлах кластера и используется для запуска данных скриптов. Набор скриптов называется бандлом.

Используются 4 бандла:

* [скрипты, устанавливающие сам bashible](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/bootstrap)
* [скрипты, которые нужны для бутстрапа первого узла](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/common-steps/cluster-bootstrap)
* [скрипты, необходимые для настройки узла для определенного облачного провайдера (например, AWS)](https://github.com/deckhouse/deckhouse/tree/main/candi/cloud-providers/aws/bashible)
* [основные (common) скрипты](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/common-steps/all).

Скрипты представляют собой *gotemplate*-шаблоны, что позволяет гибко настраивать узел в зависимости от нодгруппы. Сами скрипты должны быть написаны идемпотентно (так, чтобы без проблем могли быть перезапущены в случае ошибки и при повторном прогоне). Отдельный скрипт также называется степом (шагом).  

Основные этапы:

* Конфигурируется NodeUser's для того, чтобы иметь сразу же возможность попасть на узел.
* Устанавливаются CA-сертификаты.
* Создаются и добавляется в PATH каталог `/opt/deckhouse/bin`, в котором хранятся бинарники.
* Скачиваются необходимые пакеты из **registrypackages**.
* Устанавливается и конфигурируется CRI **containerd**.
* Скачивается и настраивается **kubernetes-api-proxy**. Компонент отвечает за доступ к API Kubernetes, представляет собой NGINX с набором апстримов до мастер-узлов. Это обеспечивает HA-доступ к API на случай, если один мастер недоступен, а также балансировку нагрузки к API.
* Устанавливается, конфигурируется и запускается [kubelet](../../kubernetes-and-scheduling/kubelet/).
* Запускается служба **bashible**, которая выполняет `bashible.sh` каждую минуту.
* Перезагружается узел при необходимости.

## bashible-apiserver

Количество модификаций bashible-скриптов для разных поддерживаемых ОС очень велико, хранить их все в базе **etcd** не представляется возможным из-за ограничения на размер ключа **etcd**, также это создает нагрузку на **etcd**. По этой причине был разработан **bashible-api-server**, который генерирует bashible-скрипты из шаблонов, которые хранятся в Custom Resources.

**bashible-api-server** представляет собой [Kubernetes Extension APIServer](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-extension-api-server/), который деплоится на Master-узлы.
При обращении к **kube-apiserver** за ресурсами, содержащими бандлы **bashible**, **kube-apiserver** обращается к **bashible-api-server** и возвращает результат от него. Взаимодействия **bashible** и **bashible-api-server** показаны на схемах архитектуры модуля **node-manager**, например, на [схеме для Cloud Ephemeral узлов](../cloud-ephemeral-nodes/).

**bashible-api-server** возвращает следующие ресурсы:

* **bootstrap-скрипт** второй фазы, который загружается из первой фазы,
* **bashibles** - сам скрипт `bashible.sh`,
* **nodegroupbundles** - в нем рендерится сам бандл, то есть набор скриптов для бутстрапа и конфигурирования узла.

Все эти ресурсы можно получить как через API, так и с помощью kubectl с указанием имени нодгруппы:

* `kubectl get bootstrap.bashible.deckhouse.io master -o yaml`,
* `kubectl get bashibles.bashible.deckhouse.io master -o yaml`,
* `kubectl get nodegroupbundles.bashible.deckhouse.io master -o yaml`.

Также **bashible-api-server** вычисляет контрольную сумму всех скриптов нодгруппы. Это необходимо для обеспечения механизма обновления и для обеспечения обновления статусов нодгруппы. Контрольные суммы записываются в секрет `d8-cloud-instance-manager/configuration-checksums`. Контрольная сумма служит для инициализации перезапуска **bashible**-скриптов на узлах в случае изменения конфигурации. Также контрольная сумма службы **bashible** сбрасывается каждые 4 часа для принудительного перезапуска **bashible**.
