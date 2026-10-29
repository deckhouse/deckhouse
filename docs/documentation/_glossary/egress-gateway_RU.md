---
title: Egress-шлюз
description: Централизованное управление исходящим трафиком кластера и его контроль, включая шифрование, маршрутизацию и мониторинг.
lang: ru
---

Egress-шлюз предоставляется модулем [`cni-cilium`](/modules/cni-cilium/) и настраивается двумя кастомными ресурсами. [EgressGateway](/modules/cni-cilium/cr.html#egressgateway) описывает группу узлов, один из которых выбирается активным шлюзом, а остальные остаются в режиме ожидания. [EgressGatewayPolicy](/modules/cni-cilium/cr.html#egressgatewaypolicy) описывает, какой трафик подов кластера направляется через этот шлюз.

Доступны два режима: базовый с заранее настроенными IP-адресами на egress-узлах и режим Virtual IP, в котором дополнительные IP-адреса назначаются узлам динамически. Подробнее в разделе [«Настройка Egress Gateway»](admin/configuration/network/egress/gateway.html).
