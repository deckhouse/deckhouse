---
title: Host firewall
description: Режим работы Cilium, в котором сетевые политики применяются не к подам, а к самим узлам кластера.
lang: ru
---

Host firewall настраивается с помощью объектов CiliumClusterwideNetworkPolicy с полем `nodeSelector`. Такая политика применяется к host endpoint с лейблом `reserved:host` и фильтрует трафик, входящий на узел и исходящий с него, включая поды с `hostNetwork`. Доступен только в кластерах с модулем [`cni-cilium`](/modules/cni-cilium/).

Политики для узлов добавляют слой фильтрации внутри кластера и не заменяют защиту на уровне инфраструктуры: аппаратный файрвол и группы безопасности облака. Ошибка в них может нарушить доступ по SSH, работу control plane, kubelet или etcd, поэтому их сначала включают в режиме аудита. Подробнее в разделе [«Host firewall на узлах»](admin/configuration/network/policy/host_firewall.html).
