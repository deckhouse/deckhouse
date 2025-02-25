---
title: Рекомендации к конфигурации кластера
permalink: ru/admin/high-reliability-and-availability/recomendations.html
description: Рекомендации к конфигурации кластера
lang: ru
---

{% alert %}
В кластере должно быть три master-узла с быстрыми дисками 400+ IOPS.
{% endalert %}

Всегда используйте три master-узла — такое количество обеспечит отказоустойчивость и позволит безопасно выполнять обновление master-узлов. В большем числе master-узлов нет необходимости, а два узла не обеспечат кворума.

- [Как добавить мастер-узлы в облачном кластере](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/control-plane-manager/faq.html#как-добавить-master-узлы-в-облачном-кластере-single-master-в-multi-master)
- [Работа со статическими узлами](https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/node-manager/#работа-со-статическими-узлами)

При использовании одного master-узла при выходе его из строя сломается весь кластер, т.к. именно master-узел отвечает за работу ключевых компонентов кластера, обеспечивающих работу всего кластера.

## Резервное копирование

{% alert %}
Настройте [резервное копирование etcd](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/control-plane-manager/faq.html#как-сделать-бэкап-etcd-вручную).
{% endalert %}

**Важно.** Обязательно настройте [резервное копирование данных etcd](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/control-plane-manager/faq.html#как-сделать-бэкап-etcd-вручную) — это последняя возможность восстановить кластер в случае непредвиденных событий. Храните резервные копии как можно *дальше* от кластера.

Резервные копии не помогут, если они не работают или вы не знаете, как их использовать для восстановления. Рекомендуем составить план восстановления на случай аварии (Disaster Recovery Plan), содержащий конкретные шаги и команды по развертыванию кластера из резервной копии.

Подробнее с рекомендациями к конфигурации кластера можно ознакомиться [в гайде «Подготовка к Production»](/products/kubernetes-platform/guides/production.html).
