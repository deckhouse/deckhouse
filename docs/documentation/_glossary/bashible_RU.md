---
title: bashible
description: Служба, которая работает на каждом узле кластера и выполняет bash-скрипты, также называемые bashible, настраивающие узел в соответствии с его группой узлов.
lang: ru
---

Скрипты представляют собой Go-шаблоны, объединённые в наборы (bundle). Отдельный скрипт называется шагом (step). Они написаны так, чтобы их можно было безопасно выполнять повторно. Служба повторно выполняет шаги, когда меняется контрольная сумма конфигурации [группы узлов](glossary.html#node-group). Шаги хранятся на узле в директории `/var/lib/bashible/bundle_steps/`. [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) добавляет в конфигурацию bashible группы пользовательские шаги.

bashible входит в [подсистему](glossary.html#subsystem) Cluster & Infrastructure. Он работает на уровне операционной системы как системная служба и не является частью модуля [`node-manager`](/modules/node-manager/).

Подробнее в разделе [«Bashible»](architecture/cluster-and-infrastructure/node-management/bashible.html).
