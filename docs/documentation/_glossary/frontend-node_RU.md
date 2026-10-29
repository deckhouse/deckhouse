---
title: Frontend-узел
description: Узел, который балансирует входящий трафик. На frontend-узлах работают Ingress-контроллеры.
lang: ru
---

В кластере нужно более одного frontend-узла, и узлы должны выдерживать трафик при отказе как минимум одного из них. При двух frontend-узлах каждый должен справляться со всей нагрузкой, при трёх — выдерживать рост нагрузки как минимум в полтора раза. Frontend-узлы обычно образуют [группу узлов](glossary.html#node-group) `front`, на них запускаются [Ingress-контроллеры](glossary.html#ingress-controller).

Подробнее в разделе [«Рекомендации по настройке узлов кластера»](admin/configuration/high-reliability-and-availability/recommendations.html).
