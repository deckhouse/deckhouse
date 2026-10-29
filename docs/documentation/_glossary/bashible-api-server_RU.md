---
title: bashible-api-server
description: Kubernetes Extension API Server на master-узлах, который генерирует скрипты bashible из шаблонов, хранящихся в кастомных ресурсах.
lang: ru
---

Хранить все варианты скриптов для поддерживаемых ОС в etcd невозможно из-за ограничения на размер ключа и нагрузки, поэтому kube-apiserver перенаправляет запросы ресурсов с [бандлами bashible](glossary.html#bashible-bundle) в bashible-api-server и возвращает сформированный результат. Он отдаёт вторую фазу [bootstrap-скрипта](glossary.html#bootstrap-script), скрипт `bashible.sh` и отрендеренный бандл [группы узлов](glossary.html#node-group).

Подробнее в разделе [«Bashible»](architecture/cluster-and-infrastructure/node-management/bashible.html).
